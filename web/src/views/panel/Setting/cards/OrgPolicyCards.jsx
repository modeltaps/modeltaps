import { useTranslation } from 'react-i18next';

import { OptionList } from '@/components/settings/cards';
import { TextRow, ToggleRow } from '../parts';
import OptionCard, { bindSwitch } from './OptionCard';

// ==============================|| PLATFORM SETTINGS — 组织策略 (root) ||============================== //
// Site-level organization options: whether organizations are open, the per-user organization limit,
// the default member limit and quota transfer.

const intOrEmpty = (v) => {
  if (v === undefined || v === '') return true;
  const n = Number(v);
  return Number.isInteger(n) && n >= 0;
};

export function OrgPolicyCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const os = (k) => t(`orgAdmin.settings.${k}`);
  const intKeys = ['OrganizationMaxPerUser', 'OrganizationDefaultMaxMembers'];
  const validate = (vals) => (intKeys.every((k) => intOrEmpty(vals[k])) ? null : os('intError'));

  return (
    <OptionCard ctx={ctx} id="org-policy" description={os('description')} options={{ validate }}>
      <OptionList>
        <ToggleRow
          id="OrganizationEnabled"
          label={os('enabledLabel')}
          description={os('enabledDesc')}
          {...bindSwitch(ctx, 'OrganizationEnabled')}
        />
        <ToggleRow
          id="OrganizationQuotaTransferEnabled"
          label={os('transferLabel')}
          description={os('transferDesc')}
          {...bindSwitch(ctx, 'OrganizationQuotaTransferEnabled')}
        />
      </OptionList>
      <div className="grid gap-5 sm:grid-cols-2">
        <TextRow
          id="OrganizationMaxPerUser"
          type="number"
          label={os('maxPerUserLabel')}
          description={os('maxPerUserDesc')}
          value={inputs.OrganizationMaxPerUser}
          onChange={(v) => setField('OrganizationMaxPerUser', v)}
          error={intOrEmpty(inputs.OrganizationMaxPerUser) ? '' : os('intError')}
        />
        <TextRow
          id="OrganizationDefaultMaxMembers"
          type="number"
          label={os('maxMembersLabel')}
          description={os('maxMembersDesc')}
          value={inputs.OrganizationDefaultMaxMembers}
          onChange={(v) => setField('OrganizationDefaultMaxMembers', v)}
          error={intOrEmpty(inputs.OrganizationDefaultMaxMembers) ? '' : os('intError')}
        />
      </div>
    </OptionCard>
  );
}
