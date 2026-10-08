import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import CodeSheet from '../components/CodeSheet';
import CodeView from '../components/CodeView';
import ConsoleActions from '../components/ConsoleActions';
import WorkspaceState from '../components/WorkspaceState';
import { buildSnippets } from '../shared/codegen';
import { useConsole } from '../shared/ConsoleContext';
import useConsoleGate from '../shared/useConsoleGate';
import useNoModels from '../shared/noModels';
import ChatThread from './ChatWorkspace';
import useChatSession, { buildMessages, CHAT_PATH } from './useChatSession';

// ==============================|| PLAYGROUND — CHAT (ASSEMBLY) ||============================== //
// 装配层：会话 hook 与代码面板开合状态起在 provider 里，工作区与右上动作两个插槽分处外壳
// 不同位置，共享同一份会话。取数三态与运行门禁在这里接上，展示件本身只吃 props。
// 参数没有设置栏：全部收在工作区输入框的 chip 里（见 ChatWorkspace）。
// 代码面板有两个入口：右上「查看代码」给整段会话，回复下方的「</>」给截到那一轮的请求。

const ChatContext = createContext(null);

const useChat = () => useContext(ChatContext);

export function ChatProvider({ children }) {
  const session = useChatSession();
  // upTo = null 表示整段会话；数字是助手轮的下标，代码只取到发起那一轮为止。
  const [code, setCode] = useState({ open: false, upTo: null });
  const openCode = useCallback((upTo = null) => setCode({ open: true, upTo: typeof upTo === 'number' ? upTo : null }), []);
  const closeCode = useCallback(() => setCode((current) => ({ ...current, open: false })), []);

  const value = useMemo(() => ({ session, model: session.model, code, openCode, closeCode }), [session, code, openCode, closeCode]);

  return <ChatContext.Provider value={value}>{children}</ChatContext.Provider>;
}

ChatProvider.propTypes = { children: PropTypes.node };

export function ChatWorkspace() {
  const { t } = useTranslation();
  const { loading, error, reload } = useConsole();
  const { gated, reason, actionHref } = useConsoleGate();
  const { session, model, code, openCode, closeCode } = useChat();

  const noModels = useNoModels('chat');

  // 没有可用模型不算白屏：外壳、输入框与代码入口照常在，说明卡摆在消息区中间。
  return (
    <WorkspaceState
      loading={loading}
      error={error}
      loadingLabel={t('playgroundConsole.state.loading')}
      onRetry={reload}
    >
      <ChatThread
        session={session}
        model={model}
        gated={gated}
        gateMessage={gated ? t(`playgroundConsole.gate.${reason}`) : ''}
        gateActionHref={actionHref}
        gateActionLabel={gated ? t(`playgroundConsole.gate.${reason}Action`) : ''}
        noModels={noModels}
        vision={(model?.info?.inputModalities || []).includes('image')}
        onViewCode={openCode}
      />
      <CodeSheet open={code.open} onOpenChange={(next) => !next && closeCode()} title={t('playgroundConsole.chat.codeTitle')}>
        <ChatCodeView upTo={code.upTo} />
      </CodeSheet>
    </WorkspaceState>
  );
}

// 右上纯图标按钮：「查看代码」常显，「清空」只在有消息时出现。
export function ChatActions() {
  const { t } = useTranslation();
  const { session, openCode } = useChat();

  return (
    <ConsoleActions
      onViewCode={() => openCode(null)}
      onClear={session.clear}
      clearVisible={session.turns.length > 0 && !session.running}
      viewCodeLabel={t('playgroundConsole.chat.viewCode')}
      clearLabel={t('playgroundConsole.chat.clear')}
    />
  );
}

export function ChatCodeView({ upTo = null }) {
  const { t } = useTranslation();
  const { baseUrl } = useConsole();
  const { session } = useChat();

  // 整段会话取 session.messages（含输入框里还没发出去的那条）；截到某一轮时只取到发起
  // 那一轮为止——助手轮自己不进请求体。
  const messages = useMemo(
    () => (upTo === null ? session.messages : buildMessages({ system: session.system, turns: session.turns.slice(0, upTo) })),
    [upTo, session.messages, session.system, session.turns]
  );

  const snippets = useMemo(
    () =>
      buildSnippets({
        modality: 'chat',
        baseUrl,
        model: session.modelId,
        params: session.params,
        messages
      }),
    [baseUrl, session.modelId, session.params, messages]
  );

  return (
    <CodeView
      snippets={snippets}
      meta={{ model: session.modelId, endpoint: CHAT_PATH }}
      labels={{
        copy: t('common.copy'),
        copied: t('apiCatalogPage.copied'),
        sync: t('playgroundConsole.code.sync'),
        keyHint: t('playground.curlHint')
      }}
    />
  );
}

ChatCodeView.propTypes = { upTo: PropTypes.number };
