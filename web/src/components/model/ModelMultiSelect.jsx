import { Fragment, useEffect, useId, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, ChevronsUpDown, Loader2, Minus, Plus, Search, SlidersHorizontal } from 'lucide-react';

import { cn } from '@/lib/utils';
import BrandIcon from '@/components/brand/BrandIcon';
import { CUSTOM_VENDOR, groupOptions, groupSelected, mergeOptions, releaseMonth, toggleGroup, toggleOne } from './modelSelectIndex';

// ==============================|| MODEL — MULTI SELECT ||============================== //
// The one model multi-select for admin forms (channel models, token and member allowlists).
// Trigger shows the count and the vendor marks of the selection; the dropdown has search,
// vendor groups with their logo (newest model first, groups ordered by their newest model),
// checkbox rows, select-all per group and "add <typed id>" for models not in the list. It
// stays open while picking. Below the trigger the selection is summarised per vendor.
// Data comes in as `options` (see modelSelectIndex); this component fetches nothing itself,
// callers load lazily through `onOpen`.

const MAX_TRIGGER_MARKS = 4;

function VendorMark({ group, className }) {
  const sample = group.items[0]?.id;
  return (
    <BrandIcon
      icon={group.icon}
      brandKey={group.key === CUSTOM_VENDOR ? undefined : group.key}
      model={sample}
      fallbackText={group.name || sample}
      className={cn('size-4', className)}
    />
  );
}

VendorMark.propTypes = { group: PropTypes.object.isRequired, className: PropTypes.string };

// Visual-only checkbox: rows and group headers are the interactive elements.
function Tick({ checked, mixed }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex size-4 shrink-0 items-center justify-center rounded-sm border border-input transition-colors',
        (checked || mixed) && 'border-primary bg-primary text-primary-foreground'
      )}
    >
      {mixed ? <Minus className="size-3" /> : checked ? <Check className="size-3" /> : null}
    </span>
  );
}

Tick.propTypes = { checked: PropTypes.bool, mixed: PropTypes.bool };

