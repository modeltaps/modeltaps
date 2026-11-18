// Package admintools holds the tools of the admin MCP endpoint (/mcp/admin, root only): they let
// an AI assistant find, explain and read the site's settings. They never change anything.
package admintools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/settings"
	"github.com/modeltaps/modeltaps/model"
)

// Tool is one admin MCP tool.
type Tool interface {
	GetTool() *protocol.Tool
	HandleRequest(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error)
}

// Tools are the admin MCP tools.
var Tools = []Tool{&searchTool{}, &describeTool{}, &effectiveTool{}, &docsTool{}}

const (
	defaultLimit = 20
	maxLimit     = 50
)

func jsonResult(v any) (*protocol.CallToolResult, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return &protocol.CallToolResult{Content: []protocol.Content{&protocol.TextContent{Type: "text", Text: buf.String()}}}, nil
}

func consoleURL(s settings.Setting) string {
	return strings.TrimRight(config.ServerAddress, "/") + settings.ConsolePath(s)
}

// current is a site setting's value as the tools show it, with whether it differs from the
// default; object fields have no single value.
func current(s settings.Setting) (value *string, modified bool) {
	if s.Scope != settings.ScopeSite {
		return nil, false
	}
	v := model.SiteSettingValue(s.Key)
	return &v, model.SiteSettingModified(s.Key)
}

// ---------------------------------------------------------------- settings_search

type searchParams struct {
	Query           string `json:"query" description:"Words to look for in setting keys, names, descriptions and keywords (all must match). Empty lists everything the filters allow." required:"false"`
	Scope           string `json:"scope" description:"Only settings of this object: site, channel, model, supply, allocation, user_group, routing_policy, organization, token." required:"false"`
	Group           string `json:"group" description:"Only one settings page group or object page, e.g. gateway, billing, channels." required:"false"`
	ModifiedOnly    bool   `json:"modified_only" description:"Only site settings whose value differs from the default." required:"false"`
	IncludeAdvanced bool   `json:"include_advanced" description:"Also list rarely changed settings when the query is empty." required:"false"`
	Limit           int    `json:"limit" description:"At most this many results (default 20, at most 50)." required:"false"`
}

type searchHit struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Scope    string  `json:"scope"`
	Where    string  `json:"where"`
	Summary  string  `json:"summary"`
	Value    *string `json:"value,omitempty"`
	Modified bool    `json:"modified,omitempty"`
	Advanced bool    `json:"advanced,omitempty"`
}

type searchTool struct{}

func (searchTool) GetTool() *protocol.Tool {
	t, _ := protocol.NewTool("settings_search",
		"Find Modeltaps settings: platform settings (the options of the admin settings page) and the fields set on "+
			"channels, models, supplies, the allocation policy, user groups, routing policies, organizations and API keys. "+
			"Returns each match's key, name, where it is set, a one-line summary and, for platform settings, the current "+
			"value (secrets only say set or unset) and whether it differs from the default. Use settings_describe for the "+
			"full explanation, examples and console link.",
		searchParams{})
	return t
}

func (searchTool) HandleRequest(_ context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var p searchParams
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &p); err != nil {
		return nil, err
	}
	limit := p.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	hits := []searchHit{}
	total := 0
	for _, s := range settings.Search(p.Query, settings.Filter{Scope: settings.Scope(p.Scope), Group: p.Group, Advanced: p.IncludeAdvanced}) {
		value, modified := current(s)
		if p.ModifiedOnly && !modified {
			continue
		}
		total++
		if len(hits) >= limit {
			continue
		}
		hits = append(hits, searchHit{
			Key: s.Key, Label: s.Label, Scope: string(s.Scope), Where: settings.Location(s),
			Summary: firstSentence(s.Description), Value: value, Modified: modified, Advanced: s.Advanced,
		})
	}
	return jsonResult(map[string]any{"total": total, "results": hits})
}

// ---------------------------------------------------------------- settings_describe

type describeParams struct {
	Keys []string `json:"keys" description:"Setting keys from settings_search, e.g. [\"RetryTimes\", \"channel.fee_rate\"]." required:"true"`
}

type described struct {
	settings.Setting
	Where      string  `json:"where"`
	ConsoleURL string  `json:"console_url"`
	Value      *string `json:"value,omitempty"`
	Default    *string `json:"default_value,omitempty"`
	Modified   bool    `json:"modified,omitempty"`
	// Effective says the object fields settings_effective can resolve per object.
	Effective bool `json:"effective_resolvable,omitempty"`
}

type describeTool struct{}

func (describeTool) GetTool() *protocol.Tool {
	t, _ := protocol.NewTool("settings_describe",
		"Explain Modeltaps settings in full: type, unit, range, allowed values, what the setting does and how it combines "+
			"with others, worked examples, related settings, the operator docs page, and the console URL where an admin "+
			"changes it. For platform settings also the current and default values (secrets only say set or unset).",
		describeParams{})
	return t
}

func (describeTool) HandleRequest(_ context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var p describeParams
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &p); err != nil {
		return nil, err
	}
	if len(p.Keys) == 0 {
		return nil, errors.New("keys is required")
	}
	resolvable := map[string]bool{}
	for _, k := range model.EffectiveResolvableKeys() {
		resolvable[k] = true
	}
	defaults := config.GlobalOption.Defaults()
	out := []described{}
	var unknown []string
	for _, key := range p.Keys[:min(len(p.Keys), maxLimit)] {
		s, ok := settings.Get(key)
		if !ok {
			unknown = append(unknown, key)
			continue
		}
		d := described{Setting: s, Where: settings.Location(s), ConsoleURL: consoleURL(s), Effective: resolvable[key]}
		d.Value, d.Modified = current(s)
		if s.Scope == settings.ScopeSite && s.Type != settings.TypeSecret {
			if def, ok := defaults[key]; ok {
				d.Default = &def
			}
		}
		out = append(out, d)
	}
	result := map[string]any{"settings": out}
	if len(unknown) > 0 {
		result["unknown_keys"] = unknown
	}
	return jsonResult(result)
}

// ---------------------------------------------------------------- settings_effective

type effectiveParams struct {
	Key      string `json:"key" description:"A setting key." required:"true"`
	ObjectId string `json:"object_id" description:"For an object field: the channel id, the model name, the user group symbol or the API key id." required:"false"`
}

type effectiveTool struct{}

func (effectiveTool) GetTool() *protocol.Tool {
	t, _ := protocol.NewTool("settings_effective",
		"The value a setting takes effect with and where it comes from. For a platform setting: its value and whether "+
			"it is the default or stored. For an object field with object_id: the object's own value or the platform "+
			"setting it follows. Resolvable object fields: "+strings.Join(model.EffectiveResolvableKeys(), ", ")+".",
		effectiveParams{})
	return t
}

func (effectiveTool) HandleRequest(_ context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var p effectiveParams
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &p); err != nil {
		return nil, err
	}
	res, err := model.ResolveEffectiveSetting(p.Key, p.ObjectId)
	if err != nil {
		return nil, err
	}
	return jsonResult(res)
}

// firstSentence is the summary line of a description.
func firstSentence(text string) string {
	for i := 0; i < len(text)-1; i++ {
		if (text[i] == '.' || text[i] == '!' || text[i] == '?') && text[i+1] == ' ' {
			before := strings.ToLower(text[:i+1])
			if strings.HasSuffix(before, "e.g.") || strings.HasSuffix(before, "i.e.") {
				continue
			}
			return text[:i+1]
		}
	}
	return text
}
