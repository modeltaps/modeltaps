import PropTypes from 'prop-types';
import { ArrowUp, Square } from 'lucide-react';

import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';

// ==============================|| CONSOLE — COMPOSER BAR ||============================== //
// 贴底的圆角大卡输入框：上面一行文本区，下面一行左右两组 chip 加发送键。参数都收在 chip 里，
// 所以本组件只排版、不认识任何具体参数。Enter 发送 / Shift+Enter 换行；运行中发送键变 ■ 停止。
// 门禁（未登录 / 无额度…）由上层判定后传 disabled + disabledReason（可带链接节点）。

// 输入法组字期间的 Enter 属于「上屏」，不能当发送；Shift+Enter 留给换行。
export function composerKeyAction(event) {
  if (event?.key !== 'Enter' || event.shiftKey) return null;
  if (event.nativeEvent?.isComposing || event.isComposing) return null;
  return 'submit';
}

export default function ComposerBar({
  value = '',
  onChange,
  placeholder,
  leading,
  trailing,
  onSubmit,
  running = false,
  onStop,
  disabled = false,
  disabledReason,
  sendLabel,
  stopLabel,
  className
}) {
  const canSubmit = !disabled && !running && String(value).trim() !== '';

  const onKeyDown = (event) => {
    if (composerKeyAction(event) !== 'submit') return;
    event.preventDefault();
    if (canSubmit) onSubmit?.(value);
  };

  return (
    <div className={cn('rounded-2xl border border-border bg-card px-2 py-3 shadow-sm sm:px-3', className)}>
      {disabledReason && <p className="mb-2 text-xs text-muted-foreground">{disabledReason}</p>}

      <Textarea
        value={value}
        onChange={(event) => onChange?.(event.target.value)}
        onKeyDown={onKeyDown}
        placeholder={placeholder}
        disabled={disabled}
        rows={2}
        className="min-h-[52px] resize-none border-0 bg-transparent px-1 py-1 text-sm shadow-none focus-visible:border-0"
      />

      <div className="mt-2 flex min-w-0 items-center gap-1 sm:gap-1.5">
        {leading}
        <div className="min-w-0 flex-1" />
        {trailing}
        {running ? (
          <button
            type="button"
            onClick={onStop}
            aria-label={typeof stopLabel === 'string' ? stopLabel : undefined}
            title={typeof stopLabel === 'string' ? stopLabel : undefined}
            className="inline-flex size-8 shrink-0 items-center justify-center rounded-full bg-foreground text-background transition-opacity hover:opacity-90"
          >
            <Square className="size-3.5 fill-current" />
          </button>
        ) : (
          <button
            type="button"
            onClick={() => canSubmit && onSubmit?.(value)}
            disabled={!canSubmit}
            aria-label={typeof sendLabel === 'string' ? sendLabel : undefined}
            title={typeof sendLabel === 'string' ? sendLabel : undefined}
            className="inline-flex size-8 shrink-0 items-center justify-center rounded-full bg-foreground text-background transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
          >
            <ArrowUp className="size-4" />
          </button>
        )}
      </div>
    </div>
  );
}

ComposerBar.propTypes = {
  value: PropTypes.string,
  onChange: PropTypes.func,
  placeholder: PropTypes.string,
  leading: PropTypes.node,
  trailing: PropTypes.node,
  onSubmit: PropTypes.func,
  running: PropTypes.bool,
  onStop: PropTypes.func,
  disabled: PropTypes.bool,
  disabledReason: PropTypes.node,
  sendLabel: PropTypes.node,
  stopLabel: PropTypes.node,
  className: PropTypes.string
};
