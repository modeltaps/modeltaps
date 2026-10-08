import { forwardRef } from 'react';
import PropTypes from 'prop-types';
import { ChevronDown } from 'lucide-react';

import { cn } from '@/lib/utils';

// ==============================|| CONSOLE — CHIP BUTTON ||============================== //
// 输入框里那颗 chip 的外观：图标 + 文字 + ⌄。只管样式与可访问性属性，
// 开合状态（expanded）由使用方传入；带 caret=false 时就是一个纯图标 / 文字小按钮。

const ChipButton = forwardRef(function ChipButton(
  { icon, children, onClick, expanded, disabled = false, caret = true, className, ...props },
  ref
) {
  return (
    <button
      ref={ref}
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-expanded={expanded}
      className={cn(
        'inline-flex h-8 min-w-0 max-w-[220px] items-center gap-1 rounded-full border border-border px-2 text-xs text-foreground transition-colors sm:gap-1.5 sm:px-2.5',
        'hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50 [&>svg]:shrink-0',
        expanded && 'bg-muted',
        className
      )}
      {...props}
    >
      {icon}
      {children != null && <span className="truncate">{children}</span>}
      {caret && <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" />}
    </button>
  );
});

ChipButton.propTypes = {
  icon: PropTypes.node,
  children: PropTypes.node,
  onClick: PropTypes.func,
  expanded: PropTypes.bool,
  disabled: PropTypes.bool,
  caret: PropTypes.bool,
  className: PropTypes.string
};

export default ChipButton;
