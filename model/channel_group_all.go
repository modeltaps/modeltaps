package model

import (
	"sort"
	"strings"
)

// ChannelGroupAll as a channel's group means every user group, including groups created later.
const ChannelGroupAll = "*"

// NormalizeChannelGroup trims and de-duplicates a comma-separated channel group list, keeping the
// first-seen order. A list that contains ChannelGroupAll becomes just ChannelGroupAll.
func NormalizeChannelGroup(group string) string {
	parts := splitChannelGroup(group)
	for _, g := range parts {
		if g == ChannelGroupAll {
			return ChannelGroupAll
		}
	}
	return strings.Join(parts, ",")
}

// ChannelGroupIsAll reports whether a channel group list means every user group.
func ChannelGroupIsAll(group string) bool {
	for _, g := range splitChannelGroup(group) {
		if g == ChannelGroupAll {
			return true
		}
	}
	return false
}

// expandChannelGroup returns the user groups a channel group list covers: all when it holds
// ChannelGroupAll, otherwise the listed symbols.
func expandChannelGroup(group string, all []string) []string {
	if ChannelGroupIsAll(group) {
		return append([]string(nil), all...)
	}
	return splitChannelGroup(group)
}

// ExpandChannelGroup expands a channel group list against the cached user groups. Use it on
// request and admin paths; the routing table loader reads the groups from the database instead,
// because it runs before the user group cache is loaded at startup.
func ExpandChannelGroup(group string) []string {
	if !ChannelGroupIsAll(group) {
		return splitChannelGroup(group)
	}
	return expandChannelGroup(group, cachedUserGroupSymbols())
}

func splitChannelGroup(group string) []string {
	seen := map[string]bool{}
	var parts []string
	for _, g := range strings.Split(group, ",") {
		g = strings.TrimSpace(g)
		if g == "" || seen[g] {
			continue
		}
		seen[g] = true
		parts = append(parts, g)
	}
	return parts
}

func cachedUserGroupSymbols() []string {
	groups := GlobalUserGroupRatio.GetAll()
	symbols := make([]string, 0, len(groups))
	for symbol := range groups {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	return symbols
}

// userGroupSymbolsFromDB lists the enabled user group symbols, sorted: the same set the user
// group cache holds, so ChannelGroupAll covers the same groups on every path.
func userGroupSymbolsFromDB() []string {
	var symbols []string
	if err := DB.Model(&UserGroup{}).Where("enable = ?", true).Order("symbol").Pluck("symbol", &symbols).Error; err != nil {
		return nil
	}
	return symbols
}
