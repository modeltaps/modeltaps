import { useTranslation } from 'react-i18next';

import QuotaInput from '@/components/QuotaInput';
import { OptionList } from '@/components/settings/cards';
import { SelectRow, TextRow, ToggleRow } from '../parts';
import OptionCard, { bindSwitch } from './OptionCard';

// ==============================|| PLATFORM SETTINGS — 计费 ||============================== //
// Quota, billing rules, periodic limit reset and invite rewards.

// Common IANA time zones for the periodic limit reset; UTC is the default.
const QUOTA_RESET_TIMEZONES = [
  'UTC',
  'Asia/Shanghai',
  'Asia/Hong_Kong',
  'Asia/Tokyo',
  'Asia/Singapore',
  'Asia/Kolkata',
  'Asia/Dubai',
  'Europe/London',
  'Europe/Paris',
  'Europe/Moscow',
  'America/New_York',
  'America/Chicago',
  'America/Los_Angeles',
  'America/Sao_Paulo',
  'Australia/Sydney',
  'Pacific/Auckland'
];

// Time zone label with the current GMT offset; falls back to the IANA name.
function tzLabel(tz) {
  if (tz === 'UTC') return 'UTC';
  try {
    const parts = new Intl.DateTimeFormat('en-US', { timeZone: tz, timeZoneName: 'shortOffset' }).formatToParts(new Date());
    const off = parts.find((p) => p.type === 'timeZoneName')?.value;
    return off ? `${tz} (${off})` : tz;
  } catch {
    return tz;
  }
}

// Origin values are strings and QuotaInput reports numbers; normalise so the dirty check holds.
const quotaSetter = (setField, key) => (v) => setField(key, typeof v === 'number' ? String(v) : v);

export function QuotaCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const qs = (k) => t(`setting_index.operationSettings.quotaSettings.${k}`);
  const ms = (k) => t(`setting_index.operationSettings.monitoringSettings.${k}`);
  const remindEnabled = inputs.QuotaRemindEnabled !== 'false';

  return (
    <OptionCard ctx={ctx} id="quota">
      <div className="grid gap-5 sm:grid-cols-2">
        <QuotaInput
          id="QuotaForNewUser"
          name="QuotaForNewUser"
          label={qs('quotaForNewUser.label')}
          value={inputs.QuotaForNewUser}
          onChange={quotaSetter(setField, 'QuotaForNewUser')}
          helperText={qs('quotaForNewUser.description')}
        />
        <QuotaInput
          id="PreConsumedQuota"
          name="PreConsumedQuota"
          label={qs('preConsumedQuota.label')}
          value={inputs.PreConsumedQuota}
          onChange={quotaSetter(setField, 'PreConsumedQuota')}
          helperText={qs('preConsumedQuota.description')}
        />
        <TextRow
          id="PreConsumedOutputTokens"
          type="number"
          label={qs('preConsumedOutputTokens.label')}
          value={inputs.PreConsumedOutputTokens}
          onChange={(v) => setField('PreConsumedOutputTokens', v)}
          description={qs('preConsumedOutputTokens.description')}
        />
      </div>
      <OptionList>
        <ToggleRow
          id="QuotaRemindEnabled"
          label={ms('quotaRemindEnabled.label')}
          description={ms('quotaRemindEnabled.description')}
          checked={remindEnabled}
          onCheckedChange={(v) => ctx.setBool('QuotaRemindEnabled', v)}
        >
          <TextRow
            id="QuotaRemindThreshold"
            type="number"
            label={ms('quotaRemindThreshold.label')}
            value={inputs.QuotaRemindThreshold}
            onChange={(v) => setField('QuotaRemindThreshold', v)}
            placeholder={ms('quotaRemindThreshold.placeholder')}
            description={ms('quotaRemindThreshold.description')}
            disabled={!remindEnabled}
          />
        </ToggleRow>
      </OptionList>
    </OptionCard>
  );
}

