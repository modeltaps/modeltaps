package model

import (
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
)

// CheaperUpstream is another id of the same model on the supply's channel that costs less than
// the one the supply sends. Prices are the channel's list prices, USD per million tokens.
type CheaperUpstream struct {
	Model  string  `json:"model"`
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

// cheaperUpstreamTypes are the channel types that list one model under several ids at different
// prices: Requesty's managed names ("claude-fable-5", priced at a regional endpoint) next to its
// provider-pinned ids ("anthropic/claude-fable-5").
var cheaperUpstreamTypes = map[int]bool{
	config.ChannelTypeRequesty: true,
}

// upstreamBareName is an upstream id without its host prefix, lower case: "claude-fable-5" for
// "anthropic/claude-fable-5".
func upstreamBareName(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		id = id[i+1:]
	}
	return strings.ToLower(id)
}

// regionalUpstream reports an id pinned to a region ("@eu") or a service tier (":flex"): the
// same model at another price, and with other terms, so it is never offered in place of another.
func regionalUpstream(id string) bool {
	return strings.ContainsAny(id, "@:")
}

func cheaperChannelPrice(a, b *ChannelPrice) bool {
	return a.Input <= b.Input && a.Output <= b.Output && (a.Input < b.Input || a.Output < b.Output)
}

// attachCheaperUpstreams sets CheaperUpstream on the views whose channel lists the same model
// under a cheaper id. Only token prices are compared, and only on cheaperUpstreamTypes channels.
func attachCheaperUpstreams(views []*ModelSupplyView, channels map[int]*Channel) error {
	var ids []int
	seen := map[int]bool{}
	for _, v := range views {
		ch := channels[v.ChannelId]
		if v.UpstreamPrice == nil || ch == nil || !cheaperUpstreamTypes[ch.Type] || seen[ch.Id] {
			continue
		}
		seen[ch.Id] = true
		ids = append(ids, ch.Id)
	}
	if len(ids) == 0 {
		return nil
	}
	var rows []*ChannelPrice
	if err := DB.Where("channel_id IN ?", ids).Find(&rows).Error; err != nil {
		return err
	}
	type key struct {
		channel int
		bare    string
	}
	byName := map[key][]*ChannelPrice{}
	for _, row := range rows {
		if regionalUpstream(row.Model) || row.Other || (row.Input <= 0 && row.Output <= 0) {
			continue
		}
		k := key{row.ChannelId, upstreamBareName(row.Model)}
		byName[k] = append(byName[k], row)
	}
	for _, v := range views {
		current := v.UpstreamPrice
		if current == nil || !seen[v.ChannelId] || current.Other || (current.Input <= 0 && current.Output <= 0) {
			continue
		}
		var best *ChannelPrice
		for _, row := range byName[key{v.ChannelId, upstreamBareName(current.Model)}] {
			if row.Model == current.Model || !cheaperChannelPrice(row, current) {
				continue
			}
			if best == nil || cheaperChannelPrice(row, best) || (!cheaperChannelPrice(best, row) && row.Model < best.Model) {
				best = row
			}
		}
		if best != nil {
			v.CheaperUpstream = &CheaperUpstream{Model: best.Model, Input: best.Input, Output: best.Output}
		}
	}
	return nil
}
