import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { AlertTriangle, KeyRound, Loader2, RefreshCw } from 'lucide-react';

import { renderSpend, timestamp2string } from 'utils/common';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';
import { Checkbox } from '@/components/ui/checkbox';
import { toast } from '@/components/ui/sonner';
import { cacheSummary, formatBalance, metricsByModel, metricsSeries, summarizeMetrics } from './channelMetrics';
import { fetchChannelCacheStat, fetchChannelMetrics, fetchUpstreamModelPicker, updateChannel } from './channelApi';
import { buildModelSuggestions, fetchPriceList } from './modelSuggestions';
import { ResponseTime } from './channelColumns';
import ModelListInput from './ModelListInput';

const RANGES = [
  { key: '24h', seconds: 24 * 3600, bucket: 3600, i18n: 'channelDetail.range24h' },
  { key: '7d', seconds: 7 * 24 * 3600, bucket: 6 * 3600, i18n: 'channelDetail.range7d' }
];
const MODEL_ROWS = 8;

const pct = (v) => (v === null || v === undefined ? '-' : `${(v * 100).toFixed(v >= 0.999 || v === 0 ? 0 : 1)}%`);
const ms = (v) => (v === null || v === undefined ? '-' : v >= 1000 ? `${(v / 1000).toFixed(2)}s` : `${Math.round(v)}ms`);
const tps = (v) => (v === null || v === undefined ? '-' : `${v.toFixed(1)} tok/s`);
const num = (v) => (v || 0).toLocaleString();
const usd = (v) => `$${Number(v || 0).toFixed(2)}`;
const splitList = (s) =>
  Array.from(
    new Set(
      (s || '')
        .split(/[,\n]/)
        .map((x) => x.trim())
        .filter(Boolean)
    )
  );

function bucketLabel(ts, bucket, long = false) {
  const d = new Date(ts * 1000);
  const hh = String(d.getHours()).padStart(2, '0');
  if (!long && bucket <= 3600) return `${hh}:00`;
  return `${d.getMonth() + 1}/${d.getDate()} ${hh}:00`;
}

function parseMapping(raw) {
  if (!raw) return {};
  try {
    const m = JSON.parse(raw);
    return m && typeof m === 'object' && !Array.isArray(m) ? m : {};
  } catch {
    return {};
  }
}

// Edit state of one section: the draft starts from the channel; saving merges the patch into
// the channel (the API writes the whole row) and reloads.
function useSectionEdit(channel, makeDraft, toPatch, onSaved) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(null);
  const [saving, setSaving] = useState(false);
  const save = async () => {
    const patch = toPatch(draft);
    if (!patch) return;
    setSaving(true);
    try {
      const { success, message } = await updateChannel(channel, patch);
      if (success) {
        toast.success(t('channelDetail.saved'));
        setDraft(null);
        onSaved();
      } else toast.error(message);
    } catch (e) {
      toast.error(e.message);
    }
    setSaving(false);
  };
  return {
    editing: draft !== null,
    draft,
    setDraft,
    saving,
    start: () => setDraft(makeDraft(channel)),
    cancel: () => setDraft(null),
    save
  };
}

