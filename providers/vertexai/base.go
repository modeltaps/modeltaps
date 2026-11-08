package vertexai

import (
	"bytes"
	"context"
	"crypto/rand"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/vertexai/category"
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const TokenCacheKey = "api_token:vertexai"
const defaultScope = "https://www.googleapis.com/auth/cloud-platform"

type VertexAIProviderFactory struct{}

// 创建 VertexAIProvider
func (f VertexAIProviderFactory) Create(channel *model.Channel) base.ProviderInterface {
	proxyAddr := channel.GetProxy()

	vertexAIProvider := &VertexAIProvider{
		BaseProvider: base.BaseProvider{
			Config:    getConfig(),
			Channel:   channel,
			Requester: requester.NewHTTPRequester(proxyAddr, nil),
		},
	}

	getKeyConfig(vertexAIProvider)
	return vertexAIProvider
}

type VertexAIProvider struct {
	base.BaseProvider
	Region    string
	ProjectID string
	Category  *category.Category
}

func getConfig() base.ProviderConfig {
	return base.ProviderConfig{
		// The placeholder count and order are described by vertexURLPlaceholders.
		BaseURL:           "https://%s/v1/projects/%s/locations/%s/publishers/%s/models/%s:%s",
		ChatCompletions:   "/",
		ImagesGenerations: "/predict",
	}
}

func getKeyConfig(vertexAI *VertexAIProvider) {
	keys := strings.Split(vertexAI.Channel.Other, "|")
	if len(keys) < 2 {
		return
	}

	vertexAI.ProjectID = keys[len(keys)-1]

	regions := keys[:len(keys)-1]
	if len(regions) == 0 {
		return
	}

	randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(regions))))
	if err != nil {
		// 如果随机数生成失败，使用第一个region作为fallback
		logger.SysError("Failed to generate random number for region selection: " + err.Error())
		vertexAI.Region = regions[0]
		return
	}

	vertexAI.Region = regions[randomIndex.Int64()]
}

// vertexHost picks the Vertex AI service host for a region:
//   - global              -> aiplatform.googleapis.com
//   - us / eu multi-region -> aiplatform.{us|eu}.rep.googleapis.com
//   - any other region    -> {region}-aiplatform.googleapis.com
func vertexHost(region string) string {
	switch region {
	case "global":
		return "aiplatform.googleapis.com"
	case "us", "eu":
		return "aiplatform." + region + ".rep.googleapis.com"
	default:
		return region + "-aiplatform.googleapis.com"
	}
}

const (
	// vertexURLPlaceholders is the number of %s in a base_url template:
	// host, project, region, publisher, model, action.
	vertexURLPlaceholders = 6
	// legacyVertexURLPlaceholders is the number of %s in the template shape used before the
	// publisher became a placeholder: host prefix, project, region, model, action. The
	// publisher was fixed to google, so such a template can only serve Gemini models.
	legacyVertexURLPlaceholders = 5
)

// GetFullRequestURL builds the request URL from the publisher of the current Category.
// With no Category set (for example Imagen image generation) the publisher is google.
// It returns "" when a custom base_url cannot be used for this request.
func (p *VertexAIProvider) GetFullRequestURL(modelName string, other string) string {
	publisher := category.PublisherGoogle
	if p.Category != nil && p.Category.Publisher != "" {
		publisher = p.Category.Publisher
	}

	tpl := p.GetBaseURL()
	switch strings.Count(tpl, "%s") {
	case vertexURLPlaceholders:
		return fmt.Sprintf(tpl, vertexHost(p.Region), p.ProjectID, p.Region, publisher, modelName, other)
	case legacyVertexURLPlaceholders:
		if publisher == category.PublisherGoogle {
			hostPrefix := ""
			if p.Region != "global" {
				hostPrefix = p.Region + "-"
			}
			return fmt.Sprintf(tpl, hostPrefix, p.ProjectID, p.Region, modelName, other)
		}
	}

	p.warnInvalidBaseURL(publisher)
	return ""
}

// invalidBaseURLWarned remembers the channels already warned about an unusable base_url so the
// log is not repeated for every request.
var invalidBaseURLWarned sync.Map

// warnInvalidBaseURL logs once per channel and publisher. A custom base_url that does not fit
// the template is rejected instead of silently falling back to the built-in one: a channel
// configured with a proxy would otherwise start sending traffic straight to Google. The
// template itself is not logged because a proxy URL may carry credentials.
func (p *VertexAIProvider) warnInvalidBaseURL(publisher string) {
	var channelID int
	if p.Channel != nil {
		channelID = p.Channel.Id
	}
	key := fmt.Sprintf("%d:%s", channelID, publisher)
	if _, warned := invalidBaseURLWarned.LoadOrStore(key, struct{}{}); warned {
		return
	}
	logger.SysError(fmt.Sprintf("vertexai channel %d: custom base_url must contain %d %%s placeholders (host/project/region/publisher/model/action) to serve publisher %s, request rejected",
		channelID, vertexURLPlaceholders, publisher))
}

