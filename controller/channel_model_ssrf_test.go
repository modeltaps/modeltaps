package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
)

// SEC-79: the model list fetch returns the upstream's error text, so a non-root admin must not be
// able to point it at the gateway's internal network; the root user still can (self-hosted models).
func TestGetModelListPrivateAddressRootOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"local-model"}]}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"internal_secret":"cluster-7"}`))
	}))
	defer internal.Close()

	fetch := func(role int, baseURL string) guardResponse {
		body := map[string]any{"type": config.ChannelTypeOpenAI, "key": "x", "base_url": baseURL}
		return callChannelHandler(t, GetModelList, http.MethodPost, "/api/channel/provider_models_list", role, body, nil)
	}

	for _, baseURL := range []string{internal.URL, internal.URL + "/x?"} {
		got := fetch(config.RoleAdminUser, baseURL)
		if got.Success || strings.Contains(got.Message, "cluster-7") {
			t.Fatalf("admin fetch from %s: %+v, want refusal without the upstream body", baseURL, got)
		}
		if !strings.Contains(got.Message, requester.ErrUpstreamNotPublic.Error()) {
			t.Fatalf("admin fetch from %s: message %q", baseURL, got.Message)
		}
	}

	if got := fetch(config.RoleRootUser, internal.URL); !got.Success {
		t.Fatalf("root fetch from a private address: %+v, want the model list", got)
	}
}
