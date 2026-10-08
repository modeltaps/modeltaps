package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestContext returns a *gin.Context backed by a throwaway recorder.
// original_model is left unset so metrics.RecordProvider is a no-op (see metrics/main.go:77).
func newTestContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

// apiErr is a small helper to build an *types.OpenAIErrorWithStatusCode.
func apiErr(status int, local bool) *types.OpenAIErrorWithStatusCode {
	e := &types.OpenAIErrorWithStatusCode{StatusCode: status, LocalError: local}
	return e
}

func TestShouldRetry_StatusCodes(t *testing.T) {
	// channelType 0 => default branch of shouldRetryBadRequest (only relevant for 400 cases).
	tests := []struct {
		name        string
		apiErr      *types.OpenAIErrorWithStatusCode
		channelType int
		want        bool
	}{
		{"nil apiErr", nil, 0, false},
		{"429 too many requests", apiErr(http.StatusTooManyRequests, false), 0, true},
		{"307 temporary redirect", apiErr(http.StatusTemporaryRedirect, false), 0, true},
		{"408 request timeout", apiErr(http.StatusRequestTimeout, false), 0, false},
		{"500 internal server error", apiErr(http.StatusInternalServerError, false), 0, true},
		{"502 bad gateway", apiErr(http.StatusBadGateway, false), 0, true},
		{"503 service unavailable", apiErr(http.StatusServiceUnavailable, false), 0, true},
		{"200 ok", apiErr(http.StatusOK, false), 0, false},
		{"201 created", apiErr(http.StatusCreated, false), 0, false},
		{"401 unauthorized retries", apiErr(http.StatusUnauthorized, false), 0, true},
		{"403 forbidden retries", apiErr(http.StatusForbidden, false), 0, true},
		{"404 not found retries", apiErr(http.StatusNotFound, false), 0, true},
		{"local error never retries", apiErr(http.StatusInternalServerError, true), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestContext()
			got := shouldRetry(c, tt.apiErr, tt.channelType)
			if got != tt.want {
				t.Fatalf("shouldRetry(status=%v local) = %v, want %v",
					statusOf(tt.apiErr), got, tt.want)
			}
		})
	}
}

func TestShouldRetry_PinnedChannel(t *testing.T) {
	t.Run("pinned specific_channel_id (no ignore) does not retry", func(t *testing.T) {
		c := newTestContext()
		c.Set("specific_channel_id", 42)
		// specific_channel_id_ignore unset => false.
		if got := shouldRetry(c, apiErr(http.StatusInternalServerError, false), 0); got != false {
			t.Fatalf("pinned channel without ignore: got %v, want false", got)
		}
	})

	t.Run("pinned specific_channel_id with ignore=true still retries", func(t *testing.T) {
		c := newTestContext()
		c.Set("specific_channel_id", 42)
		c.Set("specific_channel_id_ignore", true)
		if got := shouldRetry(c, apiErr(http.StatusInternalServerError, false), 0); got != true {
			t.Fatalf("pinned channel with ignore: got %v, want true", got)
		}
	})

	t.Run("no pinned channel retries normally", func(t *testing.T) {
		c := newTestContext()
		// specific_channel_id unset => 0 => guard is skipped.
		if got := shouldRetry(c, apiErr(http.StatusInternalServerError, false), 0); got != true {
			t.Fatalf("no pinned channel: got %v, want true", got)
		}
	})

	t.Run("pinned channel id 0 is treated as unpinned", func(t *testing.T) {
		c := newTestContext()
		c.Set("specific_channel_id", 0)
		if got := shouldRetry(c, apiErr(http.StatusInternalServerError, false), 0); got != true {
			t.Fatalf("channel id 0: got %v, want true", got)
		}
	})
}

func TestShouldRetry_BadRequestRouting(t *testing.T) {
	// shouldRetry delegates 400 to shouldRetryBadRequest; confirm the wiring via *gin.Context.
	t.Run("anthropic 400 credit balance retries", func(t *testing.T) {
		c := newTestContext()
		e := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusBadRequest}
		e.OpenAIError.Message = "Your credit balance is too low to access the API."
		if got := shouldRetry(c, e, config.ChannelTypeAnthropic); got != true {
			t.Fatalf("anthropic 400 credit balance: got %v, want true", got)
		}
	})

	t.Run("anthropic 400 unrelated does not retry", func(t *testing.T) {
		c := newTestContext()
		e := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusBadRequest}
		e.OpenAIError.Message = "invalid request"
		if got := shouldRetry(c, e, config.ChannelTypeAnthropic); got != false {
			t.Fatalf("anthropic 400 unrelated: got %v, want false", got)
		}
	})
}

