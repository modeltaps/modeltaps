import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  CloudDownload,
  Copy,
  KeyRound,
  Loader2,
  Plus,
  RefreshCw,
  Trash2,
  X,
  AlertTriangle
} from 'lucide-react';

import { timestamp2string } from 'utils/common';
import { configText } from 'i18n/configText';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { toast } from '@/components/ui/sonner';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { BrandIconRefreshButton } from '@/components/brand/BrandIconPicker';
import { iconCacheHost } from '@/components/brand/brandIconManifest';
import { formatBalance, metricsByModel, modelSignal } from './channelMetrics';
import {
  applyChannelPricing,
  fetchChannelModelDetails,
  fetchChannelPrices,
  fetchChannelUpstreamModels,
  fetchUpstreamModelPicker,
  previewChannelPricing,
  refreshChannelPrices
} from './channelApi';
import { amount, priceIndex, priceSummary, sortByTokenPrice, upstreamNameOf } from './channelPrices';
import { lossOf, orderByRank, sortByName } from './channelModelDetails';
import { STATE_FILTERS, isGone, matchesState, offerOf, snapshotOf, stateCounts, unlistedOf } from './channelModelStates';
import { CAPABILITY_FILTERS, formatContext } from './modelPicker';
import { buildModelSuggestions, fetchPriceList } from './modelSuggestions';
import { splitList } from './channelGroups';
import { usageNumbers, usageOf, usageOptions, usagePeers } from './channelUsage';
import { applyMappingToModels, normalizeKeyInput } from './newChannel';
import { BASE_URL_REQUIRED, DEFAULT_BASE_URLS, OAUTH_TYPES, typeLabels } from './providerDefaults';
import { Field, LongListMore, LongListSearch, ToggleField, useLongList } from '@/components/objects';
import { GroupOptions, KeyLink, UsageOptions } from './channelParts';
import ChannelOAuth from './ChannelOAuth';
import ModelListInput from './ModelListInput';
import ModelMappingInput from './ModelMappingInput';
import ModelHeadersInput from './ModelHeadersInput';
import PauseRulesField, { pauseRulesPatch, pauseRulesText } from './PauseRulesField';
import PluginForm from './PluginForm';
import pluginConfig from './pluginConfig';
import PriceSyncDialog from './PriceSyncDialog';
import ModelSelectorModal from './ModelSelectorModal';
import BalanceAlertField from './BalanceAlertField';
import { attachPendingModel } from '../ModelInfo/supply/supplyApi';
import { modelSupplyPath } from '../ModelInfo/supply/supplyHelpers';
import { performanceTab } from './PerformanceTab';

// Tabs of the channel panel (see components/objects/ObjectPanel). Each tab edits a part of the
// channel on its own: `make(channel)` builds the tab's draft, `patch(draft, channel, ctx)` turns
// it into the fields to save (or `{ error }`), and `Body` renders the draft.

const pct = (v) => (v === null || v === undefined ? '—' : `${(v * 100).toFixed(1)}%`);
const trimSlash = (s) => (s.endsWith('/') ? s.slice(0, -1) : s);

function prettyJson(raw) {
  if (!raw) return '';
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function headerRows(raw) {
  if (!raw) return [];
  try {
    return Object.entries(JSON.parse(raw)).map(([key, value]) => ({ key, value: String(value ?? '') }));
  } catch {
    return [];
  }
}

function headerJson(rows) {
  const obj = {};
  for (const r of rows || []) if (r?.key && r?.value && !(r.key in obj)) obj[r.key] = r.value;
  return Object.keys(obj).length ? JSON.stringify(obj) : '';
}

function parseMapping(raw) {
  if (!raw || !raw.trim()) return {};
  try {
    const m = JSON.parse(raw);
    return m && typeof m === 'object' && !Array.isArray(m) ? m : {};
  } catch {
    return null;
  }
}

function ToolButton({ busy, icon: Icon, children, ...props }) {
  return (
    <Button variant="outline" size="sm" className="h-[30px] rounded-lg bg-card" disabled={busy || props.disabled} {...props}>
      {busy ? <Loader2 className="animate-spin" /> : Icon ? <Icon /> : null}
      {children}
    </Button>
  );
}

ToolButton.propTypes = { busy: PropTypes.bool, icon: PropTypes.elementType, children: PropTypes.node, disabled: PropTypes.bool };

// What the upstream charges for one listed model, as the Input and Output cells. A model not
// priced per token (per image, per second, per request, free, other, or no price) gets one cell
// across both.
function UpstreamPriceCells({ price, publics }) {
  const { t } = useTranslation();
  const summary = priceSummary(price);
  if (summary?.kind === 'tokens') {
    return ['input', 'output'].map((key) => {
      const loss = lossOf(price, publics, key);
      return (
        <td key={key} className="text-right tabular-nums">
          {loss ? (
            <LossMark
              text={t(`channels.models.loss_${key}`, { model: loss.model, sell: amount(loss.sell), cost: amount(loss.cost) })}
              value={summary[key]}
            />
          ) : (
            summary[key]
          )}
        </td>
      );
    });
  }
  let content;
  if (!summary)
    content = (
      <span className="text-muted-foreground" title={t('channels.models.priceMissing')}>
        —
      </span>
    );
  else if (summary.kind === 'other')
    content = (
      <span className="text-muted-foreground" title={t('channels.models.priceOtherHint')}>
        {t('channels.models.priceOther')}
      </span>
    );
  else if (summary.kind === 'free') content = t('channels.models.priceFree');
  else content = t(`channels.models.price_${summary.kind}`, { price: summary.value });
  return (
    <td colSpan={2} className="text-right tabular-nums">
      {content}
    </td>
  );
}

UpstreamPriceCells.propTypes = { price: PropTypes.object, publics: PropTypes.array };

// A price the upstream charges above what a public model it serves sells for.
function LossMark({ text, value }) {
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="inline-flex items-center gap-1 text-warning" aria-label={text}>
            <AlertTriangle className="size-3" />
            {value}
          </span>
        </TooltipTrigger>
        <TooltipContent className="max-w-64 whitespace-normal text-xs">{text}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

LossMark.propTypes = { text: PropTypes.string.isRequired, value: PropTypes.node };

// Small action at the end of a model's second line.
function LineAction({ busy, onClick, children }) {
  return (
    <button
      type="button"
      disabled={busy}
      onClick={onClick}
      className="inline-flex shrink-0 items-center gap-1 rounded px-1 text-xs text-foreground underline-offset-2 hover:underline disabled:opacity-50"
    >
      {busy && <Loader2 className="size-3 animate-spin" />}
      {children}
    </button>
  );
}

LineAction.propTypes = { busy: PropTypes.bool, onClick: PropTypes.func, children: PropTypes.node };

// Second line of a model's name while supplies route requests: the public model it is offered
// as (+N more), or why it is not offered with the step that offers it.
function PublicLine({ publics, offer, busy, onAttach }) {
  const { t } = useTranslation();
  if (!publics) return null;
  if (offer && !offer.offered) {
    const first = publics[0];
    const vars = { model: first?.model || offer.sameAs || offer.suggested, other: offer.suppliedBy };
    const reason = `${t('channels.models.notOffered')} · ${t(`channels.models.unoffered.${offer.reason}`, vars)}`;
    // The line gives the reason alone (it implies "not offered"), a duplicate's in short; the
    // tooltip says it in full.
    const line = t(`channels.models.unoffered.${offer.reason === 'duplicate' ? 'duplicateShort' : offer.reason}`, vars);
    return (
      <span className="flex min-w-0 items-center gap-1 text-xs text-muted-foreground">
        <span className="min-w-0 truncate" title={reason}>
          {line}
        </span>
        {offer.reason === 'duplicate' && offer.suggested && (
          <LineAction busy={busy} onClick={() => onAttach(offer.suggested, true)}>
            {t('channels.models.addAsOwn', { model: offer.suggested })}
          </LineAction>
        )}
        {offer.reason === 'not_in_catalog' && offer.suggested && (
          <LineAction busy={busy} onClick={() => onAttach(offer.suggested, true)}>
            {t('channels.models.addToCatalog')}
          </LineAction>
        )}
        {offer.reason === 'no_supply' && (
          <LineAction busy={busy} onClick={() => onAttach(offer.suggested, false)}>
            {t('channels.models.attach', { model: offer.suggested })}
          </LineAction>
        )}
        {offer.reason === 'off' && first && (
          <Link to={modelSupplyPath(first.model)} className="shrink-0 px-1 text-foreground underline-offset-2 hover:underline">
            {t('channels.models.openSupply')}
          </Link>
        )}
      </span>
    );
  }
  if (!publics.length) return <span className="block truncate text-xs text-muted-foreground">{t('channels.models.notOffered')}</span>;
  const first = publics[0];
  return (
    <span className="flex min-w-0 items-center gap-1 text-xs text-muted-foreground">
      <span className="shrink-0">{t('channels.models.offeredAs')}</span>
      <Link
        to={modelSupplyPath(first.model)}
        className={cn('min-w-0 truncate font-mono hover:text-foreground hover:underline', !first.enabled && 'line-through')}
      >
        {first.model}
      </Link>
      {publics.length > 1 && <span className="shrink-0">+{publics.length - 1}</span>}
    </span>
  );
}

PublicLine.propTypes = { publics: PropTypes.array, offer: PropTypes.object, busy: PropTypes.bool, onAttach: PropTypes.func };

// One filter chip above the model table: a click shows only its rows, a second click shows all.
function FilterChip({ active, onClick, icon, label, count }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        'inline-flex h-[26px] items-center gap-1 rounded-full border px-2.5 text-xs',
        active ? 'border-foreground/30 bg-muted text-foreground' : 'border-border bg-field text-muted-foreground hover:text-foreground'
      )}
    >
      {icon}
      {label}
      <span className="tabular-nums">{count}</span>
    </button>
  );
}

