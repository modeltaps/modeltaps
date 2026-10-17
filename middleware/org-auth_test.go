package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

// Organization guard failures carry a code so the console can translate them.
func TestOrganizationFeatureDisabledCarriesCode(t *testing.T) {
	old := config.OrganizationEnabled
	config.OrganizationEnabled = false
	t.Cleanup(func() { config.OrganizationEnabled = old })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/org", OrganizationFeatureEnabled(), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/org", nil))

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if w.Code != http.StatusForbidden || body["code"] != common.APICodeOrgFeatureDisabled || body["message"] != "Organizations are not enabled" {
		t.Fatalf("unexpected response %d %v", w.Code, body)
	}
}
