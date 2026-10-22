// Channel form labels and hints per channel type. Display text is stored as i18n keys (or
// [key, params]); ChannelSheet renders it through configText().
const defaultConfig = {
  input: {
    name: '',
    type: 1,
    key: '',
    base_url: '',
    other: '',
    remark: '',
    proxy: '',
    test_model: '',
    model_mapping: [],
    model_headers: [],
    header_override: [],
    custom_parameter: '',
    need2response_models: '',
    models: [],
    groups: ['default'],
    plugin: {},
    tag: '',
    only_chat: false,
    pre_cost: 1,
    disabled_stream: [],
    compatible_response: false,
    allow_extra_body: false,
    pass_through_body: false,
    allow_service_tier: false,
    unified_request_response_model: false
  },
  inputLabel: {
    name: 'channelForm.label.name',
    type: 'channelForm.label.type',
    base_url: 'channelForm.label.base_url',
    key: 'channelForm.label.key',
    other: 'channelForm.label.other',
    remark: 'channelForm.label.remark',
    proxy: 'channelForm.label.proxy',
    test_model: 'channelForm.label.test_model',
    models: 'channelForm.label.models',
    model_mapping: 'channelForm.label.model_mapping',
    model_headers: 'channelForm.label.model_headers',
    header_override: 'channelForm.label.header_override',
    custom_parameter: 'channelForm.label.custom_parameter',
    need2response_models: 'channelForm.label.need2response_models',
    groups: 'channelForm.label.groups',
    only_chat: 'channelForm.label.only_chat',
    tag: 'channelForm.label.tag',
    provider_models_list: '',
    pre_cost: 'channelForm.label.pre_cost',
    disabled_stream: 'channelForm.label.disabled_stream',
    compatible_response: 'channelForm.label.compatible_response',
    allow_extra_body: 'channelForm.label.allow_extra_body',
    pass_through_body: 'channelForm.label.pass_through_body',
    allow_service_tier: 'channelForm.label.allow_service_tier',
    unified_request_response_model: 'channelForm.label.unified_request_response_model',
    balance_threshold: 'channelForm.label.balance_threshold'
  },
  prompt: {
    type: 'channelForm.prompt.type',
    name: 'channelForm.prompt.name',
    base_url: 'channelForm.prompt.base_url',
    key: 'channelForm.prompt.key',
    other: '',
    remark: 'channelForm.prompt.remark',
    proxy: 'channelForm.prompt.proxy',
    test_model: 'channelForm.prompt.test_model',
    models: 'channelForm.prompt.models',
    model_mapping: 'channelForm.prompt.model_mapping',
    model_headers: 'channelForm.prompt.model_headers',
    header_override: 'channelForm.prompt.header_override',
    custom_parameter: 'channelForm.prompt.custom_parameter',
    need2response_models: 'channelForm.prompt.need2response_models',
    groups: 'channelForm.prompt.groups',
    only_chat: 'channelForm.prompt.only_chat',
    provider_models_list: 'channelForm.prompt.provider_models_list',
    tag: 'channelForm.prompt.tag',
    pre_cost: 'channelForm.prompt.pre_cost',
    disabled_stream: 'channelForm.prompt.disabled_stream',
    compatible_response: 'channelForm.label.compatible_response',
    allow_extra_body: 'channelForm.prompt.allow_extra_body',
    pass_through_body: 'channelForm.prompt.pass_through_body',
    allow_service_tier: 'channelForm.prompt.allow_service_tier',
    unified_request_response_model: 'channelForm.prompt.unified_request_response_model',
    balance_threshold: 'channelForm.prompt.balance_threshold'
  },
  modelGroup: 'OpenAI'
}

