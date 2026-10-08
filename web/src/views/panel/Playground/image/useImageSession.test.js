import { describe, expect, it } from 'vitest';

import {
  CHAT_COMPLETIONS_PATH,
  IMAGE_EDITS_PATH,
  IMAGE_GENERATIONS_PATH,
  MAX_REFERENCE_BYTES,
  buildImageBody,
  imagePath,
  imageResultMeta,
  normalizeChatImageResult,
  normalizeImageResult,
  validateReferenceImage
} from './useImageSession';

// 会话 hook 的纯函数部分：回包归一（url / b64 两种形态）、参考图前置校验、
// 有无参考图决定打哪个端点，以及结果区那行元信息。

const file = (type, size = 1024) => ({ name: 'ref.png', type, size });

describe('normalizeImageResult', () => {
  it('url 与 b64_json 两种形态都收，b64 渲染成 data: URL', () => {
    const out = normalizeImageResult({
      data: [
        { url: ' https://cdn.example.com/a.png ', seed: 41208 },
        { b64_json: 'AAAA', revised_prompt: 'a red panda' }
      ]
    });
    expect(out).toEqual([
      { src: 'https://cdn.example.com/a.png', fileName: 'image-1.png', seed: 41208, revisedPrompt: '' },
      { src: 'data:image/png;base64,AAAA', fileName: 'image-2.png', seed: null, revisedPrompt: 'a red panda' }
    ]);
  });

  it('缺字段或非数组的响应不抛，直接给空列表', () => {
    expect(normalizeImageResult(null)).toEqual([]);
    expect(normalizeImageResult({ data: [{}, { url: '   ' }] })).toEqual([]);
  });
});

describe('validateReferenceImage', () => {
  it('没选文件 / 类型不支持 / 超过 8MB 各给一个可读的 i18n key', () => {
    expect(validateReferenceImage(null).i18nKey).toBe('playgroundConsole.image.errors.missingFile');
    expect(validateReferenceImage(file('application/pdf')).i18nKey).toBe('playgroundConsole.image.errors.unsupportedType');
    expect(validateReferenceImage(file('image/png', MAX_REFERENCE_BYTES + 1)).i18nKey).toBe('playgroundConsole.image.errors.tooLarge');
  });

  it('png / jpeg / webp 且不超限时放行', () => {
    expect(validateReferenceImage(file('image/webp', MAX_REFERENCE_BYTES)).ok).toBe(true);
  });
});

describe('imagePath', () => {
  it('无参考图走 generations，带参考图走 edits', () => {
    expect(imagePath(null)).toBe(IMAGE_GENERATIONS_PATH);
    expect(imagePath(file('image/png'))).toBe(IMAGE_EDITS_PATH);
  });

  it('对话出图模型不论有无参考图都走 chat/completions', () => {
    expect(imagePath(null, true)).toBe(CHAT_COMPLETIONS_PATH);
    expect(imagePath(file('image/png'), true)).toBe(CHAT_COMPLETIONS_PATH);
  });
});

describe('normalizeChatImageResult', () => {
  it('收 message.images[]（OpenRouter 风格），文字说明单独给出', () => {
    const out = normalizeChatImageResult({
      choices: [
        {
          message: {
            content: 'Here is your panda.',
            images: [{ type: 'image_url', image_url: { url: 'data:image/jpeg;base64,AAAA' } }]
          }
        }
      ]
    });
    expect(out).toEqual({
      images: [{ src: 'data:image/jpeg;base64,AAAA', fileName: 'image-1.jpg', seed: null, revisedPrompt: '' }],
      text: 'Here is your panda.'
    });
  });

  it('content 数组里的 image_url 块与文本里的 markdown 图片也收', () => {
    const out = normalizeChatImageResult({
      choices: [
        {
          message: {
            content: [
              { type: 'text', text: 'done ![img](https://cdn.example.com/a.png)' },
              { type: 'image_url', image_url: { url: 'data:image/png;base64,BBBB' } }
            ]
          }
        }
      ]
    });
    expect(out.images.map((image) => image.src)).toEqual(['https://cdn.example.com/a.png', 'data:image/png;base64,BBBB']);
    expect(out.text).toBe('done');
  });

  it('没有图片也没有文字时给空结果，不抛', () => {
    expect(normalizeChatImageResult(null)).toEqual({ images: [], text: '' });
    expect(normalizeChatImageResult({ choices: [{ message: { content: null } }] })).toEqual({ images: [], text: '' });
  });
});

describe('buildImageBody', () => {
  it('模型与提示词恒在，其余字段原样来自 schema 求值', () => {
    expect(buildImageBody({ model: 'dall-e-3', prompt: 'hi', params: { n: 2, size: '512x512' } })).toEqual({
      model: 'dall-e-3',
      prompt: 'hi',
      n: 2,
      size: '512x512'
    });
  });
});

describe('imageResultMeta', () => {
  it('耗时 · 张数 · 尺寸 · 质量，缺的项不占位', () => {
    expect(imageResultMeta({ images: [{}, {}], elapsedMs: 6820, values: { size: '1024x1024', quality: 'high' } })).toEqual([
      '6.82s',
      '2',
      '1024x1024',
      'high'
    ]);
    expect(imageResultMeta({})).toEqual([]);
  });
});
