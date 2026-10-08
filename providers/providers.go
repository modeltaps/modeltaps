package providers

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/ali"
	"github.com/modeltaps/modeltaps/providers/antigravity"
	"github.com/modeltaps/modeltaps/providers/azure"
	azurespeech "github.com/modeltaps/modeltaps/providers/azureSpeech"
	"github.com/modeltaps/modeltaps/providers/azure_v1"
	"github.com/modeltaps/modeltaps/providers/azuredatabricks"
	"github.com/modeltaps/modeltaps/providers/baichuan"
	"github.com/modeltaps/modeltaps/providers/baidu"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/bedrock"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/providers/claudecode"
	"github.com/modeltaps/modeltaps/providers/cloudflareAI"
	"github.com/modeltaps/modeltaps/providers/codex"
	"github.com/modeltaps/modeltaps/providers/cohere"
	"github.com/modeltaps/modeltaps/providers/coze"
	"github.com/modeltaps/modeltaps/providers/deepseek"
	"github.com/modeltaps/modeltaps/providers/gemini"
	"github.com/modeltaps/modeltaps/providers/geminicli"
	"github.com/modeltaps/modeltaps/providers/github"
	"github.com/modeltaps/modeltaps/providers/groq"
	"github.com/modeltaps/modeltaps/providers/hunyuan"
	"github.com/modeltaps/modeltaps/providers/jina"
	"github.com/modeltaps/modeltaps/providers/lingyi"
	"github.com/modeltaps/modeltaps/providers/midjourney"
	"github.com/modeltaps/modeltaps/providers/minimax"
	"github.com/modeltaps/modeltaps/providers/mistral"
	"github.com/modeltaps/modeltaps/providers/moonshot"
	"github.com/modeltaps/modeltaps/providers/ollama"
	"github.com/modeltaps/modeltaps/providers/openai"
	"github.com/modeltaps/modeltaps/providers/openrouter"
	"github.com/modeltaps/modeltaps/providers/palm"
	"github.com/modeltaps/modeltaps/providers/recraftAI"
	"github.com/modeltaps/modeltaps/providers/replicate"
	"github.com/modeltaps/modeltaps/providers/siliconflow"
	"github.com/modeltaps/modeltaps/providers/stabilityAI"
	"github.com/modeltaps/modeltaps/providers/suno"
	"github.com/modeltaps/modeltaps/providers/tencent"
	"github.com/modeltaps/modeltaps/providers/vertexai"
	vertexai_express "github.com/modeltaps/modeltaps/providers/vertexai_express"
	"github.com/modeltaps/modeltaps/providers/xAI"
	"github.com/modeltaps/modeltaps/providers/xunfei"
	"github.com/modeltaps/modeltaps/providers/zhipu"

	"github.com/gin-gonic/gin"
)

// 定义供应商工厂接口
type ProviderFactory interface {
	Create(Channel *model.Channel) base.ProviderInterface
}

// 创建全局的供应商工厂映射
var providerFactories = make(map[int]ProviderFactory)

// 在程序启动时，添加所有的供应商工厂
func init() {
	providerFactories = map[int]ProviderFactory{
		config.ChannelTypeOpenAI:          openai.OpenAIProviderFactory{},
		config.ChannelTypeAzure:           azure.AzureProviderFactory{},
		config.ChannelTypeAli:             ali.AliProviderFactory{},
		config.ChannelTypeTencent:         tencent.TencentProviderFactory{},
		config.ChannelTypeBaidu:           baidu.BaiduProviderFactory{},
		config.ChannelTypeAnthropic:       claude.ClaudeProviderFactory{},
		config.ChannelTypePaLM:            palm.PalmProviderFactory{},
		config.ChannelTypeZhipu:           zhipu.ZhipuProviderFactory{},
		config.ChannelTypeXunfei:          xunfei.XunfeiProviderFactory{},
		config.ChannelTypeAzureSpeech:     azurespeech.AzureSpeechProviderFactory{},
		config.ChannelTypeGemini:          gemini.GeminiProviderFactory{},
		config.ChannelTypeBaichuan:        baichuan.BaichuanProviderFactory{},
		config.ChannelTypeMiniMax:         minimax.MiniMaxProviderFactory{},
		config.ChannelTypeDeepseek:        deepseek.DeepseekProviderFactory{},
		config.ChannelTypeMistral:         mistral.MistralProviderFactory{},
		config.ChannelTypeGroq:            groq.GroqProviderFactory{},
		config.ChannelTypeBedrock:         bedrock.BedrockProviderFactory{},
		config.ChannelTypeMidjourney:      midjourney.MidjourneyProviderFactory{},
		config.ChannelTypeCloudflareAI:    cloudflareAI.CloudflareAIProviderFactory{},
		config.ChannelTypeCohere:          cohere.CohereProviderFactory{},
		config.ChannelTypeStabilityAI:     stabilityAI.StabilityAIProviderFactory{},
		config.ChannelTypeCoze:            coze.CozeProviderFactory{},
		config.ChannelTypeOllama:          ollama.OllamaProviderFactory{},
		config.ChannelTypeMoonshot:        moonshot.MoonshotProviderFactory{},
		config.ChannelTypeLingyi:          lingyi.LingyiProviderFactory{},
		config.ChannelTypeHunyuan:         hunyuan.HunyuanProviderFactory{},
		config.ChannelTypeSuno:            suno.SunoProviderFactory{},
		config.ChannelTypeVertexAI:        vertexai.VertexAIProviderFactory{},
		config.ChannelTypeSiliconflow:     siliconflow.SiliconflowProviderFactory{},
		config.ChannelTypeJina:            jina.JinaProviderFactory{},
		config.ChannelTypeGithub:          github.GithubProviderFactory{},
		config.ChannelTypeRecraft:         recraftAI.RecraftProviderFactory{},
		config.ChannelTypeReplicate:       replicate.ReplicateProviderFactory{},
		config.ChannelTypeOpenRouter:      openrouter.OpenRouterProviderFactory{},
		config.ChannelTypeAzureDatabricks: azuredatabricks.AzureDatabricksProviderFactory{},
		config.ChannelTypeAzureV1:         azure_v1.AzureV1ProviderFactory{},
		config.ChannelTypeXAI:             xAI.XAIProviderFactory{},
		config.ChannelTypeGeminiCli:       geminicli.GeminiCliProviderFactory{},
		config.ChannelTypeClaudeCode:      claudecode.ClaudeCodeProviderFactory{},
		config.ChannelTypeCodex:           codex.CodexProviderFactory{},
		config.ChannelTypeAntigravity:     antigravity.AntigravityProviderFactory{},
		config.ChannelTypeVertexAIExpress: vertexai_express.VertexAIExpressProviderFactory{},
	}
}

// 获取供应商
func GetProvider(channel *model.Channel, c *gin.Context) base.ProviderInterface {
	factory, ok := providerFactories[channel.Type]
	var provider base.ProviderInterface
	if !ok {
		// 处理未找到的供应商工厂
		baseURL := channel.GetBaseURL()
		if baseURL == "" {
			return nil
		}

		provider = openai.CreateOpenAIProvider(channel, baseURL)
	} else {
		provider = factory.Create(channel)
	}
	provider.SetContext(c)

	return provider
}
