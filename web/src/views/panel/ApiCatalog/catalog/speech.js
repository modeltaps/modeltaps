// ==============================|| API CATALOG — SPEECH ||============================== //
// 「音频」模态的能力目录。端点逐条对照 router/relay-router.go:
//   setOpenAIRouter -> POST /v1/audio/speech、/v1/audio/transcriptions
//                      POST /v1/chat/completions(音频输入理解 / 音频输出对话)
//                      GET  /v1/realtime(WebSocket)
//   setSunoRouter   -> POST /suno/submit/:action、/suno/fetch、GET /suno/fetch/:id
// 每个能力的 availability 只引用后端目录字段(endpoints / 模态),由 availability.js 求值;
// Suno 等私有兼容入口收进页尾 extras 折叠区,不参与能力可用性。

import { curlOpenAI, curlPost, jsClient, pyClient, shellQuote } from './_helpers';

// 实时接口走 WebSocket:把站点地址的 http(s) 换成 ws(s)。
const wsUrl = (baseUrl, model) => `${String(baseUrl).replace(/^http/, 'ws')}/v1/realtime?model=${model}`;

const SPEECH_TEXT = 'hi~';
const AUDIO_FILE = 'speech.mp3';
const AUDIO_QUESTION = 'What is said in this audio?';
const AUDIO_PLACEHOLDER = 'BASE64_AUDIO';

