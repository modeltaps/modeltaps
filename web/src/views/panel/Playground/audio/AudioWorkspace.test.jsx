import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { MemoryRouter } from 'react-router';
import { describe, expect, it } from 'vitest';

import 'i18n/i18n';
import AudioWorkspace from './AudioWorkspace';
import VoiceGallery, { clampActiveIndex, filterVoices, initialActiveIndex } from './VoiceGallery';

// 语音页两段 tab 各自的形态：合成给示例 chip + 播放条 + 编辑卡，转写给同款播放条 + 常驻结果卡。
// 没有可用模型时两段 tab 都换成 NoModelsCard；转写的控件行不再复述原因。
// 渲染走服务端渲染（测试跑在 node 环境，见 vite.config.mjs）。

const session = (overrides = {}) => ({
  options: [{ id: 'tts-1', endpoints: ['/v1/audio/speech'] }],
  model: { id: 'tts-1' },
  values: { voice: 'alloy', speed: 1, response_format: 'mp3' },
  input: '',
  status: 'idle',
  error: null,
  meta: [],
  ttsUrl: '',
  ttsFormat: 'mp3',
  file: null,
  fileError: null,
  recording: false,
  sttResult: null,
  setInput: () => {},
  setValue: () => {},
  setFile: () => {},
  setSelectedModel: () => {},
  startRecording: () => {},
  stopRecording: () => {},
  run: () => {},
  abort: () => {},
  ...overrides
});

const noModels = {
  title: '暂无可用的语音模型',
  description: '你所在的分组还没有开放语音模型',
  actionLabel: '查看可用模型',
  actionHref: '/panel/model_price'
};

const render = (props = {}) =>
  renderToStaticMarkup(
    createElement(
      MemoryRouter,
      null,
      createElement(AudioWorkspace, { session: session(), tab: 'tts', onTabChange: () => {}, noModels, ...props })
    )
  );

describe('AudioWorkspace 文字转语音', () => {
  it('分段 tab、示例 chip 与编辑卡都在，没有结果时播放条是 --:-- 占位', () => {
    const html = render();

    expect(html).toContain('文字转语音');
    expect(html).toContain('语音转文字');
    expect(html).toContain('选一个示例生成');
    expect(html).toContain('客服');
    expect(html).toContain('--:--');
    expect(html).toContain('生成语音');
    expect(html).toContain('<textarea');
  });

  it('音色与参数各一颗 chip，摘要写「1.0x · MP3」', () => {
    const html = render();

    expect(html).toContain('alloy');
    expect(html).toContain('1.0x · MP3');
  });

  it('有合成结果时播放条给下载出口', () => {
    const html = render({ session: session({ ttsUrl: 'blob:audio', ttsFormat: 'wav' }) });

    expect(html).toContain('download="speech.wav"');
    expect(html).toContain('href="blob:audio"');
  });

  it('门禁开着时给出一句原因与去向链接', () => {
    const html = render({ gated: true, gateMessage: '先创建一个令牌', gateActionHref: '/panel/token', gateActionLabel: '去创建' });

    expect(html).toContain('先创建一个令牌');
    expect(html).toContain('href="/panel/token"');
  });
});

describe('VoiceGallery 键盘高亮', () => {
  const voices = [{ id: 'alloy' }, { id: 'echo' }, { id: 'nova' }];

  it('打开时高亮当前音色，不在列表里则高亮第一项，空列表没有高亮', () => {
    expect(initialActiveIndex(voices, 'echo')).toBe(1);
    expect(initialActiveIndex(voices, 'missing')).toBe(0);
    expect(initialActiveIndex([], 'echo')).toBe(-1);
  });

  it('上下键到头停住，不循环', () => {
    expect(clampActiveIndex(0, 1, 3)).toBe(1);
    expect(clampActiveIndex(2, 1, 3)).toBe(2);
    expect(clampActiveIndex(0, -1, 3)).toBe(0);
    expect(clampActiveIndex(0, 1, 0)).toBe(-1);
  });

  it('按名称过滤，不区分大小写', () => {
    expect(filterVoices(' EC ', voices).map((voice) => voice.id)).toEqual(['echo']);
    expect(filterVoices('', voices)).toBe(voices);
  });

  it('列表是 listbox，搜索框 aria-activedescendant 指向当前音色', () => {
    const html = renderToStaticMarkup(createElement(VoiceGallery, { value: 'echo', onChange: () => {} }));
    const activeId = html.match(/aria-activedescendant="([^"]+)"/)[1];

    expect(html).toContain('role="listbox"');
    expect(html).toContain('role="option"');
    expect(html).toMatch(new RegExp(`id="${activeId}"[^>]*aria-selected="true"`));
  });

  it('模型音色按语言分组，语言为空的归「其他」', () => {
    const modelVoices = [
      { id: 'zf_xiaoxiao', language: 'zh', gender: 'female' },
      { id: 'af_heart', language: 'en', gender: 'female' },
      { id: 'mystery', language: '', gender: '' }
    ];
    const html = renderToStaticMarkup(createElement(VoiceGallery, { voices: modelVoices, value: 'af_heart', onChange: () => {} }));

    expect(html.match(/role="group"/g)).toHaveLength(3);
    expect(html).toContain('其他');
    expect(html.indexOf('zf_xiaoxiao')).toBeLessThan(html.indexOf('af_heart'));
    expect(html.indexOf('af_heart')).toBeLessThan(html.indexOf('mystery'));
  });

  it('搜索框固定在顶部，只有列表滚动', () => {
    const html = renderToStaticMarkup(createElement(VoiceGallery, { value: 'echo', onChange: () => {} }));

    expect(html).toMatch(/role="listbox"[^>]*class="[^"]*overflow-y-auto/);
    expect(html).toMatch(/<label class="[^"]*shrink-0/);
  });
});

