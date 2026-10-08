import { useTranslation } from 'react-i18next';

import { useIsAdmin } from 'utils/common';

// ==============================|| PLAYGROUND — NO MODELS COPY ||============================== //
// 「当前模态没有可用模型」的文案与出口：管理员去渠道页加渠道，普通用户去模型列表看自己
// 分组开放了什么。控制台外壳照常渲染，这里只决定卡片里写什么、按钮去哪。
// 本文件属 shared 层，禁止 import 任何 React 组件。

export const NO_MODELS_ROUTES = { admin: '/panel/channel', user: '/panel/model_price' };

// 纯函数形式，便于单测两种身份；hook 只负责把身份与翻译接进来。
export function resolveNoModels({ isAdmin = false } = {}) {
  const audience = isAdmin ? 'admin' : 'user';
  return {
    audience,
    actionHref: NO_MODELS_ROUTES[audience],
    descriptionKey: `playgroundConsole.noModels.${audience}Desc`,
    actionKey: `playgroundConsole.noModels.${audience}Action`
  };
}

// 语音转文字单独说明：站点没有任何声明 audio.transcription 的模型。管理员去接入转写渠道
// （或在模型目录给模型勾选该接口）；普通用户若有能听懂音频的对话模型，引去对话页替代。
export const TRANSCRIPTION_CHAT_ROUTE = '/panel/api/chat';

export function resolveTranscriptionNoModels({ isAdmin = false, hasAudioChat = false } = {}) {
  if (isAdmin) {
    return {
      audience: 'admin',
      actionHref: NO_MODELS_ROUTES.admin,
      descriptionKey: 'playgroundConsole.noModels.transcription.adminDesc',
      actionKey: 'playgroundConsole.noModels.adminAction'
    };
  }
  if (hasAudioChat) {
    return {
      audience: 'user',
      actionHref: TRANSCRIPTION_CHAT_ROUTE,
      descriptionKey: 'playgroundConsole.noModels.transcription.userChatDesc',
      actionKey: 'playgroundConsole.noModels.transcription.userChatAction'
    };
  }
  return {
    audience: 'user',
    actionHref: NO_MODELS_ROUTES.user,
    descriptionKey: 'playgroundConsole.noModels.transcription.userDesc',
    actionKey: 'playgroundConsole.noModels.userAction'
  };
}

export function useTranscriptionNoModels(hasAudioChat = false) {
  const { t } = useTranslation();
  const isAdmin = useIsAdmin();
  const { actionHref, descriptionKey, actionKey } = resolveTranscriptionNoModels({ isAdmin, hasAudioChat });

  return {
    title: t('playgroundConsole.noModels.transcription.title'),
    description: t(descriptionKey),
    actionLabel: t(actionKey),
    actionHref
  };
}

export default function useNoModels(modality = 'chat') {
  const { t } = useTranslation();
  const isAdmin = useIsAdmin();
  const { actionHref, descriptionKey, actionKey } = resolveNoModels({ isAdmin });
  const name = t(`playgroundConsole.modality.${modality}`);

  return {
    title: t('playgroundConsole.noModels.title', { modality: name }),
    description: t(descriptionKey, { modality: name }),
    actionLabel: t(actionKey),
    actionHref
  };
}
