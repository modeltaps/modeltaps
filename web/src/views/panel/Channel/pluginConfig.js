// Channel plugin definitions. Display text is stored as i18n keys (or [key, params]); PluginForm
// renders it through configText().

const P = 'channelPlugin';

const enableParam = (description) => ({
  name: `${P}.enable`,
  description,
  type: 'bool',
  required: true
});

const openaiApiPlugin = (vendor, descKey = `${P}.useOpenaiApi.enableDescNoPlugins`) => ({
  name: `${P}.useOpenaiApi.name`,
  description: `${P}.useOpenaiApi.name`,
  params: {
    enable: enableParam([descKey, { vendor }])
  }
});

const webSearchPlugin = {
  name: `${P}.webSearch.name`,
  description: `${P}.webSearch.description`,
  params: {
    enable: enableParam(`${P}.webSearch.enableDesc`)
  }
};

const voiceParam = (voice, value) => ({
  name: [`${P}.voice.mappingName`, { voice }],
  description: [`${P}.voice.defaultValue`, { value }],
  type: 'string',
  required: true
});

const endpointParam = (endpoint, path) => ({
  name: [`${P}.customize.endpointName`, { endpoint }],
  description: [`${P}.customize.endpointDefault`, { path }],
  type: 'string',
  required: false
});

const promptCachingPlugin = {
  "prompt_caching": {
    "name": `${P}.promptCaching.name`,
    "description": `${P}.promptCaching.description`,
    "params": {
      "enabled": {
        "name": `${P}.promptCaching.enabled.name`,
        "description": `${P}.promptCaching.enabled.description`,
        "type": "select",
        "default": "inherit",
        "required": false,
        "options": [
          { "value": "inherit", "label": `${P}.promptCaching.enabled.inherit` },
          { "value": "on", "label": `${P}.promptCaching.enabled.on` },
          { "value": "off", "label": `${P}.promptCaching.enabled.off` }
        ]
      },
      "ttl": {
        "name": `${P}.promptCaching.ttl.name`,
        "description": `${P}.promptCaching.ttl.description`,
        "type": "select",
        "default": "5m",
        "required": false,
        "options": [
          { "value": "5m", "label": `${P}.promptCaching.ttl.5m` },
          { "value": "1h", "label": `${P}.promptCaching.ttl.1h` }
        ]
      },
      "strategy": {
        "name": `${P}.promptCaching.strategy.name`,
        "description": `${P}.promptCaching.strategy.description`,
        "type": "select",
        "default": "system",
        "required": false,
        "options": [
          { "value": "system", "label": "system" },
          { "value": "system+last_user", "label": `${P}.promptCaching.strategy.systemLastUser` }
        ]
      },
      "model_match_mode": {
        "name": `${P}.promptCaching.matchMode.name`,
        "description": `${P}.promptCaching.matchMode.description`,
        "type": "select",
        "default": "auto",
        "required": false,
        "options": [
          { "value": "auto", "label": `${P}.promptCaching.matchMode.auto` },
          { "value": "custom", "label": `${P}.promptCaching.matchMode.custom` },
          { "value": "regex", "label": `${P}.promptCaching.matchMode.regex` },
          { "value": "auto+custom", "label": `${P}.promptCaching.matchMode.autoCustom` }
        ]
      },
      "model_match_patterns": {
        "name": `${P}.promptCaching.patterns.name`,
        "description": `${P}.promptCaching.patterns.description`,
        "type": "string",
        "required": false
      },
      "model_match_regex": {
        "name": `${P}.promptCaching.regex.name`,
        "description": `${P}.promptCaching.regex.description`,
        "type": "string",
        "required": false
      }
    }
  }
};

