import PropTypes from 'prop-types';

import ChipPopover from './ChipPopover';
import FieldRenderer from './FieldRenderer';

// ==============================|| CONSOLE — PARAMS CHIP ||============================== //
// 参数 chip：chip 上显示摘要（如「0.7 · 4096」），点开是现有 FieldRenderer 渲染的
// schema 驱动表单。schema / values / onChange 全部由上层持有，本组件不记住任何参数。

export default function ParamsChip({
  schema = [],
  values = {},
  onChange,
  summary,
  icon,
  model,
  disabled = false,
  side,
  open,
  onOpenChange
}) {
  return (
    <ChipPopover
      icon={icon}
      label={summary}
      open={open}
      onOpenChange={onOpenChange}
      disabled={disabled}
      side={side}
      className="shrink-0"
      contentClassName="w-[280px] space-y-3"
    >
      <FieldRenderer fields={schema} model={model} values={values} onChange={onChange} disabled={disabled} />
    </ChipPopover>
  );
}

ParamsChip.propTypes = {
  schema: PropTypes.array,
  values: PropTypes.object,
  onChange: PropTypes.func,
  summary: PropTypes.node,
  icon: PropTypes.node,
  model: PropTypes.object,
  disabled: PropTypes.bool,
  side: PropTypes.oneOf(['top', 'bottom']),
  open: PropTypes.bool,
  onOpenChange: PropTypes.func
};
