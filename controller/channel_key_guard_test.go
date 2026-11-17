package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

type guardResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func callChannelHandler(t *testing.T, handler gin.HandlerFunc, method, target string, role int, body map[string]any, params gin.Params) guardResponse {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	raw, _ := json.Marshal(body)
	c.Request = httptest.NewRequest(method, target, bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("role", role)
	c.Params = params
	handler(c)
	var got guardResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad response %q: %v", w.Body.String(), err)
	}
	return got
}

func strPtr(s string) *string { return &s }

// A non-root admin must not point a channel's stored key at another address (SEC-50).
func TestUpdateChannelBaseURLNeedsKeyForNonRoot(t *testing.T) {
	setupBalanceTestDB(t)
	gin.SetMode(gin.TestMode)
	ch := model.Channel{Name: "c", Type: config.ChannelTypeOpenAI, Key: "sk-stored", BaseURL: strPtr("https://api.example.com/"),
		Models: "m", Group: "default", Status: config.ChannelStatusEnabled}
	if err := model.DB.Create(&ch).Error; err != nil {
		t.Fatal(err)
	}
	update := func(role int, baseURL, key string) guardResponse {
		body := map[string]any{"id": ch.Id, "type": ch.Type, "name": ch.Name, "models": "m", "group": "default", "base_url": baseURL}
		if key != "" {
			body["key"] = key
		}
		return callChannelHandler(t, UpdateChannel, http.MethodPut, "/api/channel/", role, body, nil)
	}
	stored := func() string {
		var got model.Channel
		model.DB.First(&got, ch.Id)
		return *got.BaseURL
	}

	if got := update(config.RoleAdminUser, "https://attacker.example", ""); got.Success || got.Code != common.APICodeChannelBaseURLNeedsKey {
		t.Fatalf("non-root move without key: %+v, want %s", got, common.APICodeChannelBaseURLNeedsKey)
	}
	if stored() != "https://api.example.com/" {
		t.Fatalf("base url changed to %q after a refused edit", stored())
	}
	// Same address with or without the trailing slash is not a move.
	if got := update(config.RoleAdminUser, "https://api.example.com", ""); !got.Success {
		t.Fatalf("non-root edit keeping the address refused: %+v", got)
	}
	if got := update(config.RoleAdminUser, "https://other.example", "sk-new"); !got.Success {
		t.Fatalf("non-root move with a new key refused: %+v", got)
	}
	if got := update(config.RoleRootUser, "https://root.example", ""); !got.Success {
		t.Fatalf("root move refused: %+v", got)
	}
	if stored() != "https://root.example" {
		t.Fatalf("base url = %q after root edit", stored())
	}
}

func TestAddChannelCopyKeyNeedsSameTarget(t *testing.T) {
	setupBalanceTestDB(t)
	gin.SetMode(gin.TestMode)
	src := model.Channel{Name: "src", Type: config.ChannelTypeOpenAI, Key: "sk-src", BaseURL: strPtr("https://api.example.com"),
		Models: "m", Group: "default", Status: config.ChannelStatusEnabled}
	if err := model.DB.Create(&src).Error; err != nil {
		t.Fatal(err)
	}
	target := "/api/channel/?copy_key_from=" + itoa(src.Id)
	add := func(role int, name, baseURL string) guardResponse {
		body := map[string]any{"type": src.Type, "name": name, "models": "m", "group": "default", "base_url": baseURL}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		raw, _ := json.Marshal(body)
		c.Request = httptest.NewRequest(http.MethodPost, target, bytes.NewReader(raw))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("role", role)
		AddChannel(c)
		var got guardResponse
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		return got
	}
	if got := add(config.RoleAdminUser, "copy-elsewhere", "https://attacker.example"); got.Success || got.Code != common.APICodeChannelCopyKeyMismatch {
		t.Fatalf("non-root copy to another address: %+v, want %s", got, common.APICodeChannelCopyKeyMismatch)
	}
	var n int64
	model.DB.Model(&model.Channel{}).Where("name = ?", "copy-elsewhere").Count(&n)
	if n != 0 {
		t.Fatal("refused copy still created a channel")
	}
	if got := add(config.RoleAdminUser, "copy-same", "https://api.example.com/"); !got.Success {
		t.Fatalf("non-root copy to the same address refused: %+v", got)
	}
	if got := add(config.RoleRootUser, "copy-root", "https://root.example"); !got.Success {
		t.Fatalf("root copy refused: %+v", got)
	}
}

