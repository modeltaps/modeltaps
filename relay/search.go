package relay

import (
	"github.com/modeltaps/modeltaps/common/search"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/relay/relay_util"
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// From https://github.com/deepseek-ai/DeepSeek-R1?tab=readme-ov-file#official-prompts
const search_template = `# The following are search results based on the user's message:
%s
In the search results I provide, each result is in the format [webpage X begin]...[webpage X end], where X is the numeric index of the article. Your output must strictly follow markdown format, and you should cite the context in the corresponding parts of the answer. If a sentence is derived from multiple contexts, list all relevant citation numbers, for example [【1】](url)[【5】](url). Do not collect the citations at the end; list them in the corresponding parts of the answer. The answer ends with a reference list in the following format:
[【1】 Title](url)
[【2】 Title](url)
[【3】 Title](url)
When answering, keep the following in mind:
- Today is %s.
- Not all search results are closely related to the user's question; you need to evaluate and filter them in light of the question.
- For listing questions (such as listing all flight information), keep the answer within 10 points and tell the user they can check the search sources for complete information. Prioritize items that are complete and most relevant; unless necessary, do not proactively mention content the search results did not provide.
- For creative questions (such as writing a paper), be sure to cite the corresponding reference numbers within the body paragraphs, not only at the end of the article. You need to interpret and summarize the user's requirements, choose a suitable format, make full use of the search results and extract the important information, and produce an answer that meets the user's requirements with great depth of thought, creativity and professionalism. Make your piece as long as possible; for each point, infer the user's intent and give as many angles of discussion as possible, with rich information and detailed argumentation.
- If the answer is long, structure it and summarize by paragraph. If it needs to be answered in points, keep it within 5 points and merge related content.
- For factual Q&A, if the answer is very short, you may add one or two sentences of related information to enrich the content.
- Choose a suitable and attractive answer format based on the user's requirements and the answer content, to ensure good readability.
- Your answer should synthesize multiple relevant web pages and must not cite a single web page repeatedly.
- Unless the user requests otherwise, answer in the same language as the user's question.

# The user's message is:
%s`

func handleSearch(c *gin.Context, request *types.ChatCompletionRequest) {
	if !search.IsEnable() || request == nil || len(request.Messages) == 0 {
		return
	}

	msgLen := len(request.Messages)
	lastMsg := request.Messages[msgLen-1]

	// Check whether the last message is a user message
	if lastMsg.Role != types.ChatMessageRoleUser {
		return
	}

	// Extract the user message content
	userMsg := extractUserMessages(request.Messages, msgLen)
	if userMsg == "" {
		return
	}

	// Create the query request
	queryModel := "gpt-4o-mini"
	queryRequest := createSearchQueryRequest(userMsg, queryModel)

	// Get the provider and run the query
	provider, _, fail := GetProvider(c, queryModel)
	if fail != nil {
		return
	}

	chatProvider, ok := provider.(providersBase.ChatInterface)
	if !ok {
		return
	}

	// Run the query and handle the result
	queryKeyword, err := executeQuery(c, chatProvider, queryRequest, queryModel)
	if err != nil || queryKeyword == "" {
		return
	}

	// Run the search
	searchResults, err := performSearch(queryKeyword)
	if err != nil || searchResults == "" {
		return
	}

	// Update the request messages
	request.Messages[msgLen-1].Content = fmt.Sprintf(search_template,
		searchResults,
		time.Now().Format("2006-01-02 15:04:05"),
		userMsg)
}

// Extract user messages
func extractUserMessages(messages []types.ChatCompletionMessage, msgLen int) string {
	userMsg := ""

	// Take the last two messages
	lastTwoIndex := msgLen - 2
	if lastTwoIndex < 0 {
		lastTwoIndex = 0
	}

	for i := lastTwoIndex; i < msgLen; i++ {
		msg := messages[i].ParseContent()
		for _, part := range msg {
			if part.Type == types.ContentTypeText {
				userMsg += fmt.Sprintf("%s: %s\n", messages[i].Role, part.Text)
			}
		}
	}

	return userMsg
}

// Create the search query request
func createSearchQueryRequest(userMsg, model string) *types.ChatCompletionRequest {
	return &types.ChatCompletionRequest{
		Model: model,
		Messages: []types.ChatCompletionMessage{
			{
				Role:    "system",
				Content: fmt.Sprintf("Current time: %v. You are a web search bot; you need to decide whether the conversation below requires a search engine. If it does, use the tool to search, and search in the same language as the user's conversation. If it does not, simply return the number 0", time.Now().Format("2006-01-02 15:04:05")),
			},
			{
				Role:    "user",
				Content: userMsg,
			},
		},
		Tools: []*types.ChatCompletionTool{
			{
				Type: "function",
				Function: types.ChatCompletionFunction{
					Name:        "search",
					Description: "Searches the web for information.\\n\\n    Args:\\n        query: keyword to search for",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{
								"type": "string",
							},
						},
						"required": []string{"query"},
					},
				},
			},
		},
	}
}

// Run the query
func executeQuery(c *gin.Context, chatProvider providersBase.ChatInterface, queryRequest *types.ChatCompletionRequest, model string) (string, error) {
	usage := &types.Usage{}
	chatProvider.SetUsage(usage)

	// Pre-deduction (pre-authorization) must complete before starting the upstream web-search sub-call: reject immediately when quota is insufficient, at zero upstream cost
	quota := relay_util.NewQuota(c, model, 0)
	if opErr := quota.PreQuotaConsumption(); opErr != nil {
		return "", opErr
	}

	response, opErr := chatProvider.CreateChatCompletion(queryRequest)
	if opErr != nil {
		// Upstream call failed: roll back the pre-deducted quota
		quota.Undo(c)
		return "", opErr
	}
	quota.Consume(c, usage, false)

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	// Extract the query keyword
	choices := response.Choices[0]
	if choices.Message.ToolCalls == nil {
		return "", nil
	}

	toolCall := choices.Message.ToolCalls[0]
	queryMap := make(map[string]string)
	if jsonErr := json.Unmarshal([]byte(toolCall.Function.Arguments), &queryMap); jsonErr != nil {
		return "", jsonErr
	}

	return queryMap["query"], nil
}

// Run the search
func performSearch(queryKeyword string) (string, error) {
	s, err := search.Query(queryKeyword)
	if err != nil {

		return "", err
	}

	return s.ToString(), nil
}
