import PropTypes from 'prop-types';
import { Code2, Copy, RefreshCw } from 'lucide-react';

import { cn } from '@/lib/utils';

// ==============================|| CONSOLE — MESSAGE META ||============================== //
// 模型回复下方那行常显灰字：「{tokens} tokens · {ms}ms」+ 该条代码 / 复制 / 重跑三个图标。
// 缺哪个回调就不画哪个图标；数字由本组件加千位分隔，文案（单位、图标 aria-label）走 props。
// status 给了就在数字前加一个状态标记（如「已中断」），提示这条回复没有正常收完。

export function formatCount(value) {
  if (value == null || Number.isNaN(Number(value))) return null;
  return String(Math.round(Number(value))).replace(/\B(?=(\d{3})+(?!\d))/g, ',');
}

export default function MessageMeta({
  tokens,
  durationMs,
  tokensLabel = 'tokens',
  durationLabel = 'ms',
  status,
  onViewCode,
  onCopy,
  onRegenerate,
  viewCodeLabel,
  copyLabel,
  regenerateLabel,
  className
}) {
  const stats = [
    formatCount(tokens) && `${formatCount(tokens)} ${tokensLabel}`,
    formatCount(durationMs) && `${formatCount(durationMs)}${durationLabel}`
  ].filter(Boolean);

  return (
    <div className={cn('flex items-center gap-2 text-xs text-muted-foreground', className)}>
      {status && (
        <span className="inline-flex items-center rounded-md border border-amber-500/40 px-1.5 py-0.5 font-medium text-amber-600 dark:text-amber-400">
          {status}
        </span>
      )}
      {stats.length > 0 && <span className="tabular-nums">{stats.join(' · ')}</span>}
      {onViewCode && <MetaIcon onClick={onViewCode} label={viewCodeLabel} icon={<Code2 className="size-3.5" />} />}
      {onCopy && <MetaIcon onClick={onCopy} label={copyLabel} icon={<Copy className="size-3.5" />} />}
      {onRegenerate && <MetaIcon onClick={onRegenerate} label={regenerateLabel} icon={<RefreshCw className="size-3.5" />} />}
    </div>
  );
}

function MetaIcon({ onClick, label, icon }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={typeof label === 'string' ? label : undefined}
      title={typeof label === 'string' ? label : undefined}
      className="inline-flex size-6 items-center justify-center rounded-md transition-colors hover:bg-muted hover:text-foreground"
    >
      {icon}
    </button>
  );
}

MetaIcon.propTypes = { onClick: PropTypes.func, label: PropTypes.node, icon: PropTypes.node };

MessageMeta.propTypes = {
  tokens: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  durationMs: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  tokensLabel: PropTypes.string,
  durationLabel: PropTypes.string,
  status: PropTypes.node,
  onViewCode: PropTypes.func,
  onCopy: PropTypes.func,
  onRegenerate: PropTypes.func,
  viewCodeLabel: PropTypes.node,
  copyLabel: PropTypes.node,
  regenerateLabel: PropTypes.node,
  className: PropTypes.string
};
