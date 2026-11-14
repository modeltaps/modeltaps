package model

import (
	"fmt"
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common/logger"
	"gorm.io/gorm"
)

// Public names come from the model library (see PublicName). A sync that runs while the library
// is still empty, as on a fresh deployment before the first catalog sync, names rows by the
// fallback instead ("anthropic/claude-fable-5.2" becomes "claude-fable-5.2" rather than the
// library's "claude-fable-5-2"). Nothing moved that row later: once the library was filled, syncs
// wrote the library's name next to it and the channel ended up supplying both. EnsureModelLibrary
// fills the library before such syncs run, and RehomeSyncedDrafts moves the drafts named before
// the library knew better once it does.

var ensureModelLibraryMu sync.Mutex

// EnsureModelLibrary syncs the model library from models.dev (url, or the default source when
// empty) when the library has no rows, and does nothing otherwise. A failed download is logged
// and returned; callers carry on with the names they can find without the library.
func EnsureModelLibrary(url string) error {
	ensureModelLibraryMu.Lock()
	defer ensureModelLibraryMu.Unlock()
	var count int64
	if err := DB.Model(&ModelLibraryEntry{}).Count(&count).Error; err != nil {
		logger.SysError("model library check failed: " + err.Error())
		return err
	}
	if count > 0 {
		return nil
	}
	data, err := fetchCatalogData(url)
	if err != nil {
		logger.SysError("model library is empty and the models.dev download failed: " + err.Error())
		return err
	}
	entries, err := SyncModelLibrary(data)
	if err != nil {
		logger.SysError("model library sync failed: " + err.Error())
		return err
	}
	logger.SysLog(fmt.Sprintf("Model library was empty, synced from models.dev: %d models", len(entries)))
	if len(entries) > 0 {
		rehomeSyncedDraftsAfterLibrarySync()
	}
	return nil
}

// rehomeSyncedDraftsAfterLibrarySync runs RehomeSyncedDrafts after a library sync and logs what it
// did; a failure is logged and leaves the catalog as it was.
func rehomeSyncedDraftsAfterLibrarySync() {
	out, err := RehomeSyncedDrafts()
	if err != nil {
		logger.SysError("moving synced drafts to their public names failed: " + err.Error())
		return
	}
	if out != (PlatformRowMigration{}) {
		logger.SysLog(fmt.Sprintf("moved synced draft models to their public names: %d renamed, %d merged, "+
			"%d supplies moved, %d duplicate supplies removed, %d prices copied, %d alias decisions moved, "+
			"%d pauses moved, %d left alone",
			out.Renamed, out.Merged, out.SuppliesMoved, out.SuppliesMerged, out.PricesCopied,
			out.AliasesMoved, out.PausesMoved, out.Skipped))
	}
}

var rehomeSyncedDraftsMu sync.Mutex

// RehomeSyncedDrafts moves each synced draft (see isSyncedPlatformDraft) to the public name its
// supplies' upstream names now resolve to. A draft moves only when all its supplies agree on one
// public name other than its own; drafts without supplies, with supplies that disagree, or named
// by a name the library knows as a model are left alone. The move is the one migration
// 202611140001 makes (see moveDraftRows): rename when the public name has no row, otherwise merge
// into that row; prices under the old name are kept. Published, locked and admin-edited rows are
// never touched. Running it again changes nothing.
//
// Every name is resolved before the transaction opens: resolving loads the library index through
// the global connection, which a SQLite transaction holding the only connection would block.
func RehomeSyncedDrafts() (PlatformRowMigration, error) {
	rehomeSyncedDraftsMu.Lock()
	defer rehomeSyncedDraftsMu.Unlock()
	var out PlatformRowMigration
	var rows []*ModelInfo
	if err := DB.Find(&rows).Error; err != nil {
		return out, err
	}
	var drafts []*ModelInfo
	var names []string
	for _, info := range rows {
		if isSyncedPlatformDraft(info) {
			drafts = append(drafts, info)
			names = append(names, info.Model)
		}
	}
	if len(drafts) == 0 {
		return out, nil
	}
	upstreams := map[string][]string{}
	for start := 0; start < len(names); start += 500 {
		end := min(start+500, len(names))
		var supplies []*ModelSupply
		if err := DB.Where("model IN ?", names[start:end]).Find(&supplies).Error; err != nil {
			return out, err
		}
		for _, s := range supplies {
			upstream := strings.TrimSpace(s.UpstreamModel)
			if upstream == "" {
				upstream = s.Model
			}
			upstreams[s.Model] = append(upstreams[s.Model], upstream)
		}
	}

	var seed *CatalogSeed
	if loaded, err := LoadCatalogSeed(); err == nil {
		seed = loaded
	}
	resolver := newCanonicalResolver(rows, seed)
	var moves []draftMove
	for _, info := range drafts {
		listed := upstreams[info.Model]
		// A name the library knows is a model's own name ("claude-opus-4-5" for a dated
		// canonical id): moving it would only trade one valid name for another.
		if len(listed) == 0 || LibraryKnowsModel(info.Model) {
			continue
		}
		public := ""
		agree := true
		resolver.withoutRow(info, func() {
			for _, upstream := range listed {
				name := resolver.PublicName(upstream)
				if public == "" {
					public = name
				} else if name != public {
					agree = false
					return
				}
			}
		})
		if !agree || public == "" || public == info.Model || strings.HasSuffix(public, "*") {
			continue
		}
		moves = append(moves, draftMove{info, publicSpelling{public: public, raw: info.Model, rank: platformRowSourceRank[info.Source]}})
	}
	if len(moves) == 0 {
		return out, nil
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		return moveDraftRows(tx, moves, &out)
	})
	if err != nil {
		return PlatformRowMigration{}, err
	}
	if out.Renamed+out.Merged > 0 {
		refreshCatalogCaches()
		reloadSupplies()
	}
	if out.PausesMoved > 0 {
		if err := ReloadPauses(); err != nil {
			logger.SysError("reload supply pauses failed: " + err.Error())
		}
	}
	return out, nil
}

// withoutRow runs fn with info hidden from the resolver, so PublicName neither keeps info's own
// name (rule 2) nor finds info by a bare-id lookup. Another row whose name differs from info's
// only by case takes its place in the case-insensitive lookup meanwhile.
func (r *CanonicalResolver) withoutRow(info *ModelInfo, fn func()) {
	lower := strings.ToLower(info.Model)
	hidModel := r.byModel[info.Model] == info
	hidLower := r.byLower[lower] == info
	if hidModel {
		delete(r.byModel, info.Model)
	}
	if hidLower {
		delete(r.byLower, lower)
		for name, other := range r.byModel {
			if strings.ToLower(name) != lower {
				continue
			}
			if cur, ok := r.byLower[lower]; !ok || (cur.AliasOf != "" && other.AliasOf == "") {
				r.byLower[lower] = other
			}
		}
	}
	defer func() {
		if hidModel {
			r.byModel[info.Model] = info
		}
		if hidLower {
			r.byLower[lower] = info
		}
	}()
	fn()
}
