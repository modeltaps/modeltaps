package admintools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

func call(t *testing.T, tool Tool, args map[string]any) string {
	t.Helper()
	raw, _ := json.Marshal(args)
	res, err := tool.HandleRequest(context.Background(), &protocol.CallToolRequest{Name: tool.GetTool().Name, RawArguments: raw})
	if err != nil {
		t.Fatalf("%s: %v", tool.GetTool().Name, err)
	}
	return res.Content[0].(*protocol.TextContent).Text
}

func TestSettingsTools(t *testing.T) {
	model.RegisterOptions()
	old := config.GitHubClientSecret
	config.GitHubClientSecret = "very-secret-value"
	t.Cleanup(func() { config.GitHubClientSecret = old })

	out := call(t, &searchTool{}, map[string]any{"query": "fee rate"})
	if !strings.Contains(out, `"channel.fee_rate"`) {
		t.Errorf("search for fee rate misses channel.fee_rate: %s", out)
	}
	out = call(t, &searchTool{}, map[string]any{"query": "retry", "scope": "site", "limit": 2})
	var parsed struct {
		Total   int         `json:"total"`
		Results []searchHit `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil || len(parsed.Results) != 2 || parsed.Total < 2 || parsed.Results[0].Value == nil {
		t.Errorf("limited site search = %s (%v)", out, err)
	}

	out = call(t, &describeTool{}, map[string]any{"keys": []string{"GitHubClientSecret", "SupplyCostOrder", "nope"}})
	if strings.Contains(out, "very-secret-value") || !strings.Contains(out, `"(set)"`) {
		t.Errorf("describe leaks or misses the secret state: %s", out)
	}
	if !strings.Contains(out, "/panel/admin/settings/gateway?highlight=SupplyCostOrder") || !strings.Contains(out, `"unknown_keys"`) {
		t.Errorf("describe misses the console URL or the unknown key: %s", out)
	}

	out = call(t, &effectiveTool{}, map[string]any{"key": "SupplyCostOrder"})
	if !strings.Contains(out, `"level": "default"`) {
		t.Errorf("effective value of an unchanged setting should come from the default: %s", out)
	}

	out = call(t, &docsTool{}, map[string]any{"query": "fee rate top-ups"})
	if !strings.Contains(out, "pricing_and_routing.md") {
		t.Errorf("docs search misses the pricing guide: %s", out)
	}
}

func TestAdminToolNames(t *testing.T) {
	var names []string
	for _, tool := range Tools {
		names = append(names, tool.GetTool().Name)
	}
	if strings.Join(names, ",") != "settings_search,settings_describe,settings_effective,settings_docs" {
		t.Errorf("tools = %v", names)
	}
}