export default function ModelMultiSelect({
  id,
  value = [],
  onChange,
  options = [],
  onOpen,
  loading = false,
  notice,
  allowCreate = true,
  canCreateValue,
  advanced,
  disabled = false,
  placeholder,
  showSummary = true
}) {
  const { t } = useTranslation();
  const m = (key, opts) => t(`modelSelect.${key}`, opts);
  const rootRef = useRef(null);
  const searchRef = useRef(null);
  const listRef = useRef(null);
  const listId = useId();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [onlySelected, setOnlySelected] = useState(false);
  const [active, setActive] = useState(0);

  // Selected ids missing from `options` (typed by hand, gone upstream) stay pickable.
  const all = useMemo(() => mergeOptions(options, value), [options, value]);
  const selected = useMemo(() => new Set(value), [value]);
  const groups = useMemo(() => groupOptions(all, { query, selected, onlySelected }), [all, query, selected, onlySelected]);
  const summary = useMemo(() => groupSelected(all, value), [all, value]);

  const typed = query.trim();
  const canCreate =
    allowCreate &&
    typed !== '' &&
    !all.some((o) => o.id.toLowerCase() === typed.toLowerCase()) &&
    (!canCreateValue || canCreateValue(typed));

  // Flat list of keyboard stops: the create row, then each group header and its rows.
  const stops = useMemo(() => {
    const list = [];
    if (canCreate) list.push({ kind: 'create' });
    groups.forEach((g) => {
      list.push({ kind: 'group', group: g });
      g.items.forEach((item) => list.push({ kind: 'item', item }));
    });
    return list;
  }, [groups, canCreate]);

  useEffect(() => setActive(0), [query, onlySelected]);

  useEffect(() => {
    if (!open) return undefined;
    const onDoc = (e) => {
      if (rootRef.current && !rootRef.current.contains(e.target)) setOpen(false);
    };
    document.addEventListener('mousedown', onDoc);
    return () => document.removeEventListener('mousedown', onDoc);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    listRef.current?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' });
  }, [active, open]);

  const openWith = (q = '') => {
    if (disabled) return;
    setQuery(q);
    setOnlySelected(false);
    setOpen(true);
    onOpen?.();
    requestAnimationFrame(() => searchRef.current?.focus());
  };

  const create = () => {
    if (!canCreate) return;
    onChange([...value, typed]);
    setQuery('');
  };

  const activate = (stop) => {
    if (!stop) return;
    if (stop.kind === 'create') create();
    else if (stop.kind === 'group')
      onChange(
        toggleGroup(
          value,
          stop.group.items.map((i) => i.id)
        )
      );
    else onChange(toggleOne(value, stop.item.id));
  };

  const onKeyDown = (e) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive((i) => Math.min(i + 1, stops.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      activate(stops[active]);
    } else if (e.key === 'Escape') {
      // preventDefault keeps the surrounding Sheet / Dialog open (they skip handled events).
      e.preventDefault();
      setOpen(false);
    }
  };

  const total = all.length;
  let stopIndex = -1;
  const nextStop = () => {
    stopIndex += 1;
    return stopIndex;
  };

  const rowClass = (isActive) =>
    cn('flex w-full cursor-pointer select-none items-center gap-2 rounded-md px-2 text-left outline-none', isActive && 'bg-muted');

  return (
    <div ref={rootRef} className="relative space-y-2">
      <button
        id={id}
        type="button"
        role="combobox"
        aria-expanded={open}
        aria-controls={listId}
        aria-haspopup="listbox"
        disabled={disabled}
        onClick={() => (open ? setOpen(false) : openWith())}
        className="flex h-9 w-full items-center gap-2 rounded-lg border border-input bg-field px-3 text-sm transition-colors hover:bg-muted/40 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
      >
        {summary.length > 0 && (
          <span className="flex shrink-0 items-center">
            {summary.slice(0, MAX_TRIGGER_MARKS).map((g, i) => (
              <span
                key={g.key}
                className={cn('flex size-5 items-center justify-center rounded-full bg-card ring-2 ring-background', i > 0 && '-ml-1.5')}
              >
                <VendorMark group={g} className="size-3.5" />
              </span>
            ))}
          </span>
        )}
        <span className={cn('flex-1 truncate text-left', value.length === 0 && 'text-muted-foreground')}>
          {value.length > 0 ? m('selectedCount', { count: value.length }) : placeholder || m('placeholder')}
        </span>
        <ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" />
      </button>

      {open && (
        <div className="absolute inset-x-0 top-10 z-50 overflow-hidden rounded-lg border border-border bg-card text-card-foreground shadow-md">
          <div className="flex items-center gap-2 border-b border-border px-3">
            <Search className="size-4 shrink-0 text-muted-foreground" />
            <input
              ref={searchRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={m('search')}
              onKeyDown={onKeyDown}
              role="searchbox"
              aria-controls={listId}
              aria-activedescendant={stops.length > 0 ? `${listId}-${active}` : undefined}
              className="h-10 min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            />
            <button
              type="button"
              aria-pressed={onlySelected}
              onClick={() => setOnlySelected((v) => !v)}
              className={cn(
                'shrink-0 rounded-md px-2 py-1 text-xs transition-colors',
                onlySelected ? 'bg-muted text-foreground' : 'text-muted-foreground hover:text-foreground'
              )}
            >
              {m('onlySelected')}
            </button>
            <span className="shrink-0 font-mono text-xs tabular-nums text-muted-foreground">
              {value.length}/{total}
            </span>
          </div>

          {(loading || notice) && (
            <div className="flex items-center gap-2 border-b border-border px-3 py-2 text-xs text-muted-foreground">
              {loading && <Loader2 className="size-3.5 animate-spin" />}
              {loading ? m('loadingUpstream') : notice}
            </div>
          )}

          <div ref={listRef} id={listId} role="listbox" aria-multiselectable="true" className="max-h-[360px] overflow-y-auto p-1">
            {canCreate &&
              (() => {
                const idx = nextStop();
                return (
                  <div
                    role="option"
                    id={`${listId}-${idx}`}
                    tabIndex={-1}
                    onKeyDown={onKeyDown}
                    aria-selected={false}
                    data-active={active === idx}
                    onMouseEnter={() => setActive(idx)}
                    onClick={create}
                    className={cn(rowClass(active === idx), 'h-8 text-sm')}
                  >
                    <Plus className="size-4 text-muted-foreground" />
                    <span className="truncate">{m('add', { model: typed })}</span>
                  </div>
                );
              })()}

            {groups.length === 0 && !canCreate && (
              <p className="px-2 py-6 text-center text-sm text-muted-foreground">{onlySelected ? m('none') : m('empty')}</p>
            )}

            {groups.map((g) => {
              const ids = g.items.map((i) => i.id);
              const on = ids.filter((x) => selected.has(x)).length;
              const headerIdx = nextStop();
              return (
                <Fragment key={g.key}>
                  <div
                    role="option"
                    id={`${listId}-${headerIdx}`}
                    tabIndex={-1}
                    onKeyDown={onKeyDown}
                    aria-selected={on === ids.length}
                    data-active={active === headerIdx}
                    onMouseEnter={() => setActive(headerIdx)}
                    onClick={() => onChange(toggleGroup(value, ids))}
                    title={on === ids.length ? m('clearGroup') : m('selectGroup')}
                    className={cn(
                      rowClass(active === headerIdx),
                      'sticky top-0 z-10 mt-1 h-8 bg-card first:mt-0',
                      active === headerIdx && 'bg-muted'
                    )}
                  >
                    <Tick checked={on === ids.length} mixed={on > 0 && on < ids.length} />
                    <VendorMark group={g} />
                    <span className="truncate text-sm font-medium">{g.name || m('custom')}</span>
                    <span className="ml-auto shrink-0 font-mono text-xs tabular-nums text-muted-foreground">
                      {on > 0 ? `${on}/${ids.length}` : ids.length}
                    </span>
                  </div>
                  {g.items.map((item) => {
                    const idx = nextStop();
                    const isOn = selected.has(item.id);
                    const month = releaseMonth(item.releasedAt);
                    return (
                      <div
                        key={item.id}
                        role="option"
                        id={`${listId}-${idx}`}
                        tabIndex={-1}
                        onKeyDown={onKeyDown}
                        aria-selected={isOn}
                        data-active={active === idx}
                        onMouseEnter={() => setActive(idx)}
                        onClick={() => onChange(toggleOne(value, item.id))}
                        className={cn(rowClass(active === idx), 'h-8 pl-8')}
                      >
                        <Tick checked={isOn} />
                        <span className="min-w-0 flex-1 truncate font-mono text-xs" title={item.id}>
                          {item.id}
                        </span>
                        {item.meta && <span className="shrink-0 text-xs text-muted-foreground">{item.meta}</span>}
                        {month && (
                          <span className="w-14 shrink-0 text-right font-mono text-xs tabular-nums text-muted-foreground">{month}</span>
                        )}
                      </div>
                    );
                  })}
                </Fragment>
              );
            })}
          </div>

          <div className="flex items-center justify-between gap-2 border-t border-border px-2 py-1.5">
            {advanced ? (
              <button
                type="button"
                onClick={() => {
                  setOpen(false);
                  advanced.onClick();
                }}
                className="inline-flex h-7 items-center gap-1.5 rounded-md px-2 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              >
                <SlidersHorizontal className="size-3.5" />
                {advanced.label || m('advanced')}
              </button>
            ) : (
              <span />
            )}
            <button
              type="button"
              onClick={() => setOpen(false)}
              className="inline-flex h-7 items-center rounded-md px-3 text-xs font-medium transition-colors hover:bg-muted"
            >
              {m('done')}
            </button>
          </div>
        </div>
      )}

      {showSummary && summary.length > 0 && (
        <div className="divide-y divide-border rounded-lg border border-border">
          {summary.map((g) => (
            <button
              key={g.key}
              type="button"
              disabled={disabled}
              onClick={() => openWith(g.key === CUSTOM_VENDOR ? '' : g.name)}
              className="flex w-full items-start gap-2.5 px-3 py-2 text-left transition-colors hover:bg-muted/40 disabled:cursor-default"
            >
              <VendorMark group={g} className="mt-0.5" />
              <span className="w-28 shrink-0 truncate text-sm">{g.name || m('custom')}</span>
              <span className="line-clamp-2 min-w-0 flex-1 break-all font-mono text-xs leading-5 text-muted-foreground">
                {g.items.map((i) => i.id).join('  ·  ')}
              </span>
              <span className="shrink-0 font-mono text-xs leading-5 tabular-nums text-muted-foreground">{g.items.length}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

ModelMultiSelect.propTypes = {
  id: PropTypes.string,
  value: PropTypes.arrayOf(PropTypes.string),
  onChange: PropTypes.func.isRequired,
  options: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string.isRequired,
      vendor: PropTypes.shape({ key: PropTypes.string, name: PropTypes.string, icon: PropTypes.string }),
      releasedAt: PropTypes.number,
      meta: PropTypes.node
    })
  ),
  onOpen: PropTypes.func,
  loading: PropTypes.bool,
  notice: PropTypes.node,
  allowCreate: PropTypes.bool,
  // Optional check of a typed value before it can be added (e.g. only "prefix*" patterns).
  canCreateValue: PropTypes.func,
  advanced: PropTypes.shape({ label: PropTypes.node, onClick: PropTypes.func.isRequired }),
  disabled: PropTypes.bool,
  placeholder: PropTypes.string,
  showSummary: PropTypes.bool
};
