import PropTypes from 'prop-types';

import { Sheet, SheetContent } from '@/components/ui/sheet';

// ==============================|| CONSOLE — CODE SHEET ||============================== //
// 右侧侧滑面板，承载现有 CodeView（语言 tab + 复制）。自己不生成代码、不选语言，
// 只负责开合与容器；children 由上层塞 <CodeView …/>。

export default function CodeSheet({ open, onOpenChange, title, children }) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent aria-label={typeof title === 'string' ? title : undefined} onClose={() => onOpenChange?.(false)} className="max-w-2xl">
        {title && <div className="shrink-0 border-b border-border px-6 py-4 text-base font-semibold text-foreground">{title}</div>}
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">{children}</div>
      </SheetContent>
    </Sheet>
  );
}

CodeSheet.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func,
  title: PropTypes.node,
  children: PropTypes.node
};
