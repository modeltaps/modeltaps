// ==============================|| API CATALOG — VIDEO ||============================== //
// 「视频」模态的能力目录。端点逐条对照 router/relay-router.go:
//   setGeminiRouter -> POST /gemini/:version/models/*action(视频输入理解;Veo 走 :predictLongRunning)
//   setKlingRouter  -> POST /kling/v1/:class/:action(class 只允许 videos,action 为 text2video/image2video)
//                      GET  /kling/v1/videos/text2video/:id、/kling/v1/videos/image2video/:id
// 视频理解只给 Gemini 兼容写法:OpenAI 兼容的 chat/completions 会把请求解析成
// types.ChatMessagePart,那里没有视频 content part,传什么都会被丢掉;Gemini 路由的
// inlineData 则是原样转发的。视频生成按行业事实标准的三步形态(创建 → 轮询 → 取文件)
// 描述,后端还没有 /v1/videos 路由,标 planned;Kling / Veo 私有入口收进页尾 extras 折叠区。

import { curlJson, curlOpenAI, jsClient, pyClient } from './_helpers';

const VIDEO_FILE = 'clip.mp4';
const VIDEO_MIME = 'video/mp4';
const VIDEO_PLACEHOLDER = 'BASE64_VIDEO';
const VIDEO_QUESTION = 'What happens in this video?';
const VIDEO_PROMPT = 'a red panda walking through a bamboo forest';

const video = {
  key: 'video',
  titleKey: 'api_video',
  apiNameKey: 'apiCatalogPage.video.apiName',
  introKey: 'apiCatalogPage.video.intro',
  modality: 'video',
  docsPath: '/guide/api',
  capabilities: [
    {
      id: 'understanding',
      titleKey: 'apiCatalogPage.video.capabilities.understanding.title',
      descKey: 'apiCatalogPage.video.capabilities.understanding.desc',
      noteKey: 'apiCatalogPage.video.capabilities.understanding.note',
      endpoint: { method: 'POST', path: '/gemini/v1beta/models/{model}:generateContent' },
      transport: 'gemini',
      enabledFlag: 'GeminiAPIEnabled',
      availability: { endpoints: ['chat'], inputModalities: ['video'], vendors: ['google'] },
      preferredModels: ['gemini-3.1-pro-preview', 'gemini-2.5-pro'],
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlJson(
            `${baseUrl}/gemini/v1beta/models/${model}:generateContent`,
            ['Content-Type: application/json', `x-goog-api-key: ${apiKey}`],
            {
              contents: [
                {
                  parts: [{ text: VIDEO_QUESTION }, { inlineData: { mimeType: VIDEO_MIME, data: VIDEO_PLACEHOLDER } }]
                }
              ]
            }
          ),
        python: ({ baseUrl, apiKey, model }) => `from google import genai
from google.genai import types

client = genai.Client(
    api_key="${apiKey}",
    http_options={"base_url": "${baseUrl}/gemini"},
)

response = client.models.generate_content(
    model="${model}",
    contents=[
        "${VIDEO_QUESTION}",
        types.Part.from_bytes(data=open("${VIDEO_FILE}", "rb").read(), mime_type="${VIDEO_MIME}"),
    ],
)
print(response.text)`,
        javascript: ({ baseUrl, apiKey, model }) => `import fs from 'node:fs';
import { GoogleGenAI } from '@google/genai';

const client = new GoogleGenAI({ apiKey: '${apiKey}', httpOptions: { baseUrl: '${baseUrl}/gemini' } });

const clip = fs.readFileSync('${VIDEO_FILE}').toString('base64');

const response = await client.models.generateContent({
  model: '${model}',
  contents: [{ text: '${VIDEO_QUESTION}' }, { inlineData: { mimeType: '${VIDEO_MIME}', data: clip } }]
});
console.log(response.text);`
      }
    },
    {
      id: 'generation',
      titleKey: 'apiCatalogPage.video.capabilities.generation.title',
      descKey: 'apiCatalogPage.video.capabilities.generation.desc',
      endpoint: { method: 'POST', path: '/v1/videos' },
      transport: 'videos',
      // router/relay-router.go 里没有 /v1/videos 这条路由,后端确实还没实装 ——
      // 标 planned,chip 显示「即将上线」而不是「暂无可用模型」。
      // 示例先按行业标准三步形态给出,供接入方预览。
      status: 'planned',
      availability: { endpoints: ['videos'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) => `${curlOpenAI(baseUrl, apiKey, '/v1/videos', { model, prompt: VIDEO_PROMPT, seconds: '8' })}

# Poll the task status
curl --url ${baseUrl}/v1/videos/{id} \\
  --header 'Authorization: Bearer ${apiKey}'

# Download the video once the task completes
curl --url ${baseUrl}/v1/videos/{id}/content \\
  --header 'Authorization: Bearer ${apiKey}' \\
  --output video.mp4`,
        python: ({ baseUrl, apiKey, model }) => `import time

${pyClient(baseUrl, apiKey)}
video = client.videos.create(
    model="${model}",
    prompt="${VIDEO_PROMPT}",
    seconds="8",
)

while video.status in ("queued", "in_progress"):
    time.sleep(5)
    video = client.videos.retrieve(video.id)

client.videos.download_content(video.id).write_to_file("video.mp4")`,
        javascript: ({ baseUrl, apiKey, model }) => `import fs from 'node:fs';
${jsClient(baseUrl, apiKey)}
let video = await client.videos.create({
  model: '${model}',
  prompt: '${VIDEO_PROMPT}',
  seconds: '8'
});

while (video.status === 'queued' || video.status === 'in_progress') {
  await new Promise((resolve) => setTimeout(resolve, 5000));
  video = await client.videos.retrieve(video.id);
}

const content = await client.videos.downloadContent(video.id);
fs.writeFileSync('video.mp4', Buffer.from(await content.arrayBuffer()));`
      }
    }
  ],
  // 私有兼容入口:不进主视图,收在页尾折叠区。
  extras: {
    titleKey: 'apiCatalogPage.video.extras.title',
    groups: [
      {
        id: 'kling',
        titleKey: 'apiCatalogPage.video.extras.kling',
        endpoints: [
          { method: 'POST', path: '/kling/v1/videos/text2video', descKey: 'apiCatalogPage.video.extras.endpoints.klingText2video' },
          { method: 'POST', path: '/kling/v1/videos/image2video', descKey: 'apiCatalogPage.video.extras.endpoints.klingImage2video' },
          {
            method: 'GET',
            path: '/kling/v1/videos/text2video/{id}',
            descKey: 'apiCatalogPage.video.extras.endpoints.klingFetchText2video'
          },
          {
            method: 'GET',
            path: '/kling/v1/videos/image2video/{id}',
            descKey: 'apiCatalogPage.video.extras.endpoints.klingFetchImage2video'
          }
        ],
        // 任务记录落在日志页的「异步任务」Tab(N1 已合并)。
        link: { to: '/panel/log?tab=task', labelKey: 'apiCatalogPage.video.extras.tasks' }
      },
      {
        id: 'veo',
        titleKey: 'apiCatalogPage.video.extras.veo',
        endpoints: [
          {
            method: 'POST',
            path: '/gemini/v1beta/models/{model}:predictLongRunning',
            descKey: 'apiCatalogPage.video.extras.endpoints.veoPredict'
          }
        ]
      }
    ]
  }
};

export default video;
