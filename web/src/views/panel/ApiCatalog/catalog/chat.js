// ==============================|| API CATALOG — CHAT ||============================== //
// 「对话」模态的能力目录。端点逐条对照 router/relay-router.go:
//   setOpenAIRouter  -> POST /v1/chat/completions、POST /v1/responses
//   setClaudeRouter  -> POST /claude/v1/messages
//   setGeminiRouter  -> POST /gemini/:version/models/*action
// 每个能力的 availability 只引用后端目录字段(endpoints / 模态 / capabilities),
// 由 availability.js 求值;示例随所选模型实时生成,密钥恒为占位符。

import { curlJson, curlOpenAI, jsClient, pyClient, pyLiteral } from './_helpers';

const USER_TEXT = 'hi~';
const IMAGE_URL = 'https://example.com/cat.png';
// 文件输入走 base64 data URL(OpenAI 规范:file_data 是 base64 内容,不接受 https 链接);
// curl 里给占位符,与 speech.js 的 BASE64_AUDIO 同一套约定。
const FILE_NAME = 'report.pdf';
const FILE_QUESTION = 'Summarize this document.';
const FILE_DATA_PREFIX = 'data:application/pdf;base64,';
const FILE_PLACEHOLDER = 'BASE64_PDF';

// 最常见的一问一答:三语言各一段,供多数能力复用(额外参数由 extra 注入)。
const basicExamples = ({ curlExtra = {}, pyExtra = '', jsExtra = '' } = {}) => ({
  curl: ({ baseUrl, apiKey, model }) =>
    curlOpenAI(baseUrl, apiKey, '/v1/chat/completions', {
      model,
      messages: [{ role: 'user', content: USER_TEXT }],
      ...curlExtra
    }),
  python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
response = client.chat.completions.create(
    model="${model}",
    messages=[{"role": "user", "content": "${USER_TEXT}"}],${pyExtra}
)
print(response.choices[0].message.content)`,
  javascript: ({ baseUrl, apiKey, model }) => `${jsClient(baseUrl, apiKey)}
const response = await client.chat.completions.create({
  model: '${model}',
  messages: [{ role: 'user', content: '${USER_TEXT}' }]${jsExtra}
});
console.log(response.choices[0].message.content);`
});

// 多模态输入(图片 / 文件):三语言共用同一段 content 数组,只有承载 URL 的字段不同。
const partsExamples = (parts, pyParts, jsParts) => ({
  curl: ({ baseUrl, apiKey, model }) =>
    curlOpenAI(baseUrl, apiKey, '/v1/chat/completions', {
      model,
      messages: [{ role: 'user', content: parts }]
    }),
  python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
response = client.chat.completions.create(
    model="${model}",
    messages=[{"role": "user", "content": ${pyParts}}],
)
print(response.choices[0].message.content)`,
  javascript: ({ baseUrl, apiKey, model }) => `${jsClient(baseUrl, apiKey)}
const response = await client.chat.completions.create({
  model: '${model}',
  messages: [{ role: 'user', content: ${jsParts} }]
});
console.log(response.choices[0].message.content);`
});

const WEATHER_TOOL = {
  type: 'function',
  function: {
    name: 'get_weather',
    description: 'Get the weather of a city',
    parameters: {
      type: 'object',
      properties: { city: { type: 'string' } },
      required: ['city']
    }
  }
};

// strict + additionalProperties: false 是 OpenAI 结构化输出给出 100% schema 遵从保证的前提,
// 缺了就只是「尽力而为」。
const JSON_SCHEMA = {
  type: 'json_schema',
  json_schema: {
    name: 'city',
    strict: true,
    schema: {
      type: 'object',
      properties: { city: { type: 'string' }, country: { type: 'string' } },
      required: ['city', 'country'],
      additionalProperties: false
    }
  }
};

