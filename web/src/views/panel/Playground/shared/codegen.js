import { curlOpenAI, curlPost, indent, jsClient, pyClient, pyLiteral, shellQuote } from 'views/panel/ApiCatalog/catalog/_helpers';

// ==============================|| PLAYGROUND — CODE GENERATOR ||============================== //
// 「代码」视图的纯函数生成器：把一次运行设置(模型 + 参数 + 输入)翻译成 curl / Python / JavaScript
// 三段可直接复制运行的代码。引号转义、Python 字面量、客户端前缀一律复用目录页的 _helpers，
// 两处示例口径才不会漂。密钥恒为占位符，真实 Key 不进生成结果。

export const API_KEY_PLACEHOLDER = 'sk-YOUR_TOKEN';

// 没选中模型时（站点暂无可用模型）代码里给占位名，空串会被复制成一段发不出去的请求。
export const MODEL_PLACEHOLDER = '<model>';

export const CODE_LANGUAGES = ['curl', 'python', 'javascript'];

// 模态 → 默认端点，与 relay 路由一致；调用方可用 endpoint 覆盖。
export const MODALITY_PATHS = {
  chat: '/v1/chat/completions',
  image: '/v1/images/generations',
  'image.edit': '/v1/images/edits',
  'image.chat': '/v1/chat/completions',
  'audio.speech': '/v1/audio/speech',
  'audio.transcription': '/v1/audio/transcriptions'
};

// 上传文件只拿得到文件名，拿不到磁盘路径；代码里给占位路径(与目录页同一套约定)。
export const AUDIO_FILE_PLACEHOLDER = '/path/to/audio.mp3';
export const IMAGE_FILE_PLACEHOLDER = '/path/to/reference.png';
// 对话出图的参考图是消息里的 data URL，代码里同样给占位串，不把整段 base64 塞进示例。
export const IMAGE_DATA_URL_PLACEHOLDER = 'data:image/png;base64,<BASE64_IMAGE>';

// 对话出图的请求体：modalities 与回包里的 message.images 是 OpenRouter 风格扩展。
// 带参考图时用户消息改成 text + image_url 两个内容块。
export function chatImageBody({ model, prompt, imageUrl }) {
  const content = imageUrl
    ? [
        { type: 'text', text: prompt },
        { type: 'image_url', image_url: { url: imageUrl } }
      ]
    : prompt;
  return { model, messages: [{ role: 'user', content }], modalities: ['image', 'text'] };
}

// 「只输出用户实际设置的参数」：未设置(undefined / null / 空串)的一律不进请求体，
// 0 与 false 是用户设过的值，保留。
const compact = (params = {}) =>
  Object.fromEntries(Object.entries(params).filter(([, value]) => value !== undefined && value !== null && value !== ''));

// Python 关键字实参 / JavaScript 对象字面量：值分别走 pyLiteral 与 JSON(JSON 是合法 JS 字面量)。
const pyKwargs = (body, level = 1) =>
  Object.entries(body)
    .map(([key, value]) => `${' '.repeat(level * 4)}${key}=${pyLiteral(value, level)},`)
    .join('\n');

const jsProps = (body) =>
  Object.entries(body)
    .map(([key, value]) => `  ${key}: ${indent(value)}`)
    .join(',\n');

// 对话消息：丢掉内容为空的轮次；includeHistory=false 时只留系统提示词与最后一条用户消息。
export function chatMessages(messages = [], includeHistory = true) {
  const kept = messages
    .filter((m) => m && typeof m.role === 'string' && String(m.content ?? '').trim() !== '')
    .map((m) => ({ role: m.role, content: m.content }));
  if (includeHistory) return kept;
  const system = kept.filter((m) => m.role === 'system');
  const lastUser = [...kept].reverse().find((m) => m.role === 'user');
  return lastUser ? [...system, lastUser] : system;
}

