package relay

// Model lists for the coding-agent client endpoints (/claude-code, /codex/v1, /coding-agent).
// Each client's model picker parses its own list shape; the models in it are the ones the API
// key can call, as on /v1/models, minus the models a coding agent cannot use for text replies
// (image, audio, embedding, rerank). See common/agentclient for the client conventions.

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/agentclient"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// agentModels returns the models the request's API key can call, filtered as the bare list
// endpoints filter them (token limits, organization guardrails, catalog visibility) and, when
// channelTypes is set, to the models some channel of those types serves. The bool is false when
// the response has already been written (no group, guardrail unavailable).
func agentModels(c *gin.Context, channelTypes []int) ([]string, bool) {
	groupName := c.GetString("token_group")
	if groupName == "" {
		groupName = c.GetString("group")
	}
	if groupName == "" {
		common.AbortWithMessage(c, http.StatusServiceUnavailable, "Group not found")
		return nil, false
	}
	models, err := model.ChannelGroup.GetGroupModels(groupName)
	if err != nil {
		return []string{}, true
	}
	models = appendRoutingPolicyModels(routingScope(c), groupName, models, channelTypes)
	sort.Strings(models)
	models, err = filterModelsByTokenScope(c, models)
	if err != nil {
		common.AbortWithMessage(c, http.StatusServiceUnavailable, err.Error())
		return nil, false
	}
	if len(channelTypes) > 0 {
		models = filterModelsByChannelTypeFor(routingScope(c), groupName, models, channelTypes)
	}
	models = filterModelsByCatalog(models)

	listed := make([]string, 0, len(models))
	for _, name := range models {
		if isAgentTextModel(name) {
			listed = append(listed, name)
		}
	}
	return listed, true
}

// nonTextModelName catches the models without catalog metadata that cannot answer a coding agent
// with text: embeddings, rerankers, speech, transcription, moderation, image and video makers.
var nonTextModelName = regexp.MustCompile(`(?i)(embed|rerank|tts|whisper|transcri|speech|moderation|dall-e|imagen|gpt-image|midjourney|^mj[-_]|suno|kling|veo|sora|flux|stable-diffusion|recraft)`)

// isAgentTextModel reports whether a coding agent can use the model for its replies. Catalog
// metadata decides when it says something (the endpoints the model serves, its task type, its
// output modalities); otherwise the model name does.
func isAgentTextModel(modelName string) bool {
	if price := model.PricingInstance.GetPrice(modelName); price != nil && price.ModelInfo != nil {
		info := price.ModelInfo
		if len(info.Endpoints) > 0 {
			return slices.Contains(info.Endpoints, model.ModelEndpointChat) || slices.Contains(info.Endpoints, model.ModelEndpointResponses)
		}
		if info.Mode == model.ModelModeImage {
			return false
		}
		if len(info.OutputModalities) > 0 && !slices.Contains(info.OutputModalities, "text") {
			return false
		}
	}
	if types.IsImageGenerationModel(modelName) || model.ResolveModelMode(modelName) == model.ModelModeImage {
		return false
	}
	return !nonTextModelName.MatchString(modelName)
}

// agentModelInfo is the catalog metadata of a model, nil when it has none.
func agentModelInfo(modelName string) *model.ModelInfoResponse {
	if price := model.PricingInstance.GetPrice(modelName); price != nil {
		return price.ModelInfo
	}
	return nil
}

// claudeCodeModel is an entry of /claude-code/v1/models. Claude Code reads id, display_name and
// description (https://code.claude.com/docs/en/llm-gateway-protocol#model-discovery); type
// completes the Anthropic models list entry.
type claudeCodeModel struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// claudeCodeModelList is the Anthropic models list response.
type claudeCodeModelList struct {
	Data    []claudeCodeModel `json:"data"`
	HasMore bool              `json:"has_more"`
	FirstID *string           `json:"first_id"`
	LastID  *string           `json:"last_id"`
}

