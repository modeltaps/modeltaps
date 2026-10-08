import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

// ==============================|| PLAYGROUND — RESULT PANEL ||============================== //
// 媒体模态右半屏的固定结果区：标题 + 状态标签 + Preview / JSON 切换 + 一行元信息（耗时 ·
// 张数 · 尺寸 …）+ 内容 + 底部动作条。状态与内容都由调用方给，本组件只吃 props。

export const RESULT_STATES = ['idle', 'running', 'done', 'error'];

const DOT_CLASS = {
  idle: 'bg-muted-foreground/40',
  running: 'bg-amber-500 animate-pulse',
  done: 'bg-emerald-500',
  error: 'bg-destructive'
};

export default function ResultPanel({
  title,
  status = 'idle',
  statusLabel,
  tabs = [],
  activeTab,
  onTabChange,
  meta = [],
  footer,
  children
}) {
  return (
    <section className="flex min-h-0 min-w-0 flex-1 flex-col gap-2 rounded-lg border border-border bg-card p-3">
      <header className="flex flex-wrap items-center gap-2">
        <span className="text-xs font-semibold text-foreground">{title}</span>
        <span className="inline-flex items-center gap-1.5 rounded-full border border-border px-2 py-0.5 text-[11px] text-muted-foreground">
          <span className={cn('inline-block size-1.5 rounded-full', DOT_CLASS[status] || DOT_CLASS.idle)} />
          {statusLabel}
        </span>
        <div className="flex-1" />
        {tabs.length > 0 && (
          <div role="tablist" aria-label="result-view" className="inline-flex gap-1">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                type="button"
                role="tab"
                aria-selected={tab.id === activeTab}
                onClick={() => onTabChange?.(tab.id)}
                className={cn(
                  'rounded-md px-2 py-0.5 text-[11px] transition-colors',
                  tab.id === activeTab ? 'bg-secondary font-semibold text-foreground' : 'text-muted-foreground hover:bg-accent'
                )}
              >
                {tab.label}
              </button>
            ))}
          </div>
        )}
      </header>

      {meta.length > 0 && (
        <p className="flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
          {meta.map((item, index) => (
            <span key={item} className="contents">
              {index > 0 && <span aria-hidden="true">·</span>}
              <span>{item}</span>
            </span>
          ))}
        </p>
      )}

      <div className="min-h-0 flex-1 overflow-auto">{children}</div>

      {footer && <div className="flex flex-wrap items-center gap-2 border-t border-border pt-2">{footer}</div>}
    </section>
  );
}

ResultPanel.propTypes = {
  title: PropTypes.node,
  status: PropTypes.oneOf(RESULT_STATES),
  statusLabel: PropTypes.node,
  tabs: PropTypes.arrayOf(PropTypes.shape({ id: PropTypes.string.isRequired, label: PropTypes.node })),
  activeTab: PropTypes.string,
  onTabChange: PropTypes.func,
  meta: PropTypes.arrayOf(PropTypes.string),
  footer: PropTypes.node,
  children: PropTypes.node
};
