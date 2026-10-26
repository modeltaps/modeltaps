import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ChevronDown, ChevronRight, Flame, ListChecks, Loader2, RefreshCw, RotateCcw, Search, SlidersHorizontal } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { toast } from '@/components/ui/sonner';
import { fetchUpstreamModelPicker } from './channelApi';
import {
  CAPABILITY_FILTERS,
  CONTEXT_STEPS,
  EMPTY_FILTERS,
  expiresSoon,
  filterOptions,
  formatContext,
  formatPrice,
  groupFamilies,
  toggleMany,
  visibleModels
} from './modelPicker';

// Upstream model picker for the channel form. Rows are grouped by model family
// in usage order (hottest first, see model/model_picker.go); by default each
// family shows only its newest version, older versions fold behind "N older".
// The filter panel narrows by status, weights, vendor, capability, output
// modality, price and context; "select results" adds every visible row.
const splitModels = (value) =>
  (value || '')
    .split(/[,\n]/)
    .map((m) => m.trim())
    .filter(Boolean);

const dateOf = (unix) => (unix ? new Date(unix * 1000).toISOString().slice(0, 10) : '');

export default function ModelSelectorModal({ open, onClose, onConfirm, channelValues }) {
  const { t } = useTranslation();
  const p = (key, opts) => t(`channel_edit.picker.${key}`, opts);
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState([]);
  const [families, setFamilies] = useState([]);
  const [selected, setSelected] = useState(new Set());
  const [filters, setFilters] = useState(EMPTY_FILTERS);
  const [latestOnly, setLatestOnly] = useState(true);
  const [expanded, setExpanded] = useState(new Set());
  const [overwrite, setOverwrite] = useState(false);
  const [showSelected, setShowSelected] = useState(false);
  // Narrow screens fold the filter panel behind a button; from md up it is a sidebar.
  const [showFilters, setShowFilters] = useState(false);

  const load = async (values) => {
    setLoading(true);
    try {
      const data = await fetchUpstreamModelPicker(values || {});
      setItems(data.models);
      setFamilies(data.families);
      // The output filter defaults to text; clear it for channels without text
      // models (image / audio only) so the list does not start empty.
      if (!data.models.some((m) => (m.output_modalities || []).includes('text'))) {
        setFilters((f) => ({ ...f, output: '' }));
      }
    } catch (error) {
      toast.error(error.message || t('channel_edit.modelListError'));
    }
    setLoading(false);
  };

  // Re-seed from the form each time the dialog opens and fetch right away:
  // opening the picker is the request for the upstream list.
  useEffect(() => {
    if (!open) return;
    setSelected(new Set(splitModels(channelValues?.models)));
    setItems([]);
    setFamilies([]);
    setFilters(EMPTY_FILTERS);
    setLatestOnly(true);
    setExpanded(new Set());
    setOverwrite(false);
    setShowSelected(false);
    load(channelValues);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, channelValues]);

  const options = useMemo(() => filterOptions(items), [items]);
  const groups = useMemo(
    () => groupFamilies(items, families, filters, selected, { latestOnly, expanded }),
    [items, families, filters, selected, latestOnly, expanded]
  );
  const visible = useMemo(() => visibleModels(groups), [groups]);
  const upstreamIds = useMemo(() => new Set(items.map((i) => i.model)), [items]);
  const selectedUpstream = [...selected].filter((m) => upstreamIds.has(m)).length;

  const setFilter = (key, value) => setFilters((f) => ({ ...f, [key]: value }));
  const toggleListFilter = (key, value) =>
    setFilters((f) => ({ ...f, [key]: f[key].includes(value) ? f[key].filter((v) => v !== value) : [...f[key], value] }));
  const toggleModel = (model) => setSelected((prev) => toggleMany(prev, [model]));
  const toggleExpanded = (family) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(family)) next.delete(family);
      else next.add(family);
      return next;
    });
  const filtersActive = JSON.stringify(filters) !== JSON.stringify(EMPTY_FILTERS);

  const handleConfirm = () => {
    onConfirm([...selected], overwrite);
    onClose();
  };

  const chip = (active, onClick, label) => (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={`rounded-md border px-2 py-0.5 text-xs transition-colors ${
        active ? 'border-primary bg-primary text-primary-foreground' : 'border-border text-muted-foreground hover:text-foreground'
      }`}
    >
      {label}
    </button>
  );

  const filterPanel = (
    <aside
      className={`${showFilters ? 'block' : 'hidden'} w-full shrink-0 space-y-4 border-border text-sm md:block md:w-56 md:overflow-y-auto md:border-r md:pr-3`}
      aria-label={p('filters')}
    >
      <div className="space-y-1.5">
        <Label className="text-xs text-muted-foreground">{p('status')}</Label>
        <div className="flex flex-wrap gap-1">
          {['all', 'selected', 'unselected', 'elsewhere'].map((s) =>
            chip(filters.status === s, () => setFilter('status', s), p(`status_${s}`))
          )}
        </div>
      </div>
      <div className="space-y-1.5">
        <Label className="text-xs text-muted-foreground">{p('weights')}</Label>
        <div className="flex flex-wrap gap-1">
          {['all', 'open', 'closed'].map((w) => chip(filters.weights === w, () => setFilter('weights', w), p(`weights_${w}`)))}
        </div>
      </div>
      <div className="space-y-1.5">
        <Label className="text-xs text-muted-foreground">{p('capabilities')}</Label>
        <div className="flex flex-wrap gap-1">
          {CAPABILITY_FILTERS.map((c) => chip(filters.capabilities.includes(c), () => toggleListFilter('capabilities', c), p(`cap_${c}`)))}
        </div>
      </div>
      {options.outputs.length > 0 && (
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">{p('output')}</Label>
          <div className="flex flex-wrap gap-1">
            {chip(filters.output === '', () => setFilter('output', ''), p('output_any'))}
            {options.outputs.map((m) => chip(filters.output === m, () => setFilter('output', m), p(`output_${m}`, { defaultValue: m })))}
          </div>
        </div>
      )}
      <div className="space-y-1.5">
        <Label className="text-xs text-muted-foreground">{p('maxPrice')}</Label>
        <div className="flex gap-1.5">
          <Input
            type="number"
            min="0"
            step="any"
            className="h-8"
            aria-label={p('maxInput')}
            placeholder={p('maxInput')}
            value={filters.maxInput}
            onChange={(e) => setFilter('maxInput', e.target.value)}
          />
          <Input
            type="number"
            min="0"
            step="any"
            className="h-8"
            aria-label={p('maxOutput')}
            placeholder={p('maxOutput')}
            value={filters.maxOutput}
            onChange={(e) => setFilter('maxOutput', e.target.value)}
          />
        </div>
        <p className="text-[11px] text-muted-foreground">{p('priceUnit')}</p>
      </div>
      <div className="space-y-1.5">
        <Label className="text-xs text-muted-foreground">{p('minContext')}</Label>
        <Select value={String(filters.minContext)} onValueChange={(v) => setFilter('minContext', Number(v))}>
          <SelectTrigger className="h-8" aria-label={p('minContext')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CONTEXT_STEPS.map((n) => (
              <SelectItem key={n} value={String(n)}>
                {n === 0 ? p('anyContext') : `≥ ${formatContext(n)}`}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {options.vendors.length > 1 && (
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">{p('vendors')}</Label>
          <div className="max-h-48 space-y-0.5 overflow-y-auto">
            {options.vendors.map((v) => (
              <label key={v.id} className="flex cursor-pointer items-center gap-2 rounded px-1 py-0.5 hover:bg-muted/50">
                <Checkbox checked={filters.vendors.includes(v.id)} onCheckedChange={() => toggleListFilter('vendors', v.id)} />
                <span className="flex-1 truncate text-xs">{v.id}</span>
                <span className="text-[11px] text-muted-foreground">{v.count}</span>
              </label>
            ))}
          </div>
        </div>
      )}
      {filtersActive && (
        <Button variant="ghost" size="sm" className="w-full" onClick={() => setFilters(EMPTY_FILTERS)}>
          <RotateCcw className="size-4" /> {p('resetFilters')}
        </Button>
      )}
    </aside>
  );

  const renderRow = (row) => (
    <label key={row.model} className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 hover:bg-muted/50" data-model={row.model}>
      <Checkbox checked={selected.has(row.model)} onCheckedChange={() => toggleModel(row.model)} aria-label={row.model} />
      <span className="min-w-0 flex-1 truncate font-mono text-xs" title={row.model}>
        {row.model}
      </span>
      <span className="flex shrink-0 items-center gap-1">
        {row.latest && <Badge className="px-1.5 py-0 text-[10px]">{p('latest')}</Badge>}
        {row.preview && (
          <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
            {p('preview')}
          </Badge>
        )}
        {row.variant && (
          <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
            {row.variant}
          </Badge>
        )}
        {row.alias && (
          <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
            {p('alias')}
          </Badge>
        )}
        {row.open_weights && (
          <Badge variant="secondary" className="px-1.5 py-0 text-[10px]">
            {p('openWeights')}
          </Badge>
        )}
        {expiresSoon(row) && (
          <Badge variant="destructive" className="px-1.5 py-0 text-[10px]" title={dateOf(row.expires_at)}>
            {p('retiring', { date: dateOf(row.expires_at) })}
          </Badge>
        )}
        {row.other_channels > 0 && (
          <span className="text-[10px] text-muted-foreground" title={p('elsewhereTip', { count: row.other_channels })}>
            {p('elsewhere', { count: row.other_channels })}
          </span>
        )}
      </span>
      <span className="hidden w-12 shrink-0 text-right text-[11px] tabular-nums text-muted-foreground sm:inline">
        {formatContext(row.context_length)}
      </span>
      <span className="hidden w-24 shrink-0 text-right text-[11px] tabular-nums text-muted-foreground sm:inline">
        {row.input_price !== null && row.input_price !== undefined
          ? `${formatPrice(row.input_price)} / ${formatPrice(row.output_price)}`
          : ''}
      </span>
    </label>
  );

  const renderGroup = (group) => {
    const ids = group.rows.map((r) => r.model);
    const count = ids.filter((m) => selected.has(m)).length;
    const isExpanded = expanded.has(group.family);
    return (
      <div key={group.family} className="border-b border-border/60 py-1 last:border-b-0" data-family={group.family}>
        <div className="flex items-center gap-2 px-2 py-1">
          <Checkbox
            checked={count === ids.length}
            indeterminate={count > 0 && count < ids.length}
            onCheckedChange={() => setSelected((prev) => toggleMany(prev, ids))}
            aria-label={p('selectFamily', { family: group.family })}
          />
          <span className="text-sm font-semibold">{group.family || p('noFamily')}</span>
          {group.requests > 0 && (
            <span className="flex items-center gap-0.5 text-[11px] text-orange-600 dark:text-orange-400" title={p('requestsTip')}>
              <Flame className="size-3" />
              {group.requests}
            </span>
          )}
          <span className="ml-auto text-[11px] text-muted-foreground">{p('familyCount', { count: group.total })}</span>
        </div>
        <div className="pl-4">{group.rows.map(renderRow)}</div>
        {(group.hidden > 0 || (isExpanded && group.matched > 1)) && (
          <button
            type="button"
            className="ml-6 flex items-center gap-1 px-2 py-0.5 text-[11px] text-muted-foreground hover:text-foreground"
            onClick={() => toggleExpanded(group.family)}
          >
            {isExpanded ? <ChevronDown className="size-3" /> : <ChevronRight className="size-3" />}
            {isExpanded ? p('collapseOlder') : p('showOlder', { count: group.hidden })}
          </button>
        )}
      </div>
    );
  };

  const selectedList = [...selected];

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-5xl">
        <DialogHeader>
          <DialogTitle>{t('channel_edit.modelSelector')}</DialogTitle>
        </DialogHeader>
        <DialogBody className="flex flex-col gap-3 md:min-h-0 md:overflow-hidden">
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            <div className="relative min-w-48 flex-1">
              <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="pl-8"
                placeholder={p('searchPlaceholder')}
                aria-label={t('channel_edit.searchModels')}
                value={filters.search}
                onChange={(e) => setFilter('search', e.target.value)}
              />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={latestOnly} onCheckedChange={setLatestOnly} aria-label={p('latestOnly')} />
              {p('latestOnly')}
            </label>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setSelected((prev) => toggleMany(prev, visible))}
              disabled={loading || visible.length === 0}
            >
              <ListChecks className="size-4" /> {p('selectResults', { count: visible.length })}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setSelected(new Set())} disabled={selected.size === 0}>
              <RotateCcw className="size-4" /> {t('channel_edit.clearModels')}
            </Button>
            <Button variant="outline" size="sm" className="md:hidden" onClick={() => setShowFilters((v) => !v)} aria-expanded={showFilters}>
              <SlidersHorizontal className="size-4" /> {p('filters')}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => load(channelValues)} disabled={loading} aria-label={p('refresh')}>
              {loading ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            </Button>
          </div>

          <div className="flex shrink-0 flex-col gap-3 md:h-[52vh] md:min-h-0 md:flex-1 md:flex-row">
            {filterPanel}
            <div
              className="h-[55vh] shrink-0 overflow-y-auto rounded-md border border-border md:h-auto md:flex-1"
              role="list"
              aria-label={p('results')}
            >
              {loading && items.length === 0 ? (
                <p className="flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground">
                  <Loader2 className="size-4 animate-spin" /> {p('loading')}
                </p>
              ) : items.length === 0 ? (
                <p className="py-12 text-center text-sm text-muted-foreground">{t('channel_edit.noModels')}</p>
              ) : groups.length === 0 ? (
                <p className="py-12 text-center text-sm text-muted-foreground">{t('channel_edit.noMatchingModels')}</p>
              ) : (
                groups.map(renderGroup)
              )}
            </div>
          </div>

          {selected.size > 0 && (
            <div className="shrink-0 rounded-md border border-border px-3 py-2">
              <button
                type="button"
                className="flex w-full items-center gap-1 text-left text-xs text-muted-foreground hover:text-foreground"
                onClick={() => setShowSelected((v) => !v)}
                aria-expanded={showSelected}
              >
                {showSelected ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                {p('selectedSummary', { count: selected.size })}
              </button>
              {showSelected && (
                <div className="mt-2 flex max-h-24 flex-wrap gap-1 overflow-y-auto">
                  {selectedList.map((m) => (
                    <Badge key={m} variant="secondary" className="gap-1 font-mono text-[11px]">
                      {m}
                      <button type="button" aria-label={p('remove', { model: m })} onClick={() => toggleModel(m)}>
                        ×
                      </button>
                    </Badge>
                  ))}
                </div>
              )}
            </div>
          )}

          <div className="flex shrink-0 items-center justify-between rounded-md border border-border p-3">
            <div className="space-y-0.5 pr-3">
              <p className="text-sm">{t('channel_edit.overwriteModels')}</p>
              <p className="text-xs text-muted-foreground">{t('channel_edit.overwriteModelsTip')}</p>
            </div>
            <Switch checked={overwrite} onCheckedChange={setOverwrite} aria-label={t('channel_edit.overwriteModels')} />
          </div>
        </DialogBody>
        <DialogFooter className="items-center">
          <span className="mr-auto text-xs text-muted-foreground" data-testid="picker-count">
            {p('selectedOfTotal', { selected: selectedUpstream, total: items.length })}
          </span>
          <Button variant="outline" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={selected.size === 0}>
            {t('common.submit')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

ModelSelectorModal.propTypes = {
  open: PropTypes.bool,
  onClose: PropTypes.func,
  onConfirm: PropTypes.func,
  channelValues: PropTypes.object
};
