import { useMemo } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { AlertCircle, Plus, SlidersHorizontal, TriangleAlert } from 'lucide-react';

import { Alert, AlertDescription } from '@/components/ui/alert';
import { cn } from '@/lib/utils';
import ChipButton from '../components/ChipButton';
import ComposerBar from '../components/ComposerBar';
import ConsolePage from '../components/ConsolePage';
import MessageThread, { threadSignal } from '../components/MessageThread';
import ModelChip from '../components/ModelChip';
import NoModelsCard from '../components/NoModelsCard';
import ParamsChip from '../components/ParamsChip';
import { normalizeModel } from '../shared/modelIndex';
import ChatEmptyState from './ChatEmptyState';
import { CHAT_FIELDS, paramsSummary } from './chatFields';
import useFollowBottom from './useFollowBottom';

// ==============================|| PLAYGROUND — CHAT WORKSPACE ||============================== //
// xAI Chat 页形态：消息列与贴底输入框同宽同中线，参数全部收在输入框的 chip 里（左参数、
// 右模型），页面没有右侧设置栏。用户消息是右对齐的浅灰圆角块，助手回复是纯正文，
// 下面一行常显「tokens · ms + 该条代码 / 复制 / 重跑」。
// 门禁只卡发送：输入框整体置灰 + 一句说明 + 去向链接，消息与代码照常可看。
// 没有可用模型同理：示例位置换成 noModels 说明卡，外壳与输入框都留在原处。
// 列宽：空态时示例网格与输入框一起放宽到 1100px，有消息后回到 768px 的阅读列宽（max-width 过渡）。

// 消息列表本身在 components/MessageThread（对比页每列共用）。
export { threadSignal };

export default function ChatWorkspace({
  session,
  model,
  gated,
  gateMessage,
  gateActionHref,
  gateActionLabel,
  noModels,
  vision = false,
  onViewCode
}) {
  const { t } = useTranslation();
  const { turns, input, running, error, setInput, setSystem, updateSettings, run, rerun, abort } = session;

  const hasModels = session.options.length > 0;
  const rows = useMemo(() => session.options.map(normalizeModel), [session.options]);
  const values = useMemo(() => ({ ...session.settings, system: session.system }), [session.settings, session.system]);
  const summary = paramsSummary(session.settings);
  // 消息区跟着新消息走：轮数变化管「发出 / 回答落地」，末轮长度管流式增量；空状态停在顶部。
  const bodyRef = useFollowBottom(threadSignal(turns), turns.length > 0);

  // 参数 chip 与系统提示词共用一张表单：system 落在会话的 system 上，其余落在 settings。
  const onField = (key, value) => (key === 'system' ? setSystem(value) : updateSettings({ [key]: value }));

  // 点示例：提示词进输入框，示例自带的参数补丁一并写进会话。
  const onPickExample = (item) => {
    setInput(item.prompt);
    if (item.settings) updateSettings(item.settings);
  };

  const composer = (
    <>
      {error && (
        <Alert variant="error" className="mb-2">
          <AlertCircle />
          <AlertDescription>
            {t(error.i18nKey)}
            {error.message ? ` — ${error.message}` : ''}
          </AlertDescription>
        </Alert>
      )}
      <ComposerBar
        value={input}
        onChange={setInput}
        placeholder={t('playgroundConsole.chat.composer.placeholder')}
        running={running}
        onStop={abort}
        onSubmit={() => run()}
        disabled={gated || !session.modelId}
        disabledReason={
          gated || !hasModels ? (
            <span className="inline-flex items-center gap-1.5">
              <TriangleAlert className="size-3.5" />
              {gated ? gateMessage : noModels?.description}
              {(gated ? gateActionHref : noModels?.actionHref) && (
                <Link to={gated ? gateActionHref : noModels.actionHref} className="font-medium text-primary hover:underline">
                  {gated ? gateActionLabel : noModels.actionLabel}
                </Link>
              )}
            </span>
          ) : null
        }
        sendLabel={t('playgroundConsole.run')}
        stopLabel={t('playgroundConsole.stop')}
        leading={
          <>
            {vision && (
              <ChipButton
                caret={false}
                disabled
                className="px-2"
                icon={<Plus className="size-4" />}
                aria-label={t('playgroundConsole.chat.composer.attach')}
                title={t('playgroundConsole.chat.composer.attach')}
              />
            )}
            <ParamsChip
              schema={CHAT_FIELDS}
              values={values}
              model={model}
              onChange={onField}
              icon={<SlidersHorizontal className="size-3.5" />}
              summary={summary || t('playgroundConsole.chat.params')}
            />
          </>
        }
        trailing={
          <ModelChip
            items={rows}
            value={session.modelId}
            disabled={!hasModels}
            label={session.modelId || t(hasModels ? 'playgroundConsole.model.pick' : 'playgroundConsole.model.none')}
            onChange={(id) => session.setSelectedModel(id)}
          />
        }
      />
    </>
  );

  return (
    <ConsolePage
      className="min-h-0 flex-1"
      columnClassName={cn(
        'transition-[max-width] duration-300 ease-out motion-reduce:transition-none',
        turns.length === 0 ? 'max-w-[1100px]' : 'max-w-3xl'
      )}
      bodyRef={bodyRef}
      composer={composer}
    >
      {turns.length === 0 ? (
        <div className="flex flex-1 items-center justify-center py-6">
          {hasModels ? (
            <ChatEmptyState onPick={onPickExample} />
          ) : (
            <NoModelsCard
              title={noModels?.title}
              description={noModels?.description}
              actionLabel={noModels?.actionLabel}
              actionHref={noModels?.actionHref}
            />
          )}
        </div>
      ) : (
        <MessageThread turns={turns} running={running} onViewCode={(index) => onViewCode?.(index)} onRegenerate={rerun} />
      )}
    </ConsolePage>
  );
}

ChatWorkspace.propTypes = {
  session: PropTypes.object.isRequired,
  model: PropTypes.object,
  gated: PropTypes.bool,
  gateMessage: PropTypes.string,
  gateActionHref: PropTypes.string,
  gateActionLabel: PropTypes.string,
  noModels: PropTypes.shape({
    title: PropTypes.string,
    description: PropTypes.string,
    actionLabel: PropTypes.string,
    actionHref: PropTypes.string
  }),
  vision: PropTypes.bool,
  onViewCode: PropTypes.func
};
