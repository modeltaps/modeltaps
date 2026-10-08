import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { SlidersHorizontal, Square, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import MessageThread, { threadSignal } from '../components/MessageThread';
import ModelChip from '../components/ModelChip';
import ParamsChip from '../components/ParamsChip';
import { CHAT_FIELDS } from '../chat/chatFields';
import useFollowBottom from '../chat/useFollowBottom';
import { OVERRIDE_KEYS, overrideCount } from './useCompareSession';

// ==============================|| PLAYGROUND — COMPARE COLUMN ||============================== //
// 对比页的一列：列头 = 模型 chip + 列内参数覆盖 chip + 「×」；列体 = 这一列自己的消息列表
// （各自跟着新消息滚到底）；列尾 = 运行中的「停止」。点列头激活该列（关闭同步输入时只发给
// 激活列），激活列加边框高亮。只吃 props。

const hasCapability = (model, capability) => (model?.info?.capabilities || []).includes(capability);

// 列内覆盖只开放三项；思考深度在列内单独出现（覆盖它就意味着这一列开思考），不依赖思考开关。
export const OVERRIDE_FIELDS = CHAT_FIELDS.filter((field) => OVERRIDE_KEYS.includes(field.key)).map((field) =>
  field.key === 'reasoningEffort' ? { ...field, showWhen: (model) => hasCapability(model, 'reasoning') } : field
);

export default function CompareColumn({
  column,
  model,
  rows,
  highlighted = false,
  removable = true,
  onActivate,
  onModelChange,
  onOverride,
  onRemove,
  onStop
}) {
  const { t } = useTranslation();
  const bodyRef = useFollowBottom(threadSignal(column.turns));
  const count = overrideCount(column.override);

  return (
    <section
      className={cn(
        'flex min-h-0 min-w-0 flex-col rounded-xl border bg-card',
        highlighted ? 'border-primary ring-1 ring-primary' : 'border-border'
      )}
      data-compare-column={column.id}
    >
      {/* 点列头（或键盘聚焦到列头的 chip）激活该列，chip 自己的开合照常；「×」不激活，直接删列。 */}
      <header
        className="flex shrink-0 flex-wrap items-center gap-1.5 border-b border-border p-2"
        onPointerDown={(event) => !event.target.closest?.('[data-remove-column]') && onActivate?.()}
        onFocus={(event) => !event.target.closest?.('[data-remove-column]') && onActivate?.()}
      >
        <ModelChip
          items={rows}
          value={column.modelId}
          disabled={rows.length === 0}
          label={model?.id || t('playgroundConsole.model.pick')}
          onChange={(id) => onModelChange?.(id)}
        />
        <ParamsChip
          schema={OVERRIDE_FIELDS}
          values={column.override}
          model={model}
          side="bottom"
          onChange={(key, value) => onOverride?.({ [key]: value })}
          icon={<SlidersHorizontal className="size-3.5" />}
          summary={count > 0 ? t('playgroundConsole.compare.overrideCount', { count }) : t('playgroundConsole.compare.override')}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="ml-auto size-7 shrink-0 text-muted-foreground"
          disabled={!removable}
          aria-label={t('playgroundConsole.compare.removeColumn')}
          title={t('playgroundConsole.compare.removeColumn')}
          data-remove-column=""
          onClick={onRemove}
        >
          <X className="size-4" />
        </Button>
      </header>

      <div ref={bodyRef} className="min-h-0 flex-1 overflow-y-auto p-3">
        {column.turns.length === 0 && !column.error ? (
          <p className="py-6 text-center text-xs text-muted-foreground">
            {model ? t('playgroundConsole.compare.emptyColumn') : t('playgroundConsole.compare.pickModel')}
          </p>
        ) : (
          <MessageThread turns={column.turns} running={column.running} error={column.error} />
        )}
      </div>

      {column.running && (
        <footer className="flex shrink-0 justify-end border-t border-border p-2">
          <Button type="button" variant="ghost" size="sm" className="h-7 gap-1.5 px-2 text-muted-foreground" onClick={onStop}>
            <Square className="size-3 fill-current" />
            {t('playgroundConsole.stop')}
          </Button>
        </footer>
      )}
    </section>
  );
}

CompareColumn.propTypes = {
  column: PropTypes.shape({
    id: PropTypes.number.isRequired,
    modelId: PropTypes.string,
    override: PropTypes.object,
    turns: PropTypes.array,
    running: PropTypes.bool,
    error: PropTypes.object
  }).isRequired,
  model: PropTypes.object,
  rows: PropTypes.array,
  highlighted: PropTypes.bool,
  removable: PropTypes.bool,
  onActivate: PropTypes.func,
  onModelChange: PropTypes.func,
  onOverride: PropTypes.func,
  onRemove: PropTypes.func,
  onStop: PropTypes.func
};
