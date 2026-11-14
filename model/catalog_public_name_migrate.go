package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/modeltaps/modeltaps/common/logger"
	"gorm.io/gorm"
)

// Before catalog rows were named by public names, the aggregator price-list syncs and the
// channel-save models.dev sync created draft rows under platform spellings
// ("anthropic/claude-opus-5"). The supplies of those channels then went to the public name (when
// it had a row) or to the spelling's own row, so one model could show up twice, once without
// supplies. This migration moves such drafts to their public names (see PublicName).

// platformRowSourceRank orders the sources of synced drafts when several spellings of one public
// name exist: the aggregator lists in their sync priority, then models.dev.
var platformRowSourceRank = map[string]int{
	ModelInfoSourceOpenRouter: 0,
	ModelInfoSourceVercel:     1,
	ModelInfoSourceModelsDev:  2,
}

// PlatformRowMigration counts what migratePlatformSpellingRows changed.
type PlatformRowMigration struct {
	Renamed        int // rows renamed to their public name
	Merged         int // rows deleted because the public name already had a row
	SuppliesMoved  int // supplies moved to the public name
	SuppliesMerged int // supplies deleted because the channel already supplied the public name
	PricesCopied   int // prices copied to a public name that had none
	AliasesMoved   int // channel alias decisions moved to the public name
	PausesMoved    int // supply pauses moved to the public name
	Skipped        int // rows left alone because the public name is an alias or a policy row
}

// isSyncedPlatformDraft reports whether a catalog row is a draft a sync created and nobody touched.
func isSyncedPlatformDraft(info *ModelInfo) bool {
	_, synced := platformRowSourceRank[info.Source]
	return synced && info.Status == ModelStatusDraft && !info.Locked && info.AliasOf == "" && info.Kind == ""
}

// migratePlatformSpellingRows moves the synced drafts named by a platform spelling to the public
// name of that spelling. When the public name has no row, the draft is renamed; when it has one,
// the draft is deleted. Either way its supplies follow (a channel that already supplies the
// public name keeps that supply and the draft's is deleted), and its price is copied to the
// public name when that has none. Prices under the spelling are kept: without supply routing a
// request is billed by the name it asked for. Published, retired and locked rows, aliases and
// rows an admin edited are left alone. Running it again changes nothing.
func migratePlatformSpellingRows(tx *gorm.DB) (PlatformRowMigration, error) {
	var out PlatformRowMigration
	var rows []*ModelInfo
	if err := tx.Find(&rows).Error; err != nil {
		return out, err
	}
	// The resolver must not see the drafts being moved: a draft would otherwise count as a row
	// of its own and keep its spelling (PublicName rule 2). Names without "/" cannot be found by
	// a bare-id lookup, so they stay in.
	var resolverRows []*ModelInfo
	var drafts []*ModelInfo
	for _, info := range rows {
		if isSyncedPlatformDraft(info) && strings.Contains(info.Model, "/") {
			drafts = append(drafts, info)
			continue
		}
		resolverRows = append(resolverRows, info)
		if isSyncedPlatformDraft(info) {
			drafts = append(drafts, info)
		}
	}
	var seed *CatalogSeed
	if loaded, err := LoadCatalogSeed(); err == nil {
		seed = loaded
	}
	resolver := newCanonicalResolver(resolverRows, seed)

	var moves []draftMove
	for _, info := range drafts {
		public := resolver.PublicName(info.Model)
		if public == "" || public == info.Model {
			continue
		}
		moves = append(moves, draftMove{info, publicSpelling{public: public, raw: info.Model, rank: platformRowSourceRank[info.Source]}})
	}
	if len(moves) == 0 {
		return out, nil
	}
	err := moveDraftRows(tx, moves, &out)
	return out, err
}

// draftMove is a synced draft and the public name it moves to.
type draftMove struct {
	info     *ModelInfo
	spelling publicSpelling
}

