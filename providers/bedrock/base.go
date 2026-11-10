package bedrock

import (
	"fmt"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/providers/bedrock/awscreds"
	"github.com/modeltaps/modeltaps/providers/bedrock/category"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type BedrockProviderFactory struct{}

// 创建 BedrockProvider
func (f BedrockProviderFactory) Create(channel *model.Channel) base.ProviderInterface {

	bedrockProvider := &BedrockProvider{
		BaseProvider: base.BaseProvider{
			Config:  getConfig(),
			Channel: channel,
			// Requester 仅用于复用 NewRequestWithCustomParams* 的请求体构造逻辑；
			// 实际发送由 bedrockruntime SDK 完成，故不需要错误处理回调。
			Requester: requester.NewHTTPRequester(channel.GetProxy(), nil),
		},
	}

	getKeyConfig(bedrockProvider)

	return bedrockProvider
}

type BedrockProvider struct {
	base.BaseProvider
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	// RoleArn, when set, makes the provider assume the role with the AK/SK and sign with the
	// temporary credentials (see the awscreds package).
	RoleArn  string
	APIToken string
	Category *category.Category
	client   *bedrockruntime.Client
	// errBody 缓存最近一次上游错误响应的原始 body（由 captureResponseMiddleware 在
	// Deserialize 前 tee 出来）。用于在 SDK 反序列化失败（如中间层返回 HTML）时，
	// 让 awsErrorToOpenAI 拿到上游真实返回而非 SDK 的解析器噪声。
	errBody []byte
}

func getConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:         "https://bedrock-runtime.%s.amazonaws.com",
		ChatCompletions: "/model/%s/invoke",
	}
}

func (p *BedrockProvider) GetRequestHeaders() (headers map[string]string) {
	headers = make(map[string]string)
	p.CommonRequestHeaders(headers)
	headers["Accept"] = "*/*"

	return headers
}

// getKeyConfig parses channel.Key; the formats are documented on awscreds.ParseKey.
func getKeyConfig(bedrock *BedrockProvider) {
	cfg, ok := awscreds.ParseKey(bedrock.Channel.Key)
	if !ok {
		// A malformed key must not be swallowed: the channel would look configured while the
		// region and credentials stay empty, and requests would only end in signing errors that
		// are hard to trace back to the key.
		logger.SysError(fmt.Sprintf("[%s] invalid aws key format, expect Region|AK|SK[|SessionToken[|RoleArn]] or Region|BearerToken", bedrock.Channel.Name))
		return
	}
	bedrock.Region = cfg.Region
	bedrock.APIToken = cfg.APIToken
	bedrock.AccessKeyID = cfg.AccessKeyID
	bedrock.SecretAccessKey = cfg.SecretAccessKey
	bedrock.SessionToken = cfg.SessionToken
	bedrock.RoleArn = cfg.RoleArn
}

// credsConfig assembles the credential configuration for the awscreds package.
func (p *BedrockProvider) credsConfig() awscreds.Config {
	return awscreds.Config{
		Region:          p.Region,
		AccessKeyID:     p.AccessKeyID,
		SecretAccessKey: p.SecretAccessKey,
		SessionToken:    p.SessionToken,
		RoleArn:         p.RoleArn,
	}
}

// awsResponseHeaderExcluded 是禁止透传的响应头（传输层 / 由下游自行设置）。
// 透传这些会与本项目自己写的响应头冲突或破坏分块传输。
//
// 注意：当前 filterAWSResponseHeaders 的白名单已限定为 x-amzn-* / apigw-requestid，
// 这些传输层头不会命中该白名单，故此排除集当前是冗余的防御性兜底——仅在未来放宽
// 白名单（如加入 x-amz-* 前缀）时才真正生效，防止误透传传输层头。
var awsResponseHeaderExcluded = map[string]struct{}{
	"content-length":    {},
	"content-type":      {},
	"content-encoding":  {},
	"transfer-encoding": {},
	"connection":        {},
	"keep-alive":        {},
}

// filterAWSResponseHeaders 从上游 Bedrock 响应头中挑出可透传给客户端的 AWS 指纹头，
// 目的是让中转的响应看起来像直连 AWS（携带 x-amzn-requestid /
// x-amzn-bedrock-input-token-count / x-amzn-bedrock-output-token-count / apigw-requestid 等）。
// 只透传 x-amzn- 前缀及 apigw-requestid，其余（包括 x-amz- 非 amzn 前缀）暂不透传，避免误伤。
func filterAWSResponseHeaders(src http.Header) http.Header {
	if len(src) == 0 {
		return nil
	}
	out := http.Header{}
	for name, values := range src {
		lower := strings.ToLower(name)
		if _, excluded := awsResponseHeaderExcluded[lower]; excluded {
			continue
		}
		if strings.HasPrefix(lower, "x-amzn-") || lower == "apigw-requestid" {
			for _, v := range values {
				out.Add(name, v)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
