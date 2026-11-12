import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2, Search } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';

import { ENDPOINT_LABEL_KEYS, ENDPOINT_VALUES, EndpointIcons, safeJsonArray } from './modelInfoHelpers';

const PAGE = 150;
const DAY = 86400;

// Pure filter over library items; exported for tests.
export function filterLibrary(items, f, now = Math.floor(Date.now() / 1000)) {
  const kw = f.keyword.trim().toLowerCase();
  return items.filter((it) => {
    if (f.hideAdded && it.in_catalog) return false;
    if (f.hideDeprecated && it.deprecated) return false;
    if (f.suppliedOnly && !(it.channels || []).length) return false;
    if (f.openOnly && it.open_weights !== true) return false;
    if (f.lab && it.lab !== f.lab) return false;
    if (f.endpoint && !safeJsonArray(it.endpoints).includes(f.endpoint)) return false;
    if (f.days && !(it.released_at && it.released_at >= now - Number(f.days) * DAY)) return false;
    if (kw && !`${it.model} ${it.name} ${it.lab}`.toLowerCase().includes(kw)) return false;
    return true;
  });
}

export const DEFAULT_LIBRARY_FILTER = {
  keyword: '',
  lab: '',
  endpoint: '',
  days: '',
  suppliedOnly: false,
  openOnly: false,
  hideAdded: true,
  hideDeprecated: true
};

const formatContext = (n) => {
  if (!n) return '-';
  if (n >= 1000000) return `${+(n / 1000000).toFixed(1)}M`;
  if (n >= 1000) return `${Math.round(n / 1000)}K`;
  return String(n);
};

const formatPrice = (v) => (v == null ? '-' : `$${+Number(v).toFixed(3)}`);

const formatDate = (ts) => (ts ? new Date(ts * 1000).toISOString().slice(0, 10) : '-');

