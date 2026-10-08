import PropTypes from 'prop-types';
import { Code2, RotateCcw } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

// ==============================|| CONSOLE — TOP RIGHT ACTIONS ||============================== //
// 页壳右上的纯图标按钮（悬停 / 聚焦出提示）：「</>」查看代码常显，「⟳」清空只在有内容时出现（clearVisible）。
// 文案由上层传入，本组件不引 i18n。

// 禁用态按钮不吃指针事件，外包一层 span 让悬停提示照常出现。
export function IconAction({ icon, label, onClick, disabled = false }) {
  const button = (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className="size-8 text-muted-foreground"
      aria-label={typeof label === 'string' ? label : undefined}
      disabled={disabled}
      onClick={onClick}
    >
      {icon}
    </Button>
  );

  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>{disabled ? <span className="inline-flex">{button}</span> : button}</TooltipTrigger>
        <TooltipContent>{label}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

IconAction.propTypes = { icon: PropTypes.node, label: PropTypes.node, onClick: PropTypes.func, disabled: PropTypes.bool };

export default function ConsoleActions({ onViewCode, onClear, clearVisible = false, viewCodeLabel, clearLabel }) {
  return (
    <>
      <IconAction icon={<Code2 className="size-4" />} label={viewCodeLabel} onClick={onViewCode} />
      {clearVisible && <IconAction icon={<RotateCcw className="size-4" />} label={clearLabel} onClick={onClear} />}
    </>
  );
}

ConsoleActions.propTypes = {
  onViewCode: PropTypes.func,
  onClear: PropTypes.func,
  clearVisible: PropTypes.bool,
  viewCodeLabel: PropTypes.node,
  clearLabel: PropTypes.node
};