function chatSnippets({ baseUrl, path, model, params, messages }) {
  const body = { model, messages, ...params };
  const streaming = body.stream === true;
  const python = `${pyClient(baseUrl, API_KEY_PLACEHOLDER)}
${streaming ? 'stream' : 'response'} = client.chat.completions.create(
${pyKwargs(body)}
)
${
  streaming
    ? `for chunk in stream:
    print(chunk.choices[0].delta.content or "", end="", flush=True)`
    : 'print(response.choices[0].message.content)'
}`;
  const javascript = `${jsClient(baseUrl, API_KEY_PLACEHOLDER)}
const ${streaming ? 'stream' : 'response'} = await client.chat.completions.create({
${jsProps(body)}
});
${
  streaming
    ? `for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content ?? '');
}`
    : 'console.log(response.choices[0].message.content);'
}`;
  return { curl: curlOpenAI(baseUrl, API_KEY_PLACEHOLDER, path, body), python, javascript };
}

function imageSnippets({ baseUrl, path, model, params, prompt }) {
  const body = { model, prompt, ...params };
  return {
    curl: curlOpenAI(baseUrl, API_KEY_PLACEHOLDER, path, body),
    python: `${pyClient(baseUrl, API_KEY_PLACEHOLDER)}
response = client.images.generate(
${pyKwargs(body)}
)
print(response.data[0].url)`,
    javascript: `${jsClient(baseUrl, API_KEY_PLACEHOLDER)}
const response = await client.images.generate({
${jsProps(body)}
});
console.log(response.data[0].url);`
  };
}

// 带参考图的生图走 /v1/images/edits：图片是文件，只能 multipart，写法与转写那段同源。
function imageEditSnippets({ baseUrl, path, model, params, prompt, image }) {
  const file = image?.path || IMAGE_FILE_PLACEHOLDER;
  const fields = { model, prompt, ...params };
  const curl = curlPost(`${baseUrl}${path}`, [
    `--header ${shellQuote(`Authorization: Bearer ${API_KEY_PLACEHOLDER}`)}`,
    `--form ${shellQuote(`image=@${file}`)}`,
    ...Object.entries(fields).map(([name, value]) => `--form ${shellQuote(`${name}=${value}`)}`)
  ]);
  const pyLines = [pyKwargs({ model }, 2), '        image=reference,', pyKwargs({ prompt, ...params }, 2)].filter(Boolean).join('\n');
  const jsLines = [jsProps({ model }), `  image: createReadStream(${JSON.stringify(file)})`, jsProps({ prompt, ...params })]
    .filter(Boolean)
    .join(',\n');
  return {
    curl,
    python: `${pyClient(baseUrl, API_KEY_PLACEHOLDER)}
with open(${JSON.stringify(file)}, "rb") as reference:
    response = client.images.edit(
${pyLines}
    )
print(response.data[0].url)`,
    javascript: `import { createReadStream } from 'node:fs';
${jsClient(baseUrl, API_KEY_PLACEHOLDER)}
const response = await client.images.edit({
${jsLines}
});
console.log(response.data[0].url);`
  };
}

// 对话出图走 chat/completions：Python SDK 不认 modalities，经 extra_body 透传；
// JS SDK 运行时照传，TS 类型里没有这个字段（与目录页示例同一说明）。
function chatImageSnippets({ baseUrl, path, model, prompt, image }) {
  const body = chatImageBody({ model, prompt, imageUrl: image ? IMAGE_DATA_URL_PLACEHOLDER : undefined });
  return {
    curl: curlOpenAI(baseUrl, API_KEY_PLACEHOLDER, path, body),
    python: `${pyClient(baseUrl, API_KEY_PLACEHOLDER)}
response = client.chat.completions.create(
${pyKwargs({ model: body.model, messages: body.messages })}
    extra_body={"modalities": ["image", "text"]},
)
print(response.choices[0].message.images)`,
    javascript: `${jsClient(baseUrl, API_KEY_PLACEHOLDER)}
const response = await client.chat.completions.create({
${jsProps(body)}
});
// modalities and message.images are gateway extensions missing from the openai SDK TypeScript types.
console.log(response.choices[0].message.images);`
  };
}

