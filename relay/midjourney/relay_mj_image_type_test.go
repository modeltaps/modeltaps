package midjourney

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

// SEC-89: only raster images are passed on, with a CSP that runs nothing when the image is
// opened at the top level; SVG (a scriptable document) is refused.
func TestRelayMidjourneyImageRasterOnly(t *testing.T) {
	setupMJTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a.svg":
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
		case "/a.webp":
			w.Header().Set("Content-Type", "IMAGE/WEBP; charset=binary")
			_, _ = w.Write([]byte("webp"))
		default:
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png"))
		}
	}))
	defer upstream.Close()

	base := upstream.URL
	if err := model.DB.Create(&model.Channel{Id: 3, Name: "mj", BaseURL: &base}).Error; err != nil {
		t.Fatal(err)
	}
	tasks := []model.Midjourney{
		{MjId: "svg", ChannelId: 3, ImageUrl: upstream.URL + "/a.svg"},
		{MjId: "webp", ChannelId: 3, ImageUrl: upstream.URL + "/a.webp"},
		{MjId: "png", ChannelId: 3, ImageUrl: upstream.URL + "/a.png"},
	}
	if err := model.DB.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}

	if w := serveMJImage(t, "svg"); w.Code == http.StatusOK || strings.Contains(w.Body.String(), "<script>") {
		t.Fatalf("SVG must not be relayed, got %d %q", w.Code, w.Body.String())
	}
	for id, wantType := range map[string]string{"webp": "image/webp", "png": "image/png"} {
		w := serveMJImage(t, id)
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != wantType {
			t.Fatalf("%s: got %d %q, want 200 %q", id, w.Code, w.Header().Get("Content-Type"), wantType)
		}
		if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "sandbox") {
			t.Fatalf("%s: Content-Security-Policy = %q, want default-src 'none' and sandbox", id, csp)
		}
	}
}

func TestMJImageContentType(t *testing.T) {
	for upstream, want := range map[string]string{
		"":                       "image/jpeg",
		"image/jpeg":             "image/jpeg",
		"image/gif":              "image/gif",
		"image/png; q=1":         "image/png",
		"image/svg+xml":          "",
		"image/svg+xml; a=b":     "",
		"text/html":              "",
		"image/x-icon":           "",
		"image/png\x00text/html": "",
	} {
		got, ok := mjImageContentType(upstream)
		if (want != "") != ok || got != want {
			t.Errorf("mjImageContentType(%q) = %q, %v; want %q", upstream, got, ok, want)
		}
	}
}
