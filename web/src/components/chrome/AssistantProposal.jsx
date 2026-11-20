import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, Loader2, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { API } from 'utils/api';

// ==============================|| CHROME — SETTINGS ASSISTANT PROPOSAL ||============================== //
// A change plan the settings assistant proposed (checked by /mcp/admin/plan): each setting with
// its current and new value and why. The admin ticks the changes to apply; "Apply" saves each one
// through the settings page's own save (PUT /api/option/, the same checks), marked so the change
// is written to the admin log. Changes that cannot be applied say why and cannot be ticked.

export const ASSISTANT_HEADER = 'X-Settings-Assistant';

const show = (value) => (value === '' || value === undefined ? '—' : value);

export default function AssistantProposal({ changes, describe, onApplied }) {
  const { t } = useTranslation();
  const applicable = changes.filter((c) => c.allowed && !c.unchanged);
  const [picked, setPicked] = useState(() => new Set(applicable.map((c) => c.key)));
  // key → 'saving' | 'done' | error message
  const [results, setResults] = useState({});
  const [busy, setBusy] = useState(false);
  const done = applicable.length > 0 && applicable.every((c) => results[c.key] === 'done');

  const apply = async () => {
    setBusy(true);
    for (const change of applicable.filter((c) => picked.has(c.key) && results[c.key] !== 'done')) {
      setResults((r) => ({ ...r, [change.key]: 'saving' }));
      try {
        const res = await API.put('/api/option/', { key: change.key, value: change.value }, { headers: { [ASSISTANT_HEADER]: '1' } });
        setResults((r) => ({ ...r, [change.key]: res.data?.success ? 'done' : res.data?.message || t('settingsAssistant.applyFailed') }));
      } catch (err) {
        setResults((r) => ({ ...r, [change.key]: err?.message || t('settingsAssistant.applyFailed') }));
      }
    }
    setBusy(false);
    onApplied?.();
  };

  const count = applicable.filter((c) => picked.has(c.key) && results[c.key] !== 'done').length;

  return (
    <div className="space-y-2 rounded-xl border border-border bg-card p-3" data-testid="assistant-proposal">
      <p className="text-[13px] font-medium">{t('settingsAssistant.proposal')}</p>
      <ul className="divide-y divide-border">
        {changes.map((change) => {
          const names = describe(change.key);
          const result = results[change.key];
          const can = change.allowed && !change.unchanged;
          const id = `assistant-change-${change.key}`;
          return (
            <li key={change.key} className={`flex items-start gap-3 py-2 ${can ? '' : 'opacity-60'}`}>
              <Checkbox
                id={id}
                className="mt-0.5"
                checked={can && picked.has(change.key)}
                disabled={!can || busy || result === 'done'}
                onCheckedChange={(on) =>
                  setPicked((p) => {
                    const next = new Set(p);
                    if (on) next.add(change.key);
                    else next.delete(change.key);
                    return next;
                  })
                }
              />
              <label htmlFor={id} className="min-w-0 flex-1 space-y-0.5">
                <span className="block text-sm">
                  {names?.label || change.label || change.key}
                  <span className="ml-2 text-xs text-muted-foreground">{names?.location || change.where}</span>
                </span>
                <span className="block font-mono text-xs">
                  {show(change.current)} → <span className="font-semibold">{show(change.value)}</span>
                  <span className="ml-2 text-muted-foreground">{change.key}</span>
                </span>
                {change.reason && <span className="block text-xs text-muted-foreground">{change.reason}</span>}
                {!change.allowed && (
                  <span className="block text-xs text-warning">{t('settingsAssistant.blocked', { reason: change.blocked })}</span>
                )}
                {change.unchanged && <span className="block text-xs text-muted-foreground">{t('settingsAssistant.unchanged')}</span>}
                {result && result !== 'saving' && result !== 'done' && <span className="block text-xs text-destructive">{result}</span>}
              </label>
              {result === 'saving' && <Loader2 className="mt-1 size-4 animate-spin text-muted-foreground" />}
              {result === 'done' && <Check className="mt-1 size-4 text-success" aria-label={t('settingsAssistant.applied')} />}
              {result && result !== 'saving' && result !== 'done' && <X className="mt-1 size-4 text-destructive" />}
            </li>
          );
        })}
      </ul>
      {applicable.length > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
          <p className="text-xs text-muted-foreground">{done ? t('settingsAssistant.appliedNote') : t('settingsAssistant.applyNote')}</p>
          {!done && (
            <Button type="button" size="sm" disabled={busy || count === 0} onClick={apply}>
              {busy && <Loader2 className="animate-spin" />}
              {t('settingsAssistant.apply', { count })}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

AssistantProposal.propTypes = {
  changes: PropTypes.array.isRequired,
  describe: PropTypes.func.isRequired,
  onApplied: PropTypes.func
};
