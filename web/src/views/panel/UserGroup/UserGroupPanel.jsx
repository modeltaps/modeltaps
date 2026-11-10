import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { MoreHorizontal, Trash2, Users } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Field, ObjectPanel, ObjectTile, ToggleField } from '@/components/objects';
import { API } from 'utils/api';

const NUMBERS = ['ratio', 'api_rate', 'min', 'max'];

function NumberRow({ id, label, hint, help, value, onChange }) {
  return (
    <Field label={label} htmlFor={id} hint={hint} help={help}>
      <Input id={id} type="number" className="w-40 tabular-nums" value={value} onChange={(e) => onChange(e.target.value)} />
    </Field>
  );
}

NumberRow.propTypes = {
  id: PropTypes.string.isRequired,
  label: PropTypes.node,
  hint: PropTypes.node,
  help: PropTypes.node,
  value: PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  onChange: PropTypes.func.isRequired
};

function SettingsBody({ object: group, draft, setDraft }) {
  const { t } = useTranslation();
  const set = (key) => (value) => setDraft((d) => ({ ...d, [key]: value }));
  return (
    <>
      <Field label={t('userGroup.symbol')} hint={t('userGroup.panel.symbolFixed')}>
        <p className="font-mono text-[13px]">{group.symbol}</p>
      </Field>
      <Field label={t('userGroup.name')} htmlFor="g-name" hint={t('userGroup.nameTip')}>
        <Input id="g-name" value={draft.name} onChange={(e) => set('name')(e.target.value)} />
      </Field>
      <NumberRow id="g-ratio" label={t('userGroup.ratio')} value={draft.ratio} onChange={set('ratio')} />
      <NumberRow
        id="g-api_rate"
        label={t('userGroup.apiRate')}
        help={t('userGroup.apiRateTip')}
        value={draft.api_rate}
        onChange={set('api_rate')}
      />
      <ToggleField
        id="g-public"
        label={t('userGroup.public')}
        hint={t('userGroup.panel.publicHint')}
        checked={draft.public}
        onChange={set('public')}
      />
      <ToggleField
        id="g-promotion"
        label={t('userGroup.promotion')}
        hint={t('userGroup.promotionTip')}
        checked={draft.promotion}
        onChange={set('promotion')}
      />
      <NumberRow id="g-min" label={`${t('userGroup.min')} ($)`} hint={t('userGroup.minTip')} value={draft.min} onChange={set('min')} />
      <NumberRow id="g-max" label={`${t('userGroup.max')} ($)`} hint={t('userGroup.maxTip')} value={draft.max} onChange={set('max')} />
    </>
  );
}

SettingsBody.propTypes = { object: PropTypes.object, draft: PropTypes.object, setDraft: PropTypes.func };

const settingsTab = {
  key: 'settings',
  make: (g) => ({
    name: g.name || '',
    ratio: g.ratio ?? 1,
    api_rate: g.api_rate ?? 0,
    public: Boolean(g.public),
    promotion: Boolean(g.promotion),
    min: g.min ?? 0,
    max: g.max ?? 0
  }),
  patch: (d) => ({
    ...d,
    name: d.name.trim(),
    ...Object.fromEntries(NUMBERS.map((k) => [k, Number(d[k]) || 0]))
  }),
  Body: SettingsBody
};

// Right-side panel of one user group. A group is small, so the panel has a single form saved at
// once; the enable switch and Delete are in the header.
export default function UserGroupPanel({ groupId, onClose, onChanged, onToggle, onDelete, reloadKey }) {
  const { t } = useTranslation();
  const [group, setGroup] = useState(null);
  const [error, setError] = useState('');
  const tabs = useMemo(() => [{ ...settingsTab, label: t('userGroup.panel.settings') }], [t]);

  const load = useCallback(async () => {
    try {
      const res = await API.get(`/api/user_group/${groupId}`);
      const { success, message, data } = res.data;
      if (success) {
        setGroup(data);
        setError('');
      } else setError(message || t('objects.notFound'));
    } catch (e) {
      setError(e.message);
    }
  }, [groupId, t]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  return (
    <ObjectPanel
      titleId="user-group-panel-title"
      size="sm"
      object={group}
      error={error}
      icon={
        <ObjectTile>
          <Users />
        </ObjectTile>
      }
      title={group?.name || group?.symbol}
      subtitle={group && `#${group.id} · ${group.symbol}`}
      subtitleMono
      actions={
        group && (
          <>
            <Switch checked={Boolean(group.enable)} aria-label={t('userGroup.enable')} onCheckedChange={() => onToggle(group)} />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="size-8 rounded-lg" aria-label={t('objects.more')}>
                  <MoreHorizontal className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-44">
                <DropdownMenuItem className="text-destructive" onClick={() => onDelete(group)}>
                  <Trash2 className="size-4" />
                  {t('common.delete')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        )
      }
      tabs={tabs}
      tab="settings"
      onTabChange={() => {}}
      onClose={onClose}
      onSave={async (patch) => (await API.put('/api/user_group/', { ...group, ...patch, id: group.id })).data}
      onSaved={async () => {
        await load();
        onChanged();
      }}
      ctx={{ t }}
      scope={t('userGroup.panel.scope')}
    />
  );
}

UserGroupPanel.propTypes = {
  groupId: PropTypes.number.isRequired,
  onClose: PropTypes.func.isRequired,
  onChanged: PropTypes.func.isRequired,
  onToggle: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  reloadKey: PropTypes.number
};
