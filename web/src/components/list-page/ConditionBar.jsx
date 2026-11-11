import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { BookmarkPlus, ListFilter, Sparkles, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { FilterBar, describeEntry, isEntryActive } from '@/components/filter-bar';

// ==============================|| LIST PAGE — CONDITION BAR + PRESETS ||============================== //
// The always-visible "Show … that" bar: active conditions as chips, "+ Add condition" and a
// Presets menu at the end. Presets are a status segment plus conditions applied in one click:
// built-in ones from the page, and combinations the user saved (kept in localStorage).

// A preset is { id, labelKey? | name, status?, conditions: { [fieldKey]: entry } }.
// `status` is the page's current segment (null when the page has none) and `statusLabel(value)`
// its readable name; `apply(preset)` sets the page's segment and conditions.
export function useFilterPresets({ storageKey, builtin = [], fields, state, status = null, statusLabel, apply }) {
  const { t } = useTranslation();
  const [saved, setSaved] = useState(() => {
    try {
      const list = JSON.parse(localStorage.getItem(storageKey));
      return Array.isArray(list) ? list.filter((p) => p && typeof p.name === 'string' && p.conditions) : [];
    } catch {
      return [];
    }
  });
  useEffect(() => {
    try {
      localStorage.setItem(storageKey, JSON.stringify(saved));
    } catch {
      /* storage unavailable -> keep in-memory only */
    }
  }, [storageKey, saved]);

  const current = Object.fromEntries(fields.filter((f) => isEntryActive(f, state[f.key])).map((f) => [f.key, state[f.key]]));
  const hasStatus = status != null && status !== 'all';
  // A readable name for a combination, e.g. "Disabled · Expiry: Expire within 14 days".
  const describe = (preset) =>
    [
      preset.status && preset.status !== 'all' && statusLabel ? statusLabel(preset.status) : null,
      ...fields.filter((f) => preset.conditions[f.key]).map((f) => describeEntry(t, f, preset.conditions[f.key]))
    ]
      .filter(Boolean)
      .join(' · ');

  return {
    builtin,
    saved,
    describe,
    apply,
    canSave: hasStatus || Object.keys(current).length > 0,
    save: () => {
      const preset = { id: `saved-${Date.now()}`, status: status ?? undefined, conditions: current };
      preset.name = describe(preset);
      setSaved((list) => [...list.filter((p) => p.name !== preset.name), preset]);
    },
    remove: (id) => setSaved((list) => list.filter((p) => p.id !== id))
  };
}

// Items of the Presets menu: built-in views with what they select, saved ones (removable), save.
export function PresetMenuItems({ presets }) {
  const { t } = useTranslation();
  return (
    <>
      <DropdownMenuLabel>{t('listPage.presetsBuiltin')}</DropdownMenuLabel>
      {presets.builtin.map((p) => (
        <DropdownMenuItem key={p.id} onClick={() => presets.apply(p)} className="flex-col items-start gap-0">
          <span>{t(p.labelKey)}</span>
          <span className="text-xs text-muted-foreground">{presets.describe(p)}</span>
        </DropdownMenuItem>
      ))}
      {presets.saved.length > 0 && (
        <>
          <DropdownMenuSeparator />
          <DropdownMenuLabel>{t('listPage.presetsSaved')}</DropdownMenuLabel>
          {presets.saved.map((p) => (
            <div key={p.id} className="group/preset flex items-center gap-1">
              <DropdownMenuItem onClick={() => presets.apply(p)} className="min-w-0 flex-1">
                <span className="truncate">{p.name}</span>
              </DropdownMenuItem>
              <button
                type="button"
                aria-label={t('listPage.presetRemove')}
                title={t('listPage.presetRemove')}
                onClick={() => presets.remove(p.id)}
                className="flex size-6 shrink-0 items-center justify-center rounded text-muted-foreground opacity-0 transition-opacity hover:bg-muted hover:text-foreground focus-visible:opacity-100 group-hover/preset:opacity-100"
              >
                <X className="size-3.5" />
              </button>
            </div>
          ))}
        </>
      )}
      <DropdownMenuSeparator />
      <DropdownMenuItem disabled={!presets.canSave} onClick={presets.save}>
        <BookmarkPlus className="size-4" />
        {t('listPage.presetSave')}
      </DropdownMenuItem>
    </>
  );
}

PresetMenuItems.propTypes = { presets: PropTypes.object.isRequired };

// Presets as tappable chips, for the phone filter sheet. `onPick` runs after applying.
export function PresetChips({ presets, onPick }) {
  const { t } = useTranslation();
  const all = [...presets.builtin, ...presets.saved];
  if (all.length === 0) return null;
  return (
    <div className="space-y-1.5">
      <div className="text-[11px] font-medium text-muted-foreground">{t('listPage.presets')}</div>
      <div className="flex flex-wrap gap-1.5">
        {all.map((p) => (
          <button
            key={p.id}
            type="button"
            onClick={() => {
              presets.apply(p);
              onPick?.();
            }}
            className="h-8 rounded-full border border-input bg-field px-3 text-[13px] hover:bg-muted"
          >
            {p.labelKey ? t(p.labelKey) : p.name}
          </button>
        ))}
      </div>
    </div>
  );
}

PresetChips.propTypes = { presets: PropTypes.object.isRequired, onPick: PropTypes.func };

// The condition bar itself. `leadingLabel` names what the list holds ("Show keys that").
export default function ConditionBar({ fields, state, onChange, onClearAll, leadingLabel, presets }) {
  const { t } = useTranslation();
  return (
    <div className="flex min-h-10 w-full items-center gap-2 rounded-lg border border-border bg-card px-3 py-1.5">
      <FilterBar
        fields={fields}
        state={state}
        onChange={onChange}
        onClearAll={onClearAll}
        triggerVariant="plus"
        triggerLabel={t('listPage.addCondition')}
        clearAlignEnd
        t={t}
        leading={
          <span className="inline-flex items-center gap-1.5 text-[13px] text-muted-foreground">
            <ListFilter className="size-3.5" />
            {leadingLabel}
          </span>
        }
        className="min-w-0 flex-1"
      />
      {presets && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" className="h-7 shrink-0 gap-1.5 px-2 text-[13px]">
              <Sparkles className="size-3.5" />
              {t('listPage.presets')}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-72">
            <PresetMenuItems presets={presets} />
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  );
}

ConditionBar.propTypes = {
  fields: PropTypes.array.isRequired,
  state: PropTypes.object.isRequired,
  onChange: PropTypes.func.isRequired,
  onClearAll: PropTypes.func.isRequired,
  leadingLabel: PropTypes.node,
  presets: PropTypes.object
};
