package settings

// Settings page group "gateway", tab "request": how a request is forwarded (model name matching,
// native protocols and prompt limits).
func gatewayRequest() []Setting {
	return []Setting{
		// ---------------------------------------------------------------- model-name-match
		{
			Key: "ModelNameCaseInsensitiveEnabled", Scope: ScopeSite, Group: "gateway", Card: "model-name-match", Type: TypeBool,
			Label: "Ignore case in model names",
			Description: "The model name a caller sends matches channels and prices whatever its case. When on, GPT-4o finds " +
				"the channels and the price of gpt-4o, including wildcard prices. When off, names must match exactly and a " +
				"differently cased name gets no channel.",
			Related:  []string{"UnifiedRequestResponseModelEnabled"},
			Keywords: []string{"case insensitive", "uppercase", "lowercase", "model name"},
		},
		{
			Key: "UnifiedRequestResponseModelEnabled", Scope: ScopeSite, Group: "gateway", Card: "model-name-match", Type: TypeBool,
			Label: "Answer with the requested model name",
			Description: "The model field of every response carries the name the caller asked for, not the upstream model it " +
				"was mapped to. Some clients check that the reply names the model they requested. When off, it is done only for " +
				"channels and supplies that switch it on themselves.",
			Examples: []Example{
				{Value: "true", Effect: "A request for my-chat mapped to gpt-4o-2024-08-06 answers with \"model\": \"my-chat\"."},
				{Value: "false", Effect: "The same request answers with \"model\": \"gpt-4o-2024-08-06\", unless its channel or supply renames it."},
			},
			Related:  []string{"ModelNameCaseInsensitiveEnabled", "FingerprintPassThroughEnabled"},
			Keywords: []string{"model mapping", "response model", "rename", "alias"},
		},

		// ---------------------------------------------------------------- native-protocol
		{
			Key: "ClaudeAPIEnabled", Scope: ScopeSite, Group: "gateway", Card: "native-protocol", Type: TypeBool,
			Label: "Claude Messages API",
			Description: "Callers can use the Anthropic Messages format (/claude/v1/messages and the coding agent endpoints) " +
				"besides the OpenAI format. Claude Code and the Anthropic SDKs need it. When off, those endpoints answer 403 and " +
				"llms.txt no longer lists them.",
			Related:  []string{"GeminiAPIEnabled", "ClaudePromptCachingEnabled"},
			Keywords: []string{"Anthropic", "Messages API", "Claude Code", "native", "/v1/messages"},
		},
		{
			Key: "GeminiAPIEnabled", Scope: ScopeSite, Group: "gateway", Card: "native-protocol", Type: TypeBool,
			Label: "Gemini API",
			Description: "Callers can use the Google Gemini format (the /gemini/v1beta endpoints, Live API included) besides " +
				"the OpenAI format. Gemini CLI and the Google SDKs need it. When off, those endpoints answer 403.",
			Related:  []string{"ClaudeAPIEnabled", "GeminiOpenThink"},
			Keywords: []string{"Google", "Gemini", "generateContent", "native", "Gemini CLI"},
		},
		{
			Key: "ClaudePromptCachingEnabled", Scope: ScopeSite, Group: "gateway", Card: "native-protocol", Type: TypeBool,
			Label: "Add prompt cache markers",
			Description: "Adds cache_control markers to requests for Anthropic models so the upstream caches the prompt. " +
				"Later requests then read it at the cache price. It covers Claude, Opus, Sonnet and Haiku models on any channel " +
				"type, and applies on channels whose prompt " +
				"caching setting is Inherit; a channel set to On or Off decides for itself, along with the TTL and which messages " +
				"are marked. Requests that already carry cache_control are left alone. An API key's prompt caching setting, or " +
				"the X-Modeltaps-Prompt-Cache header, decides for its own requests. On by default.",
			Examples: []Example{
				{Value: "true", Effect: "A client that sends no cache markers still gets its long system prompt cached upstream for 5 minutes."},
				{Value: "false", Effect: "Only clients that mark their prompts themselves, and channels set to On, use the cache."},
			},
			Related:  []string{"SessionAffinityEnabled", "ClaudeAPIEnabled"},
			Keywords: []string{"prompt caching", "cache_control", "cache", "Anthropic", "cost"},
		},
		{
			Key: "FingerprintPassThroughEnabled", Scope: ScopeSite, Group: "gateway", Card: "native-protocol", Type: TypeBool,
			Label: "Pass upstream responses through",
			Description: "Keeps upstream responses as close to the original as possible. Non-streaming Claude and Bedrock " +
				"responses are passed byte for byte, streams keep their model field, and upstream headers such as " +
				"anthropic-ratelimit-*, x-ratelimit-* and x-amzn-* reach the caller. When off, responses are rebuilt like those " +
				"of any other channel.",
			Related:  []string{"UnifiedRequestResponseModelEnabled"},
			Keywords: []string{"fingerprint", "pass-through", "raw response", "headers", "rate limit headers"},
			Advanced: true,
		},
		{
			Key: "ClaudeBudgetTokensPercentage", Scope: ScopeSite, Group: "gateway", Card: "native-protocol", Type: TypeFloat,
			Min: num(0), Max: num(1),
			Label: "Default thinking budget",
			Description: "The share of max_tokens Claude may spend thinking when a request asks for thinking without its own budget. " +
				"It applies to OpenAI-format requests sent to Claude. budget_tokens = max_tokens x this value, and never below 1024. A " +
				"request that names an effort gets 0.2, 0.5 or 0.8 for low, medium and high instead.",
			Examples: []Example{
				{Value: "0.8", Effect: "With max_tokens 10000, Claude may think for 8000 tokens and has 2000 left for the answer."},
				{Value: "0.5", Effect: "With max_tokens 4096, the thinking budget is 2048 tokens."},
				{Value: "0.1", Effect: "With max_tokens 8192, 819 would be too small, so the budget is raised to 1024."},
			},
			Related:  []string{"ClaudeDefaultMaxTokens"},
			Keywords: []string{"thinking", "budget_tokens", "reasoning", "extended thinking"},
		},
		{
			Key: "ClaudeDefaultMaxTokens", Scope: ScopeSite, Group: "gateway", Card: "native-protocol", Type: TypeJSON,
			Label: "Default max tokens",
			Description: "The max_tokens sent to Claude when an OpenAI-format or Responses request sets no output limit. " +
				"Claude requires one, so it is a JSON object of model name and tokens, with \"default\" for every other model. " +
				"Requests that set max_tokens or max_completion_tokens keep their own. Empty means {\"default\": 8192}; an object " +
				"without \"default\" leaves unlisted models without a limit and Claude rejects them, and text that is not valid JSON " +
				"is ignored.",
			Examples: []Example{
				{Value: `{"default": 8192}`, Effect: "Every Claude request without a limit may write up to 8192 tokens."},
				{Value: `{"default": 8192, "claude-sonnet-4-5": 64000}`, Effect: "claude-sonnet-4-5 requests without a limit may write up to 64000 tokens, other models 8192."},
			},
			Related:  []string{"ClaudeBudgetTokensPercentage"},
			Keywords: []string{"max_tokens", "output limit", "Claude", "JSON"},
		},
		{
			Key: "GeminiOpenThink", Scope: ScopeSite, Group: "gateway", Card: "native-protocol", Type: TypeJSON,
			Label: "Show Gemini thoughts",
			Description: "Gemini models whose responses include their thought summaries, as a JSON object of model name and " +
				"true. It only applies to models that can think; unlisted models keep thoughts out unless the request asks for " +
				"them. Empty lists none, and text that is not valid JSON is ignored.",
			Examples: []Example{
				{Value: `{"gemini-2.5-pro": true}`, Effect: "gemini-2.5-pro replies carry the model's reasoning before the answer; gemini-2.5-flash replies do not."},
			},
			Related:  []string{"GeminiAPIEnabled"},
			Keywords: []string{"thinking", "include_thoughts", "reasoning", "Gemini"},
			Advanced: true,
		},

		// ---------------------------------------------------------------- prompt-limits
		{
			Key: "MaxPromptTokens", Scope: ScopeSite, Group: "gateway", Card: "prompt-limits", Type: TypeInt, Min: num(0),
			Label: "Max prompt tokens",
			Description: "Requests to AWS Bedrock channels with more input tokens than this are refused before reaching the upstream. " +
				"They get a 400 \"prompt is too long\", which Claude Code reads like Anthropic's own; Bedrock would otherwise " +
				"hang on them until its timeout. A model's context length in the catalog takes precedence; other channel types " +
				"are not checked. 0 turns the check off.",
			Examples: []Example{
				{Value: "1000000", Effect: "A 1.2 million token prompt to a Bedrock model with no context length in the catalog gets a 400 at once."},
				{Value: "200000", Effect: "Prompts over 200000 tokens are refused for Bedrock models without a known context length."},
			},
			Keywords: []string{"context length", "prompt too long", "Bedrock", "AWS", "input limit"},
			Advanced: true,
		},
	}
}
