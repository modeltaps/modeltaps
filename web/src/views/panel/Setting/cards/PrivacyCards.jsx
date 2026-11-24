import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { ActionRow, OptionList } from '@/components/settings/cards';
import { settingDescription, settingEnumLabel } from '@/components/settings/catalog';
import { Emphasis } from '@/components/settings/SettingHelp';
import { SelectRow, TextRow, ToggleRow } from '../parts';
import OptionCard, { bindSwitch } from './OptionCard';

// ==============================|| PLATFORM SETTINGS — 日志与隐私 ||============================== //
// Consume logs (with the purge action), request / response retention and content safety.

export function LogConsumeCard({ ctx }) {
  const { t, i18n } = useTranslation();
  const { inputs, setField } = ctx;
  const ls = (k, opts) => t(`setting_index.operationSettings.logSettings.${k}`, opts);
  const [logTime, setLogTime] = useState('');
  const on = (key) => inputs[key] === 'true';

  // Localized echo of the cutoff: the native datetime picker follows the browser locale.
  const intlLocale = (i18n.language || 'zh_CN').replace('_', '-');
  const logTimeText = logTime
    ? new Intl.DateTimeFormat(intlLocale, { dateStyle: 'long', timeStyle: 'short' }).format(new Date(logTime))
    : '';

  const clearLogs = async () => {
    if (!logTime) {
      showError(ls('selectCleanupTime'));
      return false;
    }
    const ts = Math.floor(new Date(logTime).getTime() / 1000);
    const res = await API.delete(`/api/log/?target_timestamp=${ts}`);
    const { success, message } = res.data;
    // The cleanup runs in background batches; the server answers with a localized notice that it
    // started or that another cleanup is still running.
    if (success) showSuccess(message);
    else showError(ls('cleanupFailed') + message);
    return success;
  };

  return (
    <OptionCard
      ctx={ctx}
      id="log-consume"
      actions={
        <ActionRow
          title={ls('clearLogs')}
          description={t('settingsUi.platform.actions.purgeDescription')}
          actionLabel={t('settingsUi.platform.actions.purge')}
          destructive
          onAction={clearLogs}
          confirm={{
            description: logTime ? ls('confirmDescription', { time: logTimeText }) : ls('logCleanupTime.description'),
            disabled: !logTime,
            onOpen: () => setLogTime(''),
            body: (
              <FormField
                id="logTime"
                label={ls('logCleanupTime.label')}
                hint={logTime ? ls('selectedTime', { time: logTimeText }) : undefined}
              >
                <Input id="logTime" type="datetime-local" value={logTime} onChange={(e) => setLogTime(e.target.value)} />
              </FormField>
            )
          }}
        />
      }
    >
      <OptionList>
        <ToggleRow id="LogConsumeEnabled" {...bindSwitch(ctx, 'LogConsumeEnabled')} />
        <ToggleRow id="LogRelayErrorEnabled" {...bindSwitch(ctx, 'LogRelayErrorEnabled')} />
        <ToggleRow id="ChannelMetricsEnabled" {...bindSwitch(ctx, 'ChannelMetricsEnabled')}>
          <TextRow
            id="ChannelMetricsRetentionDays"
            type="number"
            value={inputs.ChannelMetricsRetentionDays}
            onChange={(v) => setField('ChannelMetricsRetentionDays', v)}
            placeholder="30"
            disabled={!on('ChannelMetricsEnabled')}
          />
        </ToggleRow>
        <ToggleRow id="LogAutoDeleteEnabled" {...bindSwitch(ctx, 'LogAutoDeleteEnabled')}>
          <TextRow
            id="LogAutoDeleteDays"
            type="number"
            value={inputs.LogAutoDeleteDays}
            onChange={(v) => setField('LogAutoDeleteDays', v)}
            disabled={!on('LogAutoDeleteEnabled')}
          />
        </ToggleRow>
      </OptionList>
    </OptionCard>
  );
}

export function LogIOCard({ ctx }) {
  return (
    <OptionCard ctx={ctx} id="log-io">
      <OptionList>
        <ToggleRow id="LogIOEnabled" {...bindSwitch(ctx, 'LogIOEnabled')} />
      </OptionList>
    </OptionCard>
  );
}

export function SafetyCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const ss = (k) => t(`setting_index.operationSettings.safetySettings.${k}`);
  const [safeTools, setSafeTools] = useState([]);

  useEffect(() => {
    API.get('/api/option/safe_tools')
      .then((res) => {
        if (res.data?.success) setSafeTools(res.data.data || []);
      })
      .catch(() => {});
  }, []);

  return (
    <OptionCard
      ctx={ctx}
      id="safety"
      title={ss('title')}
      description={<Emphasis text={settingDescription(t, 'EnableSafe')} />}
      enableKey="EnableSafe"
    >
      <SelectRow
        id="SafeToolName"
        value={inputs.SafeToolName || ''}
        onChange={(v) => setField('SafeToolName', v)}
        options={safeTools.map((tool) => ({ value: tool, label: settingEnumLabel(t, 'SafeToolName', tool) }))}
        className="sm:max-w-sm"
      />
      <TextRow
        id="SafeKeyWords"
        value={Array.isArray(inputs.SafeKeyWords) ? inputs.SafeKeyWords.join('\n') : inputs.SafeKeyWords}
        onChange={(v) => setField('SafeKeyWords', v)}
        placeholder={ss('safeKeyWords.placeholder')}
        multiline
      />
    </OptionCard>
  );
}