function speechSnippets({ baseUrl, path, model, params, prompt }) {
  const body = { model, input: prompt, ...params };
  const file = `speech.${body.response_format || 'mp3'}`;
  return {
    curl: curlOpenAI(baseUrl, API_KEY_PLACEHOLDER, path, body, [`--output ${file}`]),
    python: `${pyClient(baseUrl, API_KEY_PLACEHOLDER)}
response = client.audio.speech.create(
${pyKwargs(body)}
)
response.write_to_file(${JSON.stringify(file)})`,
    javascript: `import { writeFile } from 'node:fs/promises';
${jsClient(baseUrl, API_KEY_PLACEHOLDER)}
const response = await client.audio.speech.create({
${jsProps(body)}
});
await writeFile('${file}', Buffer.from(await response.arrayBuffer()));`
  };
}

function transcriptionSnippets({ baseUrl, path, model, params, audio }) {
  const file = audio?.path || AUDIO_FILE_PLACEHOLDER;
  const fields = { model, ...params };
  const curl = curlPost(`${baseUrl}${path}`, [
    `--header ${shellQuote(`Authorization: Bearer ${API_KEY_PLACEHOLDER}`)}`,
    `--form ${shellQuote(`file=@${file}`)}`,
    ...Object.entries(fields).map(([name, value]) => `--form ${shellQuote(`${name}=${value}`)}`)
  ]);
  // 文件是流 / 句柄，不是字面量：Python 与 JavaScript 里这一行手写，其余字段仍走统一的字面量拼装。
  const pyLines = [pyKwargs({ model }, 2), '        file=audio,', pyKwargs(params, 2)].filter(Boolean).join('\n');
  const jsLines = [jsProps({ model }), `  file: createReadStream(${JSON.stringify(file)})`, jsProps(params)].filter(Boolean).join(',\n');
  return {
    curl,
    python: `${pyClient(baseUrl, API_KEY_PLACEHOLDER)}
with open(${JSON.stringify(file)}, "rb") as audio:
    response = client.audio.transcriptions.create(
${pyLines}
    )
print(response.text)`,
    javascript: `import { createReadStream } from 'node:fs';
${jsClient(baseUrl, API_KEY_PLACEHOLDER)}
const response = await client.audio.transcriptions.create({
${jsLines}
});
console.log(response.text);`
  };
}

const BUILDERS = {
  chat: chatSnippets,
  image: imageSnippets,
  'image.edit': imageEditSnippets,
  'image.chat': chatImageSnippets,
  'audio.speech': speechSnippets,
  'audio.transcription': transcriptionSnippets
};

// 四模态共用入口：
//   chat                 messages(可选 includeHistory) + params
//   image                prompt + params(n / size / quality / style ...)
//   image.edit           同上，外加 image.path 为参考图（multipart）
//   image.chat           prompt 进用户消息，带 image 时附参考图内容块（对话出图模型）
//   audio.speech         prompt 作为 input，params 含 voice / response_format / speed
//   audio.transcription  audio.path 为上传文件，params 为表单里的其余字段
// 未登记的模态返回三段空串，调用方照常渲染空视图，不用另写守卫。
export function buildSnippets({
  modality,
  endpoint,
  baseUrl = '',
  model = '',
  params = {},
  messages = [],
  prompt = '',
  audio,
  image,
  includeHistory = true
} = {}) {
  const build = BUILDERS[modality];
  if (!build) return { curl: '', python: '', javascript: '' };
  return build({
    baseUrl,
    path: endpoint || MODALITY_PATHS[modality],
    model: model || MODEL_PLACEHOLDER,
    params: compact(params),
    messages: chatMessages(messages, includeHistory),
    prompt,
    audio,
    image
  });
}
