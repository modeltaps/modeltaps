import PropTypes from 'prop-types';
import { useEffect, useMemo, useRef, useState } from 'react';
import { X } from 'lucide-react';

import { cn } from '@/lib/utils';
import ValuePanel from './ValuePanel';
import { OP_IN, OP_NOT_IN, optionLabel } from './utils';

// ==============================|| FILTER BAR — 4-SEGMENT EDITABLE CHIP ||============================== //
// 段位:字段名(静态) | 运算符(enum 可切换 包含/排除;number 为 ≥ 静态) | 值(点击弹出编辑面板) | 移除(x)。
// 值面板复用 ValuePanel;点击外部或 Esc 关闭。

export default function FilterChip({ field, entry, onChange, onRemove, t }) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef(null);

  useEffect(() => {
    if (!open) return undefined;
    const onDown = (e) => {
      if (rootRef.current && !rootRef.current.contains(e.target)) setOpen(false);
    };
    const onKey = (e) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const isNumber = field.type === 'number';
  const isText = field.type === 'text';
  const op = entry?.op || OP_IN;

  // 运算符段:number → ≥;text → 「包含」(可由 field.opLabelKey 覆盖,如精确匹配用「是」);
  // enum 单值用「是 / 不是」,多值(≥2)用「属于 / 不属于」(single 恒为单值)。
  const isMany = !isNumber && !isText && (entry?.values || []).length >= 2;
  const opLabel = isNumber
    ? t('filterBar.opGte')
    : isText
      ? t(field.opLabelKey || 'filterBar.opContains')
      : op === OP_NOT_IN
        ? t(isMany ? 'filterBar.opIsNotMany' : 'filterBar.opIsNotOne')
        : t(isMany ? 'filterBar.opIsMany' : 'filterBar.opIsOne');

  // 运算符可切换仅限多选 enum 且后端支持 exclude(single / number / text 均为静态)。
  const opToggleable = !isNumber && !isText && !field.single && field.supportsExclude !== false;

  const valueText = useMemo(() => {
    if (isNumber || isText) return String(entry?.value ?? '');
    const values = entry?.values || [];
    const labelOf = (v) => {
      const opt = (field.options || []).find((o) => String(o.value) === v);
      return opt ? optionLabel(t, opt) : v;
    };
    if (values.length <= 2) return values.map(labelOf).join(', ');
    return `${labelOf(values[0])} +${values.length - 1}`;
  }, [isNumber, isText, entry, field.options, t]);

  const toggleOp = () => {
    if (!opToggleable) return;
    onChange({ op: op === OP_NOT_IN ? OP_IN : OP_NOT_IN, values: entry?.values || [] });
  };

  return (
    <div ref={rootRef} className="relative inline-flex">
      <div className="inline-flex h-8 items-stretch overflow-hidden rounded-md border border-input bg-background text-sm">
        <span className="flex items-center px-2 font-medium text-foreground">{t(field.labelKey)}</span>
        <button
          type="button"
          onClick={toggleOp}
          disabled={!opToggleable}
          className={cn(
            'flex items-center border-l border-input px-2 text-xs text-muted-foreground transition-colors',
            opToggleable && 'hover:bg-muted hover:text-foreground'
          )}
        >
          {opLabel}
        </button>
        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          className="flex max-w-[180px] items-center border-l border-input px-2 text-foreground hover:bg-muted"
        >
          <span className="truncate">{valueText || t('filterBar.selectValue')}</span>
        </button>
        <button
          type="button"
          onClick={onRemove}
          aria-label={t('filterBar.remove')}
          className="flex items-center border-l border-input px-1.5 text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <X className="size-3.5" />
        </button>
      </div>
      {open && (
        <div className="absolute left-0 top-full z-50 mt-1 rounded-md border border-border bg-card text-card-foreground shadow-md">
          <ValuePanel field={field} entry={entry} onChange={onChange} onConfirm={() => setOpen(false)} t={t} />
        </div>
      )}
    </div>
  );
}

FilterChip.propTypes = {
  field: PropTypes.object.isRequired,
  entry: PropTypes.object,
  onChange: PropTypes.func.isRequired,
  onRemove: PropTypes.func.isRequired,
  t: PropTypes.func.isRequired
};
