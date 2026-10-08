import { describe, expect, it } from 'vitest';

import { scanCurl } from 'views/panel/ApiCatalog/catalog/curlScanTestUtils';
import {
  API_KEY_PLACEHOLDER,
  CODE_LANGUAGES,
  IMAGE_DATA_URL_PLACEHOLDER,
  MODALITY_PATHS,
  MODEL_PLACEHOLDER,
  buildSnippets,
  chatMessages
} from './codegen';

// 代码生成器回归：三语言都要「复制即可跑」，所以 curl 走目录页同一套词法扫描，
// Python 不许漏出 JSON 的 true / false / null，且任何模态都只出现占位符密钥。

const BASE = 'https://api.example.com';

// 四个模态各来一份典型运行设置，带撇号 / 双引号 / 换行，专挑转义会翻车的输入。
const TRICKY = `What's "up"?\nnext line`;

const CASES = [
  [
    'chat',
    {
      modality: 'chat',
      baseUrl: BASE,
      model: 'gpt-4o-mini',
      messages: [
        { role: 'system', content: "You're a gateway guide." },
        { role: 'user', content: TRICKY }
      ],
      params: { temperature: 0.7, max_tokens: 1024, stream: true, stream_options: { include_usage: true } }
    }
  ],
  [
    'image',
    { modality: 'image', baseUrl: BASE, model: 'gpt-image-1', prompt: TRICKY, params: { n: 2, size: '1024x1024', quality: 'high' } }
  ],
  [
    'image.chat',
    { modality: 'image.chat', baseUrl: BASE, model: 'google/gemini-2.5-flash-image', prompt: TRICKY, image: { name: 'ref.png' } }
  ],
  [
    'audio.speech',
    {
      modality: 'audio.speech',
      baseUrl: BASE,
      model: 'tts-1',
      prompt: TRICKY,
      params: { voice: 'alloy', response_format: 'wav', speed: 1.5 }
    }
  ],
  [
    'audio.transcription',
    {
      modality: 'audio.transcription',
      baseUrl: BASE,
      model: 'whisper-1',
      audio: { path: '/path/to/audio.mp3' },
      params: { response_format: 'verbose_json', language: 'zh' }
    }
  ]
];

