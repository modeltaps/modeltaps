package requesty

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"gorm.io/datatypes"
)

func requestyChannel(plugin model.PluginType, customParameter string) *model.Channel {
	channel := &model.Channel{Type: config.ChannelTypeRequesty, Key: "rq-key"}
	if plugin != nil {
		p := datatypes.NewJSONType(plugin)
		channel.Plugin = &p
	}
	if customParameter != "" {
		channel.CustomParameter = &customParameter
	}
	return channel
}

func customParams(t *testing.T, channel *model.Channel) map[string]interface{} {
	t.Helper()
	params, err := RequestyProviderFactory{}.Create(channel).(*RequestyProvider).CustomParameterHandler()
	if err != nil {
		t.Fatal(err)
	}
	return params
}

func TestAutoCacheIsOnByDefault(t *testing.T) {
	want := map[string]interface{}{"requesty": map[string]interface{}{"auto_cache": true}}
	if got := customParams(t, requestyChannel(nil, "")); !reflect.DeepEqual(got, want) {
		t.Fatalf("params = %v, want %v", got, want)
	}
	on := requestyChannel(model.PluginType{"requesty": {"auto_cache": "on"}}, `{"temperature": 0.2}`)
	if got := customParams(t, on); got["requesty"] == nil || got["temperature"] != 0.2 {
		t.Fatalf("defaults should sit next to the channel's own parameters, got %v", got)
	}
}

func TestAutoCacheSwitchedOff(t *testing.T) {
	off := requestyChannel(model.PluginType{"requesty": {"auto_cache": "off"}}, "")
	if got := customParams(t, off); got != nil {
		t.Fatalf("params = %v, want none", got)
	}
}

func TestChannelParametersWinOverAutoCache(t *testing.T) {
	own := requestyChannel(nil, `{"requesty": {"auto_cache": false}}`)
	want := map[string]interface{}{"requesty": map[string]interface{}{"auto_cache": false}}
	if got := customParams(t, own); !reflect.DeepEqual(got, want) {
		t.Fatalf("params = %v, want %v", got, want)
	}
	perModel := requestyChannel(nil, `{"per_model": true, "claude-x": {"top_p": 1}}`)
	if got := customParams(t, perModel); got["requesty"] != nil {
		t.Fatalf("per-model parameters take no defaults, got %v", got)
	}
}

func TestMessagesRequestsTakeNoAutoCache(t *testing.T) {
	p := RequestyProviderFactory{}.Create(requestyChannel(nil, "")).(*RequestyProvider)
	params, err := p.claudeProvider().CustomParameterHandler()
	if err != nil || params != nil {
		t.Fatalf("messages params = %v, %v; want none", params, err)
	}
}

func TestChatBodyCarriesAutoCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	p := RequestyProviderFactory{}.Create(requestyChannel(nil, "")).(*RequestyProvider)
	p.SetContext(ctx)
	request := &types.ChatCompletionRequest{Model: "claude-x", Messages: []types.ChatCompletionMessage{{Role: "user", Content: "hi"}}}
	req, errWithCode := p.GetRequestTextBody(config.RelayModeChatCompletions, request.Model, request)
	if errWithCode != nil {
		t.Fatal(errWithCode)
	}
	body, _ := io.ReadAll(req.Body)
	if !gjson.GetBytes(body, "requesty.auto_cache").Bool() || gjson.GetBytes(body, "model").String() != "claude-x" {
		t.Fatalf("body = %s", body)
	}
}