// moveDraftRows moves synced drafts to their public names. When the public name has no row, the
// draft is renamed; when it has one, the draft is deleted. Either way its supplies, channel alias
// decisions and pauses follow (see moveSupplies), and its price is copied to the public name when
// that has none. A draft whose public name is an alias or a policy row is skipped. Each draft is
// read again here and skipped unless it is still the same synced draft, so a row an admin edited
// after the moves were planned is left alone.
func moveDraftRows(tx *gorm.DB, moves []draftMove, out *PlatformRowMigration) error {
	var rows []*ModelInfo
	if err := tx.Find(&rows).Error; err != nil {
		return err
	}
	// The preferred spelling of each public name goes first, so it is the one renamed and the
	// others are merged into it.
	sort.SliceStable(moves, func(i, j int) bool {
		a, b := moves[i].spelling, moves[j].spelling
		if a.public != b.public {
			return a.public < b.public
		}
		return preferredSpelling(a, b)
	})

	byID := make(map[int]*ModelInfo, len(rows))
	byModel := make(map[string]*ModelInfo, len(rows))
	byLower := make(map[string]*ModelInfo, len(rows))
	for _, info := range rows {
		byID[info.Id] = info
		byModel[info.Model] = info
		if info.AliasOf == "" && info.Kind == "" {
			if _, ok := byLower[strings.ToLower(info.Model)]; !ok {
				byLower[strings.ToLower(info.Model)] = info
			}
		}
	}

	forget := func(name string) {
		if info := byModel[name]; info != nil && byLower[strings.ToLower(name)] == info {
			delete(byLower, strings.ToLower(name))
		}
		delete(byModel, name)
	}

	for _, m := range moves {
		current := byID[m.info.Id]
		if current == nil || current.Model != m.info.Model || !isSyncedPlatformDraft(current) {
			continue
		}
		target := byModel[m.spelling.public]
		if target == nil {
			target = byLower[strings.ToLower(m.spelling.public)]
		}
		if target == current {
			continue
		}
		if target != nil && (target.AliasOf != "" || target.Kind != "") {
			out.Skipped++
			continue
		}
		old := current.Model
		public := m.spelling.public
		if target != nil {
			public = target.Model
		}
		if err := moveSupplies(tx, old, public, out); err != nil {
			return err
		}
		if err := moveChannelAliases(tx, old, public, out); err != nil {
			return err
		}
		if err := movePauses(tx, old, public, out); err != nil {
			return err
		}
		if err := copyPrice(tx, old, public, out); err != nil {
			return err
		}
		if target != nil {
			if err := tx.Delete(&ModelInfo{}, current.Id).Error; err != nil {
				return err
			}
			forget(old)
			delete(byID, current.Id)
			out.Merged++
			continue
		}
		if err := tx.Model(&ModelInfo{}).Where("id = ?", current.Id).Update("model", public).Error; err != nil {
			return err
		}
		forget(old)
		current.Model = public
		byModel[public] = current
		byLower[strings.ToLower(public)] = current
		out.Renamed++
	}
	return nil
}

// moveChannelAliases points the channel alias decisions made about model from at model to. A
// decision the channel already made about to for the same upstream name wins.
func moveChannelAliases(tx *gorm.DB, from, to string, out *PlatformRowMigration) error {
	if !tx.Migrator().HasTable(&ChannelModelAlias{}) {
		return nil
	}
	var rows []*ChannelModelAlias
	if err := tx.Where("model = ?", from).Find(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		var taken int64
		if err := tx.Model(&ChannelModelAlias{}).
			Where("channel_id = ? AND upstream_model = ? AND model = ?", r.ChannelId, r.UpstreamModel, to).
			Count(&taken).Error; err != nil {
			return err
		}
		if taken > 0 {
			if err := tx.Delete(&ChannelModelAlias{}, r.Id).Error; err != nil {
				return err
			}
			continue
		}
		if err := tx.Model(&ChannelModelAlias{}).Where("id = ?", r.Id).Update("model", to).Error; err != nil {
			return err
		}
		out.AliasesMoved++
	}
	return nil
}

// movePauses moves the pauses of model from on a channel to model to, the name requests for the
// model are routed by from now on. A pause the channel already has for to wins.
func movePauses(tx *gorm.DB, from, to string, out *PlatformRowMigration) error {
	if from == "" || !tx.Migrator().HasTable(&SupplyPause{}) {
		return nil
	}
	var pauses []*SupplyPause
	if err := tx.Where("model = ?", from).Find(&pauses).Error; err != nil {
		return err
	}
	for _, p := range pauses {
		var taken int64
		if err := tx.Model(&SupplyPause{}).Where("channel_id = ? AND model = ?", p.ChannelId, to).Count(&taken).Error; err != nil {
			return err
		}
		if taken > 0 {
			if err := tx.Delete(&SupplyPause{}, p.Id).Error; err != nil {
				return err
			}
			continue
		}
		if err := tx.Model(&SupplyPause{}).Where("id = ?", p.Id).Update("model", to).Error; err != nil {
			return err
		}
		out.PausesMoved++
	}
	return nil
}

