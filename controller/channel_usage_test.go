package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// TestUpdateChannelBalanceCodexUsage: refreshing a Codex channel's balance queries its usage
// windows, stores them and returns them instead of "not supported".
func TestUpdateChannelBalanceCodexUsage(t *testing.T) {
	setupBalanceTestDB(t)
	cache.InitCacheManager()
	var gotAuth, gotAccount string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backend-api/wham/usage" {
			http.NotFound(w, r)
			return
		}
		gotAuth, gotAccount = r.Header.Get("Authorization"), r.Header.Get("chatgpt-account-id")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false,
			"primary_window":{"used_percent":37,"reset_after_seconds":600,"limit_window_seconds":18000},
			"secondary_window":{"used_percent":12,"reset_at":1800000000,"limit_window_seconds":604800}}}`))
	}))
	defer srv.Close()

	base := srv.URL
	ch := model.Channel{Name: "codex", Type: config.ChannelTypeCodex, BaseURL: &base, Status: config.ChannelStatusEnabled,
		Key: `{"access_token":"tok","account_id":"acct-1"}`}
	if err := model.DB.Create(&ch).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel/update_balance/1", nil)
	UpdateChannelBalance(c)

	var resp struct {
		Success      bool                       `json:"success"`
		UsageWindows *model.ChannelUsageWindows `json:"usage_windows"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || !resp.Success || resp.UsageWindows == nil {
		t.Fatalf("response %s", w.Body.String())
	}
	if gotAuth != "Bearer tok" || gotAccount != "acct-1" {
		t.Errorf("auth=%q account=%q", gotAuth, gotAccount)
	}
	ws := resp.UsageWindows
	if ws.PlanType != "plus" || ws.Status != "allowed" || len(ws.Windows) != 2 ||
		ws.Windows[0].Name != "5h" || ws.Windows[0].UsedPercent != 37 || ws.Windows[0].ResetAt == 0 ||
		ws.Windows[1].Name != "7d" || ws.Windows[1].ResetAt != 1800000000 {
		t.Fatalf("windows = %+v", ws)
	}
	stored, _ := model.GetChannelById(ch.Id)
	if stored.UsageWindows == nil || len(stored.UsageWindows.Data().Windows) != 2 {
		t.Fatal("usage windows not stored")
	}
}