// ListClaudeCodeModelsByToken serves /claude-code/v1/models: the models the key can call over the
// Anthropic Messages protocol, under the IDs Claude Code's /model picker keeps
// (agentclient.ClaudeCodeModelID). The IDs are turned back into model names before routing
// (middleware.ClaudeCodeModelID).
func ListClaudeCodeModelsByToken(c *gin.Context) {
	models, ok := agentModels(c, AllowChannelType)
	if !ok {
		return
	}
	data := make([]claudeCodeModel, 0, len(models))
	for _, name := range models {
		contextLength, displayName := 0, name
		if info := agentModelInfo(name); info != nil {
			contextLength = info.ContextLength
			if info.Name != "" {
				displayName = info.Name
			}
		}
		data = append(data, claudeCodeModel{
			Type:        "model",
			ID:          agentclient.ClaudeCodeModelID(name, contextLength),
			DisplayName: displayName,
		})
	}
	c.JSON(http.StatusOK, newClaudeCodeModelList(data))
}

func newClaudeCodeModelList(data []claudeCodeModel) claudeCodeModelList {
	list := claudeCodeModelList{Data: data}
	if len(data) > 0 {
		list.FirstID = &data[0].ID
		list.LastID = &data[len(data)-1].ID
	}
	return list
}

// ListCodingAgentModelsByToken serves /coding-agent/v1/models, which both protocols share: an
// Anthropic client (it always sends anthropic-version) gets the Anthropic list, anything else the
// OpenAI list, each as on its bare path.
func ListCodingAgentModelsByToken(c *gin.Context) {
	if c.GetHeader("anthropic-version") != "" {
		ListClaudeModelsByToken(c)
		return
	}
	ListModelsByToken(c)
}

// codexModelsResponse is the ModelsResponse Codex decodes from {base_url}/models at startup:
// https://github.com/openai/codex/blob/main/codex-rs/protocol/src/openai_models.rs
// (ModelsResponse, ModelInfo). Fields without a serde default there are required, and a list
// with one malformed entry is rejected whole, so every required field is always written.
type codexModelsResponse struct {
	Models []codexModelInfo `json:"models"`
}

type codexReasoningLevel struct {
	Effort      string `json:"effort"`
	Description string `json:"description"`
}

type codexTruncationPolicy struct {
	Mode  string `json:"mode"`
	Limit int64  `json:"limit"`
}

type codexModelInfo struct {
	Slug                     string                `json:"slug"`
	DisplayName              string                `json:"display_name"`
	Description              *string               `json:"description"`
	DefaultReasoningLevel    *string               `json:"default_reasoning_level"`
	SupportedReasoningLevels []codexReasoningLevel `json:"supported_reasoning_levels"`
	// ShellType "shell_command" is understood by old and new clients alike (newer ones alias it).
	ShellType       string  `json:"shell_type"`
	Visibility      string  `json:"visibility"`
	SupportedInAPI  bool    `json:"supported_in_api"`
	Priority        int     `json:"priority"`
	AvailabilityNux *string `json:"availability_nux"`
	Upgrade         *string `json:"upgrade"`
	// BaseInstructions is required by every client version: older ones read it as a field, newer
	// ones promote it to model_messages.instructions_template.
	BaseInstructions           string                `json:"base_instructions"`
	SupportsReasoningSummaries bool                  `json:"supports_reasoning_summaries"`
	SupportVerbosity           bool                  `json:"support_verbosity"`
	DefaultVerbosity           *string               `json:"default_verbosity"`
	ApplyPatchToolType         *string               `json:"apply_patch_tool_type"`
	TruncationPolicy           codexTruncationPolicy `json:"truncation_policy"`
	SupportsParallelToolCalls  bool                  `json:"supports_parallel_tool_calls"`
	ContextWindow              *int64                `json:"context_window,omitempty"`
	MaxContextWindow           *int64                `json:"max_context_window,omitempty"`
	ExperimentalSupportedTools []string              `json:"experimental_supported_tools"`
	InputModalities            []string              `json:"input_modalities"`
}

// codexBaseInstructions is the system prompt Codex uses for a model listed by /codex/v1/models:
// the catalog replaces the prompt the client would use for an unknown model.
const codexBaseInstructions = `You are a coding agent running in the Codex CLI, a terminal-based coding assistant. You and the user share one workspace; your job is to complete the user's request.

- Read the relevant code before changing it, and keep changes focused on the task.
- Use the shell tool to inspect files and run commands, and the apply_patch tool (or the apply_patch command in the shell) to edit files.
- Follow the conventions of the repository; run its tests and checks when they exist.
- Do not undo changes you did not make, and ask before destructive actions.
- When you finish, summarize what you changed and anything left to do, briefly.`

var codexReasoningLevels = []codexReasoningLevel{
	{Effort: "low", Description: "Fast responses with lighter reasoning"},
	{Effort: "medium", Description: "Balances speed and reasoning depth for everyday tasks"},
	{Effort: "high", Description: "Greater reasoning depth for complex problems"},
}

