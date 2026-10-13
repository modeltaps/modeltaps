package model

import (
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
)

// ShouldUseResponsesForModel reports whether a chat request for modelName (the
// channel-mapped name) is sent to the Responses API on this channel.
//
// The channel setting adds to the built-in list rather than replacing it, so
// listing one new model never silently drops the defaults. An entry prefixed with
// "-" removes a model, for upstreams that serve it on chat completions.
// A removal wins over an addition of the same model.
func (channel *Channel) ShouldUseResponsesForModel(modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false
	}

	use := config.NeedsResponsesAPI(modelName)
	if channel == nil || channel.Need2ResponseModels == nil {
		return use
	}
	for _, entry := range config.ParseNeed2ResponseModels(*channel.Need2ResponseModels) {
		if name, removed := strings.CutPrefix(entry, "-"); removed {
			if strings.TrimSpace(name) == modelName {
				return false
			}
			continue
		}
		if entry == modelName {
			use = true
		}
	}
	return use
}
