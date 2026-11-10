import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { UserRound } from 'lucide-react';

import { API } from 'utils/api';
import { showError, showSuccess, trims } from 'utils/common';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { FormCard } from '@/components/settings/cards';

// ==============================|| SETTINGS — ACCOUNT / PROFILE ||============================== //
// Who you are (avatar, username, email, user group; read-only) and the display name, saved by the
// card footer. Quota figures stay on the dashboard.

export default function ProfileCard({ self }) {
  const { t } = useTranslation();
  const account = useSelector((state) => state.account);
  const { inputs, reloadUser } = self;
  const [userGroupMap, setUserGroupMap] = useState({});
  const [displayName, setDisplayName] = useState('');
  const [saving, setSaving] = useState(false);
  const saved = inputs.display_name || '';

  useEffect(() => setDisplayName(saved), [saved]);

  useEffect(() => {
    API.get('/api/user_group_map')
      .then((res) => {
        if (res.data?.success) setUserGroupMap(res.data.data || {});
      })
      .catch(() => {});
  }, []);

  const dirty = trims(displayName) !== saved;

  const save = async () => {
    const value = trims(displayName);
    setSaving(true);
    try {
      const res = await API.put('/api/user/self', { ...inputs, display_name: value, password: '' });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('profilePage.updateSuccess'));
        await reloadUser();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const group = inputs.group ? userGroupMap[inputs.group] : null;

  return (
    <FormCard
      title={t('settingsUi.personal.cards.profile.title')}
      description={t('settingsUi.personal.cards.profile.description')}
      hint={t('settingsUi.personal.cards.profile.hint')}
      highlight="profile"
      dirty={dirty}
      saving={saving}
      onSave={save}
      onReset={() => setDisplayName(saved)}
    >
      <div className="flex items-center gap-4">
        <span className="flex size-14 shrink-0 items-center justify-center overflow-hidden rounded-full border border-border bg-muted">
          {account.user?.avatar_url ? (
            <img src={account.user.avatar_url} alt={inputs.username} className="size-full object-cover" />
          ) : (
            <UserRound className="h-3/4 w-3/4 text-muted-foreground" />
          )}
        </span>
        <div className="min-w-0 space-y-0.5">
          <p className="truncate font-mono text-sm font-medium">{inputs.username}</p>
          {inputs.email && <p className="truncate text-sm text-muted-foreground">{inputs.email}</p>}
          {inputs.phone_number && <p className="text-sm text-muted-foreground">{inputs.phone_number}</p>}
          <p className="text-xs text-muted-foreground">
            {t('profilePage.group')}:{' '}
            {group
              ? `${group.name} · ${t('profilePage.rate')} ${group.ratio} / ${t('profilePage.speed')} ${group.api_rate}`
              : inputs.group || 'default'}
          </p>
        </div>
      </div>
      <div className="grid gap-5 sm:grid-cols-2">
        <FormField label={t('profilePage.displayName')} htmlFor="display_name">
          <Input
            id="display_name"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder={t('profilePage.inputDisplayNamePlaceholder')}
          />
        </FormField>
        <FormField label={t('settingsUi.personal.username')} htmlFor="username">
          <Input id="username" className="font-mono" value={inputs.username || ''} disabled readOnly />
        </FormField>
      </div>
    </FormCard>
  );
}

ProfileCard.propTypes = {
  self: PropTypes.shape({ inputs: PropTypes.object, reloadUser: PropTypes.func }).isRequired
};