export function BillingRulesCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const gs = (k) => t(`setting_index.operationSettings.generalSettings.${k}`);
  const ps = (k) => gs(`priceSyncAutoApply.${k}`);

  return (
    <OptionCard ctx={ctx} id="billing-rules">
      <OptionList>
        <ToggleRow
          id="EmptyResponseBillingEnabled"
          label={gs('emptyResponseBilling')}
          description={gs('emptyResponseBillingDescription')}
          {...bindSwitch(ctx, 'EmptyResponseBillingEnabled')}
        />
        <ToggleRow
          id="ApproximateTokenEnabled"
          label={gs('approximateToken')}
          description={gs('approximateTokenDescription')}
          {...bindSwitch(ctx, 'ApproximateTokenEnabled')}
        />
      </OptionList>
      <SelectRow
        id="PriceSyncAutoApply"
        label={ps('label')}
        description={ps('description')}
        value={inputs.PriceSyncAutoApply || 'increases'}
        onChange={(v) => setField('PriceSyncAutoApply', v)}
        options={['increases', 'all', 'none'].map((value) => ({ value, label: ps(value) }))}
        className="sm:max-w-sm"
      />
    </OptionCard>
  );
}

export function QuotaResetCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const rs = (k) => t(`setting_index.operationSettings.quotaResetSettings.${k}`);
  // A missing key falls back to UTC / Monday; a custom time zone value stays in the list.
  const tz = inputs.QuotaResetTimezone || 'UTC';
  const weekStart = inputs.QuotaResetWeekStart === 'sunday' ? 'sunday' : 'monday';
  const tzOptions = QUOTA_RESET_TIMEZONES.includes(tz) ? QUOTA_RESET_TIMEZONES : [tz, ...QUOTA_RESET_TIMEZONES];

  return (
    <OptionCard ctx={ctx} id="quota-reset" description={rs('description')}>
      <div className="grid gap-5 sm:grid-cols-2">
        <SelectRow
          id="QuotaResetTimezone"
          label={rs('timezone.label')}
          description={rs('timezone.description')}
          value={tz}
          onChange={(v) => setField('QuotaResetTimezone', v)}
          options={tzOptions.map((value) => ({ value, label: tzLabel(value) }))}
        />
        <SelectRow
          id="QuotaResetWeekStart"
          label={rs('weekStart.label')}
          description={rs('weekStart.description')}
          value={weekStart}
          onChange={(v) => setField('QuotaResetWeekStart', v)}
          options={['monday', 'sunday'].map((value) => ({ value, label: rs(`weekStart.${value}`) }))}
        />
      </div>
    </OptionCard>
  );
}

export function InviteRewardCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const qs = (k) => t(`setting_index.operationSettings.quotaSettings.${k}`);

  return (
    <OptionCard ctx={ctx} id="invite-reward" description={qs('inviteRewardEnabled.description')} enableKey="InviteRewardEnabled">
      <div className="grid gap-5 sm:grid-cols-2">
        <QuotaInput
          id="QuotaForInviter"
          name="QuotaForInviter"
          label={qs('quotaForInviter.label')}
          value={inputs.QuotaForInviter}
          onChange={quotaSetter(setField, 'QuotaForInviter')}
          helperText={qs('quotaForInviter.description')}
        />
        <QuotaInput
          id="QuotaForInvitee"
          name="QuotaForInvitee"
          label={qs('quotaForInvitee.label')}
          value={inputs.QuotaForInvitee}
          onChange={quotaSetter(setField, 'QuotaForInvitee')}
          helperText={qs('quotaForInvitee.description')}
        />
        <SelectRow
          id="InviterRewardType"
          label={qs('rechargeRewardType.label')}
          value={inputs.InviterRewardType || 'fixed'}
          onChange={(v) => setField('InviterRewardType', v)}
          options={['fixed', 'percentage'].map((value) => ({ value, label: qs(`rechargeRewardType.${value}`) }))}
        />
        <TextRow
          id="InviterRewardValue"
          type="number"
          label={qs('rechargeRewardValue.label')}
          value={inputs.InviterRewardValue}
          onChange={(v) => setField('InviterRewardValue', v)}
          description={qs('rechargeRewardValue.description')}
        />
      </div>
    </OptionCard>
  );
}
