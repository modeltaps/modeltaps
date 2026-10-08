import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Plus } from 'lucide-react';

import CodeSheet from '../components/CodeSheet';
import ConsoleActions, { IconAction } from '../components/ConsoleActions';
import WorkspaceState from '../components/WorkspaceState';
import { useConsole } from '../shared/ConsoleContext';
import useConsoleGate from '../shared/useConsoleGate';
import useNoModels from '../shared/noModels';
import CompareCodeViewPanel from './CompareCodeView';
import CompareWorkspaceView from './CompareWorkspace';
import useCompareSession, { MAX_COLUMNS } from './useCompareSession';

// ==============================|| PLAYGROUND — COMPARE (ASSEMBLY) ||============================== //
// 装配层：对比会话与代码面板开合状态起在 provider 里，工作区与右上动作共享同一份会话。
// 取数三态与运行门禁在这里接上，展示件本身只吃 props。

const CompareContext = createContext(null);

const useCompare = () => useContext(CompareContext);

export function CompareProvider({ children }) {
  const session = useCompareSession();
  const [codeOpen, setCodeOpen] = useState(false);
  const openCode = useCallback(() => setCodeOpen(true), []);
  const closeCode = useCallback(() => setCodeOpen(false), []);

  const value = useMemo(() => ({ session, codeOpen, openCode, closeCode }), [session, codeOpen, openCode, closeCode]);

  return <CompareContext.Provider value={value}>{children}</CompareContext.Provider>;
}

CompareProvider.propTypes = { children: PropTypes.node };

export function CompareWorkspace() {
  const { t } = useTranslation();
  const { loading, error, reload } = useConsole();
  const { gated, reason, actionHref } = useConsoleGate();
  const { session, codeOpen, closeCode } = useCompare();
  const noModels = useNoModels('compare');

  return (
    <WorkspaceState
      loading={loading}
      error={error}
      loadingLabel={t('playgroundConsole.state.loading')}
      onRetry={reload}
    >
      <CompareWorkspaceView
        session={session}
        gated={gated}
        gateMessage={gated ? t(`playgroundConsole.gate.${reason}`) : ''}
        gateActionHref={actionHref}
        gateActionLabel={gated ? t(`playgroundConsole.gate.${reason}Action`) : ''}
        noModels={noModels}
      />
      <CodeSheet open={codeOpen} onOpenChange={(next) => !next && closeCode()} title={t('playgroundConsole.chat.codeTitle')}>
        <CompareCodeView />
      </CodeSheet>
    </WorkspaceState>
  );
}

// 右上纯图标按钮：「+」添加列（满 4 列禁用）+「</>」查看代码常显，「清空」只在任一列有内容时出现。
export function CompareActions() {
  const { t } = useTranslation();
  const { session, openCode } = useCompare();

  return (
    <>
      <IconAction
        icon={<Plus className="size-4" />}
        label={t('playgroundConsole.compare.addColumn')}
        disabled={session.columns.length >= MAX_COLUMNS}
        onClick={() => session.addColumn()}
      />
      <ConsoleActions
        onViewCode={openCode}
        onClear={session.clearAll}
        clearVisible={session.hasContent && !session.running}
        viewCodeLabel={t('playgroundConsole.chat.viewCode')}
        clearLabel={t('playgroundConsole.chat.clear')}
      />
    </>
  );
}

export function CompareCodeView() {
  const { baseUrl } = useConsole();
  const { session } = useCompare();

  return <CompareCodeViewPanel session={session} baseUrl={baseUrl} />;
}