// One section of the document: a heading with its actions, read content, and while editing
// the same content as a form with save / cancel underneath.
function Section({ id, title, description, actions, edit, canEdit = true, editLabel, labeled = true, children }) {
  const { t } = useTranslation();
  return (
    <section aria-labelledby={`${id}-title`} className="border-t border-border py-8">
      <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 id={`${id}-title`} className="text-base font-medium">
            {title}
          </h2>
          {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
        </div>
        {!edit?.editing && (
          <div className="flex items-center gap-2">
            {actions}
            {edit && canEdit && (
              <Button variant="outline" size="sm" onClick={edit.start}>
                {editLabel || t('common.edit')}
              </Button>
            )}
          </div>
        )}
      </div>
      {children}
      {edit?.editing && (
        <div className={cn('mt-6 flex items-center gap-2', labeled && 'md:pl-[184px]')}>
          <Button size="sm" disabled={edit.saving} onClick={edit.save}>
            {edit.saving && <Loader2 className="size-4 animate-spin" />}
            {t('channelDetail.save')}
          </Button>
          <Button size="sm" variant="ghost" disabled={edit.saving} onClick={edit.cancel}>
            {t('common.cancel')}
          </Button>
        </div>
      )}
    </section>
  );
}

Section.propTypes = {
  id: PropTypes.string.isRequired,
  title: PropTypes.node,
  description: PropTypes.node,
  actions: PropTypes.node,
  edit: PropTypes.object,
  canEdit: PropTypes.bool,
  editLabel: PropTypes.string,
  labeled: PropTypes.bool,
  children: PropTypes.node
};

// Label / value rows in read mode; EditRow uses the same label column so nothing jumps.
function Fields({ children }) {
  return <dl className="grid max-w-3xl grid-cols-1 gap-x-6 gap-y-3 text-sm md:grid-cols-[160px_minmax(0,1fr)]">{children}</dl>;
}

function Field({ label, children }) {
  return (
    <>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="-mt-2 min-w-0 break-words md:mt-0">{children}</dd>
    </>
  );
}

function EditRow({ label, htmlFor, hint, children }) {
  return (
    <div className="grid max-w-3xl grid-cols-1 items-start gap-x-6 gap-y-1.5 md:grid-cols-[160px_minmax(0,1fr)]">
      <label htmlFor={htmlFor} className="text-sm text-muted-foreground md:pt-2">
        {label}
      </label>
      <div className="min-w-0 space-y-1.5">
        {children}
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      </div>
    </div>
  );
}

Fields.propTypes = { children: PropTypes.node };
Field.propTypes = { label: PropTypes.node, children: PropTypes.node };
EditRow.propTypes = { label: PropTypes.node, htmlFor: PropTypes.string, hint: PropTypes.node, children: PropTypes.node };

function Figure({ label, value, caption, tone }) {
  return (
    <div className="min-w-0 py-5 lg:px-6 lg:first:pl-0">
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className={cn('mt-1.5 text-2xl font-medium tabular-nums tracking-tight', tone)}>{value}</div>
      <div className="mt-1 truncate text-xs text-muted-foreground">{caption}</div>
    </div>
  );
}

Figure.propTypes = { label: PropTypes.node, value: PropTypes.node, caption: PropTypes.node, tone: PropTypes.string };

function Inline({ label, children }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="text-muted-foreground">{label}</span>
      <span className="tabular-nums">{children}</span>
    </span>
  );
}

Inline.propTypes = { label: PropTypes.node, children: PropTypes.node };

function Muted() {
  return <span className="text-muted-foreground">-</span>;
}

function TrafficTooltip({ active, payload, label, t, bucket }) {
  if (!active || !payload?.length) return null;
  const point = payload[0].payload;
  return (
    <div className="rounded-md border border-border bg-card px-3 py-2 text-xs shadow-sm">
      <p className="mb-1 text-muted-foreground">{bucketLabel(label, bucket, true)}</p>
      <p className="tabular-nums">
        {t('channelDetail.requests')} {num(point.requests)}
      </p>
      {point.failures > 0 && (
        <p className="tabular-nums text-destructive">
          {t('channelDetail.failures')} {num(point.failures)}
        </p>
      )}
    </div>
  );
}

TrafficTooltip.propTypes = {
  active: PropTypes.bool,
  payload: PropTypes.array,
  label: PropTypes.any,
  t: PropTypes.func,
  bucket: PropTypes.number
};

