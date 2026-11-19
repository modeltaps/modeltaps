package requesty

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

func paramsWithMode(t *testing.T, channel *model.Channel, mode string) map[string]interface{} {
	t.Helper()
	p := RequestyProviderFactory{}.Create(channel).(*RequestyProvider)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(config.GinPromptCacheModeKey, mode)
	p.Context = c
	params, err := p.CustomParameterHandler()
	if err != nil {
		t.Fatal(err)
	}
	return params
}

func TestAutoCacheFollowsRequestMode(t *testing.T) {
	off := requestyChannel(model.PluginType{"requesty": {"auto_cache": "off"}}, "")
	if got := paramsWithMode(t, off, "on"); got["requesty"] == nil {
		t.Fatalf("key on over a channel switched off: params = %v, want auto_cache", got)
	}
	if got := paramsWithMode(t, requestyChannel(nil, ""), "off"); got != nil {
		t.Fatalf("key off: params = %v, want none", got)
	}
	if got := paramsWithMode(t, off, "auto"); got != nil {
		t.Fatalf("auto follows the channel: params = %v, want none", got)
	}
}
