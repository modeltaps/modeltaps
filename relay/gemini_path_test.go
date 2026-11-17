package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setGeminiRequest(t *testing.T, version, action string) (*relayGeminiOnly, error) {
	t.Helper()
	c := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/gemini/"+version+"/models/x:generateContent", strings.NewReader(`{"contents":[]}`))
	c.Params = gin.Params{{Key: "version", Value: version}, {Key: "action", Value: action}}
	r := NewRelayGeminiOnly(c)
	return r, r.setRequest()
}

// TestGeminiNativeRejectsUnbilledMethods: the native Gemini route forwards only the methods it
// bills. Any other method from the path would reach the upstream URL and run on the channel's
// account without a charge (SEC-66).
func TestGeminiNativeRejectsUnbilledMethods(t *testing.T) {
	for _, action := range []string{
		"batchGenerateContent",
		"asyncBatchEmbedContent",
		"embedContent",
		"batchEmbedContents",
		"generateAnswer",
		"generateContent?alt=sse",
		"generateContent/../../cachedContents",
		"",
	} {
		if _, err := setGeminiRequest(t, "v1beta", "/gemini-2.5-flash:"+action); err == nil {
			t.Errorf("action %q: setRequest accepted it", action)
		}
	}
}

func TestGeminiNativeAcceptsBilledMethods(t *testing.T) {
	for _, version := range []string{"v1", "v1beta", "v1alpha"} {
		for _, action := range []string{"generateContent", "streamGenerateContent", "countTokens"} {
			r, err := setGeminiRequest(t, version, "/gemini-2.5-flash:"+action)
			if err != nil {
				t.Fatalf("%s %s: %v", version, action, err)
			}
			if r.geminiRequest.Action != action {
				t.Fatalf("%s %s: action %q", version, action, r.geminiRequest.Action)
			}
		}
	}
}

func TestGeminiNativeRejectsUnknownVersion(t *testing.T) {
	for _, version := range []string{"v2", "v1beta?key=x", "v1beta#", "..", "v1internal"} {
		if _, err := setGeminiRequest(t, version, "/gemini-2.5-flash:generateContent"); err == nil {
			t.Errorf("version %q: setRequest accepted it", version)
		}
	}
}

// TestGeminiNativeRejectsUnsafeModel: the model name goes into the upstream URL path, so it
// cannot carry a query, a fragment, an escape, whitespace or a dot segment. Slashes stay
// allowed for partner models.
func TestGeminiNativeRejectsUnsafeModel(t *testing.T) {
	for _, model := range []string{
		"gemini-2.5-flash?x=1",
		"gemini-2.5-flash#x",
		"gemini%2F..",
		"gemini 2.5",
		"gemini\t2.5",
		"../cachedContents",
		"a/./b",
		"a//b",
		"a\\b",
		"gemini flash",
	} {
		if _, err := setGeminiRequest(t, "v1beta", "/"+model+":generateContent"); err == nil {
			t.Errorf("model %q: setRequest accepted it", model)
		}
	}

	for _, model := range []string{"gemini-2.5-flash", "ai21/jamba-large-1.7", "gemini-2.0-flash-001", "claude-3-5@20240620"} {
		if _, err := setGeminiRequest(t, "v1beta", "/"+model+":generateContent"); err != nil {
			t.Errorf("model %q: %v", model, err)
		}
	}
}
