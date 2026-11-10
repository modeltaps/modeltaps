package model

import "strings"

// Ways an upstream model name can resolve to one of our canonical names, in the order they are
// tried. CanonicalSourceNone means no canonical name was found and the upstream name is used as is.
const (
	CanonicalSourceAlias = "alias" // the catalog row for the upstream name has alias_of set
	CanonicalSourceSeed  = "seed"  // the curated seed lists the upstream name as an alias
	CanonicalSourceBare  = "bare"  // the name without its "vendor/" prefix is a canonical name or alias
	CanonicalSourceNone  = ""
)

// CanonicalMatch is the canonical name suggested for an upstream model name.
type CanonicalMatch struct {
	Canonical string `json:"canonical"`
	Source    string `json:"canonical_source"`
}

// CanonicalResolver maps upstream model names to canonical names. Build it once per request with
// NewCanonicalResolver: it holds a snapshot of the catalog and the seed.
type CanonicalResolver struct {
	byModel map[string]*ModelInfo
	byLower map[string]*ModelInfo
	// seedAliases maps an alias listed in the seed to its canonical name; seedLower is the same
	// map with lower-case keys.
	seedAliases map[string]string
	seedLower   map[string]string
	// seedCanonicals holds the seed's canonical names in lower case, mapped to their exact spelling.
	seedCanonicals map[string]string
}

// NewCanonicalResolver loads the catalog and the seed. A seed that fails to load is skipped, so the
// resolver still works from the catalog alone.
func NewCanonicalResolver() (*CanonicalResolver, error) {
	var infos []*ModelInfo
	if err := DB.Find(&infos).Error; err != nil {
		return nil, err
	}
	var seed *CatalogSeed
	if loaded, err := LoadCatalogSeed(); err == nil {
		seed = loaded
	}
	return newCanonicalResolver(infos, seed), nil
}

func newCanonicalResolver(infos []*ModelInfo, seed *CatalogSeed) *CanonicalResolver {
	r := &CanonicalResolver{
		byModel:        make(map[string]*ModelInfo, len(infos)),
		byLower:        make(map[string]*ModelInfo, len(infos)),
		seedAliases:    map[string]string{},
		seedLower:      map[string]string{},
		seedCanonicals: map[string]string{},
	}
	for _, info := range infos {
		r.byModel[info.Model] = info
	}
	for _, info := range infos {
		lower := strings.ToLower(info.Model)
		// Several rows can differ only by case; prefer the canonical one, then the exact spelling
		// seen first, so lookups stay deterministic.
		if cur, ok := r.byLower[lower]; !ok || (cur.AliasOf != "" && info.AliasOf == "") {
			r.byLower[lower] = info
		}
	}
	if seed != nil {
		for _, entry := range seed.Models {
			r.seedCanonicals[strings.ToLower(entry.Canonical)] = entry.Canonical
			for _, alias := range entry.Aliases {
				if alias == "" || alias == entry.Canonical {
					continue
				}
				r.seedAliases[alias] = entry.Canonical
				r.seedLower[strings.ToLower(alias)] = entry.Canonical
			}
		}
	}
	return r
}

// canonicalOf follows alias_of from a catalog row; ok is false when the row is not an alias.
func (r *CanonicalResolver) canonicalOf(info *ModelInfo) (string, bool) {
	if info == nil || info.AliasOf == "" {
		return "", false
	}
	canonical := resolveCanonicalIn(r.byModel, info.Model)
	return canonical, canonical != info.Model
}

// Resolve returns the canonical name for an upstream model name. Rules, first hit wins:
//  1. the catalog row for the exact name is an alias;
//  2. the seed lists the exact name as an alias;
//  3. for a "vendor/model" name, the bare model id (case-insensitive) is a catalog alias, a seed
//     alias, a seed canonical name or a non-alias catalog row.
//
// When nothing matches, or the name is already canonical, Canonical is the name itself and Source is
// CanonicalSourceNone. Wildcard entries ("claude-*") are never resolved.
func (r *CanonicalResolver) Resolve(upstream string) CanonicalMatch {
	none := CanonicalMatch{Canonical: upstream, Source: CanonicalSourceNone}
	if upstream == "" || strings.HasSuffix(upstream, "*") {
		return none
	}
	if canonical, ok := r.canonicalOf(r.byModel[upstream]); ok {
		return CanonicalMatch{Canonical: canonical, Source: CanonicalSourceAlias}
	}
	if canonical, ok := r.seedAliases[upstream]; ok {
		return CanonicalMatch{Canonical: canonical, Source: CanonicalSourceSeed}
	}
	if !strings.Contains(upstream, "/") {
		return none
	}
	bare := strings.ToLower(bareModelID(upstream))
	if bare == "" {
		return none
	}
	if info := r.byLower[bare]; info != nil {
		if canonical, ok := r.canonicalOf(info); ok {
			return CanonicalMatch{Canonical: canonical, Source: CanonicalSourceBare}
		}
	}
	if canonical, ok := r.seedLower[bare]; ok {
		return CanonicalMatch{Canonical: canonical, Source: CanonicalSourceBare}
	}
	if canonical, ok := r.seedCanonicals[bare]; ok {
		return CanonicalMatch{Canonical: canonical, Source: CanonicalSourceBare}
	}
	if info := r.byLower[bare]; info != nil {
		return CanonicalMatch{Canonical: info.Model, Source: CanonicalSourceBare}
	}
	return none
}