describe('AudioWorkspace 语音转文字', () => {
  it('换成播放条（上传按钮）与常驻结果卡，示例 chip 不出现', () => {
    const html = render({ tab: 'stt' });

    expect(html).toContain('上传音频转写');
    expect(html).toContain('拖放音频到这里，或点击上传');
    expect(html).toContain('type="file"');
    expect(html).toContain('上传音频');
    expect(html).toContain('--:-- / --:--');
    expect(html).toContain('转写');
    expect(html).not.toContain('选一个示例生成');
    expect(html).not.toContain('border-dashed px-3 py-10');
  });

  it('没有结果时结果卡给居中空态，转写中改写「等待」', () => {
    expect(render({ tab: 'stt' })).toContain('选择音频开始转写');

    const html = render({ tab: 'stt', session: session({ status: 'running' }) });
    expect(html).not.toContain('选择音频开始转写');
  });

  it('回放用户自己的音频不给下载出口', () => {
    const html = render({ tab: 'stt', session: session({ file: { name: 'meeting.mp3' } }) });

    expect(html).not.toContain('download=');
  });

  it('选中文件后显示文件名，有结果时画出分段', () => {
    const html = render({
      tab: 'stt',
      session: session({
        file: { name: 'meeting.mp3' },
        sttResult: { text: '会议纪要', segments: [{ id: 0, start: 0, end: 1.5, text: '开场' }] }
      })
    });

    expect(html).toContain('meeting.mp3');
    expect(html).toContain('会议纪要');
    expect(html).toContain('开场');
    expect(html).not.toContain('选择音频开始转写');
  });
});

describe('AudioWorkspace 无可用模型', () => {
  const empty = { options: [], model: null };

  it('合成这边换成说明卡，示例 chip 让位、模型 chip 写「暂无模型」', () => {
    const html = render({ session: session(empty) });

    expect(html).toContain(noModels.title);
    expect(html).toContain(noModels.description);
    expect(html).toContain(`href="${noModels.actionHref}"`);
    expect(html).toContain('暂无模型');
    expect(html).not.toContain('选一个示例生成');
  });

  it('转写这边同一张说明卡，上传区说明让位', () => {
    const html = render({ tab: 'stt', session: session(empty) });

    expect(html).toContain(noModels.title);
    expect(html).toContain(`href="${noModels.actionHref}"`);
    expect(html).not.toContain('上传音频转写');
  });

  it('转写这边有专门说明时用它：写明没有转写模型，并引去替代路径', () => {
    const sttNoModels = {
      title: '当前站点没有接入语音转文字模型',
      description: '可以改用能听懂音频的对话模型',
      actionLabel: '去对话页',
      actionHref: '/panel/api/chat'
    };
    const html = render({ tab: 'stt', session: session(empty), sttNoModels });

    expect(html).toContain(sttNoModels.title);
    expect(html).toContain(sttNoModels.description);
    expect(html).toContain(`href="${sttNoModels.actionHref}"`);
    expect(html).not.toContain(noModels.title);
  });

  it('转写这边说明只出现一次，控件行不再复述', () => {
    const sttNoModels = {
      title: '当前站点没有接入语音转文字模型',
      description: '可以改用能听懂音频的对话模型',
      actionLabel: '去对话页',
      actionHref: '/panel/api/chat'
    };
    const html = render({ tab: 'stt', session: session(empty), sttNoModels });

    expect(html.split(sttNoModels.description)).toHaveLength(2);
    expect(html.split(`href="${sttNoModels.actionHref}"`)).toHaveLength(2);
  });

  it('转写这边门禁开着时控件行仍给门禁原因', () => {
    const html = render({
      tab: 'stt',
      session: session(empty),
      gated: true,
      gateMessage: '先创建一个令牌',
      gateActionHref: '/panel/token',
      gateActionLabel: '去创建'
    });

    expect(html).toContain('先创建一个令牌');
  });

  it('合成这边控件行仍复述原因', () => {
    const html = render({ session: session(empty) });

    expect(html.split(noModels.description)).toHaveLength(3);
  });

  it('合成这边不受转写说明影响', () => {
    const html = render({ session: session(empty), sttNoModels: { title: '当前站点没有接入语音转文字模型' } });

    expect(html).toContain(noModels.title);
    expect(html).not.toContain('当前站点没有接入语音转文字模型');
  });
});