FilterChip.propTypes = {
  active: PropTypes.bool,
  onClick: PropTypes.func,
  icon: PropTypes.node,
  label: PropTypes.string,
  count: PropTypes.number
};

const MODALITIES = ['image', 'audio', 'video', 'file', 'pdf'];

// Tooltip text of a model's name: release date, context, what it can do and why it is where it
// is in the usual order.
function modelHint(t, name, d) {
  if (!d) return name;
  const lines = [name];
  if (d.public?.length) {
    lines.push(
      `${t('channels.models.public')}: ${d.public.map((p) => (p.enabled ? p.model : `${p.model} (${t('channels.models.supplyOff')})`)).join(', ')}`
    );
  }
  const facts = [];
  if (d.released_at) facts.push(t('channels.models.hint.released', { date: new Date(d.released_at * 1000).toISOString().slice(0, 10) }));
  if (d.context_length) facts.push(t('channels.models.hint.context', { context: formatContext(d.context_length) }));
  if (facts.length) lines.push(facts.join(' · '));
  const caps = (d.capabilities || []).map((c) => (CAPABILITY_FILTERS.includes(c) ? t(`channel_edit.picker.cap_${c}`) : c));
  const inputs = (d.input_modalities || []).filter((m) => m !== 'text');
  const modality = (m) => (MODALITIES.includes(m) ? t(`channels.models.hint.modality.${m}`) : m);
  if (inputs.length) caps.push(...inputs.map((m) => t('channels.models.hint.input', { modality: modality(m) })));
  if (caps.length) lines.push(caps.join(t('channels.created.comma')));
  lines.push(
    d.family_requests
      ? t('channels.models.hint.rankUsed', { rank: d.rank, family: d.family, count: d.family_requests })
      : t('channels.models.hint.rankFallback', { rank: d.rank, family: d.family })
  );
  return lines.join('\n');
}

// Header of a sortable column: a click sorts ascending, then descending, then back to the usual
// order.
function SortHeader({ label, active, dir, onClick, title, align = 'right' }) {
  const Icon = !active ? ArrowUpDown : dir === 'asc' ? ArrowUp : ArrowDown;
  return (
    <th
      className={align === 'right' ? 'text-right' : undefined}
      aria-sort={active ? (dir === 'asc' ? 'ascending' : 'descending') : 'none'}
      title={title}
    >
      <button
        type="button"
        onClick={onClick}
        className={cn('inline-flex items-center gap-1 hover:text-foreground', active && 'text-foreground')}
      >
        {label}
        <Icon className={cn('size-3', !active && 'opacity-50')} />
      </button>
    </th>
  );
}

SortHeader.propTypes = {
  label: PropTypes.node.isRequired,
  active: PropTypes.bool,
  dir: PropTypes.oneOf(['asc', 'desc']),
  onClick: PropTypes.func.isRequired,
  title: PropTypes.string,
  align: PropTypes.oneOf(['left', 'right'])
};

// ---------- Models ----------

