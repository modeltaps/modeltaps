import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { MemoryRouter } from 'react-router';
import { describe, expect, it } from 'vitest';

import 'i18n/i18n';
import ImageWorkspace from './ImageWorkspace';

// 图像页的三种落地形态：模板空态 / 出图后的结果区 / 没有可用模型。
// 渲染走服务端渲染（测试跑在 node 环境，见 vite.config.mjs）。

const session = (overrides = {}) => ({
  prompt: '',
  status: 'idle',
  images: [],
  raw: '',
  error: null,
  fileError: null,
  reference: null,
  canUseReference: false,
  options: [],
  model: null,
  values: { size: '1024x1024', n: 1 },
  meta: [],
  resultTab: 'preview',
  setPrompt: () => {},
  setValue: () => {},
  setReference: () => {},
  setResultTab: () => {},
  setSelectedModel: () => {},
  run: () => {},
  abort: () => {},
  clear: () => {},
  ...overrides
});

const noModels = {
  title: '暂无可用的图像模型',
  description: '你所在的分组还没有开放图像模型',
  actionLabel: '查看可用模型',
  actionHref: '/panel/model_price'
};

const withModel = { options: [{ id: 'dall-e-3', endpoints: ['/v1/images/generations'] }], model: { id: 'dall-e-3' } };

const render = (props = {}) =>
  renderToStaticMarkup(createElement(MemoryRouter, null, createElement(ImageWorkspace, { session: session(), noModels, ...props })));

describe('ImageWorkspace 空态', () => {
  it('有模型时画 2×3 模板卡，参数 chip 给出组合摘要', () => {
    const html = render({ session: session(withModel) });

    expect(html).toContain('从模板开始出图');
    expect(html).toContain('活动海报');
    expect(html).toContain('1:1 · 1k · 1x');
    expect(html).toContain('想生成什么图像？');
    // 模板卡底部是参数摘要（比例 / 档位 / 张数），不再是分类 tag。
    expect(html).toContain('2:3');
    expect(html).toContain('4x');
    expect(html).not.toContain('电商');
  });

  it('图生图模型的参考图入口是带 aria-label 的「+」按钮，不再显示长文字胶囊', () => {
    const html = render({ session: session({ ...withModel, canUseReference: true }) });

    expect(html).toContain('aria-label="参考图（可选）"');
    expect(html).not.toContain('点击上传 · png');
  });

  it('选好参考图后显示文件名小胶囊与移除按钮', () => {
    const html = render({ session: session({ ...withModel, canUseReference: true, reference: { name: 'ref.png' } }) });

    expect(html).toContain('ref.png');
    expect(html).toContain('aria-label="移除参考图"');
  });

  it('没有可用模型时换成说明卡，输入框置灰并写「暂无模型」', () => {
    const html = render();

    expect(html).toContain(noModels.title);
    expect(html).toContain(`href="${noModels.actionHref}"`);
    expect(html).toContain('暂无模型');
    expect(html).not.toContain('从模板开始出图');
  });
});

describe('ImageWorkspace 结果区', () => {
  it('出图后模板卡让位给结果区，每张图带下载入口', () => {
    const html = render({
      session: session({
        ...withModel,
        status: 'done',
        raw: '{}',
        images: [{ src: 'https://cdn.example.com/a.png', fileName: 'image-1.png', seed: 41208, revisedPrompt: '' }]
      })
    });

    expect(html).not.toContain('从模板开始出图');
    expect(html).toContain('image-1.png');
    expect(html).toContain('全部下载');
  });

  it('对话出图模型：不画参数 chip，文字说明显示在结果区；没回图时给出提示', () => {
    const chatModel = {
      id: 'google/gemini-2.5-flash-image',
      endpoints: ['chat'],
      info: { inputModalities: ['text'], outputModalities: ['image', 'text'] }
    };
    const html = render({
      session: session({ options: [chatModel], model: chatModel, chat: true, status: 'done', raw: '{}', text: '我画不了这个' })
    });

    expect(html).not.toContain('1:1 · 1k · 1x');
    expect(html).toContain('我画不了这个');
    expect(html).toContain('模型没有返回图片');
  });
});
