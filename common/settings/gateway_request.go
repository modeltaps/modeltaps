package settings

// Settings page group "gateway", tab "request": how a request is forwarded (model name matching,
// native protocols, prompt limits, the image proxy, image storage and Midjourney callbacks).
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
				"are marked. Requests that already carry cache_control are left alone. Off by default.",
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

		// ---------------------------------------------------------------- image-proxy
		{
			Key: "ChatImageRequestProxy", Scope: ScopeSite, Group: "gateway", Card: "image-proxy", Type: TypeURL,
			Label: "Image download proxy",
			Description: "An HTTP or SOCKS5 proxy for downloading the images that requests link to. The gateway fetches them to " +
				"count their tokens and to pass them on as data, and without a proxy the image hosts see this server's IP. With a " +
				"Cloudflare Worker set, the proxy only carries the call to the Worker. Empty downloads directly.",
			Examples: []Example{
				{Value: "socks5://127.0.0.1:1080", Effect: "Image downloads go through the local SOCKS5 proxy on port 1080."},
				{Value: "http://proxy.internal:3128", Effect: "Image downloads go through an HTTP proxy in the private network."},
			},
			Related:  []string{"CFWorkerImageUrl"},
			Keywords: []string{"proxy", "SOCKS5", "image URL", "vision", "server IP"},
		},
		{
			Key: "CFWorkerImageUrl", Scope: ScopeSite, Group: "gateway", Card: "image-proxy", Type: TypeURL,
			Label: "Cloudflare Worker image proxy",
			Description: "A Cloudflare Worker that downloads the images requests link to, instead of this server. The gateway " +
				"posts the image address and the Worker key to it and reads the image back, so the image hosts only see Cloudflare. " +
				"It takes precedence over the image download proxy. Some image hosts refuse Cloudflare, and those images then fail. " +
				"Empty uses no Worker.",
			Examples: []Example{
				{Value: "https://img-proxy.example.workers.dev", Effect: "Every image linked in a request is fetched by that Worker."},
			},
			Related:  []string{"CFWorkerImageKey", "ChatImageRequestProxy"},
			Keywords: []string{"Cloudflare", "Worker", "image proxy", "vision"},
			Advanced: true,
		},
		{
			Key: "CFWorkerImageKey", Scope: ScopeSite, Group: "gateway", Card: "image-proxy", Type: TypeSecret,
			Label: "Cloudflare Worker key",
			Description: "The key sent to the Cloudflare Worker with every image download, for a Worker that checks one. Leave " +
				"it empty when the Worker checks none or no Worker is set.",
			Related:  []string{"CFWorkerImageUrl"},
			Keywords: []string{"Cloudflare", "Worker", "API key"},
			Advanced: true,
		},

		// ---------------------------------------------------------------- image-storage
		{
			Key: "ImageStorageProvider", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "local", Label: "This gateway", Description: "Kept on this gateway for 1 hour under the server address, at most 5 downloads per link and 2 MB/s per download. Several nodes need Redis so every node can serve every link."},
				{Value: "s3", Label: "S3 compatible", Description: "AWS S3, Cloudflare R2, MinIO, Backblaze B2 and other S3 compatible storage. Without a public URL, links are signed for 1 hour and the bucket can stay private. The gateway deletes no files: add a lifecycle rule in the storage console (for example 1 day)."},
				{Value: "alioss", Label: "Alibaba Cloud OSS", Description: "Links are signed and valid for 1 hour, so the bucket can stay private."},
				{Value: "smms", Label: "SM.MS", Description: "Free image host; images are public."},
				{Value: "imgur", Label: "Imgur", Description: "Free image host; images are public."},
				{Value: "off", Label: "Off", Description: "Images come back as base64, and clients that only read the image URL get none."},
			},
			Label: "Image storage",
			Description: "Where generated images are kept so a request that asks for an image URL gets a link. Gemini, Cloudflare " +
				"Workers AI and Stability AI return image data only. Off answers with base64 instead: images endpoints give b64_json " +
				"rather than url, and Gemini replies carry the whole image inline. A change applies at once on every node.",
			Related:  []string{"ImageStorageRehostUpstream", "ServerAddress"},
			Keywords: []string{"image hosting", "S3", "R2", "OSS", "base64", "b64_json", "generated images"},
			Docs:     "deployment/storage.md",
		},
		{
			Key: "ImageStorageRehostUpstream", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeBool,
			Label: "Also store upstream image and video links",
			Description: "Image and video links from upstreams are swapped for links on this site, which work where the upstream " +
				"host is blocked. It covers image links from DALL·E, Flux and similar upstreams and from " +
				"Midjourney tasks, and video links from Kling tasks. Images are fetched at once; a video is fetched on its first " +
				"download and kept, up to 512 MB. A link that cannot be fetched is passed on as it is. When off, upstream links " +
				"pass through.",
			Related:  []string{"ImageStorageProvider"},
			Keywords: []string{"rehost", "mirror", "image link", "video", "Midjourney", "Kling"},
			Docs:     "deployment/storage.md",
		},
		{
			Key: "ImageStorageS3Endpoint", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeURL,
			Label: "S3 endpoint",
			Description: "The S3 API address of the storage, without the bucket. Without a public URL, links point at " +
				"<endpoint>/<bucket>/... and are signed.",
			Examples: []Example{
				{Value: "https://<account>.r2.cloudflarestorage.com", Effect: "Cloudflare R2; the region is auto."},
				{Value: "https://s3.eu-central-1.amazonaws.com", Effect: "AWS S3 in Frankfurt; the region eu-central-1 is read from the address."},
			},
			Related:  []string{"ImageStorageS3Region", "ImageStorageS3BucketName", "ImageStorageS3CDNURL"},
			Keywords: []string{"S3", "R2", "MinIO", "endpoint"},
			Docs:     "deployment/storage.md",
		},
		{
			Key: "ImageStorageS3Region", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeString,
			Label: "S3 region",
			Description: "The region requests are signed for; usually left empty. Empty reads it from AWS, Backblaze B2 and " +
				"Wasabi endpoints and uses auto for anything else, which is what Cloudflare R2 expects.",
			Examples: []Example{
				{Value: "us-east-1", Effect: "Signs for us-east-1, for MinIO or another storage that expects a fixed region."},
			},
			Related:  []string{"ImageStorageS3Endpoint"},
			Keywords: []string{"region", "S3"},
		},
		{
			Key: "ImageStorageS3BucketName", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeString,
			Label:       "S3 bucket",
			Description: "The bucket images are uploaded to. It can stay private unless a public URL is set.",
			Related:     []string{"ImageStorageS3Endpoint", "ImageStorageS3CDNURL"},
			Keywords:    []string{"bucket", "S3"},
		},
		{
			Key: "ImageStorageS3AccessKeyId", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeString,
			Label:       "S3 access key ID",
			Description: "The ID of an access key that may write to the bucket; its secret goes in the next field.",
			Related:     []string{"ImageStorageS3AccessKeySecret"},
			Keywords:    []string{"access key", "credentials", "S3"},
		},
		{
			Key: "ImageStorageS3AccessKeySecret", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeSecret,
			Label: "S3 access key secret",
			Description: "The secret of the S3 access key. It is stored encrypted when an encryption key is set, never shown " +
				"again, and saving the field empty keeps the stored one.",
			Related:  []string{"ImageStorageS3AccessKeyId"},
			Keywords: []string{"secret key", "credentials", "S3"},
		},
		{
			Key: "ImageStorageS3CDNURL", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeURL,
			Label: "S3 public URL",
			Description: "A public address of the bucket or of a CDN in front of it; images then get plain public links under " +
				"it. Empty gives links signed for 1 hour, and the bucket can stay private.",
			Examples: []Example{
				{Value: "https://img.example.com", Effect: "Images get links under https://img.example.com/ that work as long as the file exists."},
			},
			Related:  []string{"ImageStorageS3Endpoint", "ImageStorageS3BucketName"},
			Keywords: []string{"CDN", "public URL", "custom domain"},
		},
		{
			Key: "ImageStorageS3ExpirationDays", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeInt, Unit: "days", Min: num(0),
			Label: "S3 Expires header",
			Description: "Days written into the Expires header of each uploaded image, for caches and CDNs. It deletes nothing: " +
				"use a lifecycle rule of the storage for that. 0 writes no header.",
			Examples: []Example{
				{Value: "1", Effect: "An image uploaded today says it expires tomorrow; the file stays until a lifecycle rule removes it."},
				{Value: "0", Effect: "No Expires header."},
			},
			Related:  []string{"ImageStorageS3CDNURL"},
			Keywords: []string{"expires", "cache header", "lifecycle"},
			Advanced: true,
		},
		{
			Key: "ImageStorageOSSEndpoint", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeString,
			Label:       "OSS endpoint",
			Description: "The endpoint of the Alibaba Cloud OSS region the bucket is in.",
			Examples: []Example{
				{Value: "oss-cn-beijing.aliyuncs.com", Effect: "A bucket in Beijing; the region cn-beijing is read from the address."},
			},
			Related:  []string{"ImageStorageOSSRegion", "ImageStorageOSSBucketName"},
			Keywords: []string{"Aliyun", "OSS", "endpoint"},
		},
		{
			Key: "ImageStorageOSSRegion", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeString,
			Label: "OSS region",
			Description: "The OSS region the bucket is in; usually left empty. Empty reads it from an endpoint of the form " +
				"oss-<region>.aliyuncs.com; other endpoints (an internal or accelerated one) need it, or uploads fail.",
			Examples: []Example{
				{Value: "cn-hangzhou", Effect: "Signs for Hangzhou, for an endpoint the region cannot be read from."},
			},
			Related:  []string{"ImageStorageOSSEndpoint"},
			Keywords: []string{"region", "OSS"},
		},
		{
			Key: "ImageStorageOSSBucketName", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeString,
			Label:       "OSS bucket",
			Description: "The OSS bucket images are uploaded to; it can stay private, since links are signed.",
			Related:     []string{"ImageStorageOSSEndpoint"},
			Keywords:    []string{"bucket", "OSS"},
		},
		{
			Key: "ImageStorageOSSAccessKeyId", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeString,
			Label:       "OSS AccessKey ID",
			Description: "The AccessKey ID of a RAM user that may write to the bucket; its secret goes in the next field.",
			Related:     []string{"ImageStorageOSSAccessKeySecret"},
			Keywords:    []string{"AccessKey", "RAM", "credentials", "OSS"},
		},
		{
			Key: "ImageStorageOSSAccessKeySecret", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeSecret,
			Label: "OSS AccessKey secret",
			Description: "The AccessKey secret of the OSS user. It is stored encrypted when an encryption key is set, never " +
				"shown again, and saving the field empty keeps the stored one.",
			Related:  []string{"ImageStorageOSSAccessKeyId"},
			Keywords: []string{"AccessKey secret", "credentials", "OSS"},
		},
		{
			Key: "ImageStorageSMMSSecret", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeSecret,
			Label:       "SM.MS API token",
			Description: "The API token of your account on the free image host, used for uploads. Images there are public.",
			Related:     []string{"ImageStorageProvider"},
			Keywords:    []string{"SM.MS", "smms", "image host", "token"},
		},
		{
			Key: "ImageStorageImgurClientId", Scope: ScopeSite, Group: "gateway", Card: "image-storage", Type: TypeSecret,
			Label:       "Imgur client ID",
			Description: "The client ID of an application registered with Imgur, used for anonymous uploads. Images there are public.",
			Related:     []string{"ImageStorageProvider"},
			Keywords:    []string{"Imgur", "image host", "client id"},
		},

		// ---------------------------------------------------------------- mj-notify
		{
			Key: "MjNotifyEnabled", Scope: ScopeSite, Group: "gateway", Card: "mj-notify", Type: TypeBool,
			Label: "Midjourney callbacks",
			Description: "Passes the callback address (notifyHook) of a Midjourney task on to the upstream, which calls it when " +
				"the task changes. The call comes from the upstream, not this site, so it carries the " +
				"upstream's view of the task and tells the upstream the caller's address. When off, the address is dropped and " +
				"callers poll the task status.",
			Keywords: []string{"Midjourney", "notifyHook", "callback", "webhook", "MJ"},
		},
	}
}