// Flag after a model's name when most of its requests in the range failed. The tooltip gives
// the numbers.
function ModelSignal({ signal, m, range }) {
  const { t } = useTranslation();
  // Unused models are not marked row by row: on a large upstream list most of them are.
  if (signal !== 'failing') return null;
  const text = t('channels.models.signal.failing', { range, rate: pct(m.successRate), failures: m.failures, requests: m.requests });
  return (
    <TooltipProvider delayDuration={150}>
      <Tooltip>
        <TooltipTrigger asChild>
          <span role="img" aria-label={text} className="inline-flex shrink-0 text-warning">
            <AlertTriangle className="size-3.5" />
          </span>
        </TooltipTrigger>
        <TooltipContent className="text-xs">{text}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

ModelSignal.propTypes = { signal: PropTypes.oneOf(['idle', 'failing']), m: PropTypes.object, range: PropTypes.string.isRequired };

// Search text of a row in the channel's model list.
const modelRowText = (row) => row.name;

function ModelsBody({ object: channel, draft, setDraft, ctx }) {
  const { t } = useTranslation();
  const labels = typeLabels(channel.type);
  const models = useMemo(() => splitList(draft.models), [draft.models]);
  // The mapping is edited on the Advanced tab; this tab reads the saved one.
  const mapping = useMemo(() => parseMapping(channel.model_mapping) || {}, [channel.model_mapping]);
  // Traffic only flags models here (unused or failing); the numbers live on the Performance
  // tab. Metrics are keyed by the name sent upstream, so a mapped model is looked up by its target.
  const perModel = useMemo(
    () => new Map(metricsByModel((ctx.metricsRows || []).filter((r) => Number(r.channel_id) === channel.id)).map((m) => [m.model, m])),
    [ctx.metricsRows, channel.id]
  );
  const hasTraffic = useMemo(() => [...perModel.values()].some((m) => m.requests > 0), [perModel]);
  // Details of the list as edited (catalog metadata, the usual order, public models and why a
  // model is not offered), read again shortly after the list changes or a model is offered.
  const [details, setDetails] = useState(null);
  const [detailsStamp, setDetailsStamp] = useState(0);
  useEffect(() => {
    let alive = true;
    const timer = setTimeout(() => {
      fetchChannelModelDetails(channel.id, splitList(draft.models), mapping)
        .then((list) => alive && setDetails(new Map(list.map((d) => [d.model, d]))))
        .catch(() => alive && setDetails((cur) => cur || new Map()));
    }, 300);
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [channel.id, draft.models, mapping, detailsStamp]);
  const detailOf = (name) => details?.get(name);

  // What the upstream listed at the last model check, read again after each check.
  const checkedAt = channel.model_drift?.checked_at || 0;
  const [upstreamList, setUpstreamList] = useState(null);
  useEffect(() => {
    let alive = true;
    fetchChannelUpstreamModels(channel.id)
      .then((data) => alive && setUpstreamList(data))
      .catch(() => alive && setUpstreamList(null));
    return () => {
      alive = false;
    };
  }, [channel.id, checkedAt]);
  const snapshot = useMemo(() => snapshotOf(upstreamList), [upstreamList]);
  const unlisted = useMemo(() => unlistedOf(upstreamList, models, mapping), [upstreamList, models, mapping]);

  // The channel's own order; the table shows the usual order below (most used families first).
  const rows = useMemo(
    () =>
      splitList(draft.models).map((name) => {
        const m = perModel.get(upstreamNameOf(name, mapping));
        return {
          name,
          m,
          signal: modelSignal(m, hasTraffic),
          offer: ctx.supplyRouting ? offerOf(details?.get(name)) : null,
          gone: isGone(name, mapping, snapshot)
        };
      }),
    [draft.models, mapping, perModel, hasTraffic, ctx.supplyRouting, details, snapshot]
  );
  const range = t(`channels.range.short.${ctx.rangeKey}`);
  // Showing only the rows of one filter (a state, or unused / failing traffic); '' shows all.
  const [picked, setOnly] = useState('');
  const counts = useMemo(() => stateCounts(rows, unlisted), [rows, unlisted]);
  // Offered / not offered only mean something while supplies route requests; gone and unlisted
  // need an upstream list.
  const stateKeys = STATE_FILTERS.filter((key) =>
    key === 'offered' || key === 'unoffered' ? ctx.supplyRouting && details : key === 'gone' ? snapshot : counts.unlisted > 0
  );
  // A filter whose chip went away (the last unlisted model was added) shows everything again.
  const only = STATE_FILTERS.includes(picked) && !stateKeys.includes(picked) ? '' : picked;
  const signalCounts = useMemo(() => {
    const out = { idle: 0, failing: 0 };
    for (const r of rows) if (r.signal) out[r.signal] += 1;
    return out;
  }, [rows]);
  const filtered = useMemo(
    () => (!only ? rows : STATE_FILTERS.includes(only) ? rows.filter((r) => matchesState(r, only)) : rows.filter((r) => r.signal === only)),
    [rows, only]
  );
  // Sorting by name or an upstream price is asked for from the column header; otherwise the usual
  // order from the model details.
  const [sort, setSort] = useState(null);
  const nextSort = (key) => setSort((cur) => (cur?.key !== key ? { key, dir: 'asc' } : cur.dir === 'asc' ? { key, dir: 'desc' } : null));
  // The mapping column only earns its width when some model is mapped.
  const hasMapping = models.some((name) => mapping[name]);
  const [adding, setAdding] = useState(false);
  const [busy, setBusy] = useState('');
  const [prices, setPrices] = useState(null);
  const [upstream, setUpstream] = useState({ status: 'idle', items: [] });
  const [pricePreview, setPricePreview] = useState(null);
  const [applying, setApplying] = useState(false);
  const [selector, setSelector] = useState(null);
  // The upstream's own price list for this channel; only providers that publish prices have one.
  const [upstreamPrices, setUpstreamPrices] = useState(null);

  useEffect(() => {
    let alive = true;
    fetchChannelPrices(channel.id)
      .then((data) => alive && setUpstreamPrices(data))
      .catch(() => alive && setUpstreamPrices(null));
    return () => {
      alive = false;
    };
  }, [channel.id]);
  const pricesByModel = useMemo(() => priceIndex(upstreamPrices?.prices), [upstreamPrices]);

  const showPrices = Boolean(upstreamPrices?.supported);
  const shown = useMemo(() => {
    if (sort?.key === 'name') return sortByName(filtered, sort.dir);
    if (sort && showPrices) {
      return sortByTokenPrice(filtered, sort.key, sort.dir, (r) => pricesByModel.get(upstreamNameOf(r.name, mapping)));
    }
    return orderByRank(filtered, (r) => details?.get(r.name));
  }, [filtered, sort, showPrices, pricesByModel, mapping, details]);
  const showPublic = Boolean(ctx.supplyRouting);
  const list = useLongList(shown, { text: modelRowText });

  useEffect(() => {
    if (adding && prices === null) fetchPriceList().then(setPrices);
  }, [adding, prices]);
  const suggestions = useMemo(() => buildModelSuggestions(channel.type, prices || []), [channel.type, prices]);
  const body = { id: channel.id, type: channel.type, key: '', base_url: channel.base_url, other: channel.other };

  const loadUpstream = async () => {
    if (!labels.canFetchModels || upstream.status !== 'idle') return;
    setUpstream({ status: 'loading', items: [] });
    try {
      const data = await fetchUpstreamModelPicker(body);
      setUpstream({ status: 'done', items: data.models.map((r) => ({ id: r.model, releasedAt: r.released_at || undefined })) });
    } catch {
      setUpstream({ status: 'error', items: [] });
    }
  };

  const run = async (name, fn) => {
    setBusy(name);
    try {
      await fn();
    } catch (e) {
      toast.error(e.message);
    } finally {
      setBusy('');
    }
  };

  // Offers a listed model: as the public model it belongs to, or (create) as a new draft catalog
  // model, which is listed once it has a price and is published on the model page.
  const attach = (name, publicModel, create) =>
    run(`attach:${name}`, async () => {
      const { success, message } = await attachPendingModel({
        upstream_model: upstreamNameOf(name, mapping),
        model: publicModel,
        channel_ids: [channel.id],
        create
      });
      if (!success) throw new Error(message);
      toast.success(t(create ? 'channels.models.addedToCatalog' : 'channels.models.attached', { model: publicModel }));
      setDetailsStamp((n) => n + 1);
    });
  const addNames = (names) => {
    setDraft({ ...draft, models: Array.from(new Set([...models, ...names])).join(',') });
    toast.success(t('channels.models.fetched', { count: names.length }));
  };
  const removeGone = () => {
    const gone = new Set(rows.filter((r) => r.gone).map((r) => r.name));
    setDraft({ ...draft, models: models.filter((m) => !gone.has(m)).join(',') });
    toast.success(t('channels.models.pruned', { count: gone.size }));
    setOnly('');
  };

  const priceChannel = () => ({
    id: channel.id,
    name: channel.name,
    type: channel.type,
    key: '',
    models: models.join(','),
    group: channel.group
  });
  const syncPrices = () =>
    run('price', async () => {
      // The button reads the upstream price list, so the channel's own copy is refreshed too.
      refreshChannelPrices(channel.id)
        .then(setUpstreamPrices)
        .catch(() => {});
      const preview = await previewChannelPricing(priceChannel());
      setPricePreview(preview);
      if (preview.unmatched.length) {
        toast.warning(t('channel_edit.syncPricingUnmatched', { count: preview.unmatched.length, models: preview.unmatched.join(', ') }));
      }
    });
  const applyPrices = async (list) => {
    setApplying(true);
    try {
      const { synced } = await applyChannelPricing(priceChannel(), list);
      toast.success(t('channel_edit.syncPricingDone', { count: synced }));
      setPricePreview(null);
    } catch (e) {
      toast.error(e.message);
    } finally {
      setApplying(false);
    }
  };

  const remove = (name) => setDraft({ ...draft, models: models.filter((m) => m !== name).join(',') });

  return (
    <>
      <div className="flex flex-wrap items-center gap-2 py-3">
        <span className="text-[13px] text-muted-foreground">
          {t('channels.models.count', { count: models.length })}
          {channel.drift_auto_add ? ` · ${t('channels.models.autoAdd')}` : ''}
          {labels.canFetchModels &&
            ` · ${checkedAt ? t('channelDetail.driftCheckedAt', { time: timestamp2string(checkedAt) }) : t('channelDetail.driftNever')}`}
        </span>
        <span className="flex-1" />
        {labels.canFetchModels && (
          <ToolButton
            icon={RefreshCw}
            busy={Boolean(ctx.busy?.drift)}
            onClick={() => ctx.actions?.onCheckDrift(channel)}
            title={t('channels.models.checkHint')}
          >
            {t('channels.models.check')}
          </ToolButton>
        )}
        {(channel.type === 20 || channel.type === 62) && (
          <ToolButton icon={CloudDownload} busy={busy === 'price'} onClick={syncPrices}>
            {t('channels.models.syncPrices')}
          </ToolButton>
        )}
        <ToolButton icon={adding ? X : Plus} onClick={() => setAdding((v) => !v)}>
          {adding ? t('channels.models.doneAdding') : t('channels.models.add')}
        </ToolButton>
      </div>

      {adding && (
        <div className="pb-3">
          <ModelListInput
            value={draft.models}
            onChange={(v) => setDraft({ ...draft, models: v })}
            suggestions={suggestions}
            upstream={upstream.items}
            onOpen={loadUpstream}
            loading={upstream.status === 'loading'}
            notice={upstream.status === 'error' ? t('modelSelect.upstreamFailed') : null}
            advanced={labels.canFetchModels ? { onClick: () => setSelector({ ...body, models: draft.models }) } : undefined}
          />
        </div>
      )}

      {rows.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border py-8 text-center text-sm text-muted-foreground">
          {t('channels.models.none')}
        </p>
      ) : (
        <>
          {(stateKeys.length > 0 || signalCounts.idle > 0 || signalCounts.failing > 0) && (
            <div className="flex flex-wrap items-center gap-1.5 pb-2 text-[13px]">
              {stateKeys.map((key) => (
                <FilterChip
                  key={key}
                  active={only === key}
                  onClick={() => setOnly(only === key ? '' : key)}
                  icon={key === 'gone' && counts.gone > 0 ? <AlertTriangle className="size-3 text-warning" /> : null}
                  label={t(`channels.models.state.${key}`)}
                  count={counts[key]}
                />
              ))}
              {(signalCounts.idle > 0 || signalCounts.failing > 0) && (
                <>
                  {stateKeys.length > 0 && <span className="mx-1 h-4 w-px bg-border" />}
                  <span className="text-muted-foreground">{t('channels.models.signal.label', { range })}</span>
                  {['idle', 'failing'].map(
                    (key) =>
                      signalCounts[key] > 0 && (
                        <FilterChip
                          key={key}
                          active={only === key}
                          onClick={() => setOnly(only === key ? '' : key)}
                          icon={key === 'failing' ? <AlertTriangle className="size-3 text-warning" /> : null}
                          label={t(`channels.models.signal.${key}Filter`)}
                          count={signalCounts[key]}
                        />
                      )
                  )}
                </>
              )}
              {only === 'gone' && counts.gone > 0 && (
                <Button variant="outline" size="sm" className="ml-auto h-[26px] rounded-lg bg-card text-xs" onClick={removeGone}>
                  {t('channels.models.removeGone', { count: counts.gone })}
                </Button>
              )}
              {only === 'unlisted' && counts.unlisted > 0 && (
                <Button
                  variant="outline"
                  size="sm"
                  className="ml-auto h-[26px] rounded-lg bg-card text-xs"
                  onClick={() => addNames(unlisted.map((u) => u.name))}
                >
                  {t('channels.models.addUnlisted', { count: unlisted.length })}
                </Button>
              )}
            </div>
          )}
          <LongListSearch list={list} className="pb-2" />
          <div className="overflow-hidden rounded-lg border border-border">
            {/* Fixed widths for the narrow columns, so the model name gets the rest of the panel. */}
            <table className="w-full table-fixed border-collapse text-[13px]">
              <colgroup>
                <col />
                {hasMapping && <col className="w-[22%]" />}
                <col className="w-[4.5rem]" />
                {showPrices && <col className="w-[5.5rem]" />}
                {showPrices && <col className="w-[5.5rem]" />}
                <col className="w-9" />
              </colgroup>
              <thead>
                <tr className="text-left text-xs text-muted-foreground [&>th]:h-9 [&>th]:whitespace-nowrap [&>th]:border-b [&>th]:border-border [&>th]:px-3 [&>th]:font-medium">
                  <SortHeader
                    align="left"
                    label={t('channels.models.model')}
                    active={sort?.key === 'name'}
                    dir={sort?.dir}
                    onClick={() => nextSort('name')}
                    title={t('channels.models.orderHint')}
                  />
                  {hasMapping && <th>{t('channels.models.mappedTo')}</th>}
                  <th className="text-right">{t('channels.models.context')}</th>
                  {showPrices &&
                    ['input', 'output'].map((key) => (
                      <SortHeader
                        key={key}
                        label={t(`channels.models.upstreamPrice_${key}`)}
                        active={sort?.key === key}
                        dir={sort?.dir}
                        onClick={() => nextSort(key)}
                        title={
                          upstreamPrices.synced_at
                            ? t('channels.models.upstreamPriceHint', { time: timestamp2string(upstreamPrices.synced_at) })
                            : t('channels.models.upstreamPriceNever')
                        }
                      />
                    ))}
                  <th />
                </tr>
              </thead>
              <tbody>
                {only !== 'unlisted' &&
                  list.visible.map(({ name, m, signal, offer, gone }) => (
                    <tr
                      key={name}
                      className="[&>td]:h-11 [&>td]:overflow-hidden [&>td]:text-ellipsis [&>td]:whitespace-nowrap [&>td]:border-b [&>td]:border-border [&>td]:px-3 last:[&>td]:border-b-0"
                    >
                      <td title={modelHint(t, name, detailOf(name))}>
                        <span className="flex min-w-0 items-center gap-1.5">
                          <span className="min-w-0 truncate font-mono">{name}</span>
                          <ModelSignal signal={signal} m={m} range={range} />
                          {gone && (
                            <span className="shrink-0 rounded bg-warning/10 px-1.5 text-[11px] leading-[18px] text-warning">
                              {t('channels.models.state.gone')}
                            </span>
                          )}
                        </span>
                        {showPublic && (
                          <PublicLine
                            publics={detailOf(name)?.public}
                            offer={offer}
                            busy={busy === `attach:${name}`}
                            onAttach={(publicModel, create) => attach(name, publicModel, create)}
                          />
                        )}
                      </td>
                      {hasMapping && (
                        <td className="font-mono text-muted-foreground" title={mapping[name] || ''}>
                          {mapping[name] || '—'}
                        </td>
                      )}
                      <td className="text-right tabular-nums text-muted-foreground">
                        {formatContext(detailOf(name)?.context_length) || '—'}
                      </td>
                      {showPrices && (
                        <UpstreamPriceCells price={pricesByModel.get(upstreamNameOf(name, mapping))} publics={detailOf(name)?.public} />
                      )}
                      <td className="!px-1 text-right">
                        <button
                          type="button"
                          aria-label={t('channels.models.remove', { model: name })}
                          onClick={() => remove(name)}
                          className="inline-flex size-6 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                        >
                          <X className="size-3.5" />
                        </button>
                      </td>
                    </tr>
                  ))}
                {only === 'unlisted' &&
                  unlisted.map((u) => (
                    <tr
                      key={u.name}
                      data-testid="unlisted-model"
                      className="[&>td]:h-11 [&>td]:overflow-hidden [&>td]:text-ellipsis [&>td]:whitespace-nowrap [&>td]:border-b [&>td]:border-border [&>td]:px-3 last:[&>td]:border-b-0"
                    >
                      <td colSpan={hasMapping ? 3 : 2}>
                        <span className="block min-w-0 truncate font-mono">{u.name}</span>
                        <span className="block truncate text-xs text-muted-foreground">
                          {t(`channels.models.unlistedState.${u.state}`, { model: u.public_model, defaultValue: u.state })}
                        </span>
                      </td>
                      {showPrices && <UpstreamPriceCells price={pricesByModel.get(u.name)} publics={[]} />}
                      <td className="!px-1 text-right">
                        <button
                          type="button"
                          aria-label={t('channels.models.addOne', { model: u.name })}
                          onClick={() => addNames([u.name])}
                          className="inline-flex size-6 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                        >
                          <Plus className="size-3.5" />
                        </button>
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
            {only !== 'unlisted' && <LongListMore list={list} />}
          </div>
        </>
      )}

      {ctx.supplyRouting && <p className="mt-2 text-xs text-muted-foreground">{t('channels.models.supplyNote')}</p>}

      <ModelSelectorModal
        open={!!selector}
        channelValues={selector}
        onClose={() => setSelector(null)}
        onConfirm={(ids, overwrite) =>
          setDraft({ ...draft, models: (overwrite ? ids : Array.from(new Set([...models, ...ids]))).join(',') })
        }
      />
      {pricePreview && (
        <PriceSyncDialog open preview={pricePreview} applying={applying} onApply={applyPrices} onClose={() => setPricePreview(null)} />
      )}
    </>
  );
}

const modelsTab = {
  key: 'models',
  make: (c) => ({ models: splitList(c.models).join(',') }),
  patch: (d, c, { t }) => {
    const models = splitList(d.models);
    if (!models.length) return { error: t('channel_edit.requiredModels') };
    return { models: applyMappingToModels(models, parseMapping(c.model_mapping) || {}).join(',') };
  },
  Body: ModelsBody
};

// ---------- Connection ----------

function UsageWindows({ channel }) {
  const { t } = useTranslation();
  const windows = channel.usage_windows?.windows || [];
  if (!windows.length) return null;
  return (
    <Field label={t('channels.conn.subscription')}>
      <div className="overflow-hidden rounded-lg border border-border">
        {windows.map((w, i) => (
          <div key={w.name || i} className="flex min-h-11 items-center gap-3 px-3 text-[13px] [&+&]:border-t [&+&]:border-border">
            <span className="w-16 shrink-0 font-mono">{w.name}</span>
            <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
              <span
                className="block h-full rounded-full bg-foreground/60"
                style={{ width: `${Math.min(100, Math.max(0, w.used_percent))}%` }}
              />
            </span>
            <span className="w-10 text-right tabular-nums">{Math.round(w.used_percent)}%</span>
            <span className="w-40 shrink-0 text-right text-xs text-muted-foreground">
              {w.reset_at ? t('channels.conn.resetsAt', { time: timestamp2string(w.reset_at) }) : ''}
            </span>
          </div>
        ))}
      </div>
    </Field>
  );
}

UsageWindows.propTypes = { channel: PropTypes.object.isRequired };

function ConnectionBody({ object: channel, draft, setDraft, ctx }) {
  const { t } = useTranslation();
  const labels = typeLabels(channel.type);
  const oauth = OAUTH_TYPES.has(channel.type);
  const [replacing, setReplacing] = useState(false);
  const ks = channel.key_status;
  const defaultUrl = DEFAULT_BASE_URLS[channel.type];
  const snapshot = channel.quota_snapshot;
  const usd = (v) => `$${Number(v || 0).toFixed(2)}`;

  return (
    <>
      <Field
        label={t('channels.conn.baseUrl')}
        htmlFor="cp-base"
        optional={!BASE_URL_REQUIRED.has(channel.type)}
        hint={
          defaultUrl
            ? t('channels.conn.baseUrlDefault', { url: defaultUrl.replace(/^https?:\/\//, '') })
            : configText(t, labels.prompt.base_url)
        }
      >
        <Input
          id="cp-base"
          className="font-mono text-[13px]"
          value={draft.base_url}
          placeholder={defaultUrl || 'https://your-host/v1'}
          onChange={(e) => setDraft({ ...draft, base_url: e.target.value })}
        />
      </Field>
      <Field label={t('channels.conn.proxy')} htmlFor="cp-proxy" optional hint={t('channels.conn.proxyHint')}>
        <Input
          id="cp-proxy"
          className="font-mono text-[13px]"
          value={draft.proxy}
          placeholder="socks5://host:port"
          onChange={(e) => setDraft({ ...draft, proxy: e.target.value })}
        />
      </Field>
      {labels.hasOther && (
        <Field label={configText(t, labels.label.other)} htmlFor="cp-other" hint={configText(t, labels.prompt.other)}>
          <Input
            id="cp-other"
            className="font-mono text-[13px]"
            value={draft.other}
            onChange={(e) => setDraft({ ...draft, other: e.target.value })}
          />
        </Field>
      )}
      <Field label={oauth ? t('channels.conn.account') : t('channels.conn.key')} extra={!oauth && <KeyLink type={channel.type} />}>
        <div className="overflow-hidden rounded-lg border border-border">
          <div className="flex min-h-11 items-center gap-3 px-3 text-[13px]">
            {ks && !ks.configured ? (
              <>
                <AlertTriangle className="size-4 text-warning" />
                <span className="flex-1">{t('channels.conn.noKey')}</span>
              </>
            ) : (
              <>
                <KeyRound className="size-4 text-muted-foreground" />
                <span className="flex-1 truncate font-mono">
                  {oauth ? t('channels.conn.authorized') : ks?.masked?.[0] || '—'}
                  {draft.key && <span className="ml-2 font-sans text-xs text-warning">{t('channels.conn.replacedPending')}</span>}
                </span>
              </>
            )}
            {!oauth && (
              <Button variant="outline" size="sm" className="h-[30px] rounded-lg" onClick={() => setReplacing((v) => !v)}>
                {replacing ? t('common.cancel') : t('channels.conn.replaceKey')}
              </Button>
            )}
          </div>
          {oauth && (
            <div className="border-t border-border px-3 py-2.5">
              <ChannelOAuth
                type={channel.type}
                channelId={channel.id}
                getValues={(name) => ({ other: draft.other ?? channel.other, proxy: draft.proxy })[name] ?? ''}
                onCredentials={(credentials) => setDraft({ ...draft, key: credentials })}
              />
            </div>
          )}
          {!oauth && replacing && (
            <div className="border-t border-border p-3">
              <Textarea
                rows={3}
                autoFocus
                className="font-mono text-[13px]"
                value={draft.key}
                placeholder={configText(t, labels.prompt.key)}
                onChange={(e) => setDraft({ ...draft, key: e.target.value })}
              />
              <p className="mt-1.5 text-xs text-muted-foreground">{t('channels.conn.replaceHint')}</p>
            </div>
          )}
        </div>
      </Field>
      <UsageWindows channel={channel} />
      {!channel.usage_windows?.windows?.length && (
        <Field
          label={t('channels.conn.balance')}
          extra={
            <button
              type="button"
              className="inline-flex items-center gap-1 text-xs text-info hover:underline disabled:opacity-50"
              disabled={ctx.busy.refreshing}
              onClick={() => ctx.actions.onRefreshBalance(channel)}
            >
              {ctx.busy.refreshing ? <Loader2 className="size-3 animate-spin" /> : <RefreshCw className="size-3" />}
              {t('channels.conn.refreshBalance')}
            </button>
          }
        >
          <p className="text-[13px]">
            {snapshot ? (
              <>
                {snapshot.limit === null || snapshot.limit === undefined
                  ? t('channelDetail.unlimited')
                  : `${usd(snapshot.remaining)} / ${usd(snapshot.limit)}`}
                <span className="ml-2 text-xs text-muted-foreground">
                  {t('channelDetail.usageLine', {
                    day: usd(snapshot.usage_daily),
                    week: usd(snapshot.usage_weekly),
                    month: usd(snapshot.usage_monthly)
                  })}
                </span>
              </>
            ) : channel.balance_updated_time ? (
              <>
                <span className="tabular-nums">{formatBalance(channel.type, channel.balance)}</span>
                <span className="ml-2 text-xs text-muted-foreground">
                  {t('channelDetail.balanceUpdated', { time: timestamp2string(channel.balance_updated_time) })}
                </span>
              </>
            ) : (
              <span className="text-muted-foreground">{t('channelDetail.balanceHint')}</span>
            )}
          </p>
        </Field>
      )}
    </>
  );
}

const connectionTab = {
  key: 'connection',
  make: (c) => ({ base_url: c.base_url || '', proxy: c.proxy || '', other: c.other || '', key: '' }),
  patch: (d, c, { t }) => {
    const base = trimSlash(d.base_url.trim());
    if (BASE_URL_REQUIRED.has(c.type) && !base) return { error: t('channel_edit.requiredBaseUrl') };
    const key = normalizeKeyInput(d.key);
    if (key.includes('\n')) return { error: t('channels.conn.oneKey') };
    return { base_url: base, proxy: d.proxy.trim(), other: d.other.trim(), ...(key ? { key } : {}) };
  },
  Body: ConnectionBody
};

// ---------- Groups & routing ----------

function RoutingBody({ object: channel, draft, setDraft, ctx }) {
  const { t } = useTranslation();
  const peers = usagePeers({ ...channel, groups: draft.groups }, ctx.allChannels);
  const usage = usageOptions(channel, peers);
  const value = usage.options.some((o) => o.key === draft.usage) ? draft.usage : 'custom';
  return (
    <>
      <Field label={t('channels.groups')} extra={t('channels.groupsHint')}>
        <GroupOptions groups={ctx.groups} value={draft.groups} onChange={(groups) => setDraft({ ...draft, groups })} />
      </Field>
      {ctx.supplyRouting ? (
        <Field label={t('channels.usage.label')}>
          <p className="text-sm text-muted-foreground">
            {t('channels.usage.byPolicy')}{' '}
            <Link to="/panel/channel" className="text-info underline-offset-2 hover:underline">
              {t('channels.usage.openPolicy')}
            </Link>
          </p>
        </Field>
      ) : (
        <Field
          label={t('channels.usage.label')}
          extra={usage.top ? t('channels.usage.peerNote', { name: usage.top.name }) : null}
          hint={t('channels.usage.syncHint')}
        >
          <UsageOptions
            usage={usage}
            value={value}
            onChange={(key) => setDraft({ ...draft, usage: key })}
            custom={draft.custom}
            onCustomChange={(custom) => setDraft({ ...draft, custom })}
          />
        </Field>
      )}
      <Field label={t('channels.tag')} htmlFor="cp-tag" optional hint={t('channels.tagHint')}>
        <Input id="cp-tag" value={draft.tag} maxLength={32} onChange={(e) => setDraft({ ...draft, tag: e.target.value })} />
      </Field>
    </>
  );
}

const routingTab = {
  key: 'routing',
  make: (c, { allChannels }) => {
    const groups = splitList(c.group);
    return {
      groups: groups.length ? groups : ['default'],
      usage: usageOf(c, usagePeers({ ...c, groups }, allChannels)),
      custom: { priority: String(c.priority ?? 0), weight: String(c.weight ?? 1) },
      tag: c.tag || ''
    };
  },
  patch: (d, c, { t, allChannels, supplyRouting }) => {
    if (!d.groups.length) return { error: t('channel_edit.requiredGroup') };
    // The allocation policy orders channels while supplies route: leave the numbers alone.
    if (supplyRouting) return { group: d.groups.join(','), tag: d.tag.trim() };
    const peers = usagePeers({ ...c, groups: d.groups }, allChannels);
    const keys = usageOptions(c, peers).options.map((o) => o.key);
    const nums = usageNumbers(keys.includes(d.usage) ? d.usage : 'custom', c, peers, d.custom);
    return { group: d.groups.join(','), priority: nums.priority, weight: nums.weight, tag: d.tag.trim() };
  },
  Body: RoutingBody
};

// ---------- Request rewriting ----------

function RewriteBody({ object: channel, draft, setDraft }) {
  const { t } = useTranslation();
  const labels = typeLabels(channel.type);
  return (
    <>
      <Field label={configText(t, labels.label.model_headers)} optional hint={configText(t, labels.prompt.model_headers)}>
        <ModelHeadersInput value={draft.model_headers} onChange={(v) => setDraft({ ...draft, model_headers: v })} />
      </Field>
      <Field label={configText(t, labels.label.header_override)} optional hint={configText(t, labels.prompt.header_override)}>
        <ModelHeadersInput value={draft.header_override} onChange={(v) => setDraft({ ...draft, header_override: v })} />
      </Field>
      <Field label={configText(t, labels.label.custom_parameter)} htmlFor="cp-custom" optional hint={t('channels.rewrite.paramsHint')}>
        <Textarea
          id="cp-custom"
          rows={5}
          className="font-mono text-[13px]"
          placeholder={'{ "temperature": 0.7 }'}
          value={draft.custom_parameter}
          onChange={(e) => setDraft({ ...draft, custom_parameter: e.target.value })}
        />
      </Field>
    </>
  );
}

const rewriteTab = {
  key: 'rewrite',
  make: (c) => ({
    model_headers: headerRows(c.model_headers),
    header_override: headerRows(c.header_override),
    custom_parameter: prettyJson(c.custom_parameter)
  }),
  patch: (d, c, { t }) => {
    const custom = d.custom_parameter.trim();
    if (custom) {
      try {
        JSON.parse(custom);
      } catch (e) {
        return { error: t('channels.rewrite.invalidJson', { message: e.message }) };
      }
    }
    return { model_headers: headerJson(d.model_headers), header_override: headerJson(d.header_override), custom_parameter: custom };
  },
  Body: RewriteBody
};

// ---------- Monitoring ----------

function MonitorBody({ object: channel, draft, setDraft, ctx }) {
  const { t } = useTranslation();
  const drift = channel.model_drift;
  const missing = drift?.missing_models || [];
  const added = drift?.new_models || [];
  const firstModel = splitList(channel.models)[0] || '';
  // Providers without a model list endpoint cannot be compared with their upstream.
  const canCheckModels = Boolean(typeLabels(channel.type).canFetchModels);
  return (
    <>
      <Field label={t('channels.monitor.testModel')} htmlFor="cp-test-model" optional hint={t('channels.monitor.testModelHint')}>
        <Input
          id="cp-test-model"
          className="font-mono text-[13px]"
          value={draft.test_model}
          placeholder={firstModel}
          onChange={(e) => setDraft({ ...draft, test_model: e.target.value })}
        />
      </Field>
      <BalanceAlertField channel={channel} value={draft.balance_threshold} onChange={(v) => setDraft({ ...draft, balance_threshold: v })} />
      <Field
        label={t('channels.monitor.drift')}
        extra={
          canCheckModels && (
            <button
              type="button"
              className="inline-flex items-center gap-1 text-xs text-info hover:underline disabled:opacity-50"
              disabled={ctx.busy.drift}
              onClick={() => ctx.actions.onCheckDrift(channel)}
            >
              {ctx.busy.drift ? <Loader2 className="size-3 animate-spin" /> : <RefreshCw className="size-3" />}
              {t('channels.monitor.checkNow')}
            </button>
          )
        }
      >
        <div className="space-y-1 text-[13px]">
          {!canCheckModels ? (
            <p className="text-muted-foreground">{t('channels.monitor.driftUnsupported')}</p>
          ) : !drift?.checked_at ? (
            <p className="text-muted-foreground">{t('channelDetail.driftNever')}</p>
          ) : (
            <p>
              {missing.length === 0 && added.length === 0 ? t('channelDetail.driftOk') : t('channelDetail.driftChanged')}
              <span className="ml-2 text-xs text-muted-foreground">
                {t('channelDetail.driftCheckedAt', { time: timestamp2string(drift.checked_at) })}
              </span>
            </p>
          )}
          {missing.length > 0 && (
            <p className="font-mono text-xs">
              <span className="font-sans text-muted-foreground">{t('channelDetail.driftMissing', { count: missing.length })}: </span>
              {missing.join(', ')}
            </p>
          )}
          {added.length > 0 && (
            <p className="font-mono text-xs">
              <span className="font-sans text-muted-foreground">{t('channelDetail.driftNew', { count: added.length })}: </span>
              {added.join(', ')}
              <button
                type="button"
                className="ml-2 font-sans text-info hover:underline disabled:opacity-50"
                disabled={ctx.busy.dismissing}
                onClick={() => ctx.actions.onDismissNewModels(channel)}
              >
                {t('channelDetail.driftDismiss')}
              </button>
            </p>
          )}
        </div>
      </Field>
      <ToggleField
        label={t('channel_edit.drift.autoAdd')}
        hint={t('channels.monitor.autoAddHint')}
        checked={draft.drift_auto_add}
        onChange={(v) => setDraft({ ...draft, drift_auto_add: v })}
      />
      <Field label={t('channels.monitor.ignore')} htmlFor="cp-ignore" optional hint={t('channels.monitor.ignoreHint')}>
        <Textarea
          id="cp-ignore"
          rows={4}
          className="font-mono text-[13px]"
          placeholder={'*-preview\n*-realtime-*'}
          value={draft.drift_ignore_rules}
          onChange={(e) => setDraft({ ...draft, drift_ignore_rules: e.target.value })}
        />
      </Field>
      <PauseRulesField channelId={channel.id} value={draft.pause_rules} onChange={(v) => setDraft({ ...draft, pause_rules: v })} />
    </>
  );
}

const monitorTab = {
  key: 'monitor',
  make: (c) => ({
    test_model: c.test_model || '',
    balance_threshold: c.balance_threshold ? String(c.balance_threshold) : '',
    drift_auto_add: !!c.drift_auto_add,
    drift_ignore_rules: c.drift_ignore_rules || '',
    pause_rules: pauseRulesText(c.pause_rules)
  }),
  patch: (d, c, { t }) => {
    const rules = pauseRulesPatch(d.pause_rules, c, t);
    if (rules.error) return rules;
    return {
      test_model: d.test_model.trim(),
      balance_threshold: Math.max(0, Number(d.balance_threshold) || 0),
      drift_auto_add: d.drift_auto_add,
      drift_ignore_rules: d.drift_ignore_rules.trim(),
      ...rules
    };
  },
  Body: MonitorBody
};

// ---------- Advanced ----------

// Same values as the backend PreCostType: 1 normal, 2 skip images, 3 skip all.
const PRE_COST_OPTIONS = [
  { value: 1, labelKey: 'channel_edit.preCostNormal' },
  { value: 2, labelKey: 'channel_edit.preCostNoImage' },
  { value: 3, labelKey: 'channel_edit.preCostNone' }
];
// cancel_on_disconnect is three-state: null follows the channel type, true / false override it.
const CANCEL_ON_DISCONNECT_OPTIONS = ['auto', 'on', 'off'];
const cancelOnDisconnectDraft = (v) => (v === true ? 'on' : v === false ? 'off' : 'auto');
const cancelOnDisconnectValue = (d) => (d === 'auto' ? null : d === 'on');

const FLAGS = [
  'only_chat',
  'compatible_response',
  'allow_extra_body',
  'pass_through_body',
  'allow_service_tier',
  'unified_request_response_model'
];

function AdvancedBody({ object: channel, draft, setDraft, ctx }) {
  const { t } = useTranslation();
  const labels = typeLabels(channel.type);
  const hasPlugin = Boolean(pluginConfig[String(channel.type)]);
  const [iconKey, setIconKey] = useState(0);
  // The model mapping is rarely needed (an upstream that wants its own deployment names), so it
  // stays folded away until it has entries or is opened.
  const [mappingOpen, setMappingOpen] = useState(false);
  const set = (k) => (e) => setDraft({ ...draft, [k]: e?.target ? e.target.value : e });
  return (
    <>
      <Field label={t('channels.advanced.name')} htmlFor="cp-name">
        <Input id="cp-name" className="font-mono text-[13px]" value={draft.name} onChange={set('name')} />
      </Field>
      <Field label={configText(t, labels.label.remark)} htmlFor="cp-remark" optional hint={configText(t, labels.prompt.remark)}>
        <Textarea id="cp-remark" rows={2} value={draft.remark} onChange={set('remark')} />
      </Field>
      {/* This provider does not read the field: it is shown only to clear a value left from before. */}
      {!labels.hasOther && String(channel.other || '').trim() !== '' && (
        <Field label={configText(t, labels.label.other)} htmlFor="cp-other-adv" optional hint={t('channels.advanced.otherUnused')}>
          <Input id="cp-other-adv" className="font-mono text-[13px]" value={draft.other} onChange={set('other')} />
        </Field>
      )}
      <div className="grid grid-cols-2 gap-4">
        <Field label={t('channels.advanced.costRatio')} htmlFor="cp-cost" optional hint={t('channels.advanced.costRatioHint')}>
          <Input
            id="cp-cost"
            type="number"
            min="0"
            step="0.01"
            className="tabular-nums"
            value={draft.cost_ratio}
            onChange={set('cost_ratio')}
          />
        </Field>
        <Field label={configText(t, labels.label.pre_cost)} hint={configText(t, labels.prompt.pre_cost)}>
          <Select value={String(draft.pre_cost)} onValueChange={(v) => setDraft({ ...draft, pre_cost: Number(v) })}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PRE_COST_OPTIONS.map((o) => (
                <SelectItem key={o.value} value={String(o.value)}>
                  {t(o.labelKey)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
      </div>
      {draft.mapping || mappingOpen ? (
        <Field label={configText(t, labels.label.model_mapping)} optional hint={t('channels.models.mappingHint')}>
          <ModelMappingInput
            key={ctx.version}
            value={draft.mapping}
            models={splitList(channel.models)}
            onChange={(v) => setDraft({ ...draft, mapping: v })}
          />
        </Field>
      ) : (
        <button
          type="button"
          onClick={() => setMappingOpen(true)}
          className="inline-flex items-center gap-1 self-start text-[13px] text-muted-foreground hover:text-foreground"
        >
          <Plus className="size-3.5" />
          {t('channels.models.addMapping')}
        </button>
      )}
      <Field
        label={configText(t, labels.label.disabled_stream)}
        htmlFor="cp-nostream"
        optional
        hint={configText(t, labels.prompt.disabled_stream)}
      >
        <Textarea
          id="cp-nostream"
          rows={2}
          className="font-mono text-[13px]"
          placeholder={splitList(channel.models).slice(0, 2).join(', ')}
          value={draft.disabled_stream}
          onChange={set('disabled_stream')}
        />
      </Field>
      <Field
        label={configText(t, labels.label.need2response_models)}
        htmlFor="cp-n2r"
        optional
        hint={configText(t, labels.prompt.need2response_models)}
      >
        <Textarea
          id="cp-n2r"
          rows={3}
          className="font-mono text-[13px]"
          placeholder={t('channels.advanced.n2rPlaceholder')}
          value={draft.need2response_models}
          onChange={set('need2response_models')}
        />
      </Field>
      {FLAGS.map((f) => (
        <ToggleField
          key={f}
          label={configText(t, labels.label[f])}
          hint={labels.prompt[f] && labels.prompt[f] !== labels.label[f] ? configText(t, labels.prompt[f]) : null}
          checked={draft[f]}
          onChange={(v) => setDraft({ ...draft, [f]: v })}
        />
      ))}
      <Field label={t('channels.advanced.cancelOnDisconnect')} hint={t('channels.advanced.cancelOnDisconnectHint')}>
        <Select value={draft.cancel_on_disconnect} onValueChange={(v) => setDraft({ ...draft, cancel_on_disconnect: v })}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CANCEL_ON_DISCONNECT_OPTIONS.map((o) => (
              <SelectItem key={o} value={o}>
                {o === 'auto'
                  ? t('channels.advanced.cancelOnDisconnectAuto', {
                      state: t(
                        channel.cancel_on_disconnect_auto
                          ? 'channels.advanced.cancelOnDisconnectOn'
                          : 'channels.advanced.cancelOnDisconnectOff'
                      )
                    })
                  : t(o === 'on' ? 'channels.advanced.cancelOnDisconnectOn' : 'channels.advanced.cancelOnDisconnectOff')}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      {hasPlugin && (
        <Field label={t('channel_edit.pluginConfig')}>
          <PluginForm type={channel.type} value={draft.plugin} onChange={(v) => setDraft({ ...draft, plugin: v })} />
        </Field>
      )}
      <Field label={t('channel_edit.icon')} hint={t('channel_edit.iconTip')}>
        <div className="flex items-center gap-3">
          <span className="flex size-10 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
            <ProviderIcon key={iconKey} type={channel.type} name={channel.name} baseUrl={channel.base_url} className="size-6" />
          </span>
          <BrandIconRefreshButton domain={iconCacheHost(channel.base_url) || undefined} onRefreshed={() => setIconKey((k) => k + 1)} />
        </div>
      </Field>
      <Field label={t('channels.advanced.danger')}>
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-destructive/30 px-3 py-2.5">
          <p className="text-xs text-muted-foreground">{t('channels.advanced.dangerHint')}</p>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" className="rounded-lg" onClick={() => ctx.actions.onCopy(channel)}>
              <Copy />
              {t('channels.action.copy')}
            </Button>
            <Button
              variant="outline"
              size="sm"
              className={cn('rounded-lg text-destructive hover:text-destructive')}
              onClick={() => ctx.actions.onDelete(channel)}
            >
              <Trash2 />
              {t('channels.action.delete')}
            </Button>
          </div>
        </div>
      </Field>
    </>
  );
}

const advancedTab = {
  key: 'advanced',
  make: (c) => ({
    name: c.name || '',
    remark: c.remark || '',
    other: c.other || '',
    cost_ratio: c.cost_ratio ? String(c.cost_ratio) : '',
    pre_cost: c.pre_cost || 1,
    disabled_stream: Array.isArray(c.disabled_stream) ? c.disabled_stream.join(',') : '',
    need2response_models: c.need2response_models || '',
    plugin: c.plugin && typeof c.plugin === 'object' ? c.plugin : {},
    mapping: (() => {
      const mapping = parseMapping(c.model_mapping);
      return mapping && Object.keys(mapping).length ? JSON.stringify(mapping) : '';
    })(),
    ...Object.fromEntries(FLAGS.map((f) => [f, !!c[f]])),
    cancel_on_disconnect: cancelOnDisconnectDraft(c.cancel_on_disconnect)
  }),
  patch: (d, c, { t }) => {
    if (!d.name.trim()) return { error: t('channel_edit.requiredName') };
    const mapping = parseMapping(d.mapping);
    if (mapping === null) return { error: t('channels.models.mappingInvalid') };
    const patch = {
      name: d.name.trim(),
      remark: d.remark.trim(),
      cost_ratio: Math.max(0, Number(d.cost_ratio) || 0),
      pre_cost: d.pre_cost,
      disabled_stream: splitList(d.disabled_stream),
      need2response_models: d.need2response_models.trim(),
      ...Object.fromEntries(FLAGS.map((f) => [f, d[f]])),
      cancel_on_disconnect: cancelOnDisconnectValue(d.cancel_on_disconnect)
    };
    // A changed mapping rewrites the model list the same way the models tab used to: mapped
    // upstream names are listed by their request names. The list is sent only then, so saving
    // this tab otherwise leaves the models and their supplies alone.
    const mappingJson = Object.keys(mapping).length ? JSON.stringify(mapping) : '';
    const before = parseMapping(c.model_mapping) || {};
    if (mappingJson !== (Object.keys(before).length ? JSON.stringify(before) : '')) {
      patch.model_mapping = mappingJson;
      patch.models = applyMappingToModels(splitList(c.models), mapping).join(',');
    }
    if (!typeLabels(c.type).hasOther) patch.other = d.other.trim();
    const plugins = pluginConfig[String(c.type)];
    if (plugins) {
      const normalized = {};
      for (const [pluginId, cfg] of Object.entries(plugins)) {
        normalized[pluginId] = {};
        for (const [paramId, param] of Object.entries(cfg.params)) {
          const v = d.plugin?.[pluginId]?.[paramId];
          normalized[pluginId][paramId] = v ?? (param.type === 'bool' ? false : (param.default ?? ''));
        }
      }
      patch.plugin = normalized;
    }
    return patch;
  },
  Body: AdvancedBody
};

export const PANEL_TABS = [connectionTab, modelsTab, performanceTab, routingTab, rewriteTab, monitorTab, advancedTab];
// Tab a channel opens on when the URL names none; it is left out of the URL.
export const DEFAULT_PANEL_TAB = PANEL_TABS[0].key;

// Old deep links named the sections of the former settings form.
export const LEGACY_SECTION_TAB = { basic: 'connection', model: 'models', request: 'rewrite', advanced: 'advanced', plugin: 'advanced' };

const bodyProps = {
  object: PropTypes.object.isRequired,
  draft: PropTypes.object.isRequired,
  setDraft: PropTypes.func.isRequired,
  ctx: PropTypes.object
};
ModelsBody.propTypes = bodyProps;
ConnectionBody.propTypes = bodyProps;
RoutingBody.propTypes = bodyProps;
RewriteBody.propTypes = bodyProps;
MonitorBody.propTypes = bodyProps;
AdvancedBody.propTypes = bodyProps;