const chat = {
  key: 'chat',
  // 页面标题走导航同款顶层 key(t('api_chat')),与 N3 的侧栏条目共用文案。
  titleKey: 'api_chat',
  apiNameKey: 'apiCatalogPage.chat.apiName',
  introKey: 'apiCatalogPage.chat.intro',
  // 「可用模型」区嵌入模型广场时预选的模态。
  modality: 'text',
  docsPath: '/guide/api',
  capabilities: [
    {
      id: 'text',
      titleKey: 'apiCatalogPage.chat.capabilities.text.title',
      descKey: 'apiCatalogPage.chat.capabilities.text.desc',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'] },
      examples: basicExamples()
    },
    {
      id: 'streaming',
      titleKey: 'apiCatalogPage.chat.capabilities.streaming.title',
      descKey: 'apiCatalogPage.chat.capabilities.streaming.desc',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlOpenAI(baseUrl, apiKey, '/v1/chat/completions', {
            model,
            messages: [{ role: 'user', content: USER_TEXT }],
            stream: true
          }),
        python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
stream = client.chat.completions.create(
    model="${model}",
    messages=[{"role": "user", "content": "${USER_TEXT}"}],
    stream=True,
)
for chunk in stream:
    print(chunk.choices[0].delta.content or "", end="")`,
        javascript: ({ baseUrl, apiKey, model }) => `${jsClient(baseUrl, apiKey)}
const stream = await client.chat.completions.create({
  model: '${model}',
  messages: [{ role: 'user', content: '${USER_TEXT}' }],
  stream: true
});
for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content ?? '');
}`
      }
    },
    {
      id: 'reasoning',
      titleKey: 'apiCatalogPage.chat.capabilities.reasoning.title',
      descKey: 'apiCatalogPage.chat.capabilities.reasoning.desc',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'], capabilities: ['reasoning'] },
      examples: basicExamples({
        curlExtra: { reasoning_effort: 'medium' },
        pyExtra: '\n    reasoning_effort="medium",',
        jsExtra: ",\n  reasoning_effort: 'medium'"
      })
    },
    {
      id: 'structured',
      titleKey: 'apiCatalogPage.chat.capabilities.structured.title',
      descKey: 'apiCatalogPage.chat.capabilities.structured.desc',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'], capabilities: ['structured_output'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlOpenAI(baseUrl, apiKey, '/v1/chat/completions', {
            model,
            messages: [{ role: 'user', content: 'Where is the Eiffel Tower?' }],
            response_format: JSON_SCHEMA
          }),
        python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
response = client.chat.completions.create(
    model="${model}",
    messages=[{"role": "user", "content": "Where is the Eiffel Tower?"}],
    response_format=${pyLiteral(JSON_SCHEMA, 1)},
)
print(response.choices[0].message.content)`,
        javascript: ({ baseUrl, apiKey, model }) => `${jsClient(baseUrl, apiKey)}
const response = await client.chat.completions.create({
  model: '${model}',
  messages: [{ role: 'user', content: 'Where is the Eiffel Tower?' }],
  response_format: ${JSON.stringify(JSON_SCHEMA)}
});
console.log(response.choices[0].message.content);`
      }
    },
    {
      id: 'tools',
      titleKey: 'apiCatalogPage.chat.capabilities.tools.title',
      descKey: 'apiCatalogPage.chat.capabilities.tools.desc',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'], capabilities: ['tool_call'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlOpenAI(baseUrl, apiKey, '/v1/chat/completions', {
            model,
            messages: [{ role: 'user', content: "What's the weather in Paris?" }],
            tools: [WEATHER_TOOL]
          }),
        python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
response = client.chat.completions.create(
    model="${model}",
    messages=[{"role": "user", "content": "What's the weather in Paris?"}],
    tools=${pyLiteral([WEATHER_TOOL], 1)},
)
print(response.choices[0].message.tool_calls)`,
        javascript: ({ baseUrl, apiKey, model }) => `${jsClient(baseUrl, apiKey)}
const response = await client.chat.completions.create({
  model: '${model}',
  messages: [{ role: 'user', content: "What's the weather in Paris?" }],
  tools: [${JSON.stringify(WEATHER_TOOL)}]
});
console.log(response.choices[0].message.tool_calls);`
      }
    },
    {
      id: 'vision',
      titleKey: 'apiCatalogPage.chat.capabilities.vision.title',
      descKey: 'apiCatalogPage.chat.capabilities.vision.desc',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'], inputModalities: ['image'] },
      examples: partsExamples(
        [
          { type: 'text', text: 'What is in this image?' },
          { type: 'image_url', image_url: { url: IMAGE_URL } }
        ],
        `[
        {"type": "text", "text": "What is in this image?"},
        {"type": "image_url", "image_url": {"url": "${IMAGE_URL}"}},
    ]`,
        `[
    { type: 'text', text: 'What is in this image?' },
    { type: 'image_url', image_url: { url: '${IMAGE_URL}' } }
  ]`
      )
    },
    {
      id: 'files',
      titleKey: 'apiCatalogPage.chat.capabilities.files.title',
      descKey: 'apiCatalogPage.chat.capabilities.files.desc',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'], inputModalities: ['file'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlOpenAI(baseUrl, apiKey, '/v1/chat/completions', {
            model,
            messages: [
              {
                role: 'user',
                content: [
                  { type: 'text', text: FILE_QUESTION },
                  { type: 'file', file: { filename: FILE_NAME, file_data: `${FILE_DATA_PREFIX}${FILE_PLACEHOLDER}` } }
                ]
              }
            ]
          }),
        python: ({ baseUrl, apiKey, model }) => `import base64

${pyClient(baseUrl, apiKey)}
pdf = base64.b64encode(open("${FILE_NAME}", "rb").read()).decode()

