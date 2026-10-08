import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import CodeSheet from '../components/CodeSheet';
import ConsoleActions from '../components/ConsoleActions';
import WorkspaceState from '../components/WorkspaceState';
import { useConsole } from '../shared/ConsoleContext';
import useConsoleGate from '../shared/useConsoleGate';
import useNoModels from '../shared/noModels';
import ImageCanvas from './ImageWorkspace';
import ImageCodeView from './ImageCodeView';
import useImageSession from './useImageSession';

// ==============================|| PLAYGROUND — IMAGE (ASSEMBLY) ||============================== //
// 装配层：会话 hook 与代码面板开合状态起在 provider 里，工作区与右上动作两个插槽分处外壳
// 不同位置，共享同一份会话。取数三态与运行门禁在这里接上，展示件本身只吃 props。
// 参数没有设置栏：全部收在工作区输入框的 chip 里（见 ImageWorkspace）。

const ImageContext = createContext(null);

const useImage = () => useContext(ImageContext);

export function ImageProvider({ children }) {
  const session = useImageSession();
  const [codeOpen, setCodeOpen] = useState(false);
  const openCode = useCallback(() => setCodeOpen(true), []);
  const closeCode = useCallback(() => setCodeOpen(false), []);

  const value = useMemo(() => ({ session, codeOpen, openCode, closeCode }), [session, codeOpen, openCode, closeCode]);

  return <ImageContext.Provider value={value}>{children}</ImageContext.Provider>;
}

ImageProvider.propTypes = { children: PropTypes.node };

export function ImageWorkspace() {
  const { t } = useTranslation();
  const { loading, error, reload } = useConsole();
  const { gated, reason, actionHref } = useConsoleGate();
  const { session, codeOpen, closeCode } = useImage();

  const noModels = useNoModels('image');

  // 没有可用模型不算白屏：外壳、输入框与代码入口照常在，说明卡摆在结果区中间。
  return (
    <WorkspaceState
      loading={loading}
      error={error}
      loadingLabel={t('playgroundConsole.state.loading')}
      onRetry={reload}
    >
      <ImageCanvas
        session={session}
        gated={gated}
        gateMessage={gated ? t(`playgroundConsole.gate.${reason}`) : ''}
        gateActionHref={actionHref}
        gateActionLabel={gated ? t(`playgroundConsole.gate.${reason}Action`) : ''}
        noModels={noModels}
      />
      <CodeSheet open={codeOpen} onOpenChange={(next) => !next && closeCode()} title={t('playgroundConsole.image.codeTitle')}>
        <ImageCodeView />
      </CodeSheet>
    </WorkspaceState>
  );
}

// 右上纯图标按钮：「查看代码」常显，「清空」只在有结果时出现。
export function ImageActions() {
  const { t } = useTranslation();
  const { session, openCode } = useImage();

  return (
    <ConsoleActions
      onViewCode={openCode}
      onClear={session.clear}
      clearVisible={(session.images.length > 0 || Boolean(session.raw)) && session.status !== 'running'}
      viewCodeLabel={t('playgroundConsole.image.viewCode')}
      clearLabel={t('playgroundConsole.image.clear')}
    />
  );
}

export { ImageCodeView };
