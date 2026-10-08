import PropTypes from 'prop-types';
import { flexRender } from '@tanstack/react-table';

import { cn } from '@/lib/utils';
import { RowActionsSurfaceContext } from '@/components/ui/row-actions';

// 手机端把 TanStack 表格实例渲染为容器内通栏分隔列表(行间 divide-y),支持两种模式:
// 1. 默认模式(未传 renderCard):每个可见单元格以列表头作为标签堆叠显示。
//    id 命中 actionColumnIds(默认 actions/detail/select)的列不带标签,统一放到行底部操作区;
//    通过 primaryColumnId 可把某列(如名称)作为行标题突出显示;
//    底部操作区通过 RowActionsSurfaceContext='card' 让 RowActions 把动作铺成带文字按钮。
// 2. 自定义模式(传入 renderCard):整行内容(含操作区)由调用方通过 renderCard(row.original) 渲染,
//    DataCards 仅负责行内边距、行间分隔线、empty/searching 态与 onRowClick 的键盘可达性。
// 使用方均嵌在外层 Card/CardContent 内,行不再带独立圆角/边框/阴影,避免"卡中卡"双边框。
// 根元素默认带 data-datacards 标记:窄屏(<md)下让外层 Card 通过 has-[[data-datacards]]
// 变体去掉边框/圆角/阴影/背景,列表贴边呈现;md+ 无影响。仪表盘等不希望去框的场景传
// deframeContainer={false} 关闭该标记。
const DEFAULT_ACTION_IDS = ['actions', 'detail', 'select'];

export default function DataCards({
  table,
  className,
  actionColumnIds = DEFAULT_ACTION_IDS,
  primaryColumnId,
  empty,
  searching,
  onRowClick,
  renderCard,
  deframeContainer = true
}) {
  const rows = table.getRowModel().rows;
  const containerMarker = deframeContainer ? { 'data-datacards': '' } : {};

  const headerLabels = {};
  table.getHeaderGroups().forEach((hg) => {
    hg.headers.forEach((h) => {
      if (!h.isPlaceholder) headerLabels[h.column.id] = flexRender(h.column.columnDef.header, h.getContext());
    });
  });

  const renderCell = (cell) => flexRender(cell.column.columnDef.cell ?? cell.getValue, cell.getContext());

  if (rows.length === 0) {
    return (
      <div className={cn('p-8 text-center text-sm text-muted-foreground', className)} {...containerMarker}>
        {searching ? '…' : empty}
      </div>
    );
  }

  return (
    <div className={cn('flex flex-col divide-y divide-border', className)} {...containerMarker}>
      {rows.map((row) => {
        const cells = row.getVisibleCells();
        const actionCells = cells.filter((c) => actionColumnIds.includes(c.column.id));
        const primaryCell = primaryColumnId ? cells.find((c) => c.column.id === primaryColumnId) : null;
        const bodyCells = cells.filter((c) => !actionColumnIds.includes(c.column.id) && c.column.id !== primaryColumnId);
        return (
          <div
            key={row.id}
            className={cn('px-4 py-3', onRowClick && 'cursor-pointer transition-colors hover:bg-muted/50 focus-visible:bg-muted/50')}
            onClick={onRowClick ? () => onRowClick(row.original) : undefined}
            onKeyDown={
              onRowClick
                ? (e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      onRowClick(row.original);
                    }
                  }
                : undefined
            }
            role={onRowClick ? 'button' : undefined}
            tabIndex={onRowClick ? 0 : undefined}
          >
            {renderCard ? (
              renderCard(row.original)
            ) : (
              <>
                {primaryCell && <div className="mb-1.5 text-sm font-medium">{renderCell(primaryCell)}</div>}
                <dl className="grid grid-cols-[auto_1fr_auto_1fr] gap-x-3 gap-y-1.5">
                  {bodyCells.map((cell) => {
                    const full = cell.column.columnDef.meta?.cardFull;
                    const hasCustomCell = typeof cell.column.columnDef.cell === 'function';
                    const raw = hasCustomCell ? undefined : cell.getValue?.();
                    const title = typeof raw === 'string' || typeof raw === 'number' ? String(raw) : undefined;
                    return (
                      <div
                        key={cell.id}
                        className={cn('grid grid-cols-subgrid items-baseline gap-x-3', full ? 'col-span-4' : 'col-span-2')}
                      >
                        <dt className="text-xs font-medium text-muted-foreground">{headerLabels[cell.column.id]}</dt>
                        <dd
                          className={cn(
                            'min-w-0 text-sm',
                            full && 'col-span-3',
                            hasCustomCell ? 'flex flex-wrap items-center gap-1' : 'truncate'
                          )}
                          title={title}
                        >
                          {renderCell(cell)}
                        </dd>
                      </div>
                    );
                  })}
                </dl>
                {actionCells.length > 0 && (
                  <RowActionsSurfaceContext.Provider value="card">
                    <div className="mt-2 flex flex-wrap items-center justify-end gap-1 border-t border-border pt-2">
                      {actionCells.map((cell) => (
                        <div key={cell.id}>{renderCell(cell)}</div>
                      ))}
                    </div>
                  </RowActionsSurfaceContext.Provider>
                )}
              </>
            )}
          </div>
        );
      })}
    </div>
  );
}

DataCards.propTypes = {
  table: PropTypes.object.isRequired,
  className: PropTypes.string,
  actionColumnIds: PropTypes.arrayOf(PropTypes.string),
  primaryColumnId: PropTypes.string,
  empty: PropTypes.node,
  searching: PropTypes.bool,
  onRowClick: PropTypes.func,
  renderCard: PropTypes.func,
  deframeContainer: PropTypes.bool
};
