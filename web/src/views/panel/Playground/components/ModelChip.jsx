import { useState } from 'react';
import PropTypes from 'prop-types';

import ChipButton from './ChipButton';
import ModelPicker from './ModelPicker';

// ==============================|| CONSOLE — MODEL CHIP ||============================== //
// 输入框右侧的模型 chip。选择器是贴着 chip 弹出的菜单（与参数 chip 同一形态），chip 作为
// 触发器传进 ModelPicker 的 children，和浮层同处一个 relative 容器，外部点击判定才算得准。
// items 由上层注入（ModelPicker 本来就只吃归一后的行），本组件不取数。

export default function ModelChip({
  items = [],
  value = '',
  onChange,
  label,
  icon,
  loading = false,
  error = false,
  disabled = false,
  open,
  onOpenChange
}) {
  const [internalOpen, setInternalOpen] = useState(false);
  const expanded = open ?? internalOpen;

  const setOpen = (next) => {
    if (open === undefined) setInternalOpen(next);
    onOpenChange?.(next);
  };

  return (
    <ModelPicker
      open={expanded}
      onOpenChange={setOpen}
      items={items}
      loading={loading}
      error={error}
      value={value}
      onSelect={(row) => onChange?.(row?.id, row)}
    >
      <ChipButton
        icon={icon}
        expanded={expanded}
        disabled={disabled}
        title={typeof (label ?? value) === 'string' ? (label ?? value) : undefined}
        onClick={() => setOpen(!expanded)}
      >
        {label ?? value}
      </ChipButton>
    </ModelPicker>
  );
}

ModelChip.propTypes = {
  items: PropTypes.array,
  value: PropTypes.string,
  onChange: PropTypes.func,
  label: PropTypes.node,
  icon: PropTypes.node,
  loading: PropTypes.bool,
  error: PropTypes.bool,
  disabled: PropTypes.bool,
  open: PropTypes.bool,
  onOpenChange: PropTypes.func
};
