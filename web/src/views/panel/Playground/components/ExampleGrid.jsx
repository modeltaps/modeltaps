import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

// ==============================|| CONSOLE — EXAMPLE GRID ||============================== //
// 空态示例卡网格：标题 + 副标 + N 列卡片（图标 / 标题 / 一句说明 / tag 列表）。
// 点卡回调整条 item，由上层决定「填进输入框还是直接跑」；本组件不认识示例内容。

const COLUMN_CLASS = {
  2: 'sm:grid-cols-2',
  3: 'sm:grid-cols-2 lg:grid-cols-3',
  4: 'sm:grid-cols-2 lg:grid-cols-4'
};

export default function ExampleGrid({ title, subtitle, items = [], onPick, columns = 4, className }) {
  return (
    <div className={cn('flex flex-col items-center gap-5', className)}>
      {(title || subtitle) && (
        <div className="text-center">
          {title && <h2 className="text-lg font-semibold text-foreground">{title}</h2>}
          {subtitle && <p className="mt-1 text-sm text-muted-foreground">{subtitle}</p>}
        </div>
      )}

      <div className={cn('grid w-full grid-cols-1 gap-3', COLUMN_CLASS[columns] ?? COLUMN_CLASS[4])}>
        {items.map((item, index) => (
          <button
            key={item.id ?? index}
            type="button"
            onClick={() => onPick?.(item)}
            className="flex h-full flex-col items-start gap-2 rounded-xl border border-border bg-card p-3 text-left transition-colors hover:border-foreground/20 hover:bg-muted/50"
          >
            {item.icon && (
              <span className="flex size-8 items-center justify-center rounded-full bg-muted text-foreground">{item.icon}</span>
            )}
            <span className="text-sm font-medium text-foreground">{item.title}</span>
            {item.description && <span className="text-xs text-muted-foreground">{item.description}</span>}
            {item.tags?.length > 0 && (
              <span className="mt-auto flex flex-wrap gap-1 pt-1">
                {item.tags.map((tag) => (
                  <span key={tag} className="rounded-full border border-border px-1.5 py-0.5 text-[0.6875rem] text-muted-foreground">
                    {tag}
                  </span>
                ))}
              </span>
            )}
          </button>
        ))}
      </div>
    </div>
  );
}

ExampleGrid.propTypes = {
  title: PropTypes.node,
  subtitle: PropTypes.node,
  items: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string,
      icon: PropTypes.node,
      title: PropTypes.node,
      description: PropTypes.node,
      tags: PropTypes.arrayOf(PropTypes.string)
    })
  ),
  onPick: PropTypes.func,
  columns: PropTypes.oneOf([2, 3, 4]),
  className: PropTypes.string
};
