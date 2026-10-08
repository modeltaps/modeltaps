import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router';

import CodeSheet from '../components/CodeSheet';
import ConsoleActions from '../components/ConsoleActions';
import WorkspaceState from '../components/WorkspaceState';
import { filterModels } from 'views/panel/ApiCatalog/availability';
import { useConsole } from '../shared/ConsoleContext';
import useConsoleGate from '../shared/useConsoleGate';
import useNoModels, { useTranscriptionNoModels } from '../shared/noModels';
import AudioCodeView from './AudioCodeView';
import AudioConsole from './AudioWorkspace';
import useAudioSession from './useAudioSession';

// ==============================|| PLAYGROUND — AUDIO (ASSEMBLY) ||============================== //
// 语音模态的装配层：会话（模块级 store，见 useAudioSession）与代码面板的开合状态接起来，
// 工作区与右上动作两个插槽分处页壳两处，靠 provider 共享同一个开合状态。
// 分段 tab 的事实来源是 URL 的 ?tab=tts|stt（默认 tts）：可直达、可前进后退，
// 会话里的 subtab 跟着它走，模型候选、参数与等效代码都随之切换。
// 页面没有设置栏：参数收在编辑卡的 chip 里（见 AudioWorkspace）。

const AudioContext = createContext(null);

const AUDIO_CHAT_RULE = { endpoints: ['chat'], inputModalities: ['audio'] };

const useAudio = () => useContext(AudioContext);

export function AudioProvider({ children }) {
  const [codeOpen, setCodeOpen] = useState(false);
  const value = useMemo(() => ({ codeOpen, setCodeOpen }), [codeOpen]);

  return <AudioContext.Provider value={value}>{children}</AudioContext.Provider>;
}

AudioProvider.propTypes = { children: PropTypes.node };

export function AudioWorkspace() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const session = useAudioSession();
  const { gated, reason, actionHref } = useConsoleGate();
  const { codeOpen, setCodeOpen } = useAudio() || {};

  const { models } = useConsole();
  const noModels = useNoModels('audio');
  // 没有转写模型时给普通用户的替代路径：能听懂音频的对话模型。
  const hasAudioChat = useMemo(() => filterModels(models, AUDIO_CHAT_RULE).length > 0, [models]);
  const sttNoModels = useTranscriptionNoModels(hasAudioChat);

  const tab = searchParams.get('tab') === 'stt' ? 'stt' : 'tts';

  useEffect(() => {
    if (session.subtab !== tab) session.setSubtab(tab);
  }, [tab, session.subtab, session.setSubtab]);

  // 只改 tab 这一项：?model= 这类落地参数原样留在 URL 上。
  const onTabChange = useCallback(
    (next) =>
      setSearchParams((prev) => {
        const params = new URLSearchParams(prev);
        params.set('tab', next);
        return params;
      }),
    [setSearchParams]
  );

  return (
    // 没有可用模型不算白屏：分段 tab、编辑卡与代码入口照常在，说明卡摆在内容区上方。
    <WorkspaceState
      loading={session.loading}
      error={session.catalogError}
      loadingLabel={t('playgroundConsole.state.loading')}
      onRetry={session.reload}
    >
      <AudioConsole
        session={session}
        tab={tab}
        onTabChange={onTabChange}
        gated={gated}
        gateMessage={gated ? t(`playgroundConsole.gate.${reason}`) : ''}
        gateActionHref={actionHref}
        gateActionLabel={gated ? t(`playgroundConsole.gate.${reason}Action`) : ''}
        noModels={noModels}
        sttNoModels={sttNoModels}
      />
      <CodeSheet
        open={Boolean(codeOpen)}
        onOpenChange={(next) => !next && setCodeOpen?.(false)}
        title={t('playgroundConsole.audio.codeTitle')}
      >
        <AudioCodeView />
      </CodeSheet>
    </WorkspaceState>
  );
}

// 右上只有「查看代码」：语音页的结果就是播放条与转写文本，没有需要清空的会话。
export function AudioActions() {
  const { t } = useTranslation();
  const { setCodeOpen } = useAudio() || {};

  return <ConsoleActions onViewCode={() => setCodeOpen?.(true)} viewCodeLabel={t('playgroundConsole.audio.viewCode')} />;
}

export { AudioCodeView };
