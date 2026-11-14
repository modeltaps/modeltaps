import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { TriangleAlert } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select';
import { Segmented } from '@/components/objects';
import ModelLimitSelector from 'views/panel/Token/ModelLimitSelector';
import { uncoveredEntries } from 'utils/modelAllowlist';

// ==============================|| MODEL ACCESS FIELD (ORG-15) ||============================== //
// One layer of an organization's model access: inherit the layer above, attach a named list, or
// pick models (and prefix rules) inline. `parents` are the allowlists of the layers above; inline
// models are offered only inside them, and entries already saved outside them are listed as no
// longer in effect (requests keep checking every layer, so they are not reachable).
//
// value: { mode: 'inherit' | 'list' | 'custom', listId: number, models: string[] }

export const accessValueFrom = (listId, models) => {
  if (listId > 0) return { mode: 'list', listId, models: [] };
  if (models?.length) return { mode: 'custom', listId: 0, models };
  return { mode: 'inherit', listId: 0, models: [] };
};

export const accessPayload = (value) => ({
  listId: value.mode === 'list' ? value.listId || 0 : 0,
  models: value.mode === 'custom' ? value.models : []
});

export default function ModelAccessField({ id, value, onChange, lists = [], models = [], parents = [], inheritLabel, inheritHint }) {
  const { t } = useTranslation();
  const set = (patch) => onChange({ ...value, ...patch });
  const current = lists.find((l) => l.id === value.listId);
  const active = value.mode === 'list' ? current?.models || [] : value.mode === 'custom' ? value.models : [];
  const outside = uncoveredEntries(parents, active);
  const options = [
    { value: 'inherit', label: inheritLabel || t('modelAccess.mode.inherit') },
    { value: 'list', label: t('modelAccess.mode.list') },
    { value: 'custom', label: t('modelAccess.mode.custom') }
  ];

  return (
    <div className="space-y-3">
      <Segmented
        label={t('modelAccess.mode.label')}
        value={value.mode}
        onChange={(mode) => set({ mode, listId: mode === 'list' ? value.listId || lists[0]?.id || 0 : value.listId })}
        options={options}
      />
      {value.mode === 'inherit' && inheritHint && <p className="text-xs text-muted-foreground">{inheritHint}</p>}
      {value.mode === 'list' &&
        (lists.length === 0 ? (
          <p className="text-xs text-muted-foreground">{t('modelAccess.noLists')}</p>
        ) : (
          <>
            <Select value={value.listId ? String(value.listId) : ''} onValueChange={(v) => set({ listId: Number(v) })}>
              <SelectTrigger id={id} className="bg-field">
                <span className="truncate">{current ? current.name : t('modelAccess.pickList')}</span>
              </SelectTrigger>
              <SelectContent>
                {lists.map((l) => (
                  <SelectItem key={l.id} value={String(l.id)}>
                    <span className="truncate">{l.name}</span>
                    <span className="ml-2 text-xs text-muted-foreground">{t('modelAccess.modelCount', { count: l.models.length })}</span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {current && (
              <div className="flex flex-wrap gap-1">
                {current.models.map((m) => (
                  <Badge key={m} variant="secondary" className="font-mono text-[12px] font-normal">
                    {m}
                  </Badge>
                ))}
              </div>
            )}
          </>
        ))}
      {value.mode === 'custom' && (
        <>
          <ModelLimitSelector id={id} models={models} value={value.models} onChange={(ids) => set({ models: ids })} parents={parents} />
          <p className="text-xs text-muted-foreground">{t('modelAccess.patternHint')}</p>
        </>
      )}
      {outside.length > 0 && (
        <p className="flex items-start gap-1.5 text-xs text-warning">
          <TriangleAlert className="mt-px size-3.5 shrink-0" />
          <span>{t('modelAccess.outside', { models: outside.join(', ') })}</span>
        </p>
      )}
    </div>
  );
}

ModelAccessField.propTypes = {
  id: PropTypes.string,
  value: PropTypes.shape({ mode: PropTypes.string, listId: PropTypes.number, models: PropTypes.arrayOf(PropTypes.string) }).isRequired,
  onChange: PropTypes.func.isRequired,
  lists: PropTypes.arrayOf(PropTypes.object),
  models: PropTypes.arrayOf(PropTypes.object),
  parents: PropTypes.arrayOf(PropTypes.arrayOf(PropTypes.string)),
  inheritLabel: PropTypes.string,
  inheritHint: PropTypes.string
};