func (p *VertexAIProvider) GetRequestHeaders() (headers map[string]string) {
	headers, _ = p.getRequestHeadersInternal()
	return headers
}

// getRequestHeadersInternal 内部方法，返回请求头和错误信息
func (p *VertexAIProvider) getRequestHeadersInternal() (headers map[string]string, err error) {
	headers = make(map[string]string)
	p.CommonRequestHeaders(headers)

	token, err := p.GetToken()
	if err != nil {
		logger.SysError("Failed to get token: " + err.Error())
		return nil, err
	}

	headers["Authorization"] = "Bearer " + token
	return headers, nil
}

// handleTokenError 处理token获取失败的错误，检查是否匹配禁用通道关键词
func (p *VertexAIProvider) handleTokenError(err error) *types.OpenAIErrorWithStatusCode {
	errMsg := err.Error()

	// 检查是否匹配禁用通道关键词
	if common.DisableChannelKeywordsInstance.IsContains(errMsg) {
		// 匹配关键词，返回非LocalError，允许重试
		return common.StringErrorWrapper(errMsg, "vertexai_token_error", http.StatusInternalServerError)
	} else {
		// 不匹配关键词，返回LocalError，保持原有行为
		return common.StringErrorWrapperLocal(errMsg, "vertexai_token_error", http.StatusInternalServerError)
	}
}

func (p *VertexAIProvider) GetToken() (string, error) {
	cacheKey := fmt.Sprintf("%s:%d", TokenCacheKey, p.Channel.Id)
	token, err := cache.GetCache[string](cacheKey)
	if err != nil {
		logger.SysError("Failed to get token from cache: " + err.Error())
	}

	if token != "" {
		return token, nil
	}

	config, err := google.JWTConfigFromJSON([]byte(p.Channel.Key), defaultScope)
	if err != nil {
		return "", fmt.Errorf("failed to parse credentials: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	httpClient, err := utils.NewProxyHTTPClient(p.Channel.GetProxy())
	if err != nil {
		return "", fmt.Errorf("failed to create proxy http client: %w", err)
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	tok, err := config.TokenSource(ctx).Token()
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	duration := time.Until(tok.Expiry) - 5*time.Minute
	if duration <= 0 {
		duration = 30 * time.Second
	}
	cache.SetCache(cacheKey, tok.AccessToken, duration)

	return tok.AccessToken, nil
}

// RequestErrorHandle gives the category's own error handler (Claude / Gemini error bodies) the
// first look, then falls back to Vertex's own {"error":{...}} shape. projectID is redacted from
// the message.
func RequestErrorHandle(otherErr requester.HttpErrorHandler, projectID string) requester.HttpErrorHandler {

	return func(resp *http.Response) *types.OpenAIError {
		requestBody, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(requestBody))

		if otherErr != nil {
			err := otherErr(resp)
			if err != nil {
				err.Message = redactProjectID(err.Message, projectID)
				return err
			}
		}
		vertexaiErrors := &VertexaiErrors{}
		if err := json.Unmarshal(requestBody, vertexaiErrors); err == nil && len(*vertexaiErrors) > 0 {
			if vertexaiError := vertexaiErrors.Error(); vertexaiError != nil {
				return errorHandle(vertexaiError, projectID)
			}
		} else {
			vertexaiError := &VertexaiError{}
			if err := json.Unmarshal(requestBody, vertexaiError); err == nil {
				return errorHandle(vertexaiError, projectID)
			}
		}

		return nil
	}
}

// errorHandle turns a Vertex error body into an OpenAI error. Like the Gemini and Claude
// channels it surfaces the real upstream message instead of a fixed text, so a model that is
// not enabled, an unsupported region or missing permissions can be diagnosed. The project ID
// is replaced (Vertex errors often contain projects/{id}/locations/... paths) and base64 data
// is truncated. URL and host masking is left to relay.FilterOpenAIErr.
func errorHandle(vertexaiError *VertexaiError, projectID string) *types.OpenAIError {
	if vertexaiError.Error.Message == "" {
		return nil
	}

	message := redactProjectID(utils.TruncateBase64InMessage(vertexaiError.Error.Message), projectID)

	logger.SysError(fmt.Sprintf("VertexAI error: %s", message))

	return &types.OpenAIError{
		Message: message,
		Type:    "vertexai_error",
		Param:   vertexaiError.Error.Status,
		Code:    vertexaiError.Error.Code,
	}
}

// redactProjectID replaces the GCP project ID in an upstream error message.
func redactProjectID(message, projectID string) string {
	if projectID == "" {
		return message
	}
	return strings.ReplaceAll(message, projectID, "xxxxx")
}
