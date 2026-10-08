// ==============================|| API CATALOG — IMAGE ||============================== //
// 「图像」模态的能力目录。同一件事(生成 / 编辑)在本站有两种写法,各自独立计算可用性:
//   Images API   -> POST /v1/images/generations、/v1/images/edits(setOpenAIRouter),
//                   可用性 = endpoints 含 images
//   Chat 写法    -> POST /v1/chat/completions + modalities: ["image","text"](OpenRouter),
//                   可用性 = endpoints 含 chat ∩ 输出模态含 image
// 图片理解 = endpoints 含 chat ∩ 输入模态含 image。
// Recraft / Midjourney 等兼容入口收进页尾 extras 折叠区,不参与能力可用性。

import { curlOpenAI, jsClient, pyClient } from './_helpers';

const PROMPT = 'a red panda coding at night';
const EDIT_PROMPT = 'make the background a snowy forest';
const IMAGE_URL = 'https://example.com/cat.png';
const SIZE = '1024x1024';

// chat 写法:messages 里给提示词,modalities 声明要图片输出,回包在 message.images 里。
const chatImageExamples = (parts, pyParts, jsParts) => ({
  curl: ({ baseUrl, apiKey, model }) =>
    curlOpenAI(baseUrl, apiKey, '/v1/chat/completions', {
      model,
      messages: [{ role: 'user', content: parts }],
      modalities: ['image', 'text']
    }),
  python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
response = client.chat.completions.create(
    model="${model}",
    messages=[{"role": "user", "content": ${pyParts}}],
    extra_body={"modalities": ["image", "text"]},
)
print(response.choices[0].message.images)`,
  // JS 示例照运行时写法给 modalities:openai SDK 的 TS 类型只认 text / audio,
  // 所以示例里挑明 TS 工程要怎么绕过,免得复制过去编译不过。
  javascript: ({ baseUrl, apiKey, model }) => `${jsClient(baseUrl, apiKey)}
const response = await client.chat.completions.create({
  model: '${model}',
  messages: [{ role: 'user', content: ${jsParts} }],
  // modalities and message.images are gateway extensions missing from the openai SDK types:
  // in a TypeScript project use "as any", or add // @ts-expect-error above this line.
  modalities: ['image', 'text']
});
console.log(response.choices[0].message.images);`
});

const image = {
  key: 'image',
  titleKey: 'api_image',
  apiNameKey: 'apiCatalogPage.image.apiName',
  introKey: 'apiCatalogPage.image.intro',
  modality: 'image',
  docsPath: '/guide/api',
  capabilities: [
    {
      id: 'generations',
      titleKey: 'apiCatalogPage.image.capabilities.generations.title',
      descKey: 'apiCatalogPage.image.capabilities.generations.desc',
      endpoint: { method: 'POST', path: '/v1/images/generations' },
      transport: 'images',
      availability: { endpoints: ['images'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlOpenAI(baseUrl, apiKey, '/v1/images/generations', { model, prompt: PROMPT, n: 1, size: SIZE }),
        python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
response = client.images.generate(
    model="${model}",
    prompt="${PROMPT}",
    n=1,
    size="${SIZE}",
)
print(response.data[0].url)`,
        javascript: ({ baseUrl, apiKey, model }) => `${jsClient(baseUrl, apiKey)}
const response = await client.images.generate({
  model: '${model}',
  prompt: '${PROMPT}',
  n: 1,
  size: '${SIZE}'
});
console.log(response.data[0].url);`
      }
    },
    {
      id: 'chatGenerations',
      titleKey: 'apiCatalogPage.image.capabilities.chatGenerations.title',
      descKey: 'apiCatalogPage.image.capabilities.chatGenerations.desc',
      noteKey: 'apiCatalogPage.image.capabilities.chatGenerations.note',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'], outputModalities: ['image'] },
      preferredModels: ['gemini-3.1-flash-image', 'gemini-3-pro-image', 'gpt-5.4-image-2'],
      examples: chatImageExamples(PROMPT, `"${PROMPT}"`, `'${PROMPT}'`)
    },
    {
      id: 'edits',
      titleKey: 'apiCatalogPage.image.capabilities.edits.title',
      descKey: 'apiCatalogPage.image.capabilities.edits.desc',
      endpoint: { method: 'POST', path: '/v1/images/edits' },
      transport: 'images',
      availability: { endpoints: ['images'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) => `curl --request POST \\
  --url ${baseUrl}/v1/images/edits \\
  --header 'Authorization: Bearer ${apiKey}' \\
  --form 'model=${model}' \\
  --form 'image=@cat.png' \\
  --form 'prompt=${EDIT_PROMPT}'`,
        python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
response = client.images.edit(
    model="${model}",
    image=open("cat.png", "rb"),
    prompt="${EDIT_PROMPT}",
)
print(response.data[0].url)`,
        javascript: ({ baseUrl, apiKey, model }) => `import fs from 'node:fs';
${jsClient(baseUrl, apiKey)}
const response = await client.images.edit({
  model: '${model}',
  image: fs.createReadStream('cat.png'),
  prompt: '${EDIT_PROMPT}'
});
console.log(response.data[0].url);`
      }
    },
    {
      id: 'chatEdits',
      titleKey: 'apiCatalogPage.image.capabilities.chatEdits.title',
      descKey: 'apiCatalogPage.image.capabilities.chatEdits.desc',
      noteKey: 'apiCatalogPage.image.capabilities.chatEdits.note',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'], inputModalities: ['image'], outputModalities: ['image'] },
      preferredModels: ['gemini-3.1-flash-image', 'gemini-3-pro-image', 'gpt-5.4-image-2'],
      examples: chatImageExamples(
        [
          { type: 'text', text: EDIT_PROMPT },
          { type: 'image_url', image_url: { url: IMAGE_URL } }
        ],
        `[
        {"type": "text", "text": "${EDIT_PROMPT}"},
        {"type": "image_url", "image_url": {"url": "${IMAGE_URL}"}},
    ]`,
        `[
    { type: 'text', text: '${EDIT_PROMPT}' },
    { type: 'image_url', image_url: { url: '${IMAGE_URL}' } }
  ]`
      )
    },
    {
      id: 'vision',
      titleKey: 'apiCatalogPage.image.capabilities.vision.title',
      descKey: 'apiCatalogPage.image.capabilities.vision.desc',
      endpoint: { method: 'POST', path: '/v1/chat/completions' },
      transport: 'chat',
      availability: { endpoints: ['chat'], inputModalities: ['image'] },
      examples: {
        curl: ({ baseUrl, apiKey, model }) =>
          curlOpenAI(baseUrl, apiKey, '/v1/chat/completions', {
            model,
            messages: [
              {
                role: 'user',
                content: [
                  { type: 'text', text: 'What is in this image?' },
                  { type: 'image_url', image_url: { url: IMAGE_URL } }
                ]
              }
            ]
          }),
        python: ({ baseUrl, apiKey, model }) => `${pyClient(baseUrl, apiKey)}
response = client.chat.completions.create(
    model="${model}",
    messages=[{"role": "user", "content": [
        {"type": "text", "text": "What is in this image?"},
        {"type": "image_url", "image_url": {"url": "${IMAGE_URL}"}},
    ]}],
)
print(response.choices[0].message.content)`,
        javascript: ({ baseUrl, apiKey, model }) => `${jsClient(baseUrl, apiKey)}
const response = await client.chat.completions.create({
  model: '${model}',
  messages: [{ role: 'user', content: [
    { type: 'text', text: 'What is in this image?' },
    { type: 'image_url', image_url: { url: '${IMAGE_URL}' } }
  ] }]
});
console.log(response.choices[0].message.content);`
      }
    }
  ],
  // 页尾折叠的兼容入口:非主流写法,保留给已在用的调用方。
  extras: {
    titleKey: 'apiCatalogPage.image.extras.title',
    hintKey: 'apiCatalogPage.image.extras.hint',
    groups: [
      {
        id: 'recraft',
        titleKey: 'apiCatalogPage.image.extras.recraft',
        endpoints: [
          { method: 'POST', path: '/recraftAI/v1/images/generations', descKey: 'apiCatalogPage.image.endpoints.recraftGenerations' },
          { method: 'POST', path: '/recraftAI/v1/images/vectorize', descKey: 'apiCatalogPage.image.endpoints.recraftVectorize' },
          {
            method: 'POST',
            path: '/recraftAI/v1/images/removeBackground',
            descKey: 'apiCatalogPage.image.endpoints.recraftRemoveBackground'
          },
          {
            method: 'POST',
            path: '/recraftAI/v1/images/clarityUpscale',
            descKey: 'apiCatalogPage.image.endpoints.recraftClarityUpscale'
          },
          {
            method: 'POST',
            path: '/recraftAI/v1/images/generativeUpscale',
            descKey: 'apiCatalogPage.image.endpoints.recraftGenerativeUpscale'
          }
        ]
      },
      {
        id: 'midjourney',
        titleKey: 'apiCatalogPage.image.extras.midjourney',
        endpoints: [
          { method: 'POST', path: '/mj/submit/imagine', descKey: 'apiCatalogPage.image.endpoints.mjImagine' },
          { method: 'POST', path: '/mj/submit/change', descKey: 'apiCatalogPage.image.endpoints.mjChange' },
          { method: 'POST', path: '/mj/submit/blend', descKey: 'apiCatalogPage.image.endpoints.mjBlend' },
          { method: 'POST', path: '/mj/submit/describe', descKey: 'apiCatalogPage.image.endpoints.mjDescribe' },
          { method: 'GET', path: '/mj/task/{id}/fetch', descKey: 'apiCatalogPage.image.endpoints.mjFetch' }
        ],
        // 任务记录落在日志页的 Midjourney Tab,这里给一条直达链接。
        link: { to: '/panel/log?tab=midjourney', labelKey: 'apiCatalogPage.image.extras.midjourneyTasks' }
      }
    ]
  }
};

export default image;
