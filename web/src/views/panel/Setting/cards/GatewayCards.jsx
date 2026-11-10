import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';

import { verifyJSON } from 'utils/common';
import { FieldSection, OptionList } from '@/components/settings/cards';
import { SelectRow, TextRow, ToggleRow } from '../parts';
import OptionCard, { bindSwitch, cardText } from './OptionCard';

// ==============================|| PLATFORM SETTINGS — 网关 ||============================== //
// Retry, error handling, channel auto disable / recover, session affinity, model name matching,
// native protocols, prompt limits, image proxy and the auto routing billing switch.

const gsKey = (k) => `setting_index.operationSettings.generalSettings.${k}`;

export function RetryCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const gs = (k) => t(gsKey(k));
  const num = (key, name) => (
    <TextRow
      id={key}
      label={gs(`${name}.label`)}
      value={inputs[key]}
      onChange={(v) => setField(key, v)}
      placeholder={gs(`${name}.placeholder`)}
      description={gs(`${name}.description`)}
    />
  );

  return (
    <OptionCard ctx={ctx} id="retry">
      <div className="grid gap-5 sm:grid-cols-3">
        {num('RetryTimes', 'retryTimes')}
        {num('RetryCooldownSeconds', 'retryCooldownSeconds')}
        {num('RetryTimeOut', 'retryTimeOut')}
      </div>
      <TextRow
        id="RetryCooldownPerStatus"
        label={gs('retryCooldownPerStatus.title')}
        value={inputs.RetryCooldownPerStatus}
        onChange={(v) => setField('RetryCooldownPerStatus', v)}
        description={gs('retryCooldownPerStatus.description')}
        placeholder='{"503": 120}'
        multiline
        rows={3}
      />
    </OptionCard>
  );
}

export function ErrorHandlingCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const gs = (k) => t(gsKey(k));
  const wrap = inputs.ChannelFailErrorWrapEnabled === 'true';

  return (
    <OptionCard ctx={ctx} id="error-handling">
      <OptionList>
        <ToggleRow
          id="ChannelFailErrorWrapEnabled"
          label={gs('channelFailErrorWrapEnabled')}
          description={gs('channelFailErrorWrapEnabledTooltip')}
          {...bindSwitch(ctx, 'ChannelFailErrorWrapEnabled')}
        />
      </OptionList>
      <TextRow
        id="ChannelFailErrorMessage"
        label={gs('channelFailErrorMessage.label')}
        value={inputs.ChannelFailErrorMessage}
        onChange={(v) => setField('ChannelFailErrorMessage', v)}
        placeholder={gs('channelFailErrorMessage.placeholder')}
        disabled={!wrap}
      />
    </OptionCard>
  );
}

export function ChannelAutoCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const ms = (k) => t(`setting_index.operationSettings.monitoringSettings.${k}`);
  const row = (key, name) => <ToggleRow id={key} label={ms(name)} description={ms(`${name}Description`)} {...bindSwitch(ctx, key)} />;

  return (
    <OptionCard ctx={ctx} id="channel-auto">
      <OptionList>
        {row('AutomaticDisableChannelEnabled', 'automaticDisableChannel')}
        {row('AutomaticEnableChannelEnabled', 'automaticEnableChannel')}
        {row('AutomaticDisableChannelNotifyEnabled', 'automaticDisableChannelNotify')}
      </OptionList>
      <div className="grid gap-5 sm:grid-cols-2">
        <SelectRow
          id="ChannelTestMode"
          label={ms('channelTestMode.label')}
          description={ms('channelTestMode.description')}
          value={inputs.ChannelTestMode || 'recovery'}
          onChange={(v) => setField('ChannelTestMode', v)}
          options={['recovery', 'all', 'off'].map((value) => ({ value, label: ms(`channelTestMode.${value}`) }))}
        />
        <TextRow
          id="ChannelTestIntervalMinutes"
          type="number"
          label={ms('channelTestInterval.label')}
          value={inputs.ChannelTestIntervalMinutes}
          onChange={(v) => setField('ChannelTestIntervalMinutes', v)}
          placeholder="10"
          description={ms('channelTestInterval.description')}
          disabled={inputs.ChannelTestMode === 'off'}
        />
        <TextRow
          id="ChannelCredentialRefreshMinutes"
          type="number"
          label={ms('credentialRefresh.label')}
          value={inputs.ChannelCredentialRefreshMinutes}
          onChange={(v) => setField('ChannelCredentialRefreshMinutes', v)}
          placeholder="30"
          description={ms('credentialRefresh.description')}
        />
        <TextRow
          id="ChannelDisableThreshold"
          type="number"
          label={ms('channelDisableThreshold.label')}
          value={inputs.ChannelDisableThreshold}
          onChange={(v) => setField('ChannelDisableThreshold', v)}
          placeholder={ms('channelDisableThreshold.placeholder')}
          description={ms('channelDisableThreshold.description')}
        />
      </div>
      <TextRow
        id="DisableChannelKeywords"
        label={t('setting_index.operationSettings.disableChannelKeywordsSettings.label')}
        description={t('setting_index.operationSettings.disableChannelKeywordsSettings.description')}
        value={inputs.DisableChannelKeywords}
        onChange={(v) => setField('DisableChannelKeywords', v)}
        placeholder={t('setting_index.operationSettings.disableChannelKeywordsSettings.info')}
        multiline
      />
    </OptionCard>
  );
}

