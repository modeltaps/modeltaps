package model

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/gorm"
)

// The model library holds every model models.dev knows, one row per model (deduplicated by
// canonical_model_id), and how each platform spells it. It is written only by the catalog sync
// and is never public: the public catalog (model_info) holds only the models an admin added.
// The platform spellings serve the channel side: recognising an upstream name and filling a
// supply's upstream_model.

// ModelLibraryEntry is one model as its lab publishes it.
type ModelLibraryEntry struct {
	Id int `json:"id"`
	// Canonical is the lab-qualified id, "anthropic/claude-fable-5-2".
	Canonical string `json:"canonical" gorm:"type:varchar(255);uniqueIndex"`
	// Model is the bare id used as the public model name, "claude-fable-5-2".
	Model string `json:"model" gorm:"type:varchar(255);index"`
	// Lab is the canonical prefix as models.dev writes it ("anthropic", "zhipuai", "nvidia").
	Lab string `json:"lab" gorm:"type:varchar(64);index"`
	// VendorID is the matching vendor row, 0 when the lab has no vendor row.
	VendorID         int      `json:"vendor_id" gorm:"index;default:0"`
	Name             string   `json:"name" gorm:"type:varchar(255)"`
	Family           string   `json:"family" gorm:"type:varchar(100)"`
	ContextLength    int      `json:"context_length"`
	MaxTokens        int      `json:"max_tokens"`
	InputModalities  string   `json:"input_modalities" gorm:"type:text"`
	OutputModalities string   `json:"output_modalities" gorm:"type:text"`
	Capabilities     string   `json:"capabilities" gorm:"type:text"`
	Endpoints        string   `json:"endpoints" gorm:"type:text"`
	OpenWeights      *bool    `json:"open_weights"`
	ReleasedAt       int64    `json:"released_at" gorm:"default:0"`
	KnowledgeCutoff  string   `json:"knowledge_cutoff" gorm:"type:varchar(16)"`
	Deprecated       bool     `json:"deprecated" gorm:"default:false"`
	InputPrice       *float64 `json:"input_price"`
	OutputPrice      *float64 `json:"output_price"`
	CacheReadPrice   *float64 `json:"cache_read_price"`
	// Source is the models.dev provider the metadata was taken from.
	Source   string `json:"source" gorm:"type:varchar(64)"`
	SyncedAt int64  `json:"synced_at"`
}

func (ModelLibraryEntry) TableName() string { return "model_library" }

// ModelLibraryHost is how one platform (a models.dev provider) names one library model.
type ModelLibraryHost struct {
	Id        int    `json:"id"`
	Canonical string `json:"canonical" gorm:"type:varchar(255);index"`
	Provider  string `json:"provider" gorm:"type:varchar(64);uniqueIndex:idx_model_library_host"`
	HostModel string `json:"host_model" gorm:"type:varchar(255);uniqueIndex:idx_model_library_host"`
}

func (ModelLibraryHost) TableName() string { return "model_library_hosts" }

// labProviders are the models.dev providers run by the lab itself, keyed by the prefix the lab
// uses in canonical ids (the provider id and the prefix are the same for all of them).
var labProviders = map[string]bool{
	"openai": true, "anthropic": true, "google": true, "alibaba": true, "deepseek": true,
	"moonshotai": true, "mistral": true, "cohere": true, "xai": true, "zhipuai": true,
	"minimax": true, "stepfun": true, "perplexity": true, "xiaomi": true,
}

// libraryCandidate is one models.dev entry pointing at a canonical model.
type libraryCandidate struct {
	provider string
	id       string
	model    catalogModel
}

// better reports whether c should supply the metadata instead of cur: the lab's own entry whose
// id is the canonical id, then any lab entry, then a priced one; ties keep the first (input is
// sorted).
func (c libraryCandidate) better(cur libraryCandidate, canonical string) bool {
	rank := func(x libraryCandidate) int {
		lab, bare := splitCanonical(canonical)
		switch {
		case x.provider == lab && strings.EqualFold(x.id, bare):
			return 3
		case labProviders[x.provider]:
			return 2
		case x.model.Cost.Input != nil:
			return 1
		}
		return 0
	}
	return rank(c) > rank(cur)
}

func splitCanonical(canonical string) (lab, bare string) {
	if i := strings.Index(canonical, "/"); i > 0 {
		return canonical[:i], canonical[i+1:]
	}
	return "", canonical
}

// libraryCanonicalOf returns the canonical id an entry belongs to: its canonical_model_id, or for
// an entry of a lab's own provider without one, "<lab>/<id>". Other entries without a canonical
// id are left out of the library.
func libraryCanonicalOf(provider, id string, m catalogModel) string {
	if c := strings.TrimSpace(m.CanonicalModelID); c != "" && strings.Contains(c, "/") {
		return strings.ToLower(c)
	}
	if labProviders[provider] {
		return provider + "/" + strings.ToLower(id)
	}
	return ""
}

