package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

// AddChannel answers with the ids it created so the console can open the new channel.
func TestAddChannelReturnsIDs(t *testing.T) {
	setupChannelAPICodeDB(t)
	gin.SetMode(gin.TestMode)

	post := func(body map[string]any) []int {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		raw, _ := json.Marshal(body)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/", bytes.NewReader(raw))
		c.Request.Header.Set("Content-Type", "application/json")
		AddChannel(c)
		var got struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Data    struct {
				IDs []int `json:"ids"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode body %q: %v", w.Body.String(), err)
		}
		if !got.Success {
			t.Fatalf("add failed: %s", got.Message)
		}
		return got.Data.IDs
	}

	one := post(map[string]any{"type": config.ChannelTypeOpenAI, "key": "sk-one", "name": "one", "models": "gpt-4o", "group": "default"})
	if len(one) != 1 || one[0] <= 0 {
		t.Fatalf("single key ids = %v, want one positive id", one)
	}

	many := post(map[string]any{"type": config.ChannelTypeOpenAI, "key": "sk-a\nsk-b", "name": "many", "models": "gpt-4o", "group": "default"})
	if len(many) != 2 || many[0] == many[1] || many[0] == one[0] {
		t.Fatalf("batch ids = %v, want two new distinct ids", many)
	}
}
