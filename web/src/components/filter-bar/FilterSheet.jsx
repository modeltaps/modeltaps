import PropTypes from 'prop-types';
import { Fragment } from 'react';
import { X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Sheet } from '@/components/ui/sheet';
import { OP_IN, isEntryActive, optionLabel } from './utils';

// ==============================|| FILTER BAR — PHONE SHEET ||============================== //
// The same filter fields as FilterBar, laid out for a phone: a bottom sheet with every field at
// once instead of a cascading flyout. Single-choice enum fields are a row of chips (tap again to
// clear), multi-choice enums toggle chips, number fields are one input. `top` renders above the
// fields (e.g. presets). Controlled like FilterBar: onChange(updater | nextState).

export default function FilterSheet({ open, onOpenChange, title, fields, state, onChange, onClearAll, top, t }) {
  const setEntry = (key, entry) =>
    onChange((prev) => {
      const next = { ...prev };
      if (entry) next[key] = entry;
      else delete next[key];
      return next;
    });

  const toggleValue = (f, value) => {
    const cur = state[f.key]?.values || [];
    if (f.single) {
      setEntry(f.key, cur[0] === value ? null : { op: OP_IN, values: [value] });
      return;
    }
    const values = cur.includes(value) ? cur.filter((v) => v !== value) : [...cur, value];
    setEntry(f.key, values.length ? { op: state[f.key]?.op || OP_IN, values } : null);
  };

  const activeCount = fields.filter((f) => isEntryActive(f, state[f.key])).length;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="absolute inset-x-0 bottom-0 flex max-h-[85vh] flex-col rounded-t-2xl border-t border-border bg-popover shadow-2xl"
      >
        <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
          <h2 className="text-base font-medium">{title}</h2>
          <Button variant="ghost" size="icon" className="size-8" aria-label={t('common.close')} onClick={() => onOpenChange(false)}>
            <X className="size-4" />
          </Button>
        </div>
        <div className="flex-1 space-y-4 overflow-y-auto px-4 py-3">
          {top}
          {fields.map((f, i) => {
            // A section heading only groups two or more fields; a lone field's own label says enough.
            const sectionSize = f.sectionKey ? fields.filter((x) => x.sectionKey === f.sectionKey).length : 0;
            const heading =
              sectionSize > 1 && f.sectionKey !== fields[i - 1]?.sectionKey ? (
                <div className="pt-1 text-[11px] font-medium text-muted-foreground">{t(f.sectionKey)}</div>
              ) : null;
            const entry = state[f.key];
            return (
              <Fragment key={f.key}>
                {heading}
                <div className="space-y-1.5">
                  <div className="text-sm">{t(f.labelKey)}</div>
                  {f.type === 'number' || f.type === 'text' ? (
                    <Input
                      type={f.type === 'number' ? 'number' : 'text'}
                      inputMode={f.type === 'number' ? 'decimal' : undefined}
                      min={f.type === 'number' ? '0' : undefined}
                      value={entry?.value ?? ''}
                      onChange={(e) => setEntry(f.key, e.target.value === '' ? null : { value: e.target.value })}
                      placeholder={f.placeholderKey ? t(f.placeholderKey) : ''}
                      className="h-9"
                    />
                  ) : (
                    <div className="flex flex-wrap gap-1.5">
                      {(f.options || []).map((o) => {
                        const value = String(o.value);
                        const on = (entry?.values || []).includes(value);
                        return (
                          <button
                            key={value}
                            type="button"
                            aria-pressed={on}
                            onClick={() => toggleValue(f, value)}
                            className={cn(
                              'h-8 rounded-full border px-3 text-[13px] transition-colors',
                              on ? 'border-primary bg-primary text-primary-foreground' : 'border-input bg-field hover:bg-muted'
                            )}
                          >
                            {optionLabel(t, o)}
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              </Fragment>
            );
          })}
        </div>
        <div className="flex items-center gap-2 border-t border-border px-4 pb-[max(0.75rem,env(safe-area-inset-bottom))] pt-3">
          <Button variant="outline" className="flex-1" disabled={activeCount === 0} onClick={onClearAll}>
            {t('filterBar.clearAll')}
          </Button>
          <Button className="flex-1" onClick={() => onOpenChange(false)}>
            {t('filterBar.apply')}
          </Button>
        </div>
      </div>
    </Sheet>
  );
}

FilterSheet.propTypes = {
  open: PropTypes.bool.isRequired,
  onOpenChange: PropTypes.func.isRequired,
  title: PropTypes.string.isRequired,
  fields: PropTypes.array.isRequired,
  state: PropTypes.object.isRequired,
  onChange: PropTypes.func.isRequired,
  onClearAll: PropTypes.func.isRequired,
  top: PropTypes.node,
  t: PropTypes.func.isRequired
};
