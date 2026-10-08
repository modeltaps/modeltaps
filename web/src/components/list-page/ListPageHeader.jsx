import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

// ==============================|| LIST PAGE — HEADER ||============================== //
// 标题 + 说明 + 主操作区(actions 槽),下方一行可点击统计 chip(快捷筛选,active 高亮)。
// 窄屏统计行不换行、横向滚动。

const TONE_DOT = {
  default: 'bg-muted-foreground',
  success: 'bg-success',
  warning: 'bg-warning',
  destructive: 'bg-destructive',
  info: 'bg-info'
};

export function StatChip({ label, count, active = false, onClick, tone }) {
  const className = cn(
    'inline-flex flex-none items-center gap-1.5 whitespace-nowrap rounded-full border px-2.5 py-0.5 text-xs transition-colors',
    active ? 'border-primary bg-primary text-primary-foreground' : 'border-border bg-card text-foreground',
    onClick && !active && 'hover:bg-muted'
  );
  const content = (
    <>
      {tone && <span className={cn('size-1.5 rounded-full', active ? 'bg-primary-foreground' : TONE_DOT[tone] || TONE_DOT.default)} />}
      <span>{label}</span>
      {count != null && <span className="font-semibold tabular-nums">{count}</span>}
    </>
  );
  if (!onClick) {
    return <span className={className}>{content}</span>;
  }
  return (
    <button type="button" aria-pressed={active} onClick={onClick} className={className}>
      {content}
    </button>
  );
}

StatChip.propTypes = {
  label: PropTypes.node.isRequired,
  count: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  active: PropTypes.bool,
  onClick: PropTypes.func,
  tone: PropTypes.oneOf(['default', 'success', 'warning', 'destructive', 'info'])
};

export default function ListPageHeader({ title, description, stats = [], actions, className }) {
  return (
    <div className={cn('mb-3', className)}>
      <div className="flex flex-wrap items-start gap-x-4 gap-y-3">
        <div className="min-w-0 grow basis-48">
          <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
          {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
        </div>
        {actions && <div className="flex min-w-0 flex-wrap items-center justify-end gap-2">{actions}</div>}
      </div>
      {stats.length > 0 && (
        <div className="mt-3 flex flex-nowrap items-center gap-1 overflow-x-auto sm:flex-wrap">
          {stats.map(({ id, ...stat }) => (
            <StatChip key={id} {...stat} />
          ))}
        </div>
      )}
    </div>
  );
}

ListPageHeader.propTypes = {
  title: PropTypes.node.isRequired,
  description: PropTypes.node,
  stats: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.oneOfType([PropTypes.string, PropTypes.number]).isRequired,
      label: PropTypes.node.isRequired,
      count: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
      active: PropTypes.bool,
      onClick: PropTypes.func,
      tone: PropTypes.oneOf(['default', 'success', 'warning', 'destructive', 'info'])
    })
  ),
  actions: PropTypes.node,
  className: PropTypes.string
};