// inputLabel.provider_models_list shows the "fetch models" button. Only set it for types whose
// backend provider implements GetModelList against an endpoint that exists upstream (see
// providers/*/model.go): a button that always fails is worse than none.
//
// input.models are the defaults filled in when the type is picked; list only models the backend
// can forward for that type (e.g. the bedrockMap in providers/bedrock/category/base.go).
// input.test_model should be a cheap model every account of that type can call.
const typeConfig = {
  1: {
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'OpenAI' }]
    }
  },
  8: {
    inputLabel: {
      provider_models_list: 'channelForm.fetchModelsFromChannel'
    },
    prompt: {
      other: ''
    }
  },
  3: {
    inputLabel: {
      base_url: 'AZURE_OPENAI_ENDPOINT',
      other: 'channelForm.label.apiVersion',
      provider_models_list: 'channelForm.fetchAzureDeployments'
    },
    prompt: {
      base_url: ['channelForm.prompt.enterValue', { name: 'AZURE_OPENAI_ENDPOINT' }],
      other: ['channelForm.prompt.apiVersionExample', { example: '2024-05-01-preview' }]
    }
  },
  55: {
    inputLabel: {
      base_url: 'AZURE_OPENAI_ENDPOINT',
      other: 'channelForm.label.apiVersion',
      provider_models_list: 'channelForm.fetchAzureDeployments'
    },
    prompt: {
      base_url: ['channelForm.prompt.enterValue', { name: 'AZURE_OPENAI_ENDPOINT' }],
      other: ['channelForm.prompt.apiVersionExample', { example: 'preview OR latest' }]
    }
  },
  11: {
    input: {
      models: ['PaLM-2'],
      test_model: 'PaLM-2'
    },
    modelGroup: 'Google PaLM'
  },
  14: {
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'Claude' }]
    },
    input: {
      models: [
        'claude-fable-5-1',
        'claude-opus-5-5',
        'claude-sonnet-5-5',
        'claude-haiku-5-5',
        'claude-fable-5',
        'claude-opus-5',
        'claude-sonnet-5',
        'claude-opus-4-8',
        'claude-opus-4-7',
        'claude-opus-4-6',
        'claude-sonnet-4-6',
        'claude-opus-4-5-20251101',
        'claude-haiku-4-5-20251001'
      ],
      test_model: 'claude-haiku-5-5'
    },
    modelGroup: 'Anthropic'
  },
  15: {
    input: {
      models: [
        'ERNIE-4.0-Turbo-8K',
        'ERNIE-4.0-8K-Latest',
        'ERNIE-4.0-8K-0613',
        'ERNIE-3.5-8K-0613',
        'ERNIE-Bot-turbo',
        'ERNIE-Lite-8K-0922',
        'ERNIE-Lite-8K',
        'ERNIE-Lite-8K-0308',
        'ERNIE-3.5-8K',
        'ERNIE-Bot',
        'ERNIE-4.0-8K',
        'ERNIE-4.0-8K-Preview',
        'ERNIE-4.0-8K-Preview-0518',
        'ERNIE-4.0-8K-0329',
        'ERNIE-4.0-8K-0104',
        'ERNIE-Bot-4',
        'ERNIE-Bot-8k',
        'ERNIE-3.5-128K',
        'ERNIE-3.5-8K-preview',
        'ERNIE-3.5-8K-0329',
        'ERNIE-3.5-4K-0205',
        'ERNIE-3.5-8K-0205',
        'ERNIE-3.5-8K-1222',
        'ERNIE-Speed',
        'ERNIE-Speed-8K',
        'ERNIE-Speed-128K',
        'ERNIE-Tiny-8K',
        'ERNIE-Function-8K',
        'ERNIE-Character-8K',
        'ERNIE-Character-Fiction-8K',
        'ERNIE-Bot-turbo-AI',
        'Embedding-V1'
      ],
      test_model: 'ERNIE-Speed'
    },
    prompt: {
      key: 'channelForm.prompt.keyFormatBaidu'
    },
    modelGroup: 'Baidu'
  },
  16: {
    input: {
      models: ['glm-5.3', 'glm-5.3-flash', 'glm-5.2', 'embedding-3'],
      test_model: 'glm-5.3-flash'
    },
    modelGroup: 'Zhipu'
  },
  17: {
    inputLabel: {
      other: 'channelForm.label.pluginParams',
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'Alibaba' }]
    },
    input: {
      models: ['qwen-turbo', 'qwen-plus', 'qwen-max', 'qwen-max-longcontext', 'text-embedding-v1'],
      test_model: 'qwen-turbo'
    },
    prompt: {
      other: 'channelForm.prompt.dashscopePlugin'
    },
    modelGroup: 'Ali'
  },
  18: {
    inputLabel: {
      other: 'channelForm.label.version'
    },
    input: {
      models: ['SparkDesk', 'SparkDesk-v1.1', 'SparkDesk-v2.1', 'SparkDesk-v3.1', 'SparkDesk-v3.5']
    },
    prompt: {
      key: ['channelForm.prompt.keyFormat', { format: 'APPID|APISecret|APIKey' }],
      other: ['channelForm.prompt.versionExample', { example: 'v3.1' }]
    },
    modelGroup: 'Xunfei'
  },
  19: {
    input: {
      models: ['360GPT_S2_V9', 'embedding-bert-512-v1', 'embedding_s1_v1', 'semantic_similarity_s1_v1'],
      test_model: '360GPT_S2_V9'
    },
    modelGroup: '360'
  },
  22: {
    prompt: {
      key: ['channelForm.prompt.keyFormatExample', { format: 'APIKey-AppId', example: 'fastgpt-0sp2gtvfdgyi4k30jwlgwf1i-64f335d84283f05518e9e041' }]
    }
  },
  23: {
    input: {
      models: ['ChatStd', 'ChatPro'],
      test_model: 'ChatStd'
    },
    prompt: {
      key: ['channelForm.prompt.keyFormat', { format: 'AppId|SecretId|SecretKey' }]
    },
    modelGroup: 'Tencent'
  },
  25: {
    inputLabel: {
      other: 'channelForm.label.version',
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'Gemini' }]
    },
    input: {
      models: [
        'gemini-4-argon',
        'gemini-3.8-flash',
        'gemini-3.6-flash',
        'gemini-3.5-flash-lite',
        'gemini-3.1-flash-lite',
        'gemini-3.1-pro-preview',
        'gemini-embedding-001'
      ],
      test_model: 'gemini-3.5-flash-lite'
    },
    prompt: {
      other: ['channelForm.prompt.versionExample', { example: 'v1' }]
    },
    modelGroup: 'Google Gemini'
  },
  26: {
    input: {
      models: ['Baichuan2-Turbo', 'Baichuan2-Turbo-192k', 'Baichuan2-53B', 'Baichuan-Text-Embedding'],
      test_model: 'Baichuan2-Turbo'
    },
    modelGroup: 'Baichuan'
  },
  24: {
    inputLabel: {
      other: 'channelForm.label.region'
    },
    input: {
      models: ['tts-1', 'tts-1-hd']
    },
    prompt: {
      test_model: '',
      base_url: '',
      other: 'channelForm.prompt.speechRegion'
    }
  },
  27: {
    input: {
      models: ['MiniMax-M3', 'MiniMax-M2.7', 'MiniMax-M2.7-highspeed'],
      test_model: 'MiniMax-M2.7'
    },
    modelGroup: 'MiniMax'
  },
  28: {
    input: {
      models: ['deepseek-flash', 'deepseek-v4-pro'],
      test_model: 'deepseek-flash'
    },
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'DeepSeek' }]
    },
    modelGroup: 'Deepseek'
  },
  29: {
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'Moonshot' }]
    },
    input: {
      models: ['kimi-k3', 'kimi-k2.7-code', 'kimi-k2.7-code-highspeed', 'kimi-k2.6'],
      test_model: 'kimi-k2.6'
    },
    modelGroup: 'Moonshot'
  },
  30: {
    input: {
      models: [
        'mistral-large-latest',
        'mistral-medium-latest',
        'mistral-small-latest',
        'ministral-8b-latest',
        'codestral-latest',
        'mistral-embed'
      ],
      test_model: 'mistral-small-latest'
    },
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'Mistral' }]
    },
    modelGroup: 'Mistral'
  },
  31: {
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'Groq' }]
    },
    modelGroup: 'Groq'
  },
  32: {
    input: {
      models: [
        'claude-fable-5-1',
        'claude-opus-5-5',
        'claude-sonnet-5-5',
        'claude-haiku-5-5',
        'claude-fable-5',
        'claude-opus-5',
        'claude-sonnet-5',
        'claude-opus-4-8',
        'claude-opus-4-7',
        'claude-opus-4-6',
        'claude-sonnet-4-6',
        'claude-opus-4-5-20251101',
        'claude-haiku-4-5-20251001'
      ],
      test_model: 'claude-haiku-4-5-20251001'
    },
    prompt: {
      key: 'channelForm.prompt.keyFormatAws'
    },
    modelGroup: 'Anthropic'
  },
  33: {
    input: {
      models: ['yi-34b-chat-0205', 'yi-34b-chat-200k', 'yi-vl-plus'],
      test_model: 'yi-34b-chat-0205'
    },
    modelGroup: 'Lingyiwanwu'
  },
  34: {
    input: {
      models: [
        'mj_imagine',
        'mj_variation',
        'mj_reroll',
        'mj_blend',
        'mj_modal',
        'mj_zoom',
        'mj_shorten',
        'mj_high_variation',
        'mj_low_variation',
        'mj_pan',
        'mj_inpaint',
        'mj_custom_zoom',
        'mj_describe',
        'mj_upscale',
        'swap_face',
        'mj_upload'
      ]
    },
    prompt: {
      key: ['channelForm.prompt.proxyKey', { service: 'midjourney-proxy' }],
      base_url: ['channelForm.prompt.proxyAddress', { service: 'midjourney-proxy' }],
      test_model: '',
      model_mapping: ''
    },
    modelGroup: 'Midjourney'
  },
  35: {
    input: {
      models: [
        '@cf/stabilityai/stable-diffusion-xl-base-1.0',
        '@cf/lykon/dreamshaper-8-lcm',
        '@cf/bytedance/stable-diffusion-xl-lightning',
        '@cf/qwen/qwen1.5-7b-chat-awq',
        '@cf/qwen/qwen1.5-14b-chat-awq',
        '@hf/google/gemma-7b-it',
        '@hf/thebloke/deepseek-coder-6.7b-base-awq',
        '@hf/thebloke/llama-2-13b-chat-awq',
        '@cf/openai/whisper'
      ],
      test_model: '@hf/google/gemma-7b-it'
    },
    prompt: {
      key: ['channelForm.prompt.keyFormat', { format: 'CLOUDFLARE_ACCOUNT_ID|CLOUDFLARE_API_TOKEN' }],
      base_url: ''
    },
    modelGroup: 'Cloudflare AI'
  },
  36: {
    input: {
      models: [
        'command-a-plus-05-2026',
        'command-a-reasoning-08-2025',
        'command-a-03-2025',
        'command-r-plus-08-2024',
        'command-r-08-2024',
        'command-r7b-12-2024',
        'rerank-v4.0-pro',
        'rerank-v4.0-fast',
        'rerank-v3.5'
      ],
      test_model: 'command-r7b-12-2024'
    },
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'Cohere' }]
    },
    modelGroup: 'Cohere'
  },
  37: {
    input: {
      models: ['sd3', 'sd3-turbo', 'stable-image-core']
    },
    prompt: {
      test_model: ''
    },
    modelGroup: 'Stability AI'
  },
  38: {
    input: {
      models: ['coze-*']
    },
    prompt: {
      models: 'channelForm.prompt.cozeModels',
      model_mapping:
        'channelForm.prompt.cozeMapping'
    },
    modelGroup: 'Coze'
  },
  39: {
    input: {
      models: ['phi3', 'llama3']
    },
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'Ollama' }]
    },
    prompt: {
      base_url: 'channelForm.prompt.ollamaUrl',
      key: 'channelForm.prompt.anyValue'
    }
  },
  40: {
    input: {
      models: ['hunyuan-lite', 'hunyuan-pro', 'hunyuan-standard-256K', 'hunyuan-standard'],
      test_model: 'hunyuan-lite'
    },
    prompt: {
      key: ['channelForm.prompt.keyFormat', { format: 'SecretId|SecretKey' }]
    },
    modelGroup: 'Hunyuan'
  },
  41: {
    input: {
      models: ['suno_lyrics', 'chirp-v3-0', 'chirp-v3-5']
    },
    prompt: {
      key: ['channelForm.prompt.proxyKey', { service: 'Suno-API' }],
      base_url: ['channelForm.prompt.proxyAddress', { service: 'Suno-API' }],
      test_model: '',
      model_mapping: ''
    },
    modelGroup: 'Suno'
  },
  42: {
    input: {
      models: ['gemini-4-argon', 'gemini-3.8-flash', 'gemini-3.6-flash', 'gemini-3.5-flash-lite', 'gemini-3.1-flash-lite'],
      test_model: 'gemini-3.5-flash-lite'
    },
    prompt: {
      key: 'channelForm.prompt.keySeeDocs',
      other: 'channelForm.prompt.vertexRegion',
      base_url: ''
    },
    modelGroup: 'VertexAI'
  },
  45: {
    input: {
      base_url: '',
      models: ['black-forest-labs/FLUX.1-dev', 'black-forest-labs/FLUX.1-schnell']
    },
    inputLabel: {
      base_url: 'channelForm.label.base_url',
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'SiliconFlow' }]
    },
    prompt: {
      base_url: 'channelForm.prompt.siliconflowUrl'
    },
    modelGroup: 'Siliconflow'
  },
  47: {
    input: {
      models: ['jina-reranker-v2-base-multilingual']
    },
    prompt: {
      test_model: ''
    },
    modelGroup: 'Jina'
  },
  49: {
    input: {
      models: ['gpt-4o', 'gpt-4o-mini', 'text-embedding-3-large', 'text-embedding-3-small', 'Cohere-command-r-plus', 'Cohere-command-r'],
      test_model: 'gpt-4o-mini'
    },
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'GitHub' }]
    },
    prompt: {
      key: ['channelForm.prompt.keySeeUrl', { url: 'https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens' }],
      base_url: 'https://models.inference.ai.azure.com'
    },
    modelGroup: 'Github'
  },
  51: {
    input: {
      models: [
        'recraftv3',
        'recraft20b',
        'recraft_vectorize',
        'recraft_removeBackground',
        'recraft_clarityUpscale',
        'recraft_generativeUpscale',
        'recraft_styles'
      ]
    }
  },
  53: {
    input: {
      models: [
        'kling-video_kling-v1_std_5',
        'kling-video_kling-v1_std_10',
        'kling-video_kling-v1_pro_5',
        'kling-video_kling-v1_pro_10',

        'kling-video_kling-v1-5_std_5',
        'kling-video_kling-v1-5_std_10',
        'kling-video_kling-v1-5_pro_5',
        'kling-video_kling-v1-5_pro_10',

        'kling-video_kling-v1-10_std_5',
        'kling-video_kling-v1-10_std_10',
        'kling-video_kling-v1-10_pro_5',
        'kling-video_kling-v1-10_pro_10'
      ]
    },
    prompt: {
      key: ['channelForm.prompt.keyFormat', { format: 'accessKey|secretKey' }]
    },
    modelGroup: 'Kling'
  },
  54: {
    input: {
      models: [
        'databricks-gpt-oss-120b',
        'databricks-gpt-oss-20b',
        'databricks-meta-llama-3-3-70b-instruct',
        'databricks-meta-llama-3-1-8b-instruct'
      ],
      test_model: 'databricks-meta-llama-3-1-8b-instruct'
    },
    inputLabel: {
      base_url: 'Azure Databricks Endpoint',
      key: 'DATABRICKS_TOKEN'
    },
    prompt: {
      base_url: ['channelForm.prompt.enterValue', { name: 'Azure Databricks Endpoint' }],
      key: ['channelForm.prompt.enterValue', { name: 'DATABRICKS_TOKEN' }]
    }
  },
  20: {
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'OpenRouter' }]
    }
  },
  56: {
    input: {
      models: ['grok-4.7'],
      test_model: 'grok-4.7'
    },
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'xAI' }]
    }
  },
  57: {
    inputLabel: {
      other: 'Project ID'
    },
    prompt: {
      key: 'channelForm.prompt.oauthJson',
      other: 'channelForm.prompt.gcpProjectId'
    }
  },
  58: {
    input: {
      models: ['claude-opus-5-5', 'claude-sonnet-5-5', 'claude-haiku-5-5'],
      test_model: 'claude-haiku-5-5'
    },
    inputLabel: {
      other: ''
    },
    prompt: {
      key: 'channelForm.prompt.claudeOauth',
      other: ''
    }
  },
  59: {
    input: {
      models: ['gpt-6.1-sol', 'gpt-6-astra', 'gpt-5.6-sol'],
      test_model: 'gpt-6.1-sol'
    },
    inputLabel: {
      other: ''
    },
    prompt: {
      key: 'channelForm.prompt.oauthJson',
      other: ''
    }
  },
  60: {
    inputLabel: {
      other: 'channelForm.label.projectIdOptional'
    },
    prompt: {
      key: 'channelForm.prompt.oauthJson',
      other: 'channelForm.prompt.projectIdOptional'
    }
  },
  61: {
    input: {
      models: ['gemini-4-argon', 'gemini-3.8-flash', 'gemini-3.6-flash', 'gemini-3.5-flash-lite', 'gemini-3.1-flash-lite'],
      test_model: 'gemini-3.5-flash-lite'
    },
    inputLabel: {
      other: 'Region|ProjectID'
    },
    prompt: {
      key: 'channelForm.prompt.vertexExpressKey',
      other: ['channelForm.prompt.format', { format: 'us-central1|your-project-id' }]
    },
    modelGroup: 'VertexAI Express'
  },
  62: {
    input: {
      models: ['anthropic/claude-opus-5', 'openai/gpt-6-astra', 'google/gemini-3.5-flash-lite'],
      test_model: 'google/gemini-3.5-flash-lite'
    },
    inputLabel: {
      provider_models_list: ['channelForm.fetchModelsFrom', { source: 'Vercel AI Gateway' }]
    },
    modelGroup: 'Vercel AI Gateway'
  }
}

export { defaultConfig, typeConfig }
