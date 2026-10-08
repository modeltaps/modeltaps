package ali_test

import (
	"github.com/modeltaps/modeltaps/common/test"
	_ "github.com/modeltaps/modeltaps/common/test/init"
	"github.com/modeltaps/modeltaps/providers"
	providers_base "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func getChatProvider(url string, context *gin.Context) providers_base.ChatInterface {
	channel := getAliChannel(url)
	provider := providers.GetProvider(&channel, context)
	chatProvider, _ := provider.(providers_base.ChatInterface)

	return chatProvider
}

func TestChatCompletions(t *testing.T) {
	url, server, teardown := setupAliTestServer()
	context, _ := test.GetContext("POST", "/v1/chat/completions", test.RequestJSONConfig(), nil)
	defer teardown()
	server.RegisterHandler("/api/v1/services/aigc/text-generation/generation", handleChatCompletionEndpoint)

	chatRequest := test.GetChatCompletionRequest("default", "qwen-turbo", "false")

	chatProvider := getChatProvider(url, context)
	usage := &types.Usage{}
	chatProvider.SetUsage(usage)
	response, errWithCode := chatProvider.CreateChatCompletion(chatRequest)

	assert.Nil(t, errWithCode)
	assert.IsType(t, &types.Usage{}, usage)
	assert.Equal(t, 33, usage.TotalTokens)
	assert.Equal(t, 14, usage.PromptTokens)
	assert.Equal(t, 19, usage.CompletionTokens)

	// 转换成JSON字符串
	responseBody, err := json.Marshal(response)
	if err != nil {
		fmt.Println(err)
		assert.Fail(t, "json marshal error")
	}
	fmt.Println(string(responseBody))

	test.CheckChat(t, response, "qwen-turbo", usage)
}

func TestChatCompletionsError(t *testing.T) {
	url, server, teardown := setupAliTestServer()
	context, _ := test.GetContext("POST", "/v1/chat/completions", test.RequestJSONConfig(), nil)
	defer teardown()
	server.RegisterHandler("/api/v1/services/aigc/text-generation/generation", handleChatCompletionErrorEndpoint)

	chatRequest := test.GetChatCompletionRequest("default", "qwen-turbo", "false")

	chatProvider := getChatProvider(url, context)
	_, err := chatProvider.CreateChatCompletion(chatRequest)
	usage := chatProvider.GetUsage()

	assert.NotNil(t, err)
	assert.Nil(t, usage)
	assert.Equal(t, "InvalidParameter", err.Code)
}

func handleChatCompletionEndpoint(w http.ResponseWriter, r *http.Request) {
	// completions only accepts POST requests
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}

	response := `{"output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"您好！我可以帮您查询最近的公园，请问您现在所在的位置是哪里呢？"}}]},"usage":{"total_tokens":33,"output_tokens":19,"input_tokens":14},"request_id":"2479f818-9717-9b0b-9769-0d26e873a3f6"}`

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, response)
}

func handleChatCompletionErrorEndpoint(w http.ResponseWriter, r *http.Request) {
	// completions only accepts POST requests
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}

	response := `{"code":"InvalidParameter","message":"Role must be user or assistant and Content length must be greater than 0","request_id":"4883ee8d-f095-94ff-a94a-5ce0a94bc81f"}`

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, response)
}
