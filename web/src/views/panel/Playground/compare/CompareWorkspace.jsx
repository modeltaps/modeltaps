import { useMemo } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { FileText, Link2, SlidersHorizontal, TriangleAlert, Unlink2 } from 'lucide-react';

import { cn } from '@/lib/utils';
import ChipButton from '../components/ChipButton';
import ComposerBar from '../components/ComposerBar';
import ConsolePage from '../components/ConsolePage';
import NoModelsCard from '../components/NoModelsCard';
import ParamsChip from '../components/ParamsChip';
import { normalizeModel } from '../shared/modelIndex';
import { CHAT_FIELDS, paramsSummary } from '../chat/chatFields';
import CompareColumn from './CompareColumn';
import { MIN_COLUMNS } from './useCompareSession';

// ==============================|| PLAYGROUND — COMPARE WORKSPACE ||============================== //
// 对比页形态：同一个 ConsolePage，但主区放宽到全宽、换成等宽的列区；底部共享一个输入框。
// 输入框左侧三颗 chip：系统提示词 / 全局参数（温度 · 最大 tokens · 思考）/ 同步输入开关。
// 同步输入开着发给所有已选模型的列，关着只发给激活列。任一列运行中发送键变「全部停止」。
// 没有可用的对话模型时整页换成说明卡。只吃 props。

const SYSTEM_FIELDS = CHAT_FIELDS.filter((field) => field.key === 'system');
const GLOBAL_FIELDS = CHAT_FIELDS.filter((field) => ['temperature', 'maxTokens', 'thinking', 'reasoningEffort'].includes(field.key));

// 2 列直接等分；3–4 列在 ≥1280px 并排，以下每列保底宽度、横向滚动。
const GRID_CLASS = {
  2: 'grid-cols-2',
  3: 'grid-cols-[repeat(3,minmax(18rem,1fr))] overflow-x-auto xl:grid-cols-3 xl:overflow-visible',
  4: 'grid-cols-[repeat(4,minmax(18rem,1fr))] overflow-x-auto xl:grid-cols-4 xl:overflow-visible'
};

export default function CompareWorkspace({ session, gated, gateMessage, gateActionHref, gateActionLabel, noModels }) {
  const { t } = useTranslation();
  const { columns, options, input, running, sync, activeIndex } = session;

  const hasModels = options.length > 0;
  const rows = useMemo(() => options.map(normalizeModel), [options]);
  const modelOf = (id) => (id ? options.find((item) => item.id === id) || null : null);
  // 全局参数 chip 按所有列模型的能力并集决定显隐：任一列支持思考，思考项就出现。
  const unionModel = useMemo(() => {
    const capabilities = new Set();
    for (const column of columns) for (const cap of modelOf(column.modelId)?.info?.capabilities || []) capabilities.add(cap);
    return { info: { capabilities: [...capabilities] } };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [columns, options]);

  const targets = sync ? columns : [columns[activeIndex]];
  const canSend = targets.some((column) => column && modelOf(column.modelId));

  if (!hasModels) {
    return (
      <ConsolePage className="min-h-0 flex-1" columnClassName="max-w-none">
        <div className="flex flex-1 items-center justify-center py-6">
          <NoModelsCard
            title={noModels?.title}
            description={noModels?.description}
            actionLabel={noModels?.actionLabel}
            actionHref={noModels?.actionHref}
          />
        </div>
      </ConsolePage>
    );
  }

  const composer = (
    <ComposerBar
      value={input}
      onChange={session.setInput}
      placeholder={t('playgroundConsole.compare.placeholder')}
      running={running}
      onStop={session.stopAll}
      onSubmit={() => (sync ? session.sendAll() : session.sendTo(activeIndex))}
      disabled={gated || !canSend}
      disabledReason={
        gated ? (
          <span className="inline-flex items-center gap-1.5">
            <TriangleAlert className="size-3.5" />
            {gateMessage}
            {gateActionHref && (
              <Link to={gateActionHref} className="font-medium text-primary hover:underline">
                {gateActionLabel}
              </Link>
            )}
          </span>
        ) : null
      }
      sendLabel={t('playgroundConsole.run')}
      stopLabel={t('playgroundConsole.compare.stopAll')}
      leading={
        <>
          <ParamsChip
            schema={SYSTEM_FIELDS}
            values={{ system: session.system }}
            onChange={(_, value) => session.setSystem(value)}
            icon={<FileText className="size-3.5" />}
            summary={t('playgroundConsole.chat.settings.system')}
          />
          <ParamsChip
            schema={GLOBAL_FIELDS}
            values={session.settings}
            model={unionModel}
            onChange={(key, value) => session.updateSettings({ [key]: value })}
            icon={<SlidersHorizontal className="size-3.5" />}
            summary={paramsSummary(session.settings) || t('playgroundConsole.chat.params')}
          />
          <ChipButton
            caret={false}
            aria-pressed={sync}
            className={cn(sync && 'bg-muted')}
            icon={sync ? <Link2 className="size-3.5" /> : <Unlink2 className="size-3.5" />}
            onClick={() => session.setSync(!sync)}
          >
            {t(sync ? 'playgroundConsole.compare.syncOn' : 'playgroundConsole.compare.syncOff')}
          </ChipButton>
        </>
      }
    />
  );

  return (
    <ConsolePage className="min-h-0 flex-1" columnClassName="max-w-none h-full" composer={composer}>
      <div className={cn('grid min-h-0 flex-1 grid-rows-[minmax(0,1fr)] gap-3', GRID_CLASS[columns.length])}>
        {columns.map((column, index) => (
          <CompareColumn
            key={column.id}
            column={column}
            model={modelOf(column.modelId)}
            rows={rows}
            highlighted={!sync && index === activeIndex}
            removable={columns.length > MIN_COLUMNS}
            onActivate={() => session.setActive(index)}
            onModelChange={(id) => session.setColumnModel(index, id)}
            onOverride={(patch) => session.setOverride(index, patch)}
            onRemove={() => session.removeColumn(index)}
            onStop={() => session.stop(index)}
          />
        ))}
      </div>
    </ConsolePage>
  );
}

CompareWorkspace.propTypes = {
  session: PropTypes.object.isRequired,
  gated: PropTypes.bool,
  gateMessage: PropTypes.string,
  gateActionHref: PropTypes.string,
  gateActionLabel: PropTypes.string,
  noModels: PropTypes.shape({
    title: PropTypes.string,
    description: PropTypes.string,
    actionLabel: PropTypes.string,
    actionHref: PropTypes.string
  })
};