export function SessionAffinityCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const gs = (k) => t(gsKey(k));

  return (
    <OptionCard ctx={ctx} id="session-affinity">
      <OptionList>
        <ToggleRow
          id="SessionAffinityEnabled"
          label={gs('sessionAffinityEnabled')}
          description={gs('sessionAffinityEnabledDescription')}
          {...bindSwitch(ctx, 'SessionAffinityEnabled')}
        />
        <ToggleRow
          id="SessionAffinityStrict"
          label={gs('sessionAffinityStrict')}
          description={gs('sessionAffinityStrictDescription')}
          {...bindSwitch(ctx, 'SessionAffinityStrict')}
        />
      </OptionList>
      <TextRow
        id="SessionAffinityTTLSeconds"
        type="number"
        label={gs('sessionAffinityTTL.label')}
        value={inputs.SessionAffinityTTLSeconds}
        onChange={(v) => setField('SessionAffinityTTLSeconds', v)}
        placeholder={gs('sessionAffinityTTL.placeholder')}
        description={gs('sessionAffinityTTL.description')}
      />
    </OptionCard>
  );
}

export function ModelNameMatchCard({ ctx }) {
  const { t } = useTranslation();
  const gs = (k) => t(gsKey(k));
  return (
    <OptionCard ctx={ctx} id="model-name-match">
      <OptionList>
        <ToggleRow
          id="ModelNameCaseInsensitiveEnabled"
          label={gs('modelNameCaseInsensitive')}
          description={gs('modelNameCaseInsensitiveDescription')}
          {...bindSwitch(ctx, 'ModelNameCaseInsensitiveEnabled')}
        />
        <ToggleRow
          id="UnifiedRequestResponseModelEnabled"
          label={gs('unifiedRequestResponseModel')}
          description={gs('unifiedRequestResponseModelDescription')}
          {...bindSwitch(ctx, 'UnifiedRequestResponseModelEnabled')}
        />
      </OptionList>
    </OptionCard>
  );
}