// moveSupplies moves the supplies of model from to model to. A channel supplies a model once, so
// a supply whose channel already supplies to is deleted instead.
func moveSupplies(tx *gorm.DB, from, to string, out *PlatformRowMigration) error {
	var supplies []*ModelSupply
	if err := tx.Where("model = ?", from).Find(&supplies).Error; err != nil {
		return err
	}
	for _, s := range supplies {
		var taken int64
		if err := tx.Model(&ModelSupply{}).Where("model = ? AND channel_id = ?", to, s.ChannelId).Count(&taken).Error; err != nil {
			return err
		}
		if taken > 0 {
			if err := tx.Delete(&ModelSupply{}, s.Id).Error; err != nil {
				return err
			}
			out.SuppliesMerged++
			continue
		}
		if err := tx.Model(&ModelSupply{}).Where("id = ?", s.Id).Update("model", to).Error; err != nil {
			return err
		}
		out.SuppliesMoved++
	}
	return nil
}

// copyPrice copies the price of from to to when to has no price. The price of from is kept.
func copyPrice(tx *gorm.DB, from, to string, out *PlatformRowMigration) error {
	var existing int64
	if err := tx.Model(&Price{}).Where("model = ?", to).Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	var price Price
	result := tx.Where("model = ?", from).Limit(1).Find(&price)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	price.Model = to
	price.Locked = false
	if err := tx.Create(&price).Error; err != nil {
		return err
	}
	out.PricesCopied++
	return nil
}

func migratePlatformSpellingDrafts() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611140001",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable(&ModelInfo{}) || !tx.Migrator().HasTable(&ModelSupply{}) || !tx.Migrator().HasTable(&Price{}) {
				return nil
			}
			out, err := migratePlatformSpellingRows(tx)
			if err != nil {
				return err
			}
			if out != (PlatformRowMigration{}) {
				logger.SysLog(fmt.Sprintf("moved synced draft models to their public names: %d renamed, %d merged, "+
					"%d supplies moved, %d duplicate supplies removed, %d prices copied, %d left alone",
					out.Renamed, out.Merged, out.SuppliesMoved, out.SuppliesMerged, out.PricesCopied, out.Skipped))
			}
			return nil
		},
	}
}

// attachRenamedPublicModels attaches each channel to the public models its list names but it
// does not supply yet. Migration 202611140001 moved the drafts to their public names but only
// carried supplies that existed; names whose supply had been skipped (their public name had no
// row when the channel was saved) stayed without one. Only channels that already have supplies
// made from their list are handled, as those are the ones the save-time attach ran for.
func attachRenamedPublicModels(tx *gorm.DB) (int, error) {
	var ids []int
	if err := tx.Model(&ModelSupply{}).
		Where("source IN ?", []string{ModelSupplySourceChannel, ModelSupplySourceMigrated}).
		Distinct().Pluck("channel_id", &ids).Error; err != nil {
		return 0, err
	}
	var infos []*ModelInfo
	if err := tx.Find(&infos).Error; err != nil {
		return 0, err
	}
	var seed *CatalogSeed
	if loaded, err := LoadCatalogSeed(); err == nil {
		seed = loaded
	}
	resolver := newCanonicalResolver(infos, seed)
	attached := 0
	for _, id := range ids {
		var channel Channel
		result := tx.Where("id = ?", id).Limit(1).Find(&channel)
		if result.Error != nil {
			return attached, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		candidates := autoSupplyCandidates(resolver, &channel, SplitChannelModels(channel.Models))
		created, err := createAutoSupplies(tx, &channel, candidates)
		if err != nil {
			return attached, err
		}
		attached += len(created)
	}
	return attached, nil
}

func migrateAttachRenamedPublicModels() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611140002",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable(&ModelInfo{}) || !tx.Migrator().HasTable(&ModelSupply{}) || !tx.Migrator().HasTable(&Channel{}) {
				return nil
			}
			attached, err := attachRenamedPublicModels(tx)
			if err != nil {
				return err
			}
			if attached > 0 {
				logger.SysLog(fmt.Sprintf("attached channels to %d public models their lists name", attached))
			}
			return nil
		},
	}
}
