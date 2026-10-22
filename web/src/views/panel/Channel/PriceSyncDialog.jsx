import PropTypes from 'prop-types';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, Loader2 } from 'lucide-react';

import { Alert } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';

// Review of a channel price sync: each channel model with its upstream price change.
// Changed, unlocked models start selected; only the selected ones are written.

const KIND_TONE = {
  new: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
  raised: 'bg-destructive/10 text-destructive',
  lowered: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  changed: 'bg-muted text-foreground',
  unchanged: 'bg-muted text-muted-foreground'
};

export const selectableChange = (c) => !c.locked && c.kind !== 'unchanged';

const usd = (v) => `$${Number(v || 0).toLocaleString(undefined, { maximumFractionDigits: 4 })}`;

function PriceCell({ oldValue, newValue, isNew }) {
  if (isNew) return <span className="tabular-nums">{usd(newValue)}</span>;
  if (oldValue === newValue) return <span className="tabular-nums text-muted-foreground">{usd(newValue)}</span>;
  return (
    <span className="tabular-nums">
      <span className="text-muted-foreground line-through">{usd(oldValue)}</span> → {usd(newValue)}
    </span>
  );
}

PriceCell.propTypes = { oldValue: PropTypes.number, newValue: PropTypes.number, isNew: PropTypes.bool };

export default function PriceSyncDialog({ open, preview, applying, onApply, onClose }) {
  const { t } = useTranslation();
  const changes = useMemo(() => preview?.changes || [], [preview]);
  const losses = preview?.losses || [];
  // Mounted per preview (the sheet renders it only while a preview is open), so the initial
  // selection is computed once from that preview.
  const [selected, setSelected] = useState(() => new Set(changes.filter(selectableChange).map((c) => c.model)));

  const selectable = changes.filter(selectableChange);
  const allSelected = selectable.length > 0 && selectable.every((c) => selected.has(c.model));
  const toggle = (model) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(model)) next.delete(model);
      else next.add(model);
      return next;
    });
  const toggleAll = () => setSelected(allSelected ? new Set() : new Set(selectable.map((c) => c.model)));

  return (
    <Dialog open={open} onOpenChange={(v) => !v && !applying && onClose()}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>{t('channel_edit.priceSync.title')}</DialogTitle>
          <DialogDescription>{t('channel_edit.priceSync.description')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          {losses.length > 0 && (
            <Alert variant="error">
              <AlertTriangle />
              <div>
                <div className="font-medium">{t('channel_edit.priceSync.lossTitle')}</div>
                {losses.map((l) => (
                  <div key={l.group}>
                    {t('channel_edit.priceSync.lossRow', { group: l.group, groupRatio: l.group_ratio, costRatio: l.cost_ratio })}
                  </div>
                ))}
              </div>
            </Alert>
          )}
          {changes.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('channel_edit.priceSync.empty')}</p>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="w-8 py-2">
                    <Checkbox
                      checked={allSelected}
                      indeterminate={!allSelected && selected.size > 0}
                      onCheckedChange={toggleAll}
                      aria-label={t('channel_edit.priceSync.selectAll')}
                    />
                  </th>
                  <th className="py-2">{t('channel_edit.priceSync.model')}</th>
                  <th className="py-2">{t('channel_edit.priceSync.change')}</th>
                  <th className="py-2">{t('channel_edit.priceSync.input')}</th>
                  <th className="py-2">{t('channel_edit.priceSync.output')}</th>
                </tr>
              </thead>
              <tbody>
                {changes.map((c) => (
                  <tr key={c.model} className="border-b border-border/60 last:border-0">
                    <td className="py-2">
                      <Checkbox
                        checked={selected.has(c.model)}
                        disabled={!selectableChange(c)}
                        onCheckedChange={() => toggle(c.model)}
                        aria-label={c.model}
                      />
                    </td>
                    <td className="break-all py-2 pr-2 font-mono text-xs">{c.model}</td>
                    <td className="py-2 pr-2">
                      <span className={cn('rounded px-1.5 py-0.5 text-xs font-medium', KIND_TONE[c.kind])}>
                        {t(`channel_edit.priceSync.kind.${c.kind}`)}
                      </span>
                      {c.locked && <span className="ml-1 text-xs text-muted-foreground">{t('channel_edit.priceSync.locked')}</span>}
                    </td>
                    <td className="py-2 pr-2">
                      <PriceCell oldValue={c.old_input} newValue={c.new_input} isNew={c.kind === 'new'} />
                    </td>
                    <td className="py-2">
                      <PriceCell oldValue={c.old_output} newValue={c.new_output} isNew={c.kind === 'new'} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          <p className="text-xs text-muted-foreground">{t('channel_edit.priceSync.unit')}</p>
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={applying}>
            {t('common.cancel')}
          </Button>
          <Button onClick={() => onApply([...selected])} disabled={applying || selected.size === 0}>
            {applying && <Loader2 className="size-4 animate-spin" />}
            {t('channel_edit.priceSync.apply', { count: selected.size })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

PriceSyncDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  preview: PropTypes.shape({ changes: PropTypes.array, losses: PropTypes.array }),
  applying: PropTypes.bool,
  onApply: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired
};
