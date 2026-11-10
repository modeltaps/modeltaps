import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { ObjectPanel, ObjectTile } from '@/components/objects';
import { API } from 'utils/api';
import { trims } from 'utils/common';
import { USER_TABS } from './userTabs';
import { ActionsCell, QuotaDialog, StatusCell } from './UserRowCells';

const fetchUser = async (id) => (await API.get(`/api/user/${id}`)).data;

// Right-side panel of one user: header with the enable switch and the user menu (adjust balance,
// role, delete), tabs Overview, Profile, Quota & groups and Security. `reloadKey` changes when
// the list changed the user (switch, menu actions), so the panel shows the stored values.
export default function UserPanel({ userId, tab, onTabChange, onClose, onChanged, manageUser, groups, isRoot, reloadKey }) {
  const { t } = useTranslation();
  const [user, setUser] = useState(null);
  const [error, setError] = useState('');
  const [quotaOpen, setQuotaOpen] = useState(false);
  const tabs = useMemo(
    () =>
      USER_TABS.map((x) => ({
        ...x,
        label: t(`userPage.panel.tab.${x.key}`),
        saveLabel: x.key === 'security' ? t('userPage.panel.setPassword') : undefined
      })),
    [t]
  );

  const load = useCallback(async () => {
    try {
      const { success, message, data } = await fetchUser(userId);
      if (success) {
        setUser(data);
        setError('');
      } else setError(message || t('objects.notFound'));
    } catch (e) {
      setError(e.message);
    }
  }, [userId, t]);

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  // The update endpoint takes the whole user: start from what is stored now (the balance may have
  // moved since the panel opened) and replace the tab's fields. An empty password keeps it.
  const save = async (patch) => {
    const fresh = await fetchUser(userId);
    if (!fresh.success) return fresh;
    const res = await API.put('/api/user/', trims({ ...fresh.data, password: '', ...patch, id: userId }));
    return res.data;
  };

  const initials = (user?.username || '?').slice(0, 2);
  const subtitle = user
    ? [`#${user.id}`, user.display_name !== user.username ? user.display_name : '', user.email].filter(Boolean).join(' · ')
    : '';

  return (
    <>
      <ObjectPanel
        titleId="user-panel-title"
        object={user}
        error={error}
        icon={
          <ObjectTile className="rounded-full text-xs font-medium uppercase text-foreground">
            <span>{initials}</span>
          </ObjectTile>
        }
        title={user?.username}
        subtitle={subtitle}
        actions={
          user && (
            <>
              <span className="text-xs text-muted-foreground">{t('userPage.enableSwitch')}</span>
              <StatusCell item={user} manageUser={manageUser} />
              <ActionsCell item={user} manageUser={manageUser} triggerClassName="size-8 rounded-lg" />
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
        ctx={{ t, groups, isRoot, onAdjustQuota: () => setQuotaOpen(true) }}
        tabsLabel={t('userPage.panel.tabs')}
      />
      {user && <QuotaDialog item={user} open={quotaOpen} onOpenChange={setQuotaOpen} manageUser={manageUser} />}
    </>
  );
}

UserPanel.propTypes = {
  userId: PropTypes.number.isRequired,
  tab: PropTypes.string,
  onTabChange: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
  onChanged: PropTypes.func.isRequired,
  manageUser: PropTypes.func.isRequired,
  groups: PropTypes.array.isRequired,
  isRoot: PropTypes.bool,
  reloadKey: PropTypes.number
};
