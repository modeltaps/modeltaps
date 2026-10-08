import { useTranslation } from 'react-i18next';

import { SettingsCard, TextRow, ToggleRow, SaveButton } from './parts';

// ==============================|| SYSTEM SETTINGS — ORGANIZATIONS (root) ||============================== //
// Site-level organization options (model/option.go T7 registrations):
// OrganizationEnabled / OrganizationMaxPerUser / OrganizationDefaultMaxMembers /
// OrganizationQuotaTransferEnabled, saved via the shared /api/option/ layer.
// Moved here from OrgAdmin's "settings" tab (IA-2/IA-4): site config lives in
// Setting, OrgAdmin is a pure org list. Root-only section (see Setting/index).

export default function OrgSettings({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;

  const intError = (key) => {
    const v = inputs[key];
    if (v === undefined || v === '') return false;
    const n = Number(v);
    return !Number.isInteger(n) || n < 0;
  };

  return (
    <SettingsCard title={t('orgAdmin.settings.title')} description={t('orgAdmin.settings.description')} highlight="org-policy">
      <ToggleRow
        label={t('orgAdmin.settings.enabledLabel')}
        description={t('orgAdmin.settings.enabledDesc')}
        checked={inputs.OrganizationEnabled === 'true'}
        onCheckedChange={() => toggle('OrganizationEnabled')}
        disabled={loading}
      />
      <ToggleRow
        label={t('orgAdmin.settings.transferLabel')}
        description={t('orgAdmin.settings.transferDesc')}
        checked={inputs.OrganizationQuotaTransferEnabled === 'true'}
        onCheckedChange={() => toggle('OrganizationQuotaTransferEnabled')}
        disabled={loading}
      />
      <TextRow
        id="OrganizationMaxPerUser"
        type="number"
        label={t('orgAdmin.settings.maxPerUserLabel')}
        description={t('orgAdmin.settings.maxPerUserDesc')}
        value={inputs.OrganizationMaxPerUser}
        onChange={(v) => setField('OrganizationMaxPerUser', v)}
        disabled={loading}
        error={intError('OrganizationMaxPerUser') ? t('orgAdmin.settings.intError') : ''}
      />
      <TextRow
        id="OrganizationDefaultMaxMembers"
        type="number"
        label={t('orgAdmin.settings.maxMembersLabel')}
        description={t('orgAdmin.settings.maxMembersDesc')}
        value={inputs.OrganizationDefaultMaxMembers}
        onChange={(v) => setField('OrganizationDefaultMaxMembers', v)}
        disabled={loading}
        error={intError('OrganizationDefaultMaxMembers') ? t('orgAdmin.settings.intError') : ''}
      />
      <SaveButton
        loading={loading}
        disabled={!isDirty(['OrganizationMaxPerUser', 'OrganizationDefaultMaxMembers'])}
        onClick={() =>
          saveKeys(['OrganizationMaxPerUser', 'OrganizationDefaultMaxMembers'], {
            validate: (vals) =>
              ['OrganizationMaxPerUser', 'OrganizationDefaultMaxMembers'].some((k) => {
                const n = Number(vals[k]);
                return vals[k] !== '' && vals[k] !== undefined && (!Number.isInteger(n) || n < 0);
              })
                ? t('orgAdmin.settings.intError')
                : null
          })
        }
      >
        {t('common.save')}
      </SaveButton>
    </SettingsCard>
  );
}
