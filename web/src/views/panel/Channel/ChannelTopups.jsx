import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2, Plus, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from '@/components/ui/sonner';
import { Field } from '@/components/objects';
import { timestamp2string } from 'utils/common';
import { addChannelTopup, deleteChannelTopup, fetchChannelTopups } from './channelApi';

// The top-ups of a channel's provider account on the advanced tab: what was paid (card and
// currency fees included) for the credit added. The fee rate is derived from the latest of them
// and stored on the channel by the server, so the panel reloads the channel after a change
// (`onChanged`).

// The last summary per channel, so the block keeps its rows while the tab rebuilds after a reload.
const summaries = new Map();

const usd = (v) => `$${(Number(v) || 0).toFixed(2)}`;
const pct = (v) => `${(Math.round(v * 1000) / 10).toFixed(1)}%`;
// What one top-up implies: paid over credited, minus one (below zero when the provider added a bonus).
export const topupRate = (t) => (t.credited > 0 ? t.paid / t.credited - 1 : 0);

const EMPTY_FORM = { paid: '', credited: '', note: '' };

export default function ChannelTopups({ channelId, onChanged }) {
  const { t } = useTranslation();
  const [summary, setSummary] = useState(() => summaries.get(channelId) || null);
  const [failed, setFailed] = useState('');
  const [form, setForm] = useState(EMPTY_FORM);
  const [busy, setBusy] = useState(0); // -1 adding, a top-up id deleting

  useEffect(() => {
    let live = true;
    fetchChannelTopups(channelId)
      .then((s) => {
        if (!live) return;
        summaries.set(channelId, s);
        setSummary(s);
        setFailed('');
      })
      .catch((e) => live && setFailed(e.message || t('channels.topups.loadFailed')));
    return () => {
      live = false;
    };
  }, [channelId, t]);

  const apply = async (s) => {
    summaries.set(channelId, s);
    setSummary(s);
    await onChanged?.(s);
  };

  const paid = Number(form.paid);
  const credited = Number(form.credited);
  const valid = paid > 0 && credited > 0;

  const add = async () => {
    if (!valid) return toast.error(t('channels.topups.invalid'));
    setBusy(-1);
    try {
      await apply(await addChannelTopup(channelId, { paid, credited, note: form.note.trim() }));
      setForm(EMPTY_FORM);
      toast.success(t('channels.topups.added'));
    } catch (e) {
      toast.error(e.message);
    } finally {
      setBusy(0);
    }
    return undefined;
  };

  const remove = async (topup) => {
    if (!window.confirm(t('channels.topups.deleteConfirm', { paid: usd(topup.paid), credited: usd(topup.credited) }))) return;
    setBusy(topup.id);
    try {
      await apply(await deleteChannelTopup(channelId, topup.id));
    } catch (e) {
      toast.error(e.message);
    } finally {
      setBusy(0);
    }
  };

  const topups = summary?.topups || [];
  const used = summary ? Math.min(topups.length, summary.window || topups.length) : 0;

  return (
    <Field label={t('channels.topups.label')} optional hint={t('channels.topups.hint')}>
      {!summary && !failed ? (
        <div className="space-y-2" aria-busy="true">
          <Skeleton className="h-3.5 w-56" />
          <Skeleton className="h-9 w-full rounded-lg" />
        </div>
      ) : (
        <div className="space-y-2">
          {failed && <p className="text-xs text-destructive">{failed}</p>}
          {summary && (
            <p className="text-xs text-muted-foreground" data-testid="topup-summary">
              {summary.derived ? t('channels.topups.derived', { count: used, rate: pct(summary.fee_rate) }) : t('channels.topups.none')}
            </p>
          )}
          {topups.length > 0 && (
            <div className="overflow-x-auto rounded-lg border border-border">
              <table className="w-full text-[13px]">
                <thead>
                  <tr className="text-left text-xs text-muted-foreground [&>th]:h-8 [&>th]:px-2.5 [&>th]:font-normal">
                    <th>{t('channels.topups.date')}</th>
                    <th className="text-right">{t('channels.topups.paid')}</th>
                    <th className="text-right">{t('channels.topups.credited')}</th>
                    <th className="text-right">{t('channels.topups.rate')}</th>
                    <th>{t('channels.topups.note')}</th>
                    <th>
                      <span className="sr-only">{t('channels.topups.delete')}</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {topups.map((tp, i) => (
                    <tr
                      key={tp.id}
                      data-testid="topup-row"
                      className={i >= used ? 'border-t border-border text-muted-foreground' : 'border-t border-border'}
                    >
                      <td className="whitespace-nowrap px-2.5 py-1.5 tabular-nums">{timestamp2string(tp.created_at).slice(0, 10)}</td>
                      <td className="px-2.5 py-1.5 text-right tabular-nums">{usd(tp.paid)}</td>
                      <td className="px-2.5 py-1.5 text-right tabular-nums">{usd(tp.credited)}</td>
                      <td className="px-2.5 py-1.5 text-right tabular-nums">{pct(topupRate(tp))}</td>
                      <td className="max-w-[10rem] truncate px-2.5 py-1.5 text-muted-foreground" title={tp.note}>
                        {tp.note || '—'}
                      </td>
                      <td className="w-9 px-1 py-1 text-right">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-7 rounded-lg text-muted-foreground hover:text-destructive"
                          aria-label={t('channels.topups.delete')}
                          disabled={busy !== 0}
                          onClick={() => remove(tp)}
                        >
                          {busy === tp.id ? <Loader2 className="size-3.5 animate-spin" /> : <Trash2 className="size-3.5" />}
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <div className="flex flex-wrap items-center gap-2">
            <Input
              type="number"
              min="0"
              step="0.01"
              className="h-8 w-28 tabular-nums"
              aria-label={t('channels.topups.paid')}
              placeholder={t('channels.topups.paidPlaceholder')}
              value={form.paid}
              onChange={(e) => setForm({ ...form, paid: e.target.value })}
            />
            <Input
              type="number"
              min="0"
              step="0.01"
              className="h-8 w-28 tabular-nums"
              aria-label={t('channels.topups.credited')}
              placeholder={t('channels.topups.creditedPlaceholder')}
              value={form.credited}
              onChange={(e) => setForm({ ...form, credited: e.target.value })}
            />
            <Input
              className="h-8 min-w-[8rem] flex-1"
              aria-label={t('channels.topups.note')}
              placeholder={t('channels.topups.notePlaceholder')}
              maxLength={255}
              value={form.note}
              onChange={(e) => setForm({ ...form, note: e.target.value })}
            />
            <Button variant="outline" size="sm" className="h-8 rounded-lg" disabled={!valid || busy !== 0} onClick={add}>
              {busy === -1 ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}
              {t('channels.topups.add')}
            </Button>
          </div>
          {valid && <p className="text-xs text-muted-foreground">{t('channels.topups.implied', { rate: pct(paid / credited - 1) })}</p>}
        </div>
      )}
    </Field>
  );
}

ChannelTopups.propTypes = { channelId: PropTypes.number.isRequired, onChanged: PropTypes.func };
