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
