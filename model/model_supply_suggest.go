package model

import (
	"encoding/json"
	"strings"
)

// Where a suggested upstream name came from.
const (
	SupplyUpstreamFromMapping = "mapping" // the channel's model_mapping has an entry for the model
	SupplyUpstreamFromChannel = "channel" // the channel lists the model, or a platform spelling of it
	SupplyUpstreamFromLibrary = "library" // the model library knows how the channel's platform names it
	SupplyUpstreamAsIs        = ""        // nothing known: the public name is sent unchanged
)

// SupplyUpstreamSuggestion is what the add-supply form fills into upstream_model.
type SupplyUpstreamSuggestion struct {
	UpstreamModel string `json:"upstream_model"`
	Source        string `json:"source"`
}

// SuggestSupplyUpstream returns how the channel should name publicModel upstream. First hit wins:
// the channel's model mapping, the channel's own model list (the name itself, or a platform
// spelling the library ties to it, such as "claude-fable-5-2@default"), the library's spelling for
// the channel's platform, and otherwise the public name.
func SuggestSupplyUpstream(publicModel string, channel *Channel) SupplyUpstreamSuggestion {
	publicModel = strings.TrimSpace(publicModel)
	asIs := SupplyUpstreamSuggestion{UpstreamModel: publicModel, Source: SupplyUpstreamAsIs}
	if channel == nil || publicModel == "" {
		return asIs
	}
	if raw := strings.TrimSpace(channel.GetModelMapping()); raw != "" {
		mapping := map[string]string{}
		if json.Unmarshal([]byte(raw), &mapping) == nil {
			if mapped := strings.TrimPrefix(strings.TrimSpace(mapping[publicModel]), "+"); mapped != "" {
				return SupplyUpstreamSuggestion{UpstreamModel: mapped, Source: SupplyUpstreamFromMapping}
			}
		}
	}
	listed := SplitChannelModels(channel.Models)
	for _, name := range listed {
		if name == publicModel {
			return SupplyUpstreamSuggestion{UpstreamModel: name, Source: SupplyUpstreamFromChannel}
		}
	}
	for _, name := range listed {
		for _, public := range LibraryPublicNames(name) {
			if strings.EqualFold(public, publicModel) {
				return SupplyUpstreamSuggestion{UpstreamModel: name, Source: SupplyUpstreamFromChannel}
			}
		}
	}
	if host := LibraryUpstreamFor(channel.Type, publicModel); host != "" {
		return SupplyUpstreamSuggestion{UpstreamModel: host, Source: SupplyUpstreamFromLibrary}
	}
	return asIs
}
