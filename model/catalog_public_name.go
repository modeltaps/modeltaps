package model

import "strings"

// Platforms list models under their own spellings ("anthropic/claude-opus-5" on an aggregator,
// "claude-opus-5@default" on Vertex). The public catalog names a model once, by its public name
// ("claude-opus-5"); platform spellings stay in channel model lists and in
// model_supplies.upstream_model. The helpers here give the syncs that write catalog rows and prices
// from such lists the public name to write under.

// PublicName returns the public catalog name for a name a channel lists:
//  1. the canonical name Resolve finds (catalog alias, seed alias, model library, bare id);
//  2. otherwise the name itself when it already has a catalog row of its own, so a row an admin
//     added under a platform spelling keeps being the model that spelling belongs to;
//  3. otherwise, for a "vendor/model" name, the model id after the last "/" in its own case;
//  4. otherwise the name itself.
//
// A row created under the returned name is the one AutoSupplyChannel later attaches the spelling
// to: Resolve finds a non-alias catalog row by the case-insensitive bare id. Wildcards are returned
// unchanged.
func (r *CanonicalResolver) PublicName(upstream string) string {
	upstream = strings.TrimSpace(upstream)
	if upstream == "" || strings.HasSuffix(upstream, "*") {
		return upstream
	}
	if match := r.Resolve(upstream); match.Source != CanonicalSourceNone {
		return match.Canonical
	}
	if r.byModel[upstream] != nil || !strings.Contains(upstream, "/") {
		return upstream
	}
	if bare := bareModelID(upstream); bare != "" {
		return bare
	}
	return upstream
}

// publicSpelling is one upstream spelling of a public name, with the priority of the list it
// came from (lower wins).
type publicSpelling struct {
	public string
	raw    string
	rank   int
}

// preferredSpelling reports whether a should be kept over b when both are spellings of the same
// public name: the list with the higher priority first, then the spelling whose bare id is the
// public name, then the shorter, then the smaller, so the choice does not depend on input order.
func preferredSpelling(a, b publicSpelling) bool {
	if a.rank != b.rank {
		return a.rank < b.rank
	}
	aBare, bBare := bareModelID(a.raw) == a.public, bareModelID(b.raw) == b.public
	if aBare != bBare {
		return aBare
	}
	if len(a.raw) != len(b.raw) {
		return len(a.raw) < len(b.raw)
	}
	return a.raw < b.raw
}

// PublicNameInfos prepares synced model metadata for the catalog. groups holds the infos of each
// source, highest priority first; the result has the same shape. Each info is copied under its
// public name, one per public name across all groups (see preferredSpelling). An info whose
// upstream spelling differs from its public name but already has a catalog row also refreshes
// that row, as the sync did before; no new row is ever created under a platform spelling.
func PublicNameInfos(r *CanonicalResolver, groups [][]*ModelInfo) [][]*ModelInfo {
	type pick struct {
		spelling publicSpelling
		info     *ModelInfo
	}
	picked := map[string]pick{}
	var order []string
	refresh := make([][]*ModelInfo, len(groups))
	refreshed := map[string]bool{}
	for rank, infos := range groups {
		for _, info := range infos {
			if info == nil || strings.TrimSpace(info.Model) == "" {
				continue
			}
			raw := strings.TrimSpace(info.Model)
			public := r.PublicName(raw)
			if public != raw && r.byModel[raw] != nil && !refreshed[raw] {
				refreshed[raw] = true
				copied := *info
				copied.Model = raw
				refresh[rank] = append(refresh[rank], &copied)
			}
			s := publicSpelling{public: public, raw: raw, rank: rank}
			cur, seen := picked[public]
			if !seen {
				order = append(order, public)
			} else if !preferredSpelling(s, cur.spelling) {
				continue
			}
			picked[public] = pick{spelling: s, info: info}
		}
	}
	out := make([][]*ModelInfo, len(groups))
	for _, public := range order {
		p := picked[public]
		copied := *p.info
		copied.Model = public
		out[p.spelling.rank] = append(out[p.spelling.rank], &copied)
	}
	for rank, infos := range refresh {
		for _, info := range infos {
			if _, taken := picked[info.Model]; !taken {
				out[rank] = append(out[rank], info)
			}
		}
	}
	return out
}

// PublicNamePrices returns copies of synced prices under the public names of their upstream
// spellings, one per public name (see preferredSpelling); groups holds the prices of each source,
// highest priority first. The prices under the upstream spellings are still written by the caller:
// without supply routing a request is billed by the name it asked for. No copy is made for a
// public name that is itself one of the synced spellings, since that price is written as is.
func PublicNamePrices(r *CanonicalResolver, groups [][]*Price) []*Price {
	listed := map[string]bool{}
	for _, prices := range groups {
		for _, p := range prices {
			if p != nil {
				listed[p.Model] = true
			}
		}
	}
	type pick struct {
		spelling publicSpelling
		price    *Price
	}
	picked := map[string]pick{}
	var order []string
	for rank, prices := range groups {
		for _, price := range prices {
			if price == nil || price.Model == "" {
				continue
			}
			public := r.PublicName(price.Model)
			if public == price.Model || listed[public] {
				continue
			}
			s := publicSpelling{public: public, raw: price.Model, rank: rank}
			cur, seen := picked[public]
			if !seen {
				order = append(order, public)
			} else if !preferredSpelling(s, cur.spelling) {
				continue
			}
			picked[public] = pick{spelling: s, price: price}
		}
	}
	out := make([]*Price, 0, len(order))
	for _, public := range order {
		copied := *picked[public].price
		copied.Model = public
		copied.ModelInfo = nil
		out = append(out, &copied)
	}
	return out
}

// KeepChannelPricedOrMissing keeps the prices an aggregator price list may write under a public
// name: names with no price yet, and names whose price an aggregator list wrote. A price from
// another source (the lab's official price from models.dev, or one an admin set) stays, so the
// official price of a model is not replaced by an aggregator's and two syncs do not keep
// overwriting each other.
func KeepChannelPricedOrMissing(prices []*Price) []*Price {
	kept := make([]*Price, 0, len(prices))
	for _, price := range prices {
		if existing, ok := PricingInstance.priceSnapshot(price.Model); ok && !isChannelPriceSource(existing.ChannelType) {
			continue
		}
		kept = append(kept, price)
	}
	return kept
}