func TestShouldRetryBadRequest_Anthropic(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"exact credit balance phrase", "Your credit balance is too low", true},
		{"credit balance within longer message", "Error: Your credit balance is too low to continue.", true},
		{"unrelated 400", "The request body is malformed", false},
		{"empty message", "", false},
		{"case mismatch does not match (Contains is case-sensitive)", "your credit balance is too low", false},
		{"bedrock phrase on anthropic does not match", "Operation not allowed", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestContext()
			e := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusBadRequest}
			e.OpenAIError.Message = tt.message
			if got := shouldRetryBadRequest(c, config.ChannelTypeAnthropic, e); got != tt.want {
				t.Fatalf("anthropic message=%q: got %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}

func TestShouldRetryBadRequest_Bedrock(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"exact operation not allowed", "Operation not allowed", true},
		{"operation not allowed within longer message", "AccessDenied: Operation not allowed for this account", true},
		{"unrelated 400", "ValidationException: bad input", false},
		{"empty message", "", false},
		{"anthropic phrase on bedrock does not match", "Your credit balance is too low", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestContext()
			e := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusBadRequest}
			e.OpenAIError.Message = tt.message
			if got := shouldRetryBadRequest(c, config.ChannelTypeBedrock, e); got != tt.want {
				t.Fatalf("bedrock message=%q: got %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}

func TestShouldRetryBadRequest_DefaultGemini(t *testing.T) {
	// channelType 0 hits the default (gemini) branch: retry only when
	// Param == "INVALID_ARGUMENT" AND the (lowercased) message matches an api-key variant.
	tests := []struct {
		name        string
		channelType int
		param       string
		message     string
		want        bool
	}{
		{"invalid arg + api key not valid", 0, "INVALID_ARGUMENT", "API key not valid. Please pass a valid API key.", true},
		{"invalid arg + api key not found", 0, "INVALID_ARGUMENT", "API Key not found. Please pass a valid API key.", true},
		{"invalid arg + api key expired", 0, "INVALID_ARGUMENT", "API key expired. Please renew the API key.", true},
		{"invalid arg + already lowercase not valid", 0, "INVALID_ARGUMENT", "api key not valid", true},
		{"invalid arg + uppercase variant matches (case-insensitive)", 0, "INVALID_ARGUMENT", "API KEY EXPIRED", true},
		{"invalid arg but unrelated message", 0, "INVALID_ARGUMENT", "The request is invalid for some other reason", false},
		{"api key message but wrong param", 0, "", "API key not valid. Please pass a valid API key.", false},
		{"wrong param FAILED_PRECONDITION", 0, "FAILED_PRECONDITION", "API key expired. Please renew the API key.", false},
		{"empty everything", 0, "", "", false},
		// Confirm the branch selection is by channelType, not just value 0.
		{"unknown positive channelType uses default branch", 999, "INVALID_ARGUMENT", "API key not valid", true},
		{"unknown positive channelType unrelated message", 999, "INVALID_ARGUMENT", "some other error", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestContext()
			e := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusBadRequest}
			e.OpenAIError.Param = tt.param
			e.OpenAIError.Message = tt.message
			if got := shouldRetryBadRequest(c, tt.channelType, e); got != tt.want {
				t.Fatalf("default branch param=%q message=%q: got %v, want %v",
					tt.param, tt.message, got, tt.want)
			}
		})
	}
}

func TestIsModelNotFoundErr(t *testing.T) {
	tests := []struct {
		name    string
		apiErr  *types.OpenAIErrorWithStatusCode
		message string
		want    bool
	}{
		{"nil apiErr", nil, "", false},
		{"empty message", apiErr(http.StatusNotFound, false), "", false},
		{"openrouter no endpoints found", apiErr(http.StatusNotFound, false), "No endpoints found for anthropic/claude-3.5-haiku", true},
		{"openrouter no endpoints matching policy", apiErr(http.StatusNotFound, false), "No endpoints found matching your data policy", true},
		{"openai does not exist", apiErr(http.StatusNotFound, false), "The model `gpt-foo` does not exist or you do not have access to it.", true},
		{"generic model not found", apiErr(http.StatusBadRequest, false), "model not found", true},
		{"generic unknown model", apiErr(http.StatusBadRequest, false), "Unknown model: foo", true},
		{"case-insensitive NO ENDPOINTS FOUND", apiErr(http.StatusNotFound, false), "NO ENDPOINTS FOUND for foo", true},
		{"unrelated 5xx", apiErr(http.StatusBadGateway, false), "upstream returned 502 bad gateway", false},
		{"unrelated rate limit", apiErr(http.StatusTooManyRequests, false), "rate limit exceeded", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.apiErr != nil {
				tt.apiErr.OpenAIError.Message = tt.message
			}
			if got := isModelNotFoundErr(tt.apiErr); got != tt.want {
				t.Fatalf("isModelNotFoundErr(message=%q): got %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}

func TestFilterOpenAIErr_Collapse(t *testing.T) {
	// FilterOpenAIErr 的坍缩只在总开关开启时生效；保存并恢复全局开关，避免污染其他测试。
	prevWrap := config.ChannelFailErrorWrapEnabled
	config.ChannelFailErrorWrapEnabled = true
	defer func() { config.ChannelFailErrorWrapEnabled = prevWrap }()

	t.Run("model-not-found exempts 503 collapse -> 404 model_not_available", func(t *testing.T) {
		c := newTestContext()
		e := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusNotFound}
		e.OpenAIError.Message = "No endpoints found for anthropic/claude-3.5-haiku"
		got := FilterOpenAIErr(c, e)
		if got.StatusCode != http.StatusNotFound {
			t.Fatalf("status: got %d, want %d", got.StatusCode, http.StatusNotFound)
		}
		if code, _ := got.OpenAIError.Code.(string); code != "model_not_available" {
			t.Fatalf("code: got %v, want model_not_available", got.OpenAIError.Code)
		}
	})

	t.Run("model-not-found uses original_model in message", func(t *testing.T) {
		c := newTestContext()
		c.Set("original_model", "gpt-foo")
		e := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusNotFound}
		e.OpenAIError.Message = "The model does not exist"
		got := FilterOpenAIErr(c, e)
		if got.StatusCode != http.StatusNotFound {
			t.Fatalf("status: got %d, want %d", got.StatusCode, http.StatusNotFound)
		}
		if !strings.Contains(got.OpenAIError.Message, "gpt-foo") {
			t.Fatalf("message should contain model name: got %q", got.OpenAIError.Message)
		}
	})

	t.Run("generic 5xx still collapses to 503 service_unavailable", func(t *testing.T) {
		c := newTestContext()
		e := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusBadGateway}
		e.OpenAIError.Message = "upstream returned 502 bad gateway"
		got := FilterOpenAIErr(c, e)
		if got.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("status: got %d, want %d", got.StatusCode, http.StatusServiceUnavailable)
		}
		if code, _ := got.OpenAIError.Code.(string); code != "service_unavailable" {
			t.Fatalf("code: got %v, want service_unavailable", got.OpenAIError.Code)
		}
	})

	t.Run("429 preserved -> rate_limit_exceeded", func(t *testing.T) {
		c := newTestContext()
		e := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusTooManyRequests}
		e.OpenAIError.Message = "rate limit exceeded"
		got := FilterOpenAIErr(c, e)
		if got.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("status: got %d, want %d", got.StatusCode, http.StatusTooManyRequests)
		}
		if code, _ := got.OpenAIError.Code.(string); code != "rate_limit_exceeded" {
			t.Fatalf("code: got %v, want rate_limit_exceeded", got.OpenAIError.Code)
		}
	})

	t.Run("429 preservation wins over model-not-found (no regression)", func(t *testing.T) {
		c := newTestContext()
		c.Set("upstream_seen_429", true)
		e := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusNotFound}
		e.OpenAIError.Message = "No endpoints found for foo"
		got := FilterOpenAIErr(c, e)
		if got.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("status: got %d, want %d", got.StatusCode, http.StatusTooManyRequests)
		}
		if code, _ := got.OpenAIError.Code.(string); code != "rate_limit_exceeded" {
			t.Fatalf("code: got %v, want rate_limit_exceeded", got.OpenAIError.Code)
		}
	})

	t.Run("wrap disabled skips collapse (no 404 mapping)", func(t *testing.T) {
		config.ChannelFailErrorWrapEnabled = false
		defer func() { config.ChannelFailErrorWrapEnabled = true }()
		c := newTestContext()
		e := &types.OpenAIErrorWithStatusCode{StatusCode: http.StatusNotFound}
		e.OpenAIError.Message = "No endpoints found for foo"
		got := FilterOpenAIErr(c, e)
		if got.StatusCode != http.StatusNotFound {
			t.Fatalf("status: got %d, want %d (passthrough)", got.StatusCode, http.StatusNotFound)
		}
		if code, _ := got.OpenAIError.Code.(string); code == "model_not_available" {
			t.Fatalf("wrap disabled should not map to model_not_available")
		}
	})
}

