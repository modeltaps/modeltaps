import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { KeyRound, MoreHorizontal } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { ObjectPanel, ObjectTile } from '@/components/objects';
import { API } from 'utils/api';
import { useOrg } from 'contexts/OrgContext';
import { isPlaygroundToken } from 'utils/playgroundToken';
import { toLimitModels } from './ModelLimitSelector';
import { buildTokenActions } from './TokenTable';
import { TOKEN_TABS } from './tokenTabs';
import { fetchToken, saveToken } from './tokenData';
import { useScopedNavigate } from 'routes/useScope';
import { isOrgOwnedToken } from './orgOwned';

// Right-side panel of one API key. Header: the enable switch (applies at once) and a menu with
// test command, client setup, usage, logs and delete. Tabs: Overview, Basics, Access, Advanced;
// a key the playground manages only shows its overview. `reloadKey` changes when the list changed
// the key (switch, menu), so the panel shows what is stored.
export default function TokenPanel({
  tokenId,
  tab,
  onTabChange,
  onClose,
  onChanged,
  adminMode,
  reloadKey,
  userGroup,
  groupOptions,
  userIsReliable,
  orgMemberNames,
  todayUsage,
  resetTimezone,
  resetWeekStart,
  onToggleStatus,
  onResetKey,
  canResetKey,
  onTransferToOrg,
  onCopyText,
  onDelete,
  onShowUsage,
  onConnectOpenClaw
}) {
  const { t, i18n } = useTranslation();
  const navigate = useScopedNavigate();
  const [token, setToken] = useState(null);
  const [logIO, setLogIO] = useState(null);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      const res = await fetchToken(tokenId, adminMode);
      if (res.success) {
        setToken(res.data);
        setLogIO(res.logIO);
        setError('');
      } else setError(res.message || t('objects.notFound'));
    } catch (e) {
      setError(e.message);
    }
  }, [tokenId, adminMode, t]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const playground = token ? isPlaygroundToken(token) : false;
  const tabs = useMemo(
    () => (playground ? TOKEN_TABS.slice(0, 1) : TOKEN_TABS).map((x) => ({ ...x, label: t(`apiKey.panel.tab.${x.key}`) })),
    [t, playground]
  );

  // Models offered by the model limit picker, fetched once per panel.
  const loadModels = useMemo(() => {
    let cached;
    return () => {
      cached ||= API.get('/api/available_model')
        .then((res) => (res.data.success ? toLimitModels(res.data.data) : []))
        .catch(() => []);
      return cached;
    };
  }, []);

  // Organization keys: the model access of the key's creator (ORG-15), fetched once per creator.
  const { currentOrgId } = useOrg();
  const loadModelAccess = useMemo(() => {
    if (!currentOrgId || adminMode) return undefined;
    const cache = {};
    return (userId) => {
      const key = userId || 0;
      cache[key] ||= API.get(`/api/org/${currentOrgId}/model_access`, { params: userId ? { user_id: userId } : {} })
        .then((res) => (res.data.success ? res.data.data : { scope: null, lists: [] }))
        .catch(() => ({ scope: null, lists: [] }));
      return cache[key];
    };
  }, [currentOrgId, adminMode]);

  // Owners and admins can make a member's key owned by the organization (ORG-16).
  const transferToOrg = token && onTransferToOrg && !isOrgOwnedToken(token) ? () => onTransferToOrg(token) : undefined;

  // Reset is offered on the key row of the overview, not in this menu.
  const resetKey = token && onResetKey && !playground && (!canResetKey || canResetKey(token)) ? () => onResetKey(token) : undefined;
  const menu = token
    ? buildTokenActions({ item: token, t, navigate, onEdit: () => {}, onDelete, onShowUsage, onConnectOpenClaw }).filter(
        (a) => a && a.overflow
      )
    : [];

  return (
    <ObjectPanel
      titleId="token-panel-title"
      object={token}
      error={error}
      icon={
        <ObjectTile>
          <KeyRound />
        </ObjectTile>
      }
      title={token?.name}
      subtitle={token && !playground ? token.key_hint || null : null}
      subtitleMono
      actions={
        token && (
          <>
            <span className="text-xs text-muted-foreground">{t('apiKey.panel.enabled')}</span>
            <Switch
              checked={token.status === 1}
              aria-label={t('apiKey.panel.enabled')}
              onCheckedChange={async () => {
                await onToggleStatus(token);
                await load();
              }}
            />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="size-8 rounded-lg" aria-label={t('objects.more')}>
                  <MoreHorizontal className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {menu.map((a) => {
                  const Icon = a.icon;
                  return (
                    <div key={a.key}>
                      {a.destructive && <DropdownMenuSeparator />}
                      <DropdownMenuItem
                        disabled={a.disabled}
                        className={a.destructive ? 'text-destructive' : undefined}
                        onClick={a.onClick}
                      >
                        <Icon /> {a.label}
                      </DropdownMenuItem>
                    </div>
                  );
                })}
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        )
      }
      tabs={tabs}
      tab={tab}
      onTabChange={onTabChange}
      onClose={onClose}
      onSave={(patch) => saveToken(tokenId, patch, adminMode)}
      onSaved={async () => {
        await load();
        onChanged();
      }}
      ctx={{
        t,
        transferToOrg,
        adminMode,
        userGroup,
        groupOptions,
        userIsReliable,
        orgMemberNames,
        todayUsage,
        resetTimezone,
        resetWeekStart,
        locale: i18n.language,
        loadModels,
        loadModelAccess,
        resetKey,
        onCopyText,
        logIO
      }}
      tabsLabel={t('apiKey.panel.tabs')}
    />
  );
}

TokenPanel.propTypes = {
  tokenId: PropTypes.number.isRequired,
  tab: PropTypes.string,
  onTabChange: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
  onChanged: PropTypes.func.isRequired,
  adminMode: PropTypes.bool,
  reloadKey: PropTypes.number,
  userGroup: PropTypes.object,
  groupOptions: PropTypes.array.isRequired,
  userIsReliable: PropTypes.bool,
  orgMemberNames: PropTypes.object,
  todayUsage: PropTypes.object,
  resetTimezone: PropTypes.string,
  resetWeekStart: PropTypes.string,
  onToggleStatus: PropTypes.func.isRequired,
  onResetKey: PropTypes.func,
  canResetKey: PropTypes.func,
  onTransferToOrg: PropTypes.func,
  onCopyText: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired,
  onShowUsage: PropTypes.func.isRequired,
  onConnectOpenClaw: PropTypes.func.isRequired
};