// codexFreeformPatchModel reports whether Codex may give the model apply_patch as a freeform
// (custom) tool: only OpenAI's GPT-5 and Codex models take custom tools, and the protocol
// conversions to Chat Completions and Anthropic Messages drop them. Other models get the default,
// where Codex has the model call apply_patch through the shell.
func codexFreeformPatchModel(modelName string) bool {
	name := strings.ToLower(modelName)
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return strings.HasPrefix(name, "gpt-5") || strings.Contains(name, "codex")
}

func newCodexModelInfo(modelName string, priority int) codexModelInfo {
	entry := codexModelInfo{
		Slug:                       modelName,
		DisplayName:                modelName,
		ShellType:                  "shell_command",
		Visibility:                 "list",
		SupportedInAPI:             true,
		Priority:                   priority,
		BaseInstructions:           codexBaseInstructions,
		TruncationPolicy:           codexTruncationPolicy{Mode: "tokens", Limit: 10000},
		SupportsParallelToolCalls:  true,
		SupportedReasoningLevels:   []codexReasoningLevel{},
		ExperimentalSupportedTools: []string{},
		InputModalities:            []string{"text"},
	}
	if codexFreeformPatchModel(modelName) {
		freeform := "freeform"
		entry.ApplyPatchToolType = &freeform
	}

	info := agentModelInfo(modelName)
	reasoning := true // unknown capabilities: offer the levels, as Codex does for unknown models
	if info != nil {
		if info.Name != "" {
			entry.DisplayName = info.Name
		}
		if info.Description != "" {
			description := info.Description
			entry.Description = &description
		}
		if info.ContextLength > 0 {
			window := int64(info.ContextLength)
			entry.ContextWindow = &window
			entry.MaxContextWindow = &window
		}
		if slices.Contains(info.InputModalities, "image") {
			entry.InputModalities = append(entry.InputModalities, "image")
		}
		if len(info.Capabilities) > 0 {
			reasoning = slices.Contains(info.Capabilities, model.ModelCapabilityReasoning)
		}
	}
	if reasoning {
		medium := "medium"
		entry.DefaultReasoningLevel = &medium
		entry.SupportedReasoningLevels = codexReasoningLevels
		entry.SupportsReasoningSummaries = true
	}
	return entry
}

// ListCodexModelsByToken serves /codex/v1/models in Codex's ModelsResponse shape. Codex calls it
// as {base_url}/models?client_version=<version>; the version does not change the list.
func ListCodexModelsByToken(c *gin.Context) {
	models, ok := agentModels(c, nil)
	if !ok {
		return
	}
	entries := make([]codexModelInfo, 0, len(models))
	for i, name := range models {
		entries = append(entries, newCodexModelInfo(name, i+1))
	}
	c.JSON(http.StatusOK, codexModelsResponse{Models: entries})
}

// CountClaudeTokens serves /v1/messages/count_tokens on the Anthropic-shaped endpoints with a
// local estimate of the request's input tokens (messages, system prompt and tool definitions).
// It neither routes nor bills. Claude Code uses it for /context; without it Claude Code falls
// back to a character-based estimate.
func CountClaudeTokens(c *gin.Context) {
	var request claude.ClaudeRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		claudeErr := claude.StringErrorWrapper(err.Error(), "invalid_request_error", http.StatusBadRequest, true)
		c.JSON(http.StatusBadRequest, claudeErr.ClaudeError)
		return
	}
	tokens := countClaudeMessageTokens(&request)
	encoder := common.GetTokenEncoder(request.Model)
	if system := claudeSystemText(request.System); system != "" {
		tokens += common.GetTokenNum(encoder, system)
	}
	if len(request.Tools) > 0 {
		if raw, err := json.Marshal(request.Tools); err == nil {
			tokens += common.GetTokenNum(encoder, string(raw))
		}
	}
	c.JSON(http.StatusOK, gin.H{"input_tokens": tokens})
}

// claudeSystemText flattens a Messages system prompt (a string or text blocks) to text.
func claudeSystemText(system any) string {
	switch v := system.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, block := range v {
			if m, ok := block.(map[string]any); ok {
				if text, ok := m["text"].(string); ok {
					b.WriteString(text)
				}
			}
		}
		return b.String()
	}
	return ""
}
