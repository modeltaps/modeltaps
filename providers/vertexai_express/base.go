package vertexai_express

import (
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/gemini"
	"github.com/modeltaps/modeltaps/providers/openai"
	"github.com/modeltaps/modeltaps/types"
	"fmt"
	"net/http"
	"strings"
)

type VertexAIExpressProviderFactory struct{}

// 创建 VertexAIExpressProvider
func (f VertexAIExpressProviderFactory) Create(channel *model.Channel) base.ProviderInterface {
	config := getConfig()
	// 从 Other 字段解析 Region 和 ProjectID
	region, projectID := parseOtherConfig(channel.Other)
	if region == "" {
		region = "us-central1"
	}

	// The base URL is built from the region, which becomes part of the host, so a value that is not a plain label
	// (it could name another host) leaves the base URL empty and the requests are rejected.
	if base.IsHostLabel(region) {
		config.BaseURL = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1beta1/projects/%s/locations/%s/publishers/google/models",
			region, projectID, region)
	} else {
		logger.SysError(fmt.Sprintf("vertexai express channel %d: region must contain only lowercase letters, digits and hyphens", channel.Id))
	}

	proxyAddr := channel.GetProxy()

	return &VertexAIExpressProvider{
		GeminiProvider: gemini.GeminiProvider{
			OpenAIProvider: openai.OpenAIProvider{
				BaseProvider: base.BaseProvider{
					Config:    config,
					Channel:   channel,
					Requester: requester.NewHTTPRequester(proxyAddr, gemini.RequestErrorHandle(channel.Key)),
				},
				SupportStreamOptions: true,
			},
			UseOpenaiAPI:     false,
			UseCodeExecution: false,
		},
		Region:    region,
		ProjectID: projectID,
	}
}

type VertexAIExpressProvider struct {
	gemini.GeminiProvider
	Region    string
	ProjectID string
}

func getConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:           "", // 动态设置
		ChatCompletions:   "/",
		ModelList:         "/models",
		ImagesGenerations: "1",
	}
}

// parseOtherConfig 解析 Other 字段，格式为 "Region|ProjectID"
func parseOtherConfig(other string) (region, projectID string) {
	parts := strings.Split(other, "|")
	if len(parts) >= 2 {
		region = parts[0]
		projectID = parts[1]
	} else if len(parts) == 1 {
		// 只有一个值时作为 ProjectID
		projectID = parts[0]
	}
	return
}

// GetFullRequestURL 构建完整的请求 URL
// It returns "" when the channel has no usable base URL (see Create).
func (p *VertexAIExpressProvider) GetFullRequestURL(requestURL string, modelName string) string {
	baseURL := strings.TrimSuffix(p.GetBaseURL(), "/")
	if baseURL == "" {
		return ""
	}

	// 构建 URL: baseURL/modelName:action
	actionURL := requestURL
	if actionURL == "" {
		actionURL = "generateContent"
	}

	return fmt.Sprintf("%s/%s:%s", baseURL, modelName, actionURL)
}

// GetRequestHeaders 获取请求头
// The key goes in the x-goog-api-key header, the way Google Cloud recommends, and not in the
// ?key= query: a URL ends up in transport errors, logs and proxies.
func (p *VertexAIExpressProvider) GetRequestHeaders() (headers map[string]string) {
	headers = make(map[string]string)
	p.CommonRequestHeaders(headers)
	headers["x-goog-api-key"] = p.Channel.Key
	return headers
}

// RequestErrorHandle 请求错误处理
func RequestErrorHandle(resp *http.Response) *types.OpenAIError {
	return gemini.RequestErrorHandle("")(resp)
}
