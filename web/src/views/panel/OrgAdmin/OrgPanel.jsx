import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Building2, MoreHorizontal, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { ObjectPanel, ObjectTile } from '@/components/objects';
import { API } from 'utils/api';
import { ORG_TABS } from './orgTabs';

// Right-side panel of one organization (site admin): header with the enable switch and Dissolve,
// tabs Overview, Balance (adjust the pool) and Members (member limit). Organizations are created
// by their owners, so there is no new-organization sheet.
export default function OrgPanel({ orgId, tab, onTabChange, onClose, onChanged, onToggleStatus, onDissolve, reloadKey }) {
  const { t } = useTranslation();
  const [org, setOrg] = useState(null);
  const [error, setError] = useState('');
  const tabs = useMemo(
    () =>
      ORG_TABS.map((x) => ({
        ...x,
        label: t(`orgAdmin.panel.tab.${x.key}`),
        saveLabel: x.key === 'balance' ? t('orgAdmin.panel.apply') : undefined
      })),
    [t]
  );

  const load = useCallback(async () => {
    try {
      const res = await API.get(`/api/organization/${orgId}`);
      const { success, message, data } = res.data;
      if (success) {
        setOrg(data);
        setError('');
      } else setError(message || t('objects.notFound'));
    } catch (e) {
      setError(e.message);
    }
  }, [orgId, t]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const save = async (patch, { tab: key }) => {
    const urls = { balance: 'quota', members: 'max_members', models: 'approved_models' };
    const res = await API.put(`/api/organization/${orgId}/${urls[key]}`, patch);
    const conflicts = res.data.data?.conflicts;
    if (!res.data.success && conflicts?.length) {
      // Lower lists of the organization that still use models outside the new approved models.
      const lines = conflicts.map((c) => `${t(`modelAccess.conflict.${c.kind}`, { name: c.name })}: ${c.entries.join(', ')}`);
      return { ...res.data, message: `${res.data.message} — ${lines.join('; ')}` };
    }
    return { ...res.data, toast: res.data.success ? t('common.operationSuccess') : undefined };
  };

  return (
    <ObjectPanel
      titleId="org-panel-title"
      object={org}
      error={error}
      icon={
        <ObjectTile>
          <Building2 />
        </ObjectTile>
      }
      title={org?.name}
      subtitle={
        org && [`#${org.id}`, org.slug, org.owner_id ? t('orgAdmin.panel.ownerId', { id: org.owner_id }) : ''].filter(Boolean).join(' · ')
      }
      actions={
        org && (
          <>
            <span className="text-xs text-muted-foreground">{t('common.enable')}</span>
            <Switch checked={org.status === 1} aria-label={t('common.enable')} onCheckedChange={() => onToggleStatus(org)} />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="size-8 rounded-lg" aria-label={t('objects.more')}>
                  <MoreHorizontal className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48">
                <DropdownMenuItem className="text-destructive" onClick={() => onDissolve(org)}>
                  <Trash2 className="size-4" />
                  {t('orgAdmin.dissolve')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        )
      }
      tabs={tabs}
      tab={tab}
      onTabChange={onTabChange}
      onClose={onClose}
      onSave={save}
      onSaved={async () => {
        await load();
        onChanged();
      }}
      ctx={{ t }}
      tabsLabel={t('orgAdmin.panel.tabs')}
    />
  );
}

OrgPanel.propTypes = {
  orgId: PropTypes.number.isRequired,
  tab: PropTypes.string,
  onTabChange: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
  onChanged: PropTypes.func.isRequired,
  onToggleStatus: PropTypes.func.isRequired,
  onDissolve: PropTypes.func.isRequired,
  reloadKey: PropTypes.number
};
