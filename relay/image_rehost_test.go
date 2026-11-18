package relay

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/safefetch"
	"github.com/modeltaps/modeltaps/types"
	"github.com/spf13/viper"
)

func TestRehostUpstreamImages(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/broken.png" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(buf.Bytes())
	}))
	defer upstream.Close()
	restore := safefetch.SetAllowAddrForTest(func(netip.AddrPort) bool { return true })
	defer restore()

	viper.Set("media_dir", t.TempDir())
	savedAddr, savedProvider, savedRehost := config.ServerAddress, config.ImageStorageProvider, config.ImageStorageRehostUpstream
	config.ServerAddress, config.ImageStorageProvider, config.ImageStorageRehostUpstream = "https://gw.example.com", "local", true
	t.Cleanup(func() {
		viper.Set("media_dir", nil)
		config.ServerAddress, config.ImageStorageProvider, config.ImageStorageRehostUpstream = savedAddr, savedProvider, savedRehost
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Set(config.GinRawResponseBodyKey, []byte(`{"raw":true}`))

	resp := &types.ImageResponse{Data: []types.ImageResponseDataInner{
		{URL: upstream.URL + "/a.png"},
		{URL: upstream.URL + "/broken.png"},
		{B64JSON: "AAAA"},
		{URL: "https://gw.example.com/media/own.png"},
	}}
	rehostUpstreamImages(c, nil, resp)

	if !strings.HasPrefix(resp.Data[0].URL, "https://gw.example.com/media/") {
		t.Fatalf("first image not moved: %s", resp.Data[0].URL)
	}
	if resp.Data[1].URL != upstream.URL+"/broken.png" {
		t.Fatalf("an unreachable link must be passed on, got %s", resp.Data[1].URL)
	}
	if resp.Data[3].URL != "https://gw.example.com/media/own.png" {
		t.Fatalf("own link touched: %s", resp.Data[3].URL)
	}
	if raw, _ := c.Get(config.GinRawResponseBodyKey); len(raw.([]byte)) != 0 {
		t.Fatal("raw upstream bytes must be dropped once a link changed")
	}

	// Switched off: nothing is fetched.
	config.ImageStorageRehostUpstream = false
	off := &types.ImageResponse{Data: []types.ImageResponseDataInner{{URL: upstream.URL + "/a.png"}}}
	rehostUpstreamImages(c, nil, off)
	if off.Data[0].URL != upstream.URL+"/a.png" {
		t.Fatal("rehosted while switched off")
	}
}