func statusOf(e *types.OpenAIErrorWithStatusCode) any {
	if e == nil {
		return "nil"
	}
	return e.StatusCode
}

// TestGeminiSetRequest_SlashModel 覆盖含斜杠模型名的解析：*action 通配路由会带上前导斜杠，
// setRequest 需正确剥离并拆分 model:action；同时验证旧的 :model 单段参数路径不回归。
func TestGeminiSetRequest_SlashModel(t *testing.T) {
	tests := []struct {
		name       string
		params     gin.Params
		wantModel  string
		wantAction string
		wantStream bool
	}{
		{
			name:       "wildcard action with slash model",
			params:     gin.Params{{Key: "version", Value: "v1beta"}, {Key: "action", Value: "/ai21/jamba-large-1.7:generateContent"}},
			wantModel:  "ai21/jamba-large-1.7",
			wantAction: "generateContent",
			wantStream: false,
		},
		{
			name:       "wildcard action stream with slash model",
			params:     gin.Params{{Key: "version", Value: "v1beta"}, {Key: "action", Value: "/ai21/jamba-large-1.7:streamGenerateContent"}},
			wantModel:  "ai21/jamba-large-1.7",
			wantAction: "streamGenerateContent",
			wantStream: true,
		},
		{
			name:       "legacy :model single-segment param still parsed (no regression)",
			params:     gin.Params{{Key: "version", Value: "v1beta"}, {Key: "model", Value: "gemini-2.0-flash:generateContent"}},
			wantModel:  "gemini-2.0-flash",
			wantAction: "generateContent",
			wantStream: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestContext()
			c.Request = httptest.NewRequest(http.MethodPost, "/gemini/v1beta/models/x:generateContent", strings.NewReader(`{"contents":[]}`))
			c.Params = tt.params

			r := NewRelayGeminiOnly(c)
			if err := r.setRequest(); err != nil {
				t.Fatalf("setRequest returned error: %v", err)
			}
			if r.geminiRequest.Model != tt.wantModel {
				t.Fatalf("model: got %q want %q", r.geminiRequest.Model, tt.wantModel)
			}
			if r.geminiRequest.Action != tt.wantAction {
				t.Fatalf("action: got %q want %q", r.geminiRequest.Action, tt.wantAction)
			}
			if r.geminiRequest.Stream != tt.wantStream {
				t.Fatalf("stream: got %v want %v", r.geminiRequest.Stream, tt.wantStream)
			}
			if r.getOriginalModel() != tt.wantModel {
				t.Fatalf("originalModel: got %q want %q", r.getOriginalModel(), tt.wantModel)
			}
		})
	}
}

