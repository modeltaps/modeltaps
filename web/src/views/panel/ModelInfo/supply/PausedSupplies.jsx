import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { Info, Loader2, Play, RotateCcw } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { toast } from '@/components/ui/sonner';
import { listPauses, resumePause } from './supplyApi';
import { modelSupplyPath } from './supplyHelpers';
import { StateDot, durationLabel, formatUntil, ruleLabel, useSupplySwitches } from './supplyParts';

// "Paused" tab of the model page: every channel or model on a channel that a pause rule (or an
// admin) has taken out of rotation, with a button to put it back now.
export default function PausedSupplies() {
  const { t } = useTranslation();
  const switches = useSupplySwitches();
  const [pauses, setPauses] = useState(null);
  const [busyId, setBusyId] = useState(0);
  const [now, setNow] = useState(0);

  const load = useCallback(async () => {
    try {
      const { success, message, data } = await listPauses();
      setPauses(success ? data || [] : []);
      setNow(Math.floor(Date.now() / 1000));
      if (!success) toast.error(message);
    } catch {
      setPauses([]);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const resume = async (pause) => {
    setBusyId(pause.id);
    try {
      const { success, message } = await resumePause(pause.id);
      if (success) toast.success(t('modelSupply.resumed'));
      else toast.error(message);
    } catch {
      /* shown by the API client */
    } finally {
      setBusyId(0);
      load();
    }
  };

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        <p className="min-w-0 flex-1 text-[13px] text-muted-foreground">{t('modelSupply.pausedPage.description')}</p>
        <Button variant="outline" size="sm" className="rounded-[10px]" onClick={load}>
          <RotateCcw className="size-4" />
          {t('modelInfoPage.refresh')}
        </Button>
      </div>

      {switches && !switches.pauseRulesEnabled && (
        <p className="flex items-start gap-2 rounded-[10px] bg-muted px-3 py-2 text-[13px] text-muted-foreground">
          <Info className="mt-0.5 size-3.5 shrink-0" />
          <span>{t('modelSupply.pausedPage.rulesOff')}</span>
        </p>
      )}

      <div className="overflow-x-auto rounded-2xl border border-border bg-card">
        {pauses === null ? (
          <div className="flex h-40 items-center justify-center text-muted-foreground">
            <Loader2 className="size-5 animate-spin" />
          </div>
        ) : pauses.length === 0 ? (
          <div className="px-6 py-14 text-center">
            <p className="text-sm">{t('modelSupply.pausedPage.empty')}</p>
            <p className="mt-1 text-xs text-muted-foreground">{t('modelSupply.pausedPage.emptyHint')}</p>
          </div>
        ) : (
          <table className="w-full min-w-[720px] border-collapse text-[13px]">
            <thead>
              <tr className="text-left text-xs text-muted-foreground [&>th]:h-10 [&>th]:whitespace-nowrap [&>th]:border-b [&>th]:border-border [&>th]:px-4 [&>th]:font-medium">
                <th>{t('modelSupply.channel')}</th>
                <th>{t('modelSupply.pausedPage.scope')}</th>
                <th>{t('modelSupply.pausedPage.rule')}</th>
                <th>{t('modelSupply.pausedPage.until')}</th>
                <th>{t('modelSupply.pausedPage.reason')}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {pauses.map((p) => {
                const waiting = p.until <= now;
                return (
                  <tr
                    key={p.id}
                    data-testid="supply-pause"
                    className="align-middle [&>td]:h-12 [&>td]:border-b [&>td]:border-border [&>td]:px-4 last:[&>td]:border-b-0"
                  >
                    <td className="whitespace-nowrap">
                      <Link to={`/panel/channel/${p.channel_id}`} className="hover:underline">
                        {p.channel_name || `#${p.channel_id}`}
                      </Link>
                    </td>
                    <td className="whitespace-nowrap">
                      {p.model ? (
                        <Link to={modelSupplyPath(p.model)} className="font-mono hover:underline">
                          {p.model}
                        </Link>
                      ) : (
                        <span className="text-muted-foreground">{t('modelSupply.state.wholeChannel')}</span>
                      )}
                    </td>
                    <td className="whitespace-nowrap">{ruleLabel(t, p.rule)}</td>
                    <td className="whitespace-nowrap">
                      <span className="inline-flex items-center gap-1.5">
                        <StateDot tone="warn" />
                        {waiting ? t('modelSupply.state.probing') : formatUntil(p.until)}
                      </span>
                      <span className="ml-2 text-xs text-muted-foreground">
                        {p.seconds > 0 && durationLabel(t, p.seconds)}
                        {p.probe && ` · ${t('modelSupply.pausedPage.probe')}`}
                        {p.probes > 0 && ` · ${t('modelSupply.pausedPage.probes', { count: p.probes })}`}
                      </span>
                    </td>
                    <td className="max-w-[20rem] truncate text-muted-foreground" title={p.reason}>
                      {p.reason || '—'}
                    </td>
                    <td className="whitespace-nowrap text-right">
                      <Button variant="outline" size="sm" className="rounded-[10px]" disabled={busyId === p.id} onClick={() => resume(p)}>
                        {busyId === p.id ? <Loader2 className="animate-spin" /> : <Play className="size-4" />}
                        {t('modelSupply.resumeNow')}
                      </Button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
