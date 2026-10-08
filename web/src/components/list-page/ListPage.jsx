import PropTypes from 'prop-types';

import { cn } from '@/lib/utils';

// ==============================|| LIST PAGE — LAYOUT SHELL ||============================== //
// 列表页四段骨架:页头 → 工具栏(有选中时由批量条替换)→ 表格 → 固定页脚。纯布局、不取数。

export default function ListPage({ header, toolbar, batchBar, selectedCount = 0, footer, children, className }) {
  const showBatch = selectedCount > 0 && batchBar;
  return (
    <section data-slot="list-page" className={cn('flex min-w-0 flex-col', className)}>
      {header && <div data-slot="list-page-header">{header}</div>}
      {(toolbar || showBatch) && <div data-slot="list-page-toolbar">{showBatch ? batchBar : toolbar}</div>}
      <div data-slot="list-page-body" className="min-w-0 overflow-hidden rounded-t-lg border border-border bg-card">
        {children}
      </div>
      {footer && (
        <div
          data-slot="list-page-footer"
          className="sticky bottom-0 z-10 overflow-hidden rounded-b-lg border border-t-0 border-border bg-card"
        >
          {footer}
        </div>
      )}
    </section>
  );
}

ListPage.propTypes = {
  header: PropTypes.node,
  toolbar: PropTypes.node,
  batchBar: PropTypes.node,
  selectedCount: PropTypes.number,
  footer: PropTypes.node,
  children: PropTypes.node,
  className: PropTypes.string
};
