import PropTypes from 'prop-types';
import { useEffect, useMemo, useState } from 'react';
import { Check, Plus } from 'lucide-react';

import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { OP_IN, OP_NOT_IN, emptyEnumEntry, optionLabel } from './utils';

// ==============================|| FILTER BAR — LEVEL 2 VALUE PANEL ||============================== //
// enum(多选):包含 / 排除 两个页签 + 搜索 + 多选勾选列表(freeText 字段可即输即加自定义值);
// enum(single):无页签、单选(点选替换),落标量参数;
// number:单个数字输入(下限语义,>=);text:单行输入 + 确认(标量,后端多为 LIKE/精确)。
// 受控:enum/number 值改动通过 onChange(entry) 立即回传;text 仅在确认(Enter/按钮)时回传并 onConfirm 关闭。

function EnumPanel({ field, entry, onChange, t }) {
  const [search, setSearch] = useState('');
  const single = !!field.single;
  const cur = entry && Array.isArray(entry.values) ? entry : emptyEnumEntry();
  const values = cur.values;

  const options = useMemo(() => {
    // option.icon 为可选 ReactNode(如 App 字段的 favicon),透传给渲染;无 icon 字段零回归。
    const base = (field.options || []).map((o) => ({ value: String(o.value), label: optionLabel(t, o), icon: o.icon }));
    // 已选中的自定义值(不在预设项里)也并入列表,保证可见可取消。
    const known = new Set(base.map((o) => o.value));
    const extra = values.filter((v) => !known.has(v)).map((v) => ({ value: v, label: v }));
    return [...base, ...extra];
  }, [field.options, values, t]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return options;
    return options.filter((o) => o.label.toLowerCase().includes(q) || o.value.toLowerCase().includes(q));
  }, [options, search]);

  const canAddCustom =
    field.freeText && search.trim() !== '' && !options.some((o) => o.value === search.trim() || o.label === search.trim());

  const setOp = (op) => onChange({ op, values });
  const toggle = (val) => {
    // single:点选即替换为该单值(radio 语义,清除走 chip 的 ×);多选:勾选切换。
    if (single) {
      onChange({ op: OP_IN, values: [val] });
      return;
    }
    const next = values.includes(val) ? values.filter((v) => v !== val) : [...values, val];
    onChange({ op: cur.op || OP_IN, values: next });
  };
  const addCustom = () => {
    const val = search.trim();
    if (!val || values.includes(val)) return;
    onChange({ op: cur.op || OP_IN, values: single ? [val] : [...values, val] });
    setSearch('');
  };

  return (
    <div className="w-72">
      {!single && field.supportsExclude !== false && (
        <div className="flex gap-1 border-b border-border p-1">
          {[
            { op: OP_IN, label: t('filterBar.include') },
            { op: OP_NOT_IN, label: t('filterBar.exclude') }
          ].map((tab) => (
            <button
              key={tab.op}
              type="button"
              onClick={() => setOp(tab.op)}
              className={cn(
                'flex-1 rounded-sm px-2 py-1 text-xs font-medium transition-colors',
                (cur.op || OP_IN) === tab.op ? 'bg-foreground text-background' : 'text-muted-foreground hover:bg-muted'
              )}
            >
              {tab.label}
            </button>
          ))}
        </div>
      )}
      <div className="p-2">
        <Input
          autoFocus
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && canAddCustom) {
              e.preventDefault();
              addCustom();
            }
          }}
          placeholder={field.freeText ? t('filterBar.searchOrType') : t('filterBar.search')}
          className="h-8"
        />
      </div>
      <div className="max-h-60 overflow-y-auto px-1 pb-1">
        {canAddCustom && (
          <button
            type="button"
            onClick={addCustom}
            className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm text-foreground outline-none hover:bg-muted"
          >
            <Plus className="size-4" />
            <span className="truncate">{t('filterBar.addValue', { value: search.trim() })}</span>
          </button>
        )}
        {filtered.length === 0 && !canAddCustom ? (
          <div className="py-6 text-center text-sm text-muted-foreground">{t('filterBar.empty')}</div>
        ) : (
          filtered.map((o) => {
            const selected = values.includes(o.value);
            return (
              <button
                key={o.value}
                type="button"
                onClick={() => toggle(o.value)}
                className="relative flex w-full items-center rounded-sm py-1.5 pl-8 pr-2 text-left text-sm outline-none hover:bg-muted"
              >
                <span className="absolute left-2 flex size-4 items-center justify-center">
                  {selected && <Check className="size-3.5 text-foreground" />}
                </span>
                {o.icon ? <span className="mr-1.5 flex shrink-0 items-center">{o.icon}</span> : null}
                <span className="truncate">{o.label}</span>
              </button>
            );
          })
        )}
      </div>
    </div>
  );
}

EnumPanel.propTypes = { field: PropTypes.object, entry: PropTypes.object, onChange: PropTypes.func, t: PropTypes.func };

function NumberPanel({ field, entry, onChange, t }) {
  return (
    <div className="w-60 p-2">
      <Input
        autoFocus
        type="number"
        min="0"
        value={entry?.value ?? ''}
        onChange={(e) => onChange({ value: e.target.value })}
        placeholder={field.placeholderKey ? t(field.placeholderKey) : ''}
        className="h-8"
      />
    </div>
  );
}

NumberPanel.propTypes = { field: PropTypes.object, entry: PropTypes.object, onChange: PropTypes.func, t: PropTypes.func };

// text:单行输入 + 确认。用本地草稿避免逐字触发请求,Enter 或「应用」按钮才提交并关闭面板。
function TextPanel({ field, entry, onChange, onConfirm, t }) {
  const [draft, setDraft] = useState(entry?.value ?? '');
  useEffect(() => {
    setDraft(entry?.value ?? '');
  }, [entry?.value]);
  const apply = () => {
    onChange({ value: draft.trim() });
    onConfirm?.();
  };
  return (
    <div className="w-64 p-2">
      <div className="flex items-center gap-1">
        <Input
          autoFocus
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              apply();
            }
          }}
          placeholder={field.placeholderKey ? t(field.placeholderKey) : t('filterBar.search')}
          className="h-8"
        />
        <button
          type="button"
          onClick={apply}
          className="flex h-8 shrink-0 items-center rounded-md bg-primary px-2.5 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90"
        >
          {t('filterBar.apply')}
        </button>
      </div>
    </div>
  );
}

TextPanel.propTypes = {
  field: PropTypes.object,
  entry: PropTypes.object,
  onChange: PropTypes.func,
  onConfirm: PropTypes.func,
  t: PropTypes.func
};

export default function ValuePanel({ field, entry, onChange, onConfirm, t }) {
  if (field.type === 'number') return <NumberPanel field={field} entry={entry} onChange={onChange} t={t} />;
  if (field.type === 'text') return <TextPanel field={field} entry={entry} onChange={onChange} onConfirm={onConfirm} t={t} />;
  return <EnumPanel field={field} entry={entry} onChange={onChange} t={t} />;
}

ValuePanel.propTypes = {
  field: PropTypes.object.isRequired,
  entry: PropTypes.object,
  onChange: PropTypes.func.isRequired,
  onConfirm: PropTypes.func,
  t: PropTypes.func.isRequired
};