// Add models from the model library: everything models.dev knows, one row per model. Filter,
// pick (or pick every match), choose draft or published, and add; the backend copies the
// metadata, sets the lab's list price when the model has none and supplies it from the channels
// that already list it.
export default function LibrarySheet({ open, onOpenChange, vendors, onAdded }) {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  // Only the first open shows a spinner; reopening keeps the last list while it refreshes.
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState(DEFAULT_LIBRARY_FILTER);
  const [selected, setSelected] = useState(() => new Set());
  const [limit, setLimit] = useState(PAGE);
  const [status, setStatus] = useState('draft');
  const [supplies, setSupplies] = useState(true);
  const [adding, setAdding] = useState(false);

  useEffect(() => {
    if (!open) return undefined;
    let cancelled = false;
    API.get('/api/model_info/library')
      .then((res) => {
        const { success, data } = res.data || {};
        if (!cancelled && success) setItems(Array.isArray(data) ? data : []);
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open]);

  const labs = useMemo(() => {
    const counts = new Map();
    items.forEach((it) => counts.set(it.lab, (counts.get(it.lab) || 0) + 1));
    return [...counts.entries()].sort((a, b) => b[1] - a[1]).map(([lab]) => lab);
  }, [items]);

  const labLabel = (lab) => {
    const item = items.find((it) => it.lab === lab && it.vendor_id);
    return (item && vendors[item.vendor_id]?.name) || lab;
  };

  const filtered = useMemo(() => filterLibrary(items, filter), [items, filter]);
  const shown = filtered.slice(0, limit);
  const selectable = filtered.filter((it) => !it.in_catalog);
  const allSelected = selectable.length > 0 && selectable.every((it) => selected.has(it.canonical));
  const someSelected = selectable.some((it) => selected.has(it.canonical));

  const set = (patch) => {
    setFilter((f) => ({ ...f, ...patch }));
    setLimit(PAGE);
  };

  const toggle = (canonical, on) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (on) next.add(canonical);
      else next.delete(canonical);
      return next;
    });

  const toggleAll = (on) =>
    setSelected((prev) => {
      const next = new Set(prev);
      selectable.forEach((it) => (on ? next.add(it.canonical) : next.delete(it.canonical)));
      return next;
    });

  const close = () => {
    setSelected(new Set());
    setFilter(DEFAULT_LIBRARY_FILTER);
    onOpenChange(false);
  };

  const add = async () => {
    if (selected.size === 0) return;
    setAdding(true);
    try {
      const res = await API.post('/api/model_info/library/add', { canonicals: [...selected], status, supplies });
      const { success, message, data } = res.data || {};
      if (!success) return toast.error(message);
      toast.success(t('modelLibrary.added', { count: data.added.length, supplies: data.supplies }));
      onAdded?.();
      close();
    } catch {
      /* shown by the API client */
    } finally {
      setAdding(false);
    }
  };

  return (
    <Sheet open={open} onOpenChange={(v) => (v ? onOpenChange(true) : close())}>
      <SheetContent onClose={close} className="max-w-5xl">
        <SheetHeader>
          <SheetTitle>{t('modelLibrary.title')}</SheetTitle>
          <SheetDescription>{t('modelLibrary.description', { count: items.length })}</SheetDescription>
        </SheetHeader>
        <div className="shrink-0 space-y-2 border-b border-border px-6 py-3">
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative min-w-[12rem] flex-1">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={filter.keyword}
                onChange={(e) => set({ keyword: e.target.value })}
                placeholder={t('modelLibrary.search')}
                className="h-8 pl-8"
                aria-label={t('modelLibrary.search')}
              />
            </div>
            <Select value={filter.lab || 'all'} onValueChange={(v) => set({ lab: v === 'all' ? '' : v })}>
              <SelectTrigger className="h-8 w-40" aria-label={t('modelLibrary.vendor')}>
                <SelectValue placeholder={t('modelLibrary.allVendors')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t('modelLibrary.allVendors')}</SelectItem>
                {labs.map((lab) => (
                  <SelectItem key={lab} value={lab}>
                    {labLabel(lab)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={filter.endpoint || 'all'} onValueChange={(v) => set({ endpoint: v === 'all' ? '' : v })}>
              <SelectTrigger className="h-8 w-36" aria-label={t('modelLibrary.endpoint')}>
                <SelectValue placeholder={t('modelLibrary.allEndpoints')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t('modelLibrary.allEndpoints')}</SelectItem>
                {ENDPOINT_VALUES.map((e) => (
                  <SelectItem key={e} value={e}>
                    {t(ENDPOINT_LABEL_KEYS[e])}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select value={filter.days || 'all'} onValueChange={(v) => set({ days: v === 'all' ? '' : v })}>
              <SelectTrigger className="h-8 w-36" aria-label={t('modelLibrary.released')}>
                <SelectValue placeholder={t('modelLibrary.anyTime')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t('modelLibrary.anyTime')}</SelectItem>
                {['30', '90', '365'].map((d) => (
                  <SelectItem key={d} value={d}>
                    {t('modelLibrary.withinDays', { count: Number(d) })}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[13px]">
            {[
              ['suppliedOnly', 'modelLibrary.suppliedOnly'],
              ['openOnly', 'modelLibrary.openOnly'],
              ['hideAdded', 'modelLibrary.hideAdded'],
              ['hideDeprecated', 'modelLibrary.hideDeprecated']
            ].map(([key, label]) => (
              <label key={key} className="flex cursor-pointer items-center gap-1.5">
                <Switch size="sm" checked={filter[key]} onCheckedChange={(v) => set({ [key]: v })} />
                {t(label)}
              </label>
            ))}
            <span className="ml-auto tabular-nums text-muted-foreground">{t('modelLibrary.matches', { count: filtered.length })}</span>
          </div>
        </div>
        <SheetBody className="px-0 py-0">
          {loading ? (
            <div className="flex items-center justify-center py-16 text-muted-foreground">
              <Loader2 className="size-5 animate-spin" />
            </div>
          ) : filtered.length === 0 ? (
            <p className="py-16 text-center text-sm text-muted-foreground">{t('modelLibrary.empty')}</p>
          ) : (
            <table className="w-full text-[13px]">
              <thead className="sticky top-0 z-10 bg-card text-left text-xs text-muted-foreground">
                <tr className="border-b border-border">
                  <th className="w-14 py-2 pl-6">
                    <Checkbox
                      checked={allSelected}
                      indeterminate={!allSelected && someSelected}
                      onCheckedChange={toggleAll}
                      aria-label={t('modelLibrary.selectAll')}
                    />
                  </th>
                  <th className="py-2 font-medium">{t('modelLibrary.model')}</th>
                  <th className="py-2 font-medium">{t('modelLibrary.vendor')}</th>
                  <th className="py-2 text-right font-medium">{t('modelLibrary.context')}</th>
                  <th className="py-2 pl-4 font-medium">{t('modelLibrary.endpoint')}</th>
                  <th className="py-2 font-medium">{t('modelLibrary.released')}</th>
                  <th className="py-2 text-right font-medium">{t('modelLibrary.price')}</th>
                  <th className="py-2 pr-6 text-right font-medium">{t('modelLibrary.channels')}</th>
                </tr>
              </thead>
              <tbody>
                {shown.map((it) => {
                  const checked = selected.has(it.canonical);
                  const channels = it.channels || [];
                  return (
                    <tr
                      key={it.canonical}
                      className={`border-b border-border/60 ${it.in_catalog ? 'opacity-50' : 'cursor-pointer hover:bg-muted/40'}`}
                      onClick={() => !it.in_catalog && toggle(it.canonical, !checked)}
                    >
                      <td className="w-14 py-2 pl-6" onClick={(e) => e.stopPropagation()}>
                        <Checkbox
                          checked={it.in_catalog || checked}
                          disabled={it.in_catalog}
                          onCheckedChange={(v) => toggle(it.canonical, v)}
                          aria-label={it.model}
                        />
                      </td>
                      <td className="max-w-[16rem] py-2">
                        <div className="truncate font-medium">{it.name || it.model}</div>
                        <div className="truncate font-mono text-xs text-muted-foreground">
                          {it.model}
                          {it.in_catalog && ` · ${t('modelLibrary.inCatalog')}`}
                          {it.deprecated && ` · ${t('modelLibrary.deprecated')}`}
                        </div>
                      </td>
                      <td className="whitespace-nowrap py-2">{vendors[it.vendor_id]?.name || it.lab}</td>
                      <td className="py-2 text-right tabular-nums">{formatContext(it.context_length)}</td>
                      <td className="py-2 pl-4">
                        <EndpointIcons json={it.endpoints} />
                      </td>
                      <td className="whitespace-nowrap py-2 tabular-nums text-muted-foreground">{formatDate(it.released_at)}</td>
                      <td className="whitespace-nowrap py-2 text-right tabular-nums">
                        {formatPrice(it.input_price)} / {formatPrice(it.output_price)}
                      </td>
                      <td
                        className="whitespace-nowrap py-2 pr-6 text-right tabular-nums"
                        title={channels.map((c) => `${c.name} → ${c.upstream_model}`).join('\n')}
                      >
                        {channels.length || <span className="text-muted-foreground">-</span>}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
          {filtered.length > shown.length && (
            <div className="py-3 text-center">
              <Button variant="ghost" size="sm" onClick={() => setLimit((n) => n + PAGE)}>
                {t('modelLibrary.showMore', { count: filtered.length - shown.length })}
              </Button>
            </div>
          )}
        </SheetBody>
        <SheetFooter className="justify-between">
          <span className="text-[13px] tabular-nums text-muted-foreground">{t('modelLibrary.selected', { count: selected.size })}</span>
          <div className="flex flex-wrap items-center gap-3">
            <label className="flex cursor-pointer items-center gap-1.5 text-[13px]">
              <Switch size="sm" checked={supplies} onCheckedChange={setSupplies} />
              {t('modelLibrary.createSupplies')}
            </label>
            <Select value={status} onValueChange={setStatus}>
              <SelectTrigger className="h-8 w-28" aria-label={t('modelLibrary.status')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="draft">{t('modelLibrary.asDraft')}</SelectItem>
                <SelectItem value="published">{t('modelLibrary.asPublished')}</SelectItem>
              </SelectContent>
            </Select>
            <Button size="sm" disabled={adding || selected.size === 0} onClick={add}>
              {adding && <Loader2 className="size-4 animate-spin" />}
              {t('modelLibrary.add', { count: selected.size })}
            </Button>
          </div>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

LibrarySheet.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func.isRequired,
  vendors: PropTypes.object,
  onAdded: PropTypes.func
};