const pluginConfig = {
  "14": promptCachingPlugin,
  "58": promptCachingPlugin,
  "16": {
    "retrieval": {
      "name": `${P}.retrieval.name`,
      "description": `${P}.retrieval.description`,
      "params": {
        "knowledge_id": {
          "name": `${P}.retrieval.knowledgeId.name`,
          "description": `${P}.retrieval.knowledgeId.description`,
          "type": "string",
          "required": true
        },
        "prompt_template": {
          "name": `${P}.retrieval.promptTemplate.name`,
          "description": `${P}.retrieval.promptTemplate.description`,
          "type": "string",
          "required": false
        }
      }
    },
    "web_search": webSearchPlugin,
    "web_browser": {
      "name": `${P}.webBrowser.name`,
      "description": `${P}.webBrowser.description`,
      "params": {
        "enable": enableParam(`${P}.webBrowser.enableDesc`)
      }
    },
    "drawing_tool": {
      "name": `${P}.drawingTool.name`,
      "description": `${P}.drawingTool.description`,
      "params": {
        "enable": enableParam(`${P}.drawingTool.enableDesc`)
      }
    },
    "code_interpreter": {
      "name": `${P}.codeInterpreter.name`,
      "description": `${P}.codeInterpreter.description`,
      "params": {
        "sandbox": {
          "name": `${P}.codeInterpreter.sandbox.name`,
          "description": `${P}.codeInterpreter.sandbox.description`,
          "type": "string",
          "required": false
        }
      }
    }
  },
  "15": {
    "use_openai_api": openaiApiPlugin('Baidu')
  },
  "17": {
    "web_search": webSearchPlugin,
    "use_openai_api": openaiApiPlugin('Alibaba')
  },
  "24": {
    "voice": {
      "name": `${P}.voice.name`,
      "description": `${P}.voice.azureDescription`,
      "params": {
        "alloy": voiceParam('alloy', 'zh-CN-YunxiNeural'),
        "echo": voiceParam('echo', 'zh-CN-YunyangNeural'),
        "fable": voiceParam('fable', 'zh-CN-YunxiNeural|boy'),
        "onyx": voiceParam('onyx', 'zh-CN-YunyeNeural'),
        "nova": voiceParam('nova', 'zh-CN-XiaochenNeural'),
        "shimmer": voiceParam('shimmer', 'zh-CN-XiaohanNeural')
      }
    }
  },
  "39": {
    "headers": {
      "name": `${P}.headers.name`,
      "description": `${P}.headers.description`,
      "params": {
        "CF-Access-Client-Id": {
          "name": "CF-Access-Client-Id",
          "description": "CF-Access-Client-Id",
          "type": "string",
          "required": true
        },
        "CF-Access-Client-Secret": {
          "name": "CF-Access-Client-Secret",
          "description": "CF-Access-Client-Secret",
          "type": "string",
          "required": true
        }
      }
    }
  },
  "25": {
    "code_execution": {
      "name": `${P}.codeExecution.name`,
      "description": `${P}.codeExecution.description`,
      "params": {
        "enable": enableParam(`${P}.codeExecution.enableDesc`)
      }
    },
    "use_openai_api": openaiApiPlugin('Gemini', `${P}.useOpenaiApi.enableDesc`)
  },

  "59": {
    "codex": {
      "name": `${P}.codex.name`,
      "description": `${P}.codex.description`,
      "params": {
        "images_main_model": {
          "name": `${P}.codex.imagesMainModel.name`,
          "description": `${P}.codex.imagesMainModel.description`,
          "type": "string",
          "required": false
        }
      }
    }
  },

  "27": {
    "voice": {
      "name": `${P}.voice.name`,
      "description": `${P}.voice.minimaxDescription`,
      "params": {
        "alloy": voiceParam('alloy', 'female-chengshu'),
        "echo": voiceParam('echo', 'male-qn-qingse'),
        "fable": voiceParam('fable', 'male-qn-jingying'),
        "onyx": voiceParam('onyx', 'presenter_male'),
        "nova": voiceParam('nova', 'presenter_female'),
        "shimmer": voiceParam('shimmer', 'audiobook_female_1')
      }
    }
  },

  "8": {
    "customize": {
      "name": `${P}.customize.name`,
      "description": `${P}.customize.description`,
      "params": {
        "1": endpointParam('ChatCompletions', '/v1/chat/completions'),
        "2": endpointParam('Completion', '/v1/completions'),
        "3": endpointParam('Embeddings', '/v1/embeddings'),
        "4": endpointParam('moderations', '/v1/moderations'),
        "5": endpointParam('ImagesGenerations', '/v1/images/generations'),
        "6": endpointParam('ImagesEdit', '/v1/images/edits'),
        "7": endpointParam('ImagesVariations', '/v1/images/variations'),
        "9": endpointParam('AudioSpeech', '/v1/audio/speech'),
        "10": endpointParam('AudioTranscriptions', '/v1/audio/transcriptions'),
        "11": endpointParam('AudioTranslations', '/v1/audio/translations'),
        "16": endpointParam('Responses', '/v1/responses')
      }
    }
  },
  "20": {
    "other": {
      "name": `${P}.override.name`,
      "description": `${P}.override.name`,
      "params": {
        "provider": {
          "name": `${P}.override.provider`,
          "description": [
            'channelForm.prompt.format',
            { format: '{"anthropic": { "order": ["Anthropic", "Amazon Bedrock"], "ignore": ["Google"], "allow_fallbacks": true }}' }
          ],
          "type": "string",
          "required": true
        }
      }
    },
    ...promptCachingPlugin
  }
}
;

export default pluginConfig;