const speech = {
  key: 'speech',
  titleKey: 'api_audio',
  apiNameKey: 'apiCatalogPage.speech.apiName',
  introKey: 'apiCatalogPage.speech.intro',
  modality: 'audio',
  docsPath: '/guide/api',
  capabilities: [
    {
      id: 'tts',
      titleKey: 'apiCatalogPage.speech.capabilities.tts.title',
      descKey: 'apiCatalogPage.speech.capabilities.tts.desc',
      endpoint: { method: 'POST', path: '/v1/audio/speech' },
      transport: 'audio.speech',
      availability: { endpoints: ['audio.speech'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlOpenAI(baseUrl, apiKey, '/v1/audio/speech', { model, input: SPEECH_TEXT, voice: 'alloy' }, [`--output ${AUDIO_FILE}`]),
        python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
with client.audio.speech.with_streaming_response.create(
    model="${model}",
    voice="alloy",
    input="${SPEECH_TEXT}",
) as response:
    response.stream_to_file("${AUDIO_FILE}")`,
        javascript: ({ baseUrl, apiKey, model }) => `import fs from 'node:fs';
${jsClient(baseUrl, apiKey)}
const response = await client.audio.speech.create({
  model: '${model}',
  voice: 'alloy',
  input: '${SPEECH_TEXT}'
});
fs.writeFileSync('${AUDIO_FILE}', Buffer.from(await response.arrayBuffer()));`
      }
    },
    {
      id: 'stt',
      titleKey: 'apiCatalogPage.speech.capabilities.stt.title',
      descKey: 'apiCatalogPage.speech.capabilities.stt.desc',
      endpoint: { method: 'POST', path: '/v1/audio/transcriptions' },
      transport: 'audio.transcription',
      availability: { endpoints: ['audio.transcription'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlPost(`${baseUrl}/v1/audio/transcriptions`, [
            `--header ${shellQuote(`Authorization: Bearer ${apiKey}`)}`,
            `--form ${shellQuote(`file=@${AUDIO_FILE}`)}`,
            `--form ${shellQuote(`model=${model}`)}`
          ]),
        python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
response = client.audio.transcriptions.create(
    model="${model}",
    file=open("${AUDIO_FILE}", "rb"),
)
print(response.text)`,
        javascript: ({ baseUrl, apiKey, model }) => `import fs from 'node:fs';
${jsClient(baseUrl, apiKey)}
const response = await client.audio.transcriptions.create({
  model: '${model}',
  file: fs.createReadStream('${AUDIO_FILE}')
});
console.log(response.text);`
      }
    },
    {
      id: 'audioInput',
      titleKey: 'apiCatalogPage.speech.capabilities.audioInput.title',
      descKey: 'apiCatalogPage.speech.capabilities.audioInput.desc',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'], inputModalities: ['audio'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlOpenAI(baseUrl, apiKey, '/v1/chat/completions', {
            model,
            messages: [
              {
                role: 'user',
                content: [
                  { type: 'text', text: AUDIO_QUESTION },
                  { type: 'input_audio', input_audio: { data: AUDIO_PLACEHOLDER, format: 'mp3' } }
                ]
              }
            ]
          }),
        python: ({ baseUrl, apiKey, model }) => `import base64

${pyClient(baseUrl, apiKey)}
audio = base64.b64encode(open("${AUDIO_FILE}", "rb").read()).decode()

response = client.chat.completions.create(
    model="${model}",
    messages=[{"role": "user", "content": [
        {"type": "text", "text": "${AUDIO_QUESTION}"},
        {"type": "input_audio", "input_audio": {"data": audio, "format": "mp3"}},
    ]}],
)
print(response.choices[0].message.content)`,
        javascript: ({ baseUrl, apiKey, model }) => `import fs from 'node:fs';
${jsClient(baseUrl, apiKey)}
const audio = fs.readFileSync('${AUDIO_FILE}').toString('base64');

const response = await client.chat.completions.create({
  model: '${model}',
  messages: [{ role: 'user', content: [
    { type: 'text', text: '${AUDIO_QUESTION}' },
    { type: 'input_audio', input_audio: { data: audio, format: 'mp3' } }
  ] }]
});
console.log(response.choices[0].message.content);`
      }
    },
    {
      id: 'audioOutput',
      titleKey: 'apiCatalogPage.speech.capabilities.audioOutput.title',
      descKey: 'apiCatalogPage.speech.capabilities.audioOutput.desc',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'], outputModalities: ['audio'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlOpenAI(baseUrl, apiKey, '/v1/chat/completions', {
            model,
            modalities: ['text', 'audio'],
            audio: { voice: 'alloy', format: 'wav' },
            messages: [{ role: 'user', content: SPEECH_TEXT }]
          }),
        python: ({ baseUrl, apiKey, model }) => `import base64

${pyClient(baseUrl, apiKey)}
response = client.chat.completions.create(
    model="${model}",
    modalities=["text", "audio"],
    audio={"voice": "alloy", "format": "wav"},
    messages=[{"role": "user", "content": "${SPEECH_TEXT}"}],
)
open("reply.wav", "wb").write(base64.b64decode(response.choices[0].message.audio.data))`,
        javascript: ({ baseUrl, apiKey, model }) => `import fs from 'node:fs';
${jsClient(baseUrl, apiKey)}
const response = await client.chat.completions.create({
  model: '${model}',
  modalities: ['text', 'audio'],
  audio: { voice: 'alloy', format: 'wav' },
  messages: [{ role: 'user', content: '${SPEECH_TEXT}' }]
});
fs.writeFileSync('reply.wav', Buffer.from(response.choices[0].message.audio.data, 'base64'));`
      }
    },
    {
      id: 'realtime',
      titleKey: 'apiCatalogPage.speech.capabilities.realtime.title',
      descKey: 'apiCatalogPage.speech.capabilities.realtime.desc',
      endpoint: { method: 'GET', path: '/v1/realtime' },
      transport: 'realtime',
      // GET /v1/realtime 是实装路由,不是「即将上线」;后端目录词表补上 realtime 之前,
      // 这里按普通规则求值,没有模型就照实显示「暂无可用模型」。
      availability: { endpoints: ['realtime'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) => `curl --include --no-buffer \\
  --header 'Authorization: Bearer ${apiKey}' \\
  --header 'Connection: Upgrade' \\
  --header 'Upgrade: websocket' \\
  --header 'Sec-WebSocket-Version: 13' \\
  --header 'Sec-WebSocket-Key: SGVsbG8sIHdvcmxkIQ==' \\
  --url '${baseUrl}/v1/realtime?model=${model}'`,
        python: ({ baseUrl, apiKey, model }) => `import asyncio
import json

import websockets


async def main():
    async with websockets.connect(
        "${wsUrl(baseUrl, model)}",
        additional_headers={"Authorization": "Bearer ${apiKey}"},
    ) as socket:
        await socket.send(json.dumps({"type": "response.create"}))
        print(await socket.recv())


asyncio.run(main())`,
        javascript: ({ baseUrl, apiKey, model }) => `import WebSocket from 'ws';

const socket = new WebSocket('${wsUrl(baseUrl, model)}', {
  headers: { Authorization: 'Bearer ${apiKey}' }
});

socket.on('open', () => socket.send(JSON.stringify({ type: 'response.create' })));
socket.on('message', (data) => console.log(data.toString()));`
      }
    }
  ],
  // 私有兼容入口:不进主视图,收在页尾折叠区。
  extras: {
    titleKey: 'apiCatalogPage.speech.extras.title',
    groups: [
      {
        id: 'suno',
        titleKey: 'apiCatalogPage.speech.extras.suno',
        endpoints: [
          { method: 'POST', path: '/suno/submit/music', descKey: 'apiCatalogPage.speech.extras.endpoints.sunoMusic' },
          { method: 'POST', path: '/suno/submit/lyrics', descKey: 'apiCatalogPage.speech.extras.endpoints.sunoLyrics' },
          { method: 'POST', path: '/suno/fetch', descKey: 'apiCatalogPage.speech.extras.endpoints.sunoFetch' },
          { method: 'GET', path: '/suno/fetch/{id}', descKey: 'apiCatalogPage.speech.extras.endpoints.sunoFetchById' }
        ],
        link: { to: '/panel/log?tab=task', labelKey: 'apiCatalogPage.speech.extras.tasks' }
      }
    ]
  }
};

export default speech;