export function NativeProtocolCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const op = (k) => t(`setting_index.operationSettings.${k}`);
  const row = (key, name) => (
    <ToggleRow
      id={key}
      label={op(`otherSettings.${name}`)}
      description={op(`otherSettings.${name}Description`)}
      {...bindSwitch(ctx, key)}
    />
  );
  const validate = (i) => {
    if (i.ClaudeDefaultMaxTokens && !verifyJSON(i.ClaudeDefaultMaxTokens)) return op('claudeSettings.invalidJson');
    if (i.GeminiOpenThink && !verifyJSON(i.GeminiOpenThink)) return op('geminiSettings.invalidJson');
    return null;
  };

  return (
    <OptionCard ctx={ctx} id="native-protocol" options={{ validate }}>
      <OptionList>
        {row('ClaudeAPIEnabled', 'claudeAPIEnabled')}
        {row('GeminiAPIEnabled', 'geminiAPIEnabled')}
        {row('ClaudePromptCachingEnabled', 'claudePromptCachingEnabled')}
        {row('FingerprintPassThroughEnabled', 'fingerprintPassThroughEnabled')}
      </OptionList>
      <FieldSection title={op('claudeSettings.title')}>
        <TextRow
          id="ClaudeBudgetTokensPercentage"
          type="number"
          label={op('claudeSettings.budgetTokensPercentage.label')}
          value={inputs.ClaudeBudgetTokensPercentage}
          onChange={(v) => setField('ClaudeBudgetTokensPercentage', v)}
          placeholder={op('claudeSettings.budgetTokensPercentage.placeholder')}
          description={op('claudeSettings.budgetTokensPercentage.description')}
        />
        <TextRow
          id="ClaudeDefaultMaxTokens"
          label={op('claudeSettings.defaultMaxTokens.label')}
          value={inputs.ClaudeDefaultMaxTokens}
          onChange={(v) => setField('ClaudeDefaultMaxTokens', v)}
          placeholder={op('claudeSettings.defaultMaxTokens.placeholder')}
          description={op('claudeSettings.defaultMaxTokens.description')}
          multiline
        />
      </FieldSection>
      <FieldSection title={op('geminiSettings.title')}>
        <TextRow
          id="GeminiOpenThink"
          label={op('geminiSettings.geminiOpenThink.label')}
          value={inputs.GeminiOpenThink}
          onChange={(v) => setField('GeminiOpenThink', v)}
          placeholder={op('geminiSettings.geminiOpenThink.placeholder')}
          description={op('geminiSettings.geminiOpenThink.description')}
          multiline
        />
      </FieldSection>
    </OptionCard>
  );
}

export function PromptLimitsCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const op = (k) => t(`setting_index.operationSettings.promptLimits.${k}`);
  return (
    <OptionCard ctx={ctx} id="prompt-limits">
      <TextRow
        id="MaxPromptTokens"
        type="number"
        label={op('maxPromptTokens.label')}
        value={inputs.MaxPromptTokens}
        onChange={(v) => setField('MaxPromptTokens', v)}
        description={op('maxPromptTokens.description')}
      />
    </OptionCard>
  );
}

export function ImageProxyCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, secrets, setField } = ctx;
  const op = (k) => t(`setting_index.operationSettings.otherSettings.${k}`);

  return (
    <OptionCard ctx={ctx} id="image-proxy">
      <TextRow
        id="ChatImageRequestProxy"
        label={op('chatImageRequestProxy.label')}
        value={inputs.ChatImageRequestProxy}
        onChange={(v) => setField('ChatImageRequestProxy', v)}
        placeholder={op('chatImageRequestProxy.placeholder')}
        description={op('chatImageRequestProxy.description')}
      />
      <div className="grid gap-5 sm:grid-cols-2">
        <TextRow
          id="CFWorkerImageUrl"
          label={op('CFWorkerImageUrl.label')}
          value={inputs.CFWorkerImageUrl}
          onChange={(v) => setField('CFWorkerImageUrl', v)}
          description={op('CFWorkerImageUrl.description')}
        />
        <TextRow
          id="CFWorkerImageKey"
          label={op('CFWorkerImageUrl.key')}
          description={op('CFWorkerImageUrl.keyDescription')}
          value={inputs.CFWorkerImageKey}
          onChange={(v) => setField('CFWorkerImageKey', v)}
          secret={secrets?.CFWorkerImageKey}
        />
      </div>
    </OptionCard>
  );
}

// Policies live on the auto routing page; this card keeps the billing switch for decision calls.
export function AutoRoutingCard({ ctx }) {
  const { t } = useTranslation();
  const gs = (k) => t(gsKey(k));
  return (
    <OptionCard
      ctx={ctx}
      id="auto-routing"
      description={
        <>
          {cardText(t, 'auto-routing', 'description')}{' '}
          <Link to="/panel/routing_policy" className="text-primary hover:underline">
            {t('settingsUi.platform.actions.openRouting')}
          </Link>
        </>
      }
    >
      <OptionList>
        <ToggleRow
          id="RoutingDecisionBillingEnabled"
          label={gs('routingDecisionBilling')}
          description={gs('routingDecisionBillingDescription')}
          {...bindSwitch(ctx, 'RoutingDecisionBillingEnabled')}
        />
      </OptionList>
    </OptionCard>
  );
}
