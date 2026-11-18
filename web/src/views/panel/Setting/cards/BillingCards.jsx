import { useTranslation } from 'react-i18next';

import PropTypes from 'prop-types';

import QuotaInput from '@/components/QuotaInput';
import { FormField } from '@/components/ui/form-field';
import { OptionList } from '@/components/settings/cards';
import { settingDescription, settingEnumLabel } from '@/components/settings/catalog';
import { ModifiedDot } from '@/components/settings/SettingHelp';
import { SelectRow, TextRow, ToggleRow, useSettingCopy } from '../parts';
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

// A quota option (stored in quota units, typed in dollars or quota) with its text from the settings
// catalog: the label, the first sentence under the field and the rest in the help of its `?`.
function QuotaRow({ ctx, id }) {
  const copyText = useSettingCopy(id);
  return (
    <FormField
      id={id}
      data-setting-key={id}
      data-highlight={id}
      label={
        <>
          {copyText.label}
          {copyText.modified && <ModifiedDot defaultValue={copyText.modified.defaultValue} />}
        </>
      }
      help={copyText.help}
    >
      <QuotaInput id={id} name={id} value={ctx.inputs[id]} onChange={quotaSetter(ctx.setField, id)} helperText={copyText.description} />
    </FormField>
  );
}

QuotaRow.propTypes = { ctx: PropTypes.object.isRequired, id: PropTypes.string.isRequired };

const enumOptions = (t, key, values) => values.map((value) => ({ value, label: settingEnumLabel(t, key, value) }));

export function QuotaCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const ms = (k) => t(`setting_index.operationSettings.monitoringSettings.${k}`);
  const remindEnabled = inputs.QuotaRemindEnabled !== 'false';

  return (
    <OptionCard ctx={ctx} id="quota">
      <div className="grid gap-5 sm:grid-cols-2">
        <QuotaRow ctx={ctx} id="QuotaForNewUser" />
        <QuotaRow ctx={ctx} id="PreConsumedQuota" />
        <TextRow
          id="PreConsumedOutputTokens"
          type="number"
          value={inputs.PreConsumedOutputTokens}
          onChange={(v) => setField('PreConsumedOutputTokens', v)}
        />
      </div>
      <OptionList>
        <ToggleRow id="QuotaRemindEnabled" checked={remindEnabled} onCheckedChange={(v) => ctx.setBool('QuotaRemindEnabled', v)}>
          <TextRow
            id="QuotaRemindThreshold"
            type="number"
            value={inputs.QuotaRemindThreshold}
            onChange={(v) => setField('QuotaRemindThreshold', v)}
            placeholder={ms('quotaRemindThreshold.placeholder')}
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

  return (
    <OptionCard ctx={ctx} id="billing-rules">
      <OptionList>
        <ToggleRow id="EmptyResponseBillingEnabled" {...bindSwitch(ctx, 'EmptyResponseBillingEnabled')} />
        <ToggleRow id="ApproximateTokenEnabled" {...bindSwitch(ctx, 'ApproximateTokenEnabled')} />
      </OptionList>
      <SelectRow
        id="PriceSyncAutoApply"
        value={inputs.PriceSyncAutoApply || 'increases'}
        onChange={(v) => setField('PriceSyncAutoApply', v)}
        options={enumOptions(t, 'PriceSyncAutoApply', ['increases', 'all', 'none'])}
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
          value={tz}
          onChange={(v) => setField('QuotaResetTimezone', v)}
          options={tzOptions.map((value) => ({ value, label: tzLabel(value) }))}
        />
        <SelectRow
          id="QuotaResetWeekStart"
          value={weekStart}
          onChange={(v) => setField('QuotaResetWeekStart', v)}
          options={enumOptions(t, 'QuotaResetWeekStart', ['monday', 'sunday'])}
        />
      </div>
    </OptionCard>
  );
}

export function InviteRewardCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  // The header switch is InviteRewardEnabled: its catalog text describes the card.
  const description = settingDescription(t, 'InviteRewardEnabled');

  return (
    <OptionCard ctx={ctx} id="invite-reward" description={description} enableKey="InviteRewardEnabled">
      <div className="grid gap-5 sm:grid-cols-2">
        <QuotaRow ctx={ctx} id="QuotaForInviter" />
        <QuotaRow ctx={ctx} id="QuotaForInvitee" />
        <SelectRow
          id="InviterRewardType"
          value={inputs.InviterRewardType || 'fixed'}
          onChange={(v) => setField('InviterRewardType', v)}
          options={enumOptions(t, 'InviterRewardType', ['fixed', 'percentage'])}
        />
        <TextRow
          id="InviterRewardValue"
          type="number"
          value={inputs.InviterRewardValue}
          onChange={(v) => setField('InviterRewardValue', v)}
        />
      </div>
    </OptionCard>
  );
}