// TestFetchChannelByModel_ProtocolMismatchClassification 覆盖协议不匹配的错误分类：
//   - 模型在分组下只有非本协议类型的渠道（allow_channel_type 全过滤掉）→ model_not_found（404 语义）
//   - 模型有本协议类型渠道、只是被运行时过滤（skip_channel_ids）→ 运行时错误（503 语义），不误判为 model_not_found
func TestFetchChannelByModel_ProtocolMismatchClassification(t *testing.T) {
	prevRule := model.ChannelGroup.Rule
	prevChannels := model.ChannelGroup.Channels
	defer func() {
		model.ChannelGroup.Rule = prevRule
		model.ChannelGroup.Channels = prevChannels
	}()

	weight := uint(1)
	openaiChannel := &model.Channel{Id: 1, Type: config.ChannelTypeOpenAI, Weight: &weight, Status: config.ChannelStatusEnabled}
	geminiChannel := &model.Channel{Id: 2, Type: config.ChannelTypeGemini, Weight: &weight, Status: config.ChannelStatusEnabled}

	t.Run("openai-only model via gemini protocol -> model_not_found", func(t *testing.T) {
		model.ChannelGroup.Rule = map[string]map[string][][]int{"g": {"m": {{1}}}}
		model.ChannelGroup.Channels = map[int]*model.ChannelChoice{1: {Channel: openaiChannel}}

		c := newTestContext()
		c.Request = httptest.NewRequest(http.MethodPost, "/gemini/v1beta/models/m:generateContent", nil)
		c.Set("token_group", "g")
		c.Set("allow_channel_type", AllowGeminiChannelType)

		_, err := fetchChannelByModel(c, "m")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !IsModelNotFound(err) {
			t.Fatalf("expected IsModelNotFound=true, got err=%v", err)
		}
	})

	t.Run("model supports protocol but runtime-filtered -> runtime error, not model_not_found", func(t *testing.T) {
		model.ChannelGroup.Rule = map[string]map[string][][]int{"g": {"m": {{2}}}}
		model.ChannelGroup.Channels = map[int]*model.ChannelChoice{2: {Channel: geminiChannel}}

		c := newTestContext()
		c.Request = httptest.NewRequest(http.MethodPost, "/gemini/v1beta/models/m:generateContent", nil)
		c.Set("token_group", "g")
		c.Set("allow_channel_type", AllowGeminiChannelType)
		c.Set("skip_channel_ids", []int{2})

		_, err := fetchChannelByModel(c, "m")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if IsModelNotFound(err) {
			t.Fatalf("expected IsModelNotFound=false (runtime), got err=%v", err)
		}
		if !isRuntimeChannelErr(err) {
			t.Fatalf("expected isRuntimeChannelErr=true, got err=%v", err)
		}
	})
}

