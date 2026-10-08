import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { X } from 'lucide-react';

import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { cn } from '@/lib/utils';

// ==============================|| TOKEN — MODEL LIMIT SELECTOR ||============================== //
// Searchable, grouped multi-select replacing the v1 MUI Autocomplete. Stores the
// selected model ids in `setting.limits.limit_model_setting.models`.

export default function ModelLimitSelector({ models = [], value = [], onChange, getGroupLabel }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const selected = useMemo(() => new Set(value), [value]);

  const grouped = useMemo(() => {
    const q = search.trim().toLowerCase();
    const list = q ? models.filter((m) => (m.name || '').toLowerCase().includes(q)) : models;
    const sorted = [...list].sort((a, b) => (a.owned_by || '').localeCompare(b.owned_by || ''));
    const map = new Map();
    sorted.forEach((m) => {
      if (!map.has(m.owned_by)) map.set(m.owned_by, []);
      map.get(m.owned_by).push(m);
    });
    return [...map.entries()];
  }, [models, search]);

  const selectedModels = useMemo(() => models.filter((m) => selected.has(m.id)), [models, selected]);

  const toggle = (id) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    onChange?.([...next]);
  };

  return (
    <div className="space-y-2">
      {selectedModels.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {selectedModels.map((m) => (
            <Badge key={m.id} variant="secondary" className="gap-1">
              {m.name}
              <button type="button" aria-label="remove" onClick={() => toggle(m.id)} className="hover:text-foreground">
                <X className="size-3" />
              </button>
            </Badge>
          ))}
        </div>
      )}
      <Input placeholder={t('token_index.limit_models')} value={search} onChange={(e) => setSearch(e.target.value)} />
      <div className="max-h-64 overflow-y-auto rounded-md border border-border">
        {grouped.length === 0 ? (
          <p className="px-3 py-4 text-center text-sm text-muted-foreground">{t('token_index.limit_models_info')}</p>
        ) : (
          grouped.map(([owner, items]) => (
            <div key={owner}>
              <div className="sticky top-0 bg-muted/70 px-3 py-1 text-xs font-medium text-muted-foreground backdrop-blur">{owner}</div>
              {items.map((m) => (
                <div
                  key={m.id}
                  role="option"
                  tabIndex={0}
                  aria-selected={selected.has(m.id)}
                  onClick={() => toggle(m.id)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      toggle(m.id);
                    }
                  }}
                  className={cn(
                    'flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm hover:bg-muted focus:bg-muted focus:outline-none',
                    selected.has(m.id) && 'bg-muted/50'
                  )}
                >
                  <Checkbox checked={selected.has(m.id)} />
                  <span className="flex-1 truncate">{m.name}</span>
                  <span className="flex flex-wrap justify-end gap-1">
                    {m.groups?.map((g) => (
                      <Badge key={g} variant="outline" className="px-1.5 py-0 text-[10px]">
                        {getGroupLabel ? getGroupLabel(g) : g}
                      </Badge>
                    ))}
                  </span>
                </div>
              ))}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