describe('buildSnippets 通用约束', () => {
  it.each(CASES)('%s：三语言都非空且只出现占位符密钥', (label, input) => {
    const snippets = buildSnippets(input);
    for (const language of CODE_LANGUAGES) {
      expect(snippets[language].length, `${language} 是空的`).toBeGreaterThan(0);
      // 只允许占位符这一种 key 形态：其余 sk- 开头的串都是真实密钥漏出。
      const keys = snippets[language].match(/sk-[A-Za-z0-9_-]+/g) || [];
      expect(new Set(keys), `${language} 出现了非占位符密钥`).toEqual(new Set([API_KEY_PLACEHOLDER]));
    }
  });

  it.each(CASES)('%s：curl 能被完整切分，JSON 请求体合法', (label, input) => {
    const { tokens, bodies, expectedBodies } = scanCurl(buildSnippets(input).curl);
    expect(tokens[0]).toBe('curl');
    expect(bodies.length).toBe(expectedBodies);
    for (const body of bodies) expect(() => JSON.parse(body)).not.toThrow();
  });

  it.each(CASES)('%s：Python 不含 JSON 的布尔 / null 字面量', (label, input) => {
    expect(buildSnippets(input).python).not.toMatch(/[:=[,{]\s*(true|false|null)\b/);
  });

  it.each(CASES)('%s：默认端点取自模态注册表', (label, input) => {
    expect(buildSnippets(input).curl).toContain(`${BASE}${MODALITY_PATHS[input.modality]}`);
  });

  it('未登记的模态回三段空串', () => {
    expect(buildSnippets({ modality: 'video' })).toEqual({ curl: '', python: '', javascript: '' });
  });

  it('endpoint 可覆盖默认端点', () => {
    const snippets = buildSnippets({ modality: 'chat', baseUrl: BASE, endpoint: '/v1/responses', model: 'm' });
    expect(snippets.curl).toContain(`${BASE}/v1/responses`);
  });

  it('没有选中模型时给占位名，不输出空串模型', () => {
    const snippets = buildSnippets({ modality: 'chat', baseUrl: BASE, messages: [{ role: 'user', content: 'hi' }] });
    for (const language of CODE_LANGUAGES) {
      expect(snippets[language]).toContain(MODEL_PLACEHOLDER);
      expect(snippets[language]).not.toContain('model=""');
    }
    expect(snippets.curl).not.toContain('"model": ""');
  });
});

describe('对话', () => {
  const messages = [
    { role: 'system', content: 'sys prompt' },
    { role: 'user', content: 'first' },
    { role: 'assistant', content: 'answer' },
    { role: 'user', content: 'second' },
    { role: 'user', content: '   ' }
  ];

  it('includeHistory=false 只留系统提示词与最后一条用户消息', () => {
    expect(chatMessages(messages, false)).toEqual([
      { role: 'system', content: 'sys prompt' },
      { role: 'user', content: 'second' }
    ]);
  });

  it('空内容的轮次不进请求体', () => {
    expect(chatMessages(messages).map((m) => m.content)).toEqual(['sys prompt', 'first', 'answer', 'second']);
  });

  it('没有系统提示词时三语言里都不出现 system 角色', () => {
    const snippets = buildSnippets({ modality: 'chat', baseUrl: BASE, model: 'm', messages: [{ role: 'user', content: 'hi' }] });
    for (const language of CODE_LANGUAGES) expect(snippets[language]).not.toContain('"system"');
  });

  it('带系统提示词时三语言里都带上', () => {
    const snippets = buildSnippets({
      modality: 'chat',
      baseUrl: BASE,
      model: 'm',
      messages: [
        { role: 'system', content: 'sys prompt' },
        { role: 'user', content: 'hi' }
      ]
    });
    for (const language of CODE_LANGUAGES) expect(snippets[language]).toContain('sys prompt');
  });

  it('未设置的参数不出现，设置过的按语言各自的写法出现', () => {
    const snippets = buildSnippets({
      modality: 'chat',
      baseUrl: BASE,
      model: 'm',
      messages: [{ role: 'user', content: 'hi' }],
      params: { temperature: 0.7, max_tokens: null, top_p: undefined, stop: '' }
    });
    expect(snippets.curl).not.toContain('max_tokens');
    expect(snippets.python).toContain('temperature=0.7');
    expect(snippets.python).not.toContain('top_p');
    expect(snippets.javascript).toContain('temperature: 0.7');
    expect(snippets.javascript).not.toContain('stop');
  });

  it('stream=true 时 Python / JavaScript 走流式读法', () => {
    const snippets = buildSnippets({
      modality: 'chat',
      baseUrl: BASE,
      model: 'm',
      messages: [{ role: 'user', content: 'hi' }],
      params: { stream: true }
    });
    expect(snippets.python).toContain('for chunk in stream:');
    expect(snippets.javascript).toContain('for await (const chunk of stream)');
  });

  it('撇号与换行按各语言的转义写出，不提前闭合字符串', () => {
    const snippets = buildSnippets({ modality: 'chat', baseUrl: BASE, model: 'm', messages: [{ role: 'user', content: TRICKY }] });
    // curl 的单引号串里撇号必须拆成 '\'' ；Python / JavaScript 走 JSON 转义。
    expect(snippets.curl).toContain(`'\\''`);
    expect(snippets.python).toContain('What\'s \\"up\\"?\\nnext line');
    expect(snippets.javascript).toContain('What\'s \\"up\\"?\\nnext line');
  });
});

describe('图像 / 语音 / 转写', () => {
  it('图像：prompt 与已设参数进请求体，三语言用 images.generate', () => {
    const snippets = buildSnippets({ modality: 'image', baseUrl: BASE, model: 'gpt-image-1', prompt: 'a cat', params: { n: 2 } });
    expect(snippets.curl).toContain('"prompt": "a cat"');
    expect(snippets.python).toContain('client.images.generate(');
    expect(snippets.python).toContain('n=2');
    expect(snippets.javascript).toContain('client.images.generate({');
  });

  it('对话出图：走 chat.completions，带 modalities，不带 Images API 参数', () => {
    const snippets = buildSnippets({ modality: 'image.chat', baseUrl: BASE, model: 'google/gemini-2.5-flash-image', prompt: 'a cat' });
    expect(snippets.curl).toContain(`${BASE}/v1/chat/completions`);
    expect(snippets.curl).toContain('"modalities": [');
    expect(snippets.curl).not.toContain('"size"');
    expect(snippets.python).toContain('client.chat.completions.create(');
    expect(snippets.python).toContain('extra_body={"modalities": ["image", "text"]}');
    expect(snippets.javascript).toContain('modalities: [\n    "image",\n    "text"\n  ]');
  });

  it('对话出图带参考图：用户消息是 text + image_url 两个内容块，图片给占位 data URL', () => {
    const snippets = buildSnippets({ modality: 'image.chat', baseUrl: BASE, model: 'm', prompt: 'edit', image: { name: 'ref.png' } });
    expect(snippets.curl).toContain('"type": "image_url"');
    expect(snippets.curl).toContain(IMAGE_DATA_URL_PLACEHOLDER);
  });

  it('语音：prompt 变 input，curl 落盘到 response_format 对应的文件名', () => {
    const snippets = buildSnippets({
      modality: 'audio.speech',
      baseUrl: BASE,
      model: 'tts-1',
      prompt: 'hello',
      params: { voice: 'alloy', response_format: 'wav' }
    });
    expect(snippets.curl).toContain('--output speech.wav');
    expect(snippets.python).toContain('input="hello"');
    expect(snippets.python).toContain('response.write_to_file("speech.wav")');
    expect(snippets.javascript).toContain("await writeFile('speech.wav'");
  });

  it('转写：文件走 multipart / 文件句柄，其余字段照常拼', () => {
    const snippets = buildSnippets({
      modality: 'audio.transcription',
      baseUrl: BASE,
      model: 'whisper-1',
      audio: { path: '/tmp/a.mp3' },
      params: { response_format: 'json' }
    });
    expect(snippets.curl).toContain(`--form 'file=@/tmp/a.mp3'`);
    expect(snippets.curl).toContain(`--form 'model=whisper-1'`);
    expect(snippets.python).toContain('with open("/tmp/a.mp3", "rb") as audio:');
    expect(snippets.python).toContain('file=audio,');
    expect(snippets.javascript).toContain('createReadStream("/tmp/a.mp3")');
  });

  it('转写：没给文件时用占位路径', () => {
    const snippets = buildSnippets({ modality: 'audio.transcription', baseUrl: BASE, model: 'whisper-1' });
    expect(snippets.curl).toContain('file=@/path/to/audio.mp3');
  });
});