// The channel as a document: key figures, then models with their traffic, the traffic chart,
// keys and quota, routing and drift. Small sections edit in place; request rewriting,
// compatibility options and plugins stay in the full settings form.
export default function ChannelOverview({ channel, groupOptions, actions, busy, onSaved, onOpenSettings }) {
  const { t } = useTranslation();
  const [rangeKey, setRangeKey] = useState('24h');
  const range = RANGES.find((r) => r.key === rangeKey) || RANGES[0];
  const [stats, setStats] = useState({ loading: true, rows: [], cache: null, start: 0, end: 0 });
  const [showAllModels, setShowAllModels] = useState(false);

  useEffect(() => {
    let alive = true;
    const end = Math.floor(Date.now() / 1000);
    const start = end - range.seconds;
    Promise.all([
      fetchChannelMetrics(channel.id, start, end).catch(() => []),
      fetchChannelCacheStat(channel.id, start, end).catch(() => null)
    ]).then(([rows, cache]) => alive && setStats({ loading: false, rows, cache, start, end }));
    return () => {
      alive = false;
    };
  }, [channel.id, range.seconds]);

  const summary = useMemo(() => summarizeMetrics(stats.rows), [stats.rows]);
  const perModel = useMemo(() => new Map(metricsByModel(stats.rows).map((m) => [m.model, m])), [stats.rows]);
  const series = useMemo(
    () => (stats.end ? metricsSeries(stats.rows, stats.start, stats.end, range.bucket) : []),
    [stats.rows, stats.start, stats.end, range.bucket]
  );
  const cache = useMemo(() => cacheSummary(stats.cache), [stats.cache]);
  const rangeLabel = t(range.i18n);

  const models = useMemo(() => splitList(channel.models), [channel.models]);
  const mapping = useMemo(() => parseMapping(channel.model_mapping), [channel.model_mapping]);
  // Busiest first, the rest in their configured order.
  const modelRows = useMemo(
    () =>
      models
        .map((name, i) => ({ name, i, m: perModel.get(name) }))
        .sort((a, b) => (b.m?.requests || 0) - (a.m?.requests || 0) || a.i - b.i),
    [models, perModel]
  );
  const shownModels = showAllModels ? modelRows : modelRows.slice(0, MODEL_ROWS);

  const windows = channel.usage_windows?.windows || [];
  const snapshot = channel.quota_snapshot;
  const drift = channel.model_drift;
  const missing = drift?.missing_models || [];
  const added = drift?.new_models || [];
  const groups = splitList(channel.group);
  const keyStatus = channel.key_status;
  const isTagged = Boolean(channel.tag);

  const modelsEdit = useSectionEdit(
    channel,
    (c) => ({ models: c.models || '' }),
    (d) => {
      const list = splitList(d.models);
      if (!list.length) {
        toast.error(t('channel_edit.requiredModels'));
        return null;
      }
      return { models: list.join(',') };
    },
    onSaved
  );
  const keysEdit = useSectionEdit(
    channel,
    (c) => ({ key: '', threshold: String(c.balance_threshold || 0) }),
    (d) => ({
      ...(d.key.trim() ? { key: d.key.trim() } : {}),
      balance_threshold: Math.max(0, Number(d.threshold) || 0)
    }),
    onSaved
  );
  const routingEdit = useSectionEdit(
    channel,
    (c) => ({
      groups: splitList(c.group),
      priority: String(c.priority ?? 0),
      weight: String(c.weight ?? 1),
      base_url: c.base_url || '',
      proxy: c.proxy || '',
      test_model: c.test_model || ''
    }),
    (d) => {
      if (!d.groups.length) {
        toast.error(t('channel_edit.requiredGroup'));
        return null;
      }
      const base = d.base_url.trim();
      return {
        group: d.groups.join(','),
        priority: Number(d.priority) || 0,
        weight: Number(d.weight) || 1,
        base_url: base.endsWith('/') ? base.slice(0, -1) : base,
        proxy: d.proxy.trim(),
        test_model: d.test_model.trim()
      };
    },
    onSaved
  );
  const driftEdit = useSectionEdit(
    channel,
    (c) => ({ auto: !!c.drift_auto_add, rules: c.drift_ignore_rules || '' }),
    (d) => ({ drift_auto_add: d.auto, drift_ignore_rules: d.rules.trim() }),
    onSaved
  );

  // Model picker data, loaded the first time the models editor opens.
  const [prices, setPrices] = useState(null);
  const [upstream, setUpstream] = useState({ status: 'idle', items: [] });
  useEffect(() => {
    if (!modelsEdit.editing || prices !== null) return;
    fetchPriceList().then(setPrices);
  }, [modelsEdit.editing, prices]);
  const suggestions = useMemo(() => buildModelSuggestions(channel.type, prices || []), [channel.type, prices]);
  const loadUpstream = async () => {
    if (upstream.status !== 'idle') return;
    setUpstream({ status: 'loading', items: [] });
    try {
      const data = await fetchUpstreamModelPicker({
        id: channel.id,
        type: channel.type,
        key: '',
        base_url: channel.base_url,
        other: channel.other
      });
      setUpstream({ status: 'done', items: data.models.map((r) => ({ id: r.model, releasedAt: r.released_at || undefined })) });
    } catch {
      setUpstream({ status: 'error', items: [] });
    }
  };

  let balanceValue;
  let balanceCaption;
  if (windows.length) {
    const top = windows.reduce((a, b) => (b.used_percent > a.used_percent ? b : a), windows[0]);
    balanceValue = `${Math.round(top.used_percent)}%`;
    balanceCaption = t('channelDetail.windowUsed', { name: top.name });
  } else if (channel.balance_updated_time) {
    balanceValue = formatBalance(channel.type, channel.balance);
    balanceCaption = t('channelDetail.balanceUpdated', { time: timestamp2string(channel.balance_updated_time) });
  } else {
    balanceValue = <span className="text-muted-foreground">-</span>;
    balanceCaption = t('channelDetail.balanceNever');
  }
  const lowBalance =
    !windows.length && channel.balance_threshold > 0 && channel.balance_updated_time && channel.balance < channel.balance_threshold;

  const rangeSwitch = (
    <div className="flex gap-1" role="group" aria-label={t('channelDetail.range')}>
      {RANGES.map((r) => (
        <Button
          key={r.key}
          size="sm"
          variant={r.key === rangeKey ? 'secondary' : 'ghost'}
          className={cn('h-7 px-2.5 text-xs', r.key === rangeKey && 'text-foreground')}
          onClick={() => setRangeKey(r.key)}
        >
          {t(r.i18n)}
        </Button>
      ))}
    </div>
  );

  return (
    <div className="max-w-6xl">
      {channel.remark && <p className="-mt-2 mb-6 max-w-3xl text-sm text-muted-foreground">{channel.remark}</p>}

      <div className="grid grid-cols-2 gap-x-6 border-y border-border lg:grid-cols-4 lg:gap-x-0 lg:divide-x lg:divide-border">
        <Figure label={t('channelDetail.used')} value={renderSpend(channel.used_quota)} caption={t('channelDetail.usedCaption')} />
        <Figure
          label={windows.length ? t('channelDetail.subscription') : t('channelDetail.balance')}
          value={balanceValue}
          caption={balanceCaption}
          tone={lowBalance ? 'text-destructive' : undefined}
        />
        <Figure
          label={t('channelDetail.requestsIn', { range: rangeLabel })}
          value={stats.loading ? '-' : num(summary.requests)}
          caption={summary.requests ? t('channelDetail.successRate', { rate: pct(summary.successRate) }) : t('channelDetail.noTraffic')}
          tone={summary.successRate !== null && summary.successRate < 0.9 ? 'text-destructive' : undefined}
        />
        <Figure
          label={t('channelDetail.cacheHit')}
          value={stats.loading ? '-' : pct(cache.hitRate)}
          caption={
            cache.affinityRate !== null ? t('channelDetail.affinity', { rate: pct(cache.affinityRate) }) : t('channelDetail.noTraffic')
          }
        />
      </div>

      <div className="mt-8 [&>section:first-child]:border-t-0 [&>section:first-child]:pt-0">
        <Section
          id="models"
          title={t('channelDetail.modelsTitle', { count: models.length })}
          description={
            isTagged ? t('channelDetail.modelsTagged', { tag: channel.tag }) : t('channelDetail.modelsHint', { range: rangeLabel })
          }
          edit={modelsEdit}
          canEdit={!isTagged}
          labeled={false}
          actions={
            <Button variant="ghost" size="sm" onClick={() => onOpenSettings('model')}>
              {t('channelDetail.mappingSettings')}
            </Button>
          }
        >
          {modelsEdit.editing ? (
            <div className="max-w-3xl">
              <ModelListInput
                value={modelsEdit.draft.models}
                onChange={(v) => modelsEdit.setDraft({ models: v })}
                suggestions={suggestions}
                upstream={upstream.items}
                onOpen={loadUpstream}
                loading={upstream.status === 'loading'}
                notice={upstream.status === 'error' ? t('modelSelect.upstreamFailed') : null}
              />
            </div>
          ) : models.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('channelDetail.noModels')}</p>
          ) : (
            <>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border text-left text-xs text-muted-foreground">
                      <th className="pb-2 pr-4 font-normal">{t('channelDetail.colModel')}</th>
                      <th className="px-4 pb-2 text-right font-normal">{t('channelDetail.requests')}</th>
                      <th className="px-4 pb-2 text-right font-normal">{t('channelDetail.colSuccess')}</th>
                      <th className="px-4 pb-2 text-right font-normal">{t('channelDetail.avgLatency')}</th>
                      <th className="px-4 pb-2 text-right font-normal">{t('channelDetail.ttft')}</th>
                      <th className="pb-2 pl-4 text-right font-normal">{t('channelDetail.speed')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {shownModels.map(({ name, m }) => (
                      <tr key={name} className="border-b border-border last:border-0">
                        <td className="max-w-[24rem] py-2.5 pr-4">
                          <span className="block truncate font-mono text-xs" title={name}>
                            {name}
                          </span>
                          {mapping[name] && (
                            <span className="block truncate font-mono text-xs text-muted-foreground">→ {mapping[name]}</span>
                          )}
                        </td>
                        <td className="px-4 py-2.5 text-right tabular-nums">{m ? num(m.requests) : <Muted />}</td>
                        <td
                          className={cn(
                            'px-4 py-2.5 text-right tabular-nums',
                            m && m.successRate !== null && m.successRate < 0.9 && 'text-destructive'
                          )}
                        >
                          {m ? pct(m.successRate) : <Muted />}
                        </td>
                        <td className="px-4 py-2.5 text-right tabular-nums">{m ? ms(m.avgLatencyMs) : <Muted />}</td>
                        <td className="px-4 py-2.5 text-right tabular-nums">{m ? ms(m.avgTtftMs) : <Muted />}</td>
                        <td className="py-2.5 pl-4 text-right tabular-nums">{m ? tps(m.tokensPerSec) : <Muted />}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {modelRows.length > MODEL_ROWS && (
                <Button variant="ghost" size="sm" className="mt-2 px-0 text-muted-foreground" onClick={() => setShowAllModels((v) => !v)}>
                  {showAllModels ? t('channelDetail.showFewer') : t('channelDetail.showAll', { count: modelRows.length })}
                </Button>
              )}
            </>
          )}
        </Section>

        <Section id="traffic" title={t('channelDetail.trafficTitle')} actions={rangeSwitch}>
          <div className="mb-5 flex flex-wrap gap-x-8 gap-y-2 text-sm">
            <Inline label={t('channelDetail.avgLatency')}>{ms(summary.avgLatencyMs)}</Inline>
            <Inline label={t('channelDetail.ttft')}>{ms(summary.avgTtftMs)}</Inline>
            <Inline label={t('channelDetail.speed')}>{tps(summary.tokensPerSec)}</Inline>
            <Inline label={t('channelDetail.failures')}>
              <span className={cn(summary.failures > 0 && 'text-destructive')}>{num(summary.failures)}</span>
            </Inline>
            <Inline label={t('channelDetail.lastTest')}>
              {channel.test_time ? (
                <span className="inline-flex items-center gap-1.5">
                  <ResponseTime ms={channel.response_time} />
                  <span className="text-xs text-muted-foreground">{timestamp2string(channel.test_time)}</span>
                </span>
              ) : (
                <span className="text-muted-foreground">{t('channelDetail.neverTested')}</span>
              )}
            </Inline>
          </div>
          {stats.loading ? (
            <div className="flex h-60 items-center justify-center text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
            </div>
          ) : summary.requests === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-muted-foreground">{t('channelDetail.metricsEmpty')}</div>
          ) : (
            <ResponsiveContainer width="100%" height={240}>
              <BarChart data={series} margin={{ top: 4, right: 0, left: 0, bottom: 0 }} barCategoryGap={2}>
                <CartesianGrid stroke="var(--border)" vertical={false} />
                <XAxis
                  dataKey="ts"
                  tickFormatter={(v) => bucketLabel(v, range.bucket)}
                  tick={{ fill: 'var(--muted-foreground)', fontSize: 11, fontFamily: 'var(--font-mono, monospace)' }}
                  axisLine={false}
                  tickLine={false}
                  minTickGap={24}
                />
                <YAxis
                  width={40}
                  allowDecimals={false}
                  tick={{ fill: 'var(--muted-foreground)', fontSize: 11, fontFamily: 'var(--font-mono, monospace)' }}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip content={<TrafficTooltip t={t} bucket={range.bucket} />} cursor={{ fill: 'var(--muted)' }} />
                <Bar
                  dataKey={(d) => d.requests - d.failures}
                  stackId="r"
                  fill="var(--foreground)"
                  fillOpacity={0.55}
                  isAnimationActive={false}
                />
                <Bar dataKey="failures" stackId="r" fill="var(--destructive)" radius={[2, 2, 0, 0]} isAnimationActive={false} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </Section>

        <Section
          id="keys"
          title={t('channelDetail.keysTitle')}
          edit={keysEdit}
          actions={
            <Button variant="ghost" size="sm" disabled={busy.refreshing} onClick={() => actions.onRefreshBalance(channel)}>
              {busy.refreshing ? <Loader2 className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />}
              {t('channelDetail.refreshBalance')}
            </Button>
          }
        >
          {keysEdit.editing ? (
            <div className="space-y-4">
              <EditRow label={t('channelDetail.replaceKey')} htmlFor="cd-key" hint={t('channelDetail.replaceKeyHint')}>
                <Textarea
                  id="cd-key"
                  rows={2}
                  value={keysEdit.draft.key}
                  placeholder="••••••••"
                  onChange={(e) => keysEdit.setDraft({ ...keysEdit.draft, key: e.target.value })}
                />
              </EditRow>
              <EditRow label={t('channelDetail.threshold')} htmlFor="cd-threshold" hint={t('channelDetail.thresholdHint')}>
                <Input
                  id="cd-threshold"
                  type="number"
                  min="0"
                  className="w-40"
                  value={keysEdit.draft.threshold}
                  onChange={(e) => keysEdit.setDraft({ ...keysEdit.draft, threshold: e.target.value })}
                />
              </EditRow>
            </div>
          ) : (
            <Fields>
              <Field label={t('channelDetail.key')}>
                {keyStatus && !keyStatus.configured ? (
                  <span className="inline-flex items-center gap-1.5 text-amber-600 dark:text-amber-400">
                    <AlertTriangle className="size-3.5" />
                    {t('channel_edit.keyNotConfigured')}
                  </span>
                ) : keyStatus ? (
                  <span className="inline-flex flex-wrap items-center gap-1.5">
                    <KeyRound className="size-3.5 text-muted-foreground" />
                    <span className="font-mono text-xs">{keyStatus.masked.join('  ')}</span>
                    {keyStatus.count > 1 && (
                      <span className="text-xs text-muted-foreground">{t('channelDetail.keyCount', { count: keyStatus.count })}</span>
                    )}
                  </span>
                ) : (
                  <Muted />
                )}
              </Field>
              {windows.length > 0 ? (
                windows.map((w, i) => (
                  <Field key={w.name || i} label={w.name}>
                    <span className="flex flex-wrap items-center gap-3">
                      <span className="h-1.5 w-40 overflow-hidden rounded-full bg-muted">
                        <span
                          className={cn('block h-full rounded-full', w.used_percent >= 90 ? 'bg-destructive' : 'bg-foreground/60')}
                          style={{ width: `${Math.min(100, Math.max(0, w.used_percent))}%` }}
                        />
                      </span>
                      <span className="tabular-nums">{Math.round(w.used_percent)}%</span>
                      {w.reset_at ? (
                        <span className="text-xs text-muted-foreground">
                          {t('channelDetail.resetsAt', { time: timestamp2string(w.reset_at) })}
                        </span>
                      ) : null}
                    </span>
                  </Field>
                ))
              ) : snapshot ? (
                <>
                  <Field label={t('channelDetail.quotaRemaining')}>
                    <span className="tabular-nums">
                      {snapshot.limit === null || snapshot.limit === undefined
                        ? t('channelDetail.unlimited')
                        : `${usd(snapshot.remaining)} / ${usd(snapshot.limit)}`}
                    </span>
                    <span className="ml-2 text-xs text-muted-foreground">
                      {t(`channelDetail.reset.${snapshot.reset_interval || 'none'}`)}
                    </span>
                  </Field>
                  <Field label={t('channelDetail.upstreamUsage')}>
                    <span className="tabular-nums">
                      {t('channelDetail.usageLine', {
                        day: usd(snapshot.usage_daily),
                        week: usd(snapshot.usage_weekly),
                        month: usd(snapshot.usage_monthly)
                      })}
                    </span>
                  </Field>
                </>
              ) : (
                <Field label={t('channelDetail.balance')}>
                  {channel.balance_updated_time ? (
                    <span className="tabular-nums">{formatBalance(channel.type, channel.balance)}</span>
                  ) : (
                    <span className="text-muted-foreground">{t('channelDetail.balanceHint')}</span>
                  )}
                </Field>
              )}
              <Field label={t('channelDetail.threshold')}>
                {channel.balance_threshold > 0 ? (
                  <span className="tabular-nums">{formatBalance(channel.type, channel.balance_threshold)}</span>
                ) : (
                  <span className="text-muted-foreground">{t('channelDetail.thresholdOff')}</span>
                )}
              </Field>
              {(snapshot?.fetched_at || channel.balance_updated_time) > 0 && (
                <Field label={t('channelDetail.checkedAt')}>
                  <span className="text-muted-foreground">{timestamp2string(snapshot?.fetched_at || channel.balance_updated_time)}</span>
                </Field>
              )}
            </Fields>
          )}
        </Section>

        <Section id="routing" title={t('channelDetail.routingTitle')} description={t('channelDetail.routingHint')} edit={routingEdit}>
          {routingEdit.editing ? (
            <div className="space-y-4">
              <EditRow label={t('channel_index.group')}>
                <div className="flex flex-wrap gap-x-4 gap-y-2 md:pt-2">
                  {(groupOptions.length ? groupOptions : ['default']).map((g) => (
                    <label key={g} className="flex items-center gap-2 text-sm">
                      <Checkbox
                        checked={routingEdit.draft.groups.includes(g)}
                        onCheckedChange={(c) =>
                          routingEdit.setDraft({
                            ...routingEdit.draft,
                            groups: c ? [...routingEdit.draft.groups, g] : routingEdit.draft.groups.filter((x) => x !== g)
                          })
                        }
                      />
                      {g}
                    </label>
                  ))}
                </div>
              </EditRow>
              <EditRow label={t('channel_index.priority')} htmlFor="cd-priority" hint={t('channelDetail.priorityHint')}>
                <Input
                  id="cd-priority"
                  type="number"
                  className="w-32"
                  value={routingEdit.draft.priority}
                  onChange={(e) => routingEdit.setDraft({ ...routingEdit.draft, priority: e.target.value })}
                />
              </EditRow>
              <EditRow label={t('channel_index.weight')} htmlFor="cd-weight" hint={t('channelDetail.weightHint')}>
                <Input
                  id="cd-weight"
                  type="number"
                  min="1"
                  className="w-32"
                  value={routingEdit.draft.weight}
                  onChange={(e) => routingEdit.setDraft({ ...routingEdit.draft, weight: e.target.value })}
                />
              </EditRow>
              <EditRow label={t('channelDetail.baseUrl')} htmlFor="cd-base">
                <Input
                  id="cd-base"
                  className="font-mono text-xs"
                  value={routingEdit.draft.base_url}
                  placeholder={t('channelDetail.baseUrlDefault')}
                  onChange={(e) => routingEdit.setDraft({ ...routingEdit.draft, base_url: e.target.value })}
                />
              </EditRow>
              <EditRow label={t('channelDetail.proxy')} htmlFor="cd-proxy">
                <Input
                  id="cd-proxy"
                  className="font-mono text-xs"
                  value={routingEdit.draft.proxy}
                  placeholder="socks5://host:port"
                  onChange={(e) => routingEdit.setDraft({ ...routingEdit.draft, proxy: e.target.value })}
                />
              </EditRow>
              <EditRow label={t('channel_index.testModel')} htmlFor="cd-test-model">
                <Input
                  id="cd-test-model"
                  className="font-mono text-xs"
                  value={routingEdit.draft.test_model}
                  onChange={(e) => routingEdit.setDraft({ ...routingEdit.draft, test_model: e.target.value })}
                />
              </EditRow>
            </div>
          ) : (
            <Fields>
              <Field label={t('channel_index.group')}>{groups.join(', ') || <Muted />}</Field>
              <Field label={t('channel_index.priority')}>
                <span className="tabular-nums">{channel.priority ?? 0}</span>
              </Field>
              <Field label={t('channel_index.weight')}>
                <span className="tabular-nums">{channel.weight ?? 1}</span>
              </Field>
              <Field label={t('channelDetail.baseUrl')}>
                {channel.base_url ? (
                  <span className="font-mono text-xs">{channel.base_url}</span>
                ) : (
                  <span className="text-muted-foreground">{t('channelDetail.baseUrlDefault')}</span>
                )}
              </Field>
              <Field label={t('channelDetail.proxy')}>
                {channel.proxy ? <span className="font-mono text-xs">{channel.proxy}</span> : <Muted />}
              </Field>
              <Field label={t('channel_index.testModel')}>
                {channel.test_model ? (
                  <span className="font-mono text-xs">{channel.test_model}</span>
                ) : (
                  <span className="text-muted-foreground">{t('channelDetail.testModelAuto')}</span>
                )}
              </Field>
              {isTagged && <Field label={t('channelDetail.tag')}>{channel.tag}</Field>}
            </Fields>
          )}
        </Section>

        <Section
          id="drift"
          title={t('channelDetail.driftTitle')}
          description={t('channelDetail.driftHint')}
          edit={driftEdit}
          editLabel={t('channelDetail.driftRules')}
          actions={
            <Button variant="ghost" size="sm" disabled={busy.drift} onClick={() => actions.onCheckDrift(channel)}>
              {busy.drift ? <Loader2 className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />}
              {t('channelDetail.driftCheck')}
            </Button>
          }
        >
          {driftEdit.editing ? (
            <div className="space-y-4">
              <EditRow label={t('channel_edit.drift.autoAdd')} hint={t('channel_edit.drift.autoAddHelp')}>
                <div className="md:pt-1.5">
                  <Switch checked={driftEdit.draft.auto} onCheckedChange={(v) => driftEdit.setDraft({ ...driftEdit.draft, auto: v })} />
                </div>
              </EditRow>
              <EditRow label={t('channel_edit.drift.ignoreRules')} htmlFor="cd-drift-rules" hint={t('channel_edit.drift.ignoreRulesHelp')}>
                <Textarea
                  id="cd-drift-rules"
                  rows={3}
                  className="font-mono text-xs"
                  value={driftEdit.draft.rules}
                  onChange={(e) => driftEdit.setDraft({ ...driftEdit.draft, rules: e.target.value })}
                />
              </EditRow>
            </div>
          ) : (
            <Fields>
              <Field label={t('channelDetail.driftState')}>
                {!drift?.checked_at ? (
                  <span className="text-muted-foreground">{t('channelDetail.driftNever')}</span>
                ) : (
                  <>
                    {missing.length === 0 && added.length === 0 ? t('channelDetail.driftOk') : t('channelDetail.driftChanged')}
                    <span className="ml-2 text-xs text-muted-foreground">
                      {t('channelDetail.driftCheckedAt', { time: timestamp2string(drift.checked_at) })}
                    </span>
                  </>
                )}
              </Field>
              {missing.length > 0 && (
                <Field label={t('channelDetail.driftMissing', { count: missing.length })}>
                  <span className="font-mono text-xs text-amber-600 dark:text-amber-400">{missing.join(', ')}</span>
                </Field>
              )}
              {added.length > 0 && (
                <Field label={t('channelDetail.driftNew', { count: added.length })}>
                  <span className="font-mono text-xs">{added.join(', ')}</span>
                  <Button
                    variant="link"
                    size="sm"
                    className="ml-2 h-auto p-0 text-xs"
                    disabled={busy.dismissing}
                    onClick={() => actions.onDismissNewModels(channel)}
                  >
                    {t('channelDetail.driftDismiss')}
                  </Button>
                </Field>
              )}
              <Field label={t('channel_edit.drift.autoAdd')}>
                {channel.drift_auto_add ? t('channelDetail.on') : t('channelDetail.off')}
              </Field>
              {channel.drift_ignore_rules && (
                <Field label={t('channel_edit.drift.ignoreRules')}>
                  <span className="whitespace-pre-line font-mono text-xs">{channel.drift_ignore_rules}</span>
                </Field>
              )}
            </Fields>
          )}
        </Section>

        <p className="border-t border-border pt-6 text-sm text-muted-foreground">
          {t('channelDetail.moreInSettings')}{' '}
          <button
            type="button"
            className="rounded-sm text-foreground underline underline-offset-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            onClick={() => onOpenSettings('request')}
          >
            {t('channelDetail.allSettings')}
          </button>
        </p>
      </div>
    </div>
  );
}

ChannelOverview.propTypes = {
  channel: PropTypes.object.isRequired,
  groupOptions: PropTypes.array,
  actions: PropTypes.object.isRequired,
  busy: PropTypes.object.isRequired,
  onSaved: PropTypes.func.isRequired,
  onOpenSettings: PropTypes.func.isRequired
};