// BuildModelLibrary is a pure function: it turns models.dev data into library rows and platform
// spellings. OpenRouter is included as a platform; its own sync stays the source for its prices.
func BuildModelLibrary(data map[string]catalogProvider, now int64) ([]*ModelLibraryEntry, []*ModelLibraryHost) {
	providers := make([]string, 0, len(data))
	for p := range data {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	best := map[string]libraryCandidate{}
	var hosts []*ModelLibraryHost
	seenHost := map[string]bool{}
	for _, p := range providers {
		models := data[p].Models
		names := make([]string, 0, len(models))
		for name := range models {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			m := models[name]
			id := m.Id
			if id == "" {
				id = name
			}
			canonical := libraryCanonicalOf(p, id, m)
			if canonical == "" {
				continue
			}
			cand := libraryCandidate{provider: p, id: id, model: m}
			if cur, ok := best[canonical]; !ok || cand.better(cur, canonical) {
				best[canonical] = cand
			}
			key := p + "\x00" + id
			if !seenHost[key] {
				seenHost[key] = true
				hosts = append(hosts, &ModelLibraryHost{Canonical: canonical, Provider: p, HostModel: id})
			}
		}
	}

	canonicals := make([]string, 0, len(best))
	for c := range best {
		canonicals = append(canonicals, c)
	}
	sort.Strings(canonicals)
	entries := make([]*ModelLibraryEntry, 0, len(canonicals))
	for _, canonical := range canonicals {
		cand := best[canonical]
		lab, bare := splitCanonical(canonical)
		m := cand.model
		entries = append(entries, &ModelLibraryEntry{
			Canonical:        canonical,
			Model:            bare,
			Lab:              lab,
			VendorID:         VendorIDFromModelName(canonical),
			Name:             m.Name,
			Family:           m.Family,
			ContextLength:    m.Limit.Context,
			MaxTokens:        m.Limit.Output,
			InputModalities:  catalogModalitiesToJSON(m.Modalities.Input),
			OutputModalities: catalogModalitiesToJSON(m.Modalities.Output),
			Capabilities:     catalogCapabilities(m),
			Endpoints:        ModelEndpointsFromModalities(bare, m.Modalities.Input, m.Modalities.Output),
			OpenWeights:      m.OpenWeights,
			ReleasedAt:       ParseCatalogDate(m.ReleaseDate),
			KnowledgeCutoff:  m.Knowledge,
			Deprecated:       m.Status == "deprecated",
			InputPrice:       m.Cost.Input,
			OutputPrice:      m.Cost.Output,
			CacheReadPrice:   m.Cost.CacheRead,
			Source:           cand.provider,
			SyncedAt:         now,
		})
	}
	return entries, hosts
}

// ReplaceModelLibrary swaps the whole library for a fresh build in one transaction. An empty
// build is refused so a broken download never wipes the library.
func ReplaceModelLibrary(entries []*ModelLibraryEntry, hosts []*ModelLibraryHost) error {
	if len(entries) == 0 {
		return nil
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ModelLibraryHost{}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ModelLibraryEntry{}).Error; err != nil {
			return err
		}
		if err := tx.CreateInBatches(entries, 200).Error; err != nil {
			return err
		}
		return tx.CreateInBatches(hosts, 500).Error
	})
	if err == nil {
		modelLibraryIndex.invalidate()
	}
	return err
}

// libraryIndex is the in-memory lookup the resolver and the supply suggestion use.
type libraryIndex struct {
	sync.RWMutex
	loaded bool
	// byHost maps lower-case host model ids of platforms (not the labs' own providers) to
	// canonical ids. A lab's own names are names in their own right and are not rewritten.
	byHost map[string]string
	// byLabHost maps lower-case lab-provider ids to canonical ids; labNames lists, per canonical
	// id, the ids the lab itself uses.
	byLabHost map[string]string
	labNames  map[string][]string
	// byProviderModel maps "provider\x00canonical" to that platform's id.
	byProviderModel map[string]string
	// byModel maps a lower-case bare model name to its canonical id.
	byModel map[string]string
}

var modelLibraryIndex = &libraryIndex{}

func (idx *libraryIndex) invalidate() {
	idx.Lock()
	idx.loaded = false
	idx.Unlock()
}