response = client.chat.completions.create(
    model="${model}",
    messages=[{"role": "user", "content": [
        {"type": "text", "text": "${FILE_QUESTION}"},
        {"type": "file", "file": {"filename": "${FILE_NAME}", "file_data": f"${FILE_DATA_PREFIX}{pdf}"}},
    ]}],
)
print(response.choices[0].message.content)`,
        javascript: ({ baseUrl, apiKey, model }) => `import fs from 'node:fs';
${jsClient(baseUrl, apiKey)}
const pdf = fs.readFileSync('${FILE_NAME}').toString('base64');

const response = await client.chat.completions.create({
  model: '${model}',
  messages: [{ role: 'user', content: [
    { type: 'text', text: '${FILE_QUESTION}' },
    { type: 'file', file: { filename: '${FILE_NAME}', file_data: \`${FILE_DATA_PREFIX}\${pdf}\` } }
  ] }]
});
console.log(response.choices[0].message.content);`
      }
    },
    {
      id: 'responses',
      titleKey: 'apiCatalogPage.chat.capabilities.responses.title',
      descKey: 'apiCatalogPage.chat.capabilities.responses.desc',
      endpoint: { method: 'POST', path: '/v1/responses' },
      transport: 'responses',
      availability: { endpoints: ['responses'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) => curlOpenAI(baseUrl, apiKey, '/v1/responses', { model, input: USER_TEXT }),
        python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
response = client.responses.create(
    model="${model}",
    input="${USER_TEXT}",
)
print(response.output_text)`,
        javascript: ({ baseUrl, apiKey, model }) => `${jsClient(baseUrl, apiKey)}
const response = await client.responses.create({
  model: '${model}',
  input: '${USER_TEXT}'
});
console.log(response.output_text);`
      }
    },
    {
      id: 'claude',
      titleKey: 'apiCatalogPage.chat.capabilities.claude.title',
      descKey: 'apiCatalogPage.chat.capabilities.claude.desc',
      endpoint: { method: 'POST', path: '/claude/v1/messages' },
      transport: 'claude',
      // 站点可以单独关闭 Claude 协议入口（middleware/api-enabled.go 会 403），关掉时整节不渲染。
      enabledFlag: 'ClaudeAPIEnabled',
      availability: { endpoints: ['chat'], vendors: ['anthropic'] },
      preferredModels: ['claude-sonnet-5', 'claude-opus-5'],
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlJson(
            `${baseUrl}/claude/v1/messages`,
            ['Content-Type: application/json', `x-api-key: ${apiKey}`, 'anthropic-version: 2023-06-01'],
            { model, max_tokens: 1024, messages: [{ role: 'user', content: USER_TEXT }] }
          ),
        python: ({ baseUrl, apiKey, model }) => `from anthropic import Anthropic

client = Anthropic(api_key="${apiKey}", base_url="${baseUrl}/claude")

message = client.messages.create(
    model="${model}",
    max_tokens=1024,
    messages=[{"role": "user", "content": "${USER_TEXT}"}],
)
print(message.content[0].text)`,
        javascript: ({ baseUrl, apiKey, model }) => `import Anthropic from '@anthropic-ai/sdk';

const client = new Anthropic({ apiKey: '${apiKey}', baseURL: '${baseUrl}/claude' });

const message = await client.messages.create({
  model: '${model}',
  max_tokens: 1024,
  messages: [{ role: 'user', content: '${USER_TEXT}' }]
});
console.log(message.content[0].text);`
      }
    },
    {
      id: 'gemini',
      titleKey: 'apiCatalogPage.chat.capabilities.gemini.title',
      descKey: 'apiCatalogPage.chat.capabilities.gemini.desc',
      endpoint: { method: 'POST', path: '/gemini/v1beta/models/{model}:generateContent' },
      transport: 'gemini',
      enabledFlag: 'GeminiAPIEnabled',
      availability: { endpoints: ['chat'], vendors: ['google'] },
      preferredModels: ['gemini-3.1-pro-preview', 'gemini-2.5-pro'],
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlJson(
            `${baseUrl}/gemini/v1beta/models/${model}:generateContent`,
            ['Content-Type: application/json', `x-goog-api-key: ${apiKey}`],
            {
              contents: [{ parts: [{ text: USER_TEXT }] }]
            }
          ),
        python: ({ baseUrl, apiKey, model }) => `from google import genai

client = genai.Client(
    api_key="${apiKey}",
    http_options={"base_url": "${baseUrl}/gemini"},
)

response = client.models.generate_content(
    model="${model}",
    contents="${USER_TEXT}",
)
print(response.text)`,
        javascript: ({ baseUrl, apiKey, model }) => `import { GoogleGenAI } from '@google/genai';

const client = new GoogleGenAI({ apiKey: '${apiKey}', httpOptions: { baseUrl: '${baseUrl}/gemini' } });

const response = await client.models.generateContent({
  model: '${model}',
  contents: '${USER_TEXT}'
});
console.log(response.text);`
      }
    }
  ]
};

export default chat;