// TestFetchChannelByModel_ClaudeProtocolFiltering 覆盖白名单扩容后的 Claude 入口过滤口径：
//   - 非白名单渠道（Midjourney/Suno/Flux/Kling 等纯图像/音频/任务型）独占的模型 → model_not_found
//   - 新纳入白名单的对话渠道（Groq/Deepseek/Codex）独占的模型 → 正常选到该渠道
func TestFetchChannelByModel_ClaudeProtocolFiltering(t *testing.T) {
	prevRule := model.ChannelGroup.Rule
	prevChannels := model.ChannelGroup.Channels
	defer func() {
		model.ChannelGroup.Rule = prevRule
		model.ChannelGroup.Channels = prevChannels
	}()

	weight := uint(1)
	newClaudeCtx := func() *gin.Context {
		c := newTestContext()
		c.Request = httptest.NewRequest(http.MethodPost, "/claude/v1/messages", nil)
		c.Set("token_group", "g")
		c.Set("allow_channel_type", AllowChannelType)
		c.Set("prefer_channel_type", nativeClaudeChannelType)
		return c
	}
	setSingleChannel := func(channelType int) {
		model.ChannelGroup.Rule = map[string]map[string][][]int{"g": {"m": {{1}}}}
		model.ChannelGroup.Channels = map[int]*model.ChannelChoice{
			1: {Channel: &model.Channel{Id: 1, Type: channelType, Weight: &weight, Status: config.ChannelStatusEnabled}},
		}
	}

	excluded := map[string]int{
		"midjourney":  config.ChannelTypeMidjourney,
		"suno":        config.ChannelTypeSuno,
		"flux":        config.ChannelTypeFlux,
		"kling":       config.ChannelTypeKling,
		"stabilityai": config.ChannelTypeStabilityAI,
		"azurespeech": config.ChannelTypeAzureSpeech,
		"jina":        config.ChannelTypeJina,
		"rerank":      config.ChannelTypeRerank,
		"recraft":     config.ChannelTypeRecraft,
		"ideogram":    config.ChannelTypeIdeogram,
	}
	for name, channelType := range excluded {
		t.Run(name+"-only model via claude protocol -> model_not_found", func(t *testing.T) {
			setSingleChannel(channelType)
			_, err := fetchChannelByModel(newClaudeCtx(), "m")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !IsModelNotFound(err) {
				t.Fatalf("expected IsModelNotFound=true, got err=%v", err)
			}
		})
	}

	included := map[string]int{
		"groq":       config.ChannelTypeGroq,
		"deepseek":   config.ChannelTypeDeepseek,
		"codex":      config.ChannelTypeCodex,
		"bedrock":    config.ChannelTypeBedrock,
		"xai":        config.ChannelTypeXAI,
		"azurev1":    config.ChannelTypeAzureV1,
		"claudecode": config.ChannelTypeClaudeCode,
	}
	for name, channelType := range included {
		t.Run(name+"-only model via claude protocol -> routable", func(t *testing.T) {
			setSingleChannel(channelType)
			ch, err := fetchChannelByModel(newClaudeCtx(), "m")
			if err != nil {
				t.Fatalf("expected channel, got err=%v", err)
			}
			if ch.Id != 1 || ch.Type != channelType {
				t.Fatalf("got channel id=%d type=%d, want id=1 type=%d", ch.Id, ch.Type, channelType)
			}
		})
	}
}