func (idx *libraryIndex) ensure() {
	idx.RLock()
	loaded := idx.loaded
	idx.RUnlock()
	if loaded || DB == nil {
		return
	}
	var entries []*ModelLibraryEntry
	var hosts []*ModelLibraryHost
	if err := DB.Select("canonical", "model").Find(&entries).Error; err != nil {
		return
	}
	if err := DB.Find(&hosts).Error; err != nil {
		return
	}
	byHost := make(map[string]string, len(hosts))
	byLabHost := map[string]string{}
	labNames := map[string][]string{}
	byProviderModel := make(map[string]string, len(hosts))
	byModel := make(map[string]string, len(entries))
	for _, e := range entries {
		byModel[strings.ToLower(e.Model)] = e.Canonical
	}
	// A host id that two models claim is ambiguous and left out.
	ambiguous := map[string]bool{}
	for _, h := range hosts {
		key := strings.ToLower(h.HostModel)
		pk := h.Provider + "\x00" + h.Canonical
		if _, ok := byProviderModel[pk]; !ok {
			byProviderModel[pk] = h.HostModel
		}
		if labProviders[h.Provider] {
			byLabHost[key] = h.Canonical
			labNames[h.Canonical] = append(labNames[h.Canonical], h.HostModel)
			continue
		}
		if cur, ok := byHost[key]; ok && cur != h.Canonical {
			ambiguous[key] = true
		}
		byHost[key] = h.Canonical
	}
	for key := range ambiguous {
		delete(byHost, key)
	}
	idx.Lock()
	idx.byHost, idx.byLabHost, idx.labNames = byHost, byLabHost, labNames
	idx.byProviderModel, idx.byModel, idx.loaded = byProviderModel, byModel, true
	idx.Unlock()
}

// LibraryPublicNames returns the public names a platform spelling may stand for
// ("claude-fable-5-2@default" → ["claude-fable-5-2"]): the bare canonical id first, then the ids
// the lab itself uses for the model. Nil when the name is not a known platform spelling.
func LibraryPublicNames(upstream string) []string {
	idx := modelLibraryIndex
	idx.ensure()
	idx.RLock()
	defer idx.RUnlock()
	canonical, ok := idx.byHost[strings.ToLower(strings.TrimSpace(upstream))]
	if !ok {
		return nil
	}
	_, bare := splitCanonical(canonical)
	names := []string{bare}
	for _, n := range idx.labNames[canonical] {
		if !strings.EqualFold(n, bare) {
			names = append(names, n)
		}
	}
	return names
}

// channelTypeLibraryProviders lists, per channel type, the models.dev providers whose spelling
// the channel sends as is. Channel types whose provider code translates names itself (Bedrock
// adds the anthropic. and region prefixes) or whose names are per-account (Azure deployments)
// are left out: for them the public name is sent unchanged.
var channelTypeLibraryProviders = map[int][]string{
	config.ChannelTypeVertexAI:        {"google-vertex-anthropic", "google-vertex"},
	config.ChannelTypeVertexAIExpress: {"google-vertex", "google-vertex-anthropic"},
	config.ChannelTypeOpenRouter:      {"openrouter"},
	config.ChannelTypeVercelAIGateway: {"vercel"},
	config.ChannelTypeGroq:            {"groq"},
}

// LibraryUpstreamFor returns how a channel of the given type names a public model, from the
// library's platform spellings; "" when the library has nothing for that platform.
func LibraryUpstreamFor(channelType int, publicModel string) string {
	providers := channelTypeLibraryProviders[channelType]
	if len(providers) == 0 {
		return ""
	}
	idx := modelLibraryIndex
	idx.ensure()
	idx.RLock()
	defer idx.RUnlock()
	canonical, ok := idx.byModel[strings.ToLower(publicModel)]
	if !ok {
		// A public name that is a lab's own spelling of a dated canonical id
		// ("claude-opus-4-5" for "anthropic/claude-opus-4-5-20251101").
		if canonical, ok = idx.byLabHost[strings.ToLower(publicModel)]; !ok {
			return ""
		}
	}
	for _, p := range providers {
		if host, ok := idx.byProviderModel[p+"\x00"+canonical]; ok {
			return host
		}
	}
	return ""
}

// SyncModelLibrary rebuilds the library from already downloaded models.dev data.
func SyncModelLibrary(data map[string]catalogProvider) ([]*ModelLibraryEntry, error) {
	entries, hosts := BuildModelLibrary(data, time.Now().Unix())
	if err := ReplaceModelLibrary(entries, hosts); err != nil {
		return nil, err
	}
	return entries, nil
}

// catalogRefreshInfos merges what the sync knows about each model name: the library row first,
// then the per-provider models.dev entry for names the library does not have.
func catalogRefreshInfos(entries []*ModelLibraryEntry, providerInfos []*ModelInfo) []*ModelInfo {
	out := make([]*ModelInfo, 0, len(entries)+len(providerInfos))
	seen := map[string]bool{}
	for _, e := range entries {
		key := strings.ToLower(e.Model)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, libraryModelInfo(e))
	}
	for _, info := range providerInfos {
		if !seen[strings.ToLower(info.Model)] {
			out = append(out, info)
		}
	}
	return out
}
