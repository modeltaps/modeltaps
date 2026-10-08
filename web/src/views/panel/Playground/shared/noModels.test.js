import { describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import { NO_MODELS_ROUTES, TRANSCRIPTION_CHAT_ROUTE, resolveNoModels, resolveTranscriptionNoModels } from './noModels';

// 无可用模型时两种身份看到的是两句话、两个去向：管理员去渠道加渠道，普通用户去模型列表。

describe('resolveNoModels', () => {
  it('管理员指向渠道管理', () => {
    expect(resolveNoModels({ isAdmin: true })).toMatchObject({
      audience: 'admin',
      actionHref: NO_MODELS_ROUTES.admin,
      descriptionKey: 'playgroundConsole.noModels.adminDesc',
      actionKey: 'playgroundConsole.noModels.adminAction'
    });
  });

  it('普通用户指向模型列表', () => {
    expect(resolveNoModels()).toMatchObject({
      audience: 'user',
      actionHref: NO_MODELS_ROUTES.user,
      descriptionKey: 'playgroundConsole.noModels.userDesc',
      actionKey: 'playgroundConsole.noModels.userAction'
    });
  });

  it('两种身份的文案都已登记，且带模态名', () => {
    for (const isAdmin of [true, false]) {
      const { descriptionKey } = resolveNoModels({ isAdmin });
      expect(i18n.t(descriptionKey, { modality: i18n.t('playgroundConsole.modality.chat') })).toContain('对话');
    }
  });
});

// 没有转写模型时写明原因：管理员知道去接入渠道或勾选 audio.transcription，普通用户有替代路径时引去对话页。
describe('resolveTranscriptionNoModels', () => {
  it('管理员去渠道管理，说明里点出 audio.transcription', () => {
    const { actionHref, descriptionKey, actionKey } = resolveTranscriptionNoModels({ isAdmin: true, hasAudioChat: true });
    expect(actionHref).toBe(NO_MODELS_ROUTES.admin);
    expect(i18n.t(descriptionKey)).toContain('audio.transcription');
    expect(i18n.t(actionKey)).toBe('去渠道管理');
  });

  it('普通用户有能听懂音频的对话模型时引去对话页', () => {
    const { actionHref, descriptionKey, actionKey } = resolveTranscriptionNoModels({ hasAudioChat: true });
    expect(actionHref).toBe(TRANSCRIPTION_CHAT_ROUTE);
    expect(i18n.t(descriptionKey)).toContain('对话模型');
    expect(i18n.t(actionKey)).toBe('去对话页');
  });

  it('普通用户没有替代模型时去模型列表', () => {
    const { actionHref, descriptionKey } = resolveTranscriptionNoModels();
    expect(actionHref).toBe(NO_MODELS_ROUTES.user);
    expect(i18n.t(descriptionKey)).toContain('没有接入语音转文字');
  });

  it('四种语言的转写说明都已登记', () => {
    const keys = ['title', 'adminDesc', 'userDesc', 'userChatDesc', 'userChatAction'];
    for (const lng of ['zh_CN', 'zh_HK', 'en_US', 'ja_JP']) {
      for (const key of keys) {
        expect(i18n.exists(`playgroundConsole.noModels.transcription.${key}`, { lng })).toBe(true);
      }
    }
  });
});
