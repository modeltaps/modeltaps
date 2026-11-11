import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { MoreHorizontal, Split, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { ObjectPanel, ObjectTile } from '@/components/objects';
import PolicyCallName from './PolicyCallName';
import { EMPTY_POLICY, normalizePolicy, policyCallName } from './routingPolicyApi';
import { POLICY_TABS, REVIEW_TAB_KEYS } from './policyTabs';

// Right-side panel of one routing policy: header with the enable switch and Delete, tabs
// Overview (summary and statistics), Routes, Try it and Settings. `api` is the scoped client; a
// review client (other scopes' policies in the platform console) only shows Overview and the
// enable switch; a readOnly client (organization members) only shows Overview, without statistics. `listed` is the policy's row in the list (organization name, name conflicts).
export default function RoutingPolicyPanel({
  api,
  listed = null,
  policyId,
  tab,
  onTabChange,
  onClose,
  onChanged,
  onToggleEnabled,
  onDelete,
  modelOptions
}) {
  const { t } = useTranslation();
  const [policy, setPolicy] = useState(null);
  const [error, setError] = useState('');
  const review = Boolean(api.review);
  const readOnly = Boolean(api.readOnly);
  const tabs = useMemo(
    () =>
      POLICY_TABS.filter((x) => !(review || readOnly) || REVIEW_TAB_KEYS.includes(x.key)).map((x) => ({
        ...x,
        label: t(`routingPolicy.panel.tab.${x.key}`)
      })),
    [t, review, readOnly]
  );

  const load = useCallback(async () => {
    try {
      const { success, message, data } = await api.get(policyId);
      if (success) {
        setPolicy({ ...EMPTY_POLICY, ...data, routes: data.routes || [] });
        setError('');
      } else setError(message || t('objects.notFound'));
    } catch (e) {
      setError(e.message);
    }
  }, [api, policyId, t]);

  useEffect(() => {
    load();
  }, [load]);

  const toggle = async (enabled) => {
    if (await onToggleEnabled(policy, enabled)) setPolicy((p) => ({ ...p, enabled }));
  };

  const subtitle = policy
    ? [
        `#${policy.id}`,
        review
          ? policy.user_id
            ? listed?.username || `#${policy.user_id}`
            : listed?.organization_name || `#${policy.organization_id}`
          : null,
        policy.display_name,
        t('routingPolicy.panel.routeCount', { count: policy.routes.length })
      ]
        .filter(Boolean)
        .join(' · ')
    : '';

  return (
    <ObjectPanel
      titleId="routing-policy-panel-title"
      object={policy}
      error={error}
      icon={
        <ObjectTile>
          <Split />
        </ObjectTile>
      }
      title={
        api.callPrefix && policy ? <PolicyCallName name={policyCallName(api, policy)} /> : <span className="font-mono">{policy?.name}</span>
      }
      subtitle={subtitle}
      actions={
        policy && (
          <>
            {!readOnly && (
              <>
                <span className="text-xs text-muted-foreground">{t('routingPolicy.columns.enabled')}</span>
                <Switch checked={Boolean(policy.enabled)} aria-label={t('routingPolicy.columns.enabled')} onCheckedChange={toggle} />
              </>
            )}
            {!review && !readOnly && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon" className="size-8 rounded-lg" aria-label={t('objects.more')}>
                    <MoreHorizontal className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48">
                  <DropdownMenuItem className="text-destructive" onClick={() => onDelete(policy)}>
                    <Trash2 className="size-4" />
                    {t('common.delete')}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            )}
          </>
        )
      }
      tabs={tabs}
      tab={tab}
      onTabChange={onTabChange}
      onClose={onClose}
      onSave={(patch) => api.update(policy.id, normalizePolicy({ ...policy, ...patch }))}
      onSaved={async () => {
        await load();
        onChanged();
      }}
      ctx={{ t, modelOptions, api, readOnly, overridesPlatform: Boolean(listed?.overrides_platform) }}
      tabsLabel={t('routingPolicy.panel.tabs')}
    />
  );
}

RoutingPolicyPanel.propTypes = {
  api: PropTypes.object.isRequired,
  listed: PropTypes.object,
  policyId: PropTypes.number.isRequired,
  tab: PropTypes.string,
  onTabChange: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
  onChanged: PropTypes.func.isRequired,
  onToggleEnabled: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  modelOptions: PropTypes.array.isRequired
};
