package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func respondBody(t *testing.T, err error) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	APIRespondWithError(c, http.StatusOK, err)
	var body map[string]any
	if decodeErr := json.Unmarshal(w.Body.Bytes(), &body); decodeErr != nil {
		t.Fatalf("decode body: %v", decodeErr)
	}
	return body
}

// A plain error keeps the old shape: no code, no params.
func TestAPIRespondWithErrorPlainError(t *testing.T) {
	body := respondBody(t, errors.New("record not found"))
	if body["success"] != false || body["message"] != "record not found" {
		t.Fatalf("unexpected body: %v", body)
	}
	if _, ok := body["code"]; ok {
		t.Fatalf("plain error must not carry a code: %v", body)
	}
	if _, ok := body["params"]; ok {
		t.Fatalf("plain error must not carry params: %v", body)
	}
}

// An APIError adds code and params next to the unchanged English message, also when wrapped.
func TestAPIRespondWithErrorCodedError(t *testing.T) {
	coded := NewAPIError(APICodeChannelTagTypeMismatch, `tag "a" already contains channels of another type`, map[string]any{"tag": "a"})
	for name, err := range map[string]error{"direct": coded, "wrapped": fmt.Errorf("add channel: %w", coded)} {
		t.Run(name, func(t *testing.T) {
			body := respondBody(t, err)
			if body["success"] != false || body["message"] != err.Error() {
				t.Fatalf("message must stay the English text: %v", body)
			}
			if body["code"] != "channel_tag_type_mismatch" {
				t.Fatalf("code = %v, want channel_tag_type_mismatch", body["code"])
			}
			params, ok := body["params"].(map[string]any)
			if !ok || params["tag"] != "a" {
				t.Fatalf("params = %v, want tag=a", body["params"])
			}
		})
	}
}

// A code without params leaves params out.
func TestAPIRespondWithCodeOmitsEmptyParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	APIRespondWithCode(c, http.StatusOK, APICodeChannelTagRequired, "tag is required", nil)
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["code"] != "channel_tag_required" || body["message"] != "tag is required" || body["success"] != false {
		t.Fatalf("unexpected body: %v", body)
	}
	if _, ok := body["params"]; ok {
		t.Fatalf("empty params must be omitted: %v", body)
	}
}