func TestUpdateChannelsTagBaseURLRootOnly(t *testing.T) {
	setupBalanceTestDB(t)
	gin.SetMode(gin.TestMode)
	for _, name := range []string{"t1", "t2"} {
		ch := model.Channel{Name: name, Type: config.ChannelTypeOpenAI, Key: "sk-" + name, BaseURL: strPtr("https://api.example.com"),
			Models: "m", Group: "default", Tag: "grp", Status: config.ChannelStatusEnabled}
		if err := model.DB.Create(&ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	params := gin.Params{{Key: "tag", Value: "grp"}}
	body := func(baseURL string) map[string]any {
		return map[string]any{"type": config.ChannelTypeOpenAI, "name": "t", "models": "m", "group": "default", "tag": "grp", "base_url": baseURL}
	}
	if got := callChannelHandler(t, UpdateChannelsTag, http.MethodPut, "/api/channel_tag/grp", config.RoleAdminUser, body("https://attacker.example"), params); got.Success || got.Code != common.APICodeChannelBaseURLNeedsKey {
		t.Fatalf("non-root tag move: %+v", got)
	}
	if got := callChannelHandler(t, UpdateChannelsTag, http.MethodPut, "/api/channel_tag/grp", config.RoleAdminUser, body("https://api.example.com"), params); !got.Success {
		t.Fatalf("non-root tag edit keeping the address refused: %+v", got)
	}
	if got := callChannelHandler(t, UpdateChannelsTag, http.MethodPut, "/api/channel_tag/grp", config.RoleRootUser, body("https://root.example"), params); !got.Success {
		t.Fatalf("root tag move refused: %+v", got)
	}
}

func createGuardChannel(t *testing.T, ch model.Channel) model.Channel {
	t.Helper()
	if ch.Name == "" {
		ch.Name = "c"
	}
	ch.Models, ch.Group, ch.Status = "m", "default", config.ChannelStatusEnabled
	if ch.Key == "" {
		ch.Key = "sk-stored"
	}
	if err := model.DB.Create(&ch).Error; err != nil {
		t.Fatal(err)
	}
	return ch
}

func storedChannel(t *testing.T, id int) model.Channel {
	t.Helper()
	var got model.Channel
	if err := model.DB.First(&got, id).Error; err != nil {
		t.Fatal(err)
	}
	return got
}

// wholeSave is the body the console sends when it saves a channel: the stored row with the
// edited fields and an empty key.
func wholeSave(ch model.Channel, edits map[string]any) map[string]any {
	body := map[string]any{"id": ch.Id, "type": ch.Type, "name": ch.Name, "models": ch.Models, "group": ch.Group,
		"base_url": ch.GetBaseURL(), "other": ch.Other, "proxy": ch.GetProxy(), "key": ""}
	if ch.Plugin != nil {
		body["plugin"] = ch.Plugin.Data()
	}
	for k, v := range edits {
		body[k] = v
	}
	return body
}

func putChannel(t *testing.T, role int, body map[string]any) guardResponse {
	t.Helper()
	return callChannelHandler(t, UpdateChannel, http.MethodPut, "/api/channel/", role, body, nil)
}

// SEC-54: the target is more than the base URL. Other is the region in the host of Vertex AI,
// Vertex AI Express and Azure Speech channels, the type decides how the key is sent, and the proxy
// sees the requests.
func TestUpdateChannelTargetNeedsKeyForNonRoot(t *testing.T) {
	setupBalanceTestDB(t)
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name  string
		ch    model.Channel
		edits map[string]any
		check func(model.Channel) bool // true while the stored target is unchanged
	}{
		{"other (vertex express region)", model.Channel{Type: config.ChannelTypeVertexAIExpress, Other: "us-central1|proj"},
			map[string]any{"other": "attacker.tld/x?|p"}, func(c model.Channel) bool { return c.Other == "us-central1|proj" }},
		{"type", model.Channel{Type: config.ChannelTypeOpenAI},
			map[string]any{"type": config.ChannelTypeAzureSpeech, "other": "attacker"}, func(c model.Channel) bool { return c.Type == config.ChannelTypeOpenAI }},
		{"proxy", model.Channel{Type: config.ChannelTypeOpenAI},
			map[string]any{"proxy": "http://attacker.example:8080"}, func(c model.Channel) bool { return c.GetProxy() == "" }},
		{"endpoint plugin", model.Channel{Type: config.ChannelTypeGeminiCli},
			map[string]any{"plugin": map[string]any{"geminicli": map[string]any{"endpoint": "https://attacker.example"}}},
			func(c model.Channel) bool { return c.Plugin == nil || len(c.Plugin.Data()["geminicli"]) == 0 }},
		{"custom request path", model.Channel{Type: config.ChannelTypeCustom, BaseURL: strPtr("https://api.example.com")},
			map[string]any{"plugin": map[string]any{"customize": map[string]any{"1": "@attacker.example/v1/chat"}}},
			func(c model.Channel) bool { return c.Plugin == nil || len(c.Plugin.Data()["customize"]) == 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch := createGuardChannel(t, tc.ch)
			got := putChannel(t, config.RoleAdminUser, wholeSave(ch, tc.edits))
			if got.Success || got.Code != common.APICodeChannelBaseURLNeedsKey {
				t.Fatalf("non-root change without key: %+v, want %s", got, common.APICodeChannelBaseURLNeedsKey)
			}
			if !tc.check(storedChannel(t, ch.Id)) {
				t.Fatal("target changed after a refused edit")
			}
			// A save that leaves the target alone still goes through.
			if got := putChannel(t, config.RoleAdminUser, wholeSave(ch, map[string]any{"name": "renamed"})); !got.Success {
				t.Fatalf("non-root edit keeping the target refused: %+v", got)
			}
			withKey := map[string]any{"key": "sk-new"}
			for k, v := range tc.edits {
				withKey[k] = v
			}
			if got := putChannel(t, config.RoleAdminUser, wholeSave(ch, withKey)); !got.Success {
				t.Fatalf("non-root change with a new key refused: %+v", got)
			}
			ch = storedChannel(t, ch.Id)
			if got := putChannel(t, config.RoleRootUser, wholeSave(ch, tc.edits)); !got.Success {
				t.Fatalf("root change refused: %+v", got)
			}
		})
	}
}

// A partial update (no model list) writes only the fields it carries, so it is checked against
// what it would actually change.
func TestUpdateChannelPartialTargetChange(t *testing.T) {
	setupBalanceTestDB(t)
	gin.SetMode(gin.TestMode)
	ch := createGuardChannel(t, model.Channel{Type: config.ChannelTypeVertexAIExpress, Other: "us-central1|proj"})

	if got := putChannel(t, config.RoleAdminUser, map[string]any{"id": ch.Id, "other": "attacker.tld/x?|p"}); got.Success || got.Code != common.APICodeChannelBaseURLNeedsKey {
		t.Fatalf("partial other change: %+v", got)
	}
	if got := putChannel(t, config.RoleAdminUser, map[string]any{"id": ch.Id, "proxy": "socks5://attacker.example"}); got.Success {
		t.Fatalf("partial proxy change accepted: %+v", got)
	}
	if got := putChannel(t, config.RoleAdminUser, map[string]any{"id": ch.Id, "status": config.ChannelStatusManuallyDisabled}); !got.Success {
		t.Fatalf("partial status change refused: %+v", got)
	}
	if storedChannel(t, ch.Id).Other != "us-central1|proj" {
		t.Fatal("other changed by a refused edit")
	}
}

// A plugin change that does not touch an endpoint or path does not need the key, and the console
// saving unset plugin fields as "" is not a change.
func TestUpdateChannelPluginOutsideTarget(t *testing.T) {
	setupBalanceTestDB(t)
	gin.SetMode(gin.TestMode)
	ch := createGuardChannel(t, model.Channel{Type: config.ChannelTypeCustom, BaseURL: strPtr("https://api.example.com")})
	plugin := map[string]any{"customize": map[string]any{"1": "", "2": ""}, "other": map[string]any{"enable": true}}
	if got := putChannel(t, config.RoleAdminUser, wholeSave(ch, map[string]any{"plugin": plugin})); !got.Success {
		t.Fatalf("plugin edit outside the target refused: %+v", got)
	}
}

// Entering a new key while the stored management key stays would still send that key (used for
// balance reads) to the new target.
func TestUpdateChannelTargetKeepsManageKey(t *testing.T) {
	setupBalanceTestDB(t)
	gin.SetMode(gin.TestMode)
	ch := createGuardChannel(t, model.Channel{Type: config.ChannelTypeOpenAI, BaseURL: strPtr("https://api.example.com"), ManageKey: strPtr("sk-manage")})
	move := map[string]any{"base_url": "https://attacker.example", "key": "sk-new"}
	if got := putChannel(t, config.RoleAdminUser, wholeSave(ch, move)); got.Success || got.Code != common.APICodeChannelBaseURLNeedsKey {
		t.Fatalf("move keeping the management key: %+v", got)
	}
	move["manage_key"] = ""
	if got := putChannel(t, config.RoleAdminUser, wholeSave(ch, move)); !got.Success {
		t.Fatalf("move clearing the management key refused: %+v", got)
	}
}

func TestBatchUpdateChannelsAzureApiNeedsRootToMoveOther(t *testing.T) {
	setupBalanceTestDB(t)
	gin.SetMode(gin.TestMode)
	azure := createGuardChannel(t, model.Channel{Name: "azure", Type: config.ChannelTypeAzure, Other: "2024-02-01"})
	vertex := createGuardChannel(t, model.Channel{Name: "vertex", Type: config.ChannelTypeVertexAIExpress, Other: "us-central1|proj"})
	batch := func(role int, value string, ids ...int) guardResponse {
		return callChannelHandler(t, BatchUpdateChannelsAzureApi, http.MethodPut, "/api/channel/batch/azure_api", role,
			map[string]any{"value": value, "ids": ids}, nil)
	}
	if got := batch(config.RoleAdminUser, "attacker.tld/x?|p", azure.Id, vertex.Id); got.Success || got.Code != common.APICodeChannelBaseURLNeedsKey {
		t.Fatalf("non-root batch other change: %+v", got)
	}
	if storedChannel(t, vertex.Id).Other != "us-central1|proj" || storedChannel(t, azure.Id).Other != "2024-02-01" {
		t.Fatal("other changed by a refused batch edit")
	}
	if got := batch(config.RoleAdminUser, "2024-02-01", azure.Id); !got.Success {
		t.Fatalf("non-root batch keeping other refused: %+v", got)
	}
	if got := batch(config.RoleRootUser, "2024-06-01", azure.Id, vertex.Id); !got.Success {
		t.Fatalf("root batch refused: %+v", got)
	}
	if storedChannel(t, vertex.Id).Other != "2024-06-01" {
		t.Fatal("root batch edit not saved")
	}
}

func TestUpdateChannelsTagTargetRootOnly(t *testing.T) {
	setupBalanceTestDB(t)
	gin.SetMode(gin.TestMode)
	for _, name := range []string{"t1", "t2"} {
		createGuardChannel(t, model.Channel{Name: name, Type: config.ChannelTypeVertexAIExpress, Key: "sk-" + name, Tag: "grp", Other: "us-central1|proj"})
	}
	params := gin.Params{{Key: "tag", Value: "grp"}}
	body := func(edits map[string]any) map[string]any {
		b := map[string]any{"type": config.ChannelTypeVertexAIExpress, "name": "t", "models": "m", "group": "default", "tag": "grp", "other": "us-central1|proj"}
		for k, v := range edits {
			b[k] = v
		}
		return b
	}
	tagPut := func(role int, b map[string]any) guardResponse {
		return callChannelHandler(t, UpdateChannelsTag, http.MethodPut, "/api/channel_tag/grp", role, b, params)
	}
	for name, edits := range map[string]map[string]any{
		"other":         {"other": "attacker.tld/x?|p"},
		"proxy":         {"proxy": "http://attacker.example"},
		"other+new key": {"other": "attacker.tld/x?|p", "key": "sk-t1\nsk-t2"},
	} {
		if got := tagPut(config.RoleAdminUser, body(edits)); got.Success || got.Code != common.APICodeChannelBaseURLNeedsKey {
			t.Fatalf("non-root tag %s change: %+v", name, got)
		}
	}
	if got := tagPut(config.RoleAdminUser, body(nil)); !got.Success {
		t.Fatalf("non-root tag edit keeping the target refused: %+v", got)
	}
	if got := tagPut(config.RoleRootUser, body(map[string]any{"other": "europe-west4|proj"})); !got.Success {
		t.Fatalf("root tag change refused: %+v", got)
	}
}

func TestAddChannelCopyKeyComparesWholeTarget(t *testing.T) {
	setupBalanceTestDB(t)
	gin.SetMode(gin.TestMode)
	src := createGuardChannel(t, model.Channel{Name: "src", Type: config.ChannelTypeVertexAIExpress, Other: "us-central1|proj"})
	add := func(role int, name string, edits map[string]any) guardResponse {
		body := map[string]any{"type": src.Type, "name": name, "models": "m", "group": "default", "other": src.Other}
		for k, v := range edits {
			body[k] = v
		}
		return callChannelHandler(t, AddChannel, http.MethodPost, "/api/channel/?copy_key_from="+itoa(src.Id), role, body, nil)
	}
	for name, edits := range map[string]map[string]any{
		"other": {"other": "attacker.tld/x?|p"},
		"type":  {"type": config.ChannelTypeAzureSpeech},
		"proxy": {"proxy": "http://attacker.example"},
	} {
		if got := add(config.RoleAdminUser, "copy-"+name, edits); got.Success || got.Code != common.APICodeChannelCopyKeyMismatch {
			t.Fatalf("non-root copy with another %s: %+v, want %s", name, got, common.APICodeChannelCopyKeyMismatch)
		}
	}
	if got := add(config.RoleAdminUser, "copy-same", nil); !got.Success {
		t.Fatalf("non-root copy to the same target refused: %+v", got)
	}
	if got := add(config.RoleRootUser, "copy-root", map[string]any{"other": "europe-west4|proj"}); !got.Success {
		t.Fatalf("root copy refused: %+v", got)
	}
}

// SEC-64: an OpenRouter channel sends its management key to <base_url>/v1/credits when the
// balance is refreshed. A non-root admin who enters any new key but no management key must not
// move the channel to another host, whether the console saves the whole channel or the API gets
// only the changed fields.
func TestUpdateOpenRouterBaseURLKeepsManageKeyNeedsRoot(t *testing.T) {
	setupBalanceTestDB(t)
	gin.SetMode(gin.TestMode)
	ch := createGuardChannel(t, model.Channel{Type: config.ChannelTypeOpenRouter, BaseURL: strPtr("https://openrouter.ai/api"),
		ManageKey: strPtr("sk-or-manage")})
	unchanged := func() {
		t.Helper()
		got := storedChannel(t, ch.Id)
		if got.GetBaseURL() != "https://openrouter.ai/api" || got.GetManageKey() != "sk-or-manage" || got.Key != "sk-stored" {
			t.Fatalf("channel changed by a refused edit: base_url %q", got.GetBaseURL())
		}
	}

	partial := map[string]any{"id": ch.Id, "key": "dummy", "base_url": "https://attacker.example"}
	if got := putChannel(t, config.RoleAdminUser, partial); got.Success || got.Code != common.APICodeChannelBaseURLNeedsKey {
		t.Fatalf("partial move keeping the management key: %+v", got)
	}
	unchanged()
	whole := wholeSave(ch, map[string]any{"key": "dummy", "base_url": "https://attacker.example"})
	if got := putChannel(t, config.RoleAdminUser, whole); got.Success || got.Code != common.APICodeChannelBaseURLNeedsKey {
		t.Fatalf("whole move keeping the management key: %+v", got)
	}
	unchanged()

	// Entering the management key too (or clearing it) means nothing stored follows the move.
	partial["manage_key"] = "sk-or-manage-new"
	if got := putChannel(t, config.RoleAdminUser, partial); !got.Success {
		t.Fatalf("move with a new management key refused: %+v", got)
	}
	if got := storedChannel(t, ch.Id); got.GetManageKey() != "sk-or-manage-new" {
		t.Fatalf("management key %q after the move", got.GetManageKey())
	}
}
