import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { ChevronRight, Plus, X } from 'lucide-react';

import { API } from 'utils/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { durationLabel } from 'views/panel/ModelInfo/supply/supplyParts';

import { verifyJSON } from 'utils/common';
import { FieldSection, OptionList } from '@/components/settings/cards';
import { SelectRow, TextRow, ToggleRow } from '../parts';
import OptionCard, { bindSwitch, cardText } from './OptionCard';
import { describePauseRule, pauseRulesError, pauseRulesText } from './pauseRules';

// ==============================|| PLATFORM SETTINGS — 网关 ||============================== //
// Retry, error handling, channel auto disable / recover, session affinity, model name matching,
// native protocols, prompt limits, image proxy and the auto routing options.

const gsKey = (k) => `setting_index.operationSettings.generalSettings.${k}`;

// {"503": 120, "429": 30} ⇄ [{ code: '503', seconds: '120' }, …]; null when the text is not a
// JSON object, so the editor can fall back to plain text instead of dropping it.
export function parseStatusCooldown(text) {
  if (!text || !String(text).trim()) return [];
  try {
    const obj = JSON.parse(text);
    if (!obj || typeof obj !== 'object' || Array.isArray(obj)) return null;
    return Object.entries(obj).map(([code, seconds]) => ({ code: String(code), seconds: String(seconds) }));
  } catch {
    return null;
  }
}

export function serializeStatusCooldown(rows) {
  const obj = {};
  rows.forEach(({ code, seconds }) => {
    const c = String(code).trim();
    if (!c) return;
    const n = Number(seconds);
    obj[c] = Number.isFinite(n) ? n : 0;
  });
  return Object.keys(obj).length ? JSON.stringify(obj) : '';
}

// One row per status code with its cooldown in seconds; add and remove rows. Rows are kept
// locally while editing (so a half-typed code is not lost) and written back as JSON.
function StatusCooldownEditor({ id, label, description, value, onChange }) {
  const { t } = useTranslation();
  const parsed = parseStatusCooldown(value);
  const [rows, setRows] = useState(() => parsed || []);
  const [lastWritten, setLastWritten] = useState(value);
  // The card was reset or loaded from the server: take the stored value again.
  if (value !== lastWritten && parsed) {
    setRows(parsed);
    setLastWritten(value);
  }
  const write = (next) => {
    setRows(next);
    const json = serializeStatusCooldown(next);
    setLastWritten(json);
    onChange(json);
  };
  if (parsed === null && value === lastWritten && rows.length === 0) {
    return (
      <div className="space-y-2">
        <p className="text-xs text-warning">{t('settingsFields.invalidJson')}</p>
        <TextRow id={id} label={label} value={value} onChange={onChange} description={description} multiline rows={3} />
      </div>
    );
  }
  return (
    <div className="space-y-2">
      <div>
        <div className="text-sm font-medium">{label}</div>
        {description && <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>}
      </div>
      {rows.length > 0 && (
        <div className="space-y-2">
          {rows.map((row, i) => (
            <div key={i} className="flex items-center gap-2">
              <Input
                aria-label={t('settingsFields.statusCode')}
                placeholder="503"
                inputMode="numeric"
                value={row.code}
                onChange={(e) => write(rows.map((r, j) => (j === i ? { ...r, code: e.target.value } : r)))}
                className="h-9 w-28 font-mono"
              />
              <div className="relative w-36">
                <Input
                  aria-label={t('settingsFields.seconds')}
                  placeholder="120"
                  inputMode="numeric"
                  value={row.seconds}
                  onChange={(e) => write(rows.map((r, j) => (j === i ? { ...r, seconds: e.target.value } : r)))}
                  className="h-9 pr-10 tabular-nums"
                />
                <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">
                  {t('settingsUnits.seconds')}
                </span>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-8 text-muted-foreground"
                aria-label={t('settingsFields.remove')}
                onClick={() => write(rows.filter((_, j) => j !== i))}
              >
                <X className="size-4" />
              </Button>
            </div>
          ))}
        </div>
      )}
      <Button type="button" variant="outline" size="sm" onClick={() => setRows([...rows, { code: '', seconds: '' }])}>
        <Plus className="size-4" />
        {t('settingsFields.addStatus')}
      </Button>
    </div>
  );
}

StatusCooldownEditor.propTypes = {
  id: PropTypes.string.isRequired,
  label: PropTypes.string,
  description: PropTypes.string,
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired
};

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
      <StatusCooldownEditor
        id="RetryCooldownPerStatus"
        label={gs('retryCooldownPerStatus.title')}
        value={inputs.RetryCooldownPerStatus}
        onChange={(v) => setField('RetryCooldownPerStatus', v)}
        description={gs('retryCooldownPerStatus.description')}
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

// Rules that pause a channel or model on upstream errors. An empty value means the built-in
// defaults; a custom list replaces them as a whole (first matching rule wins). The backend
// validates the rules and refuses invalid ones with its own message.
const PAUSE_RULES_EXAMPLE = `[
  {
    "name": "quota",
    "status_codes": [402],
    "keywords": ["credit balance is too low"],
    "action": "pause",
    "seconds": 1800,
    "scope": "channel",
    "probe": true,
    "notify": true,
    "category": "quota"
  }
]`;

// The built-in rules from the backend ({ defaults } of /api/supply_pause/rules), or [] when
// they cannot be read.
function useDefaultPauseRules() {
  const [defaults, setDefaults] = useState([]);
  useEffect(() => {
    let alive = true;
    API.get('/api/supply_pause/rules')
      .then((res) => {
        const { success, data } = res?.data || {};
        if (alive && success && Array.isArray(data?.defaults)) setDefaults(data.defaults);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);
  return defaults;
}

// Read-only list of rules in words: name, when it matches, what it does.
function PauseRuleList({ rules }) {
  const { t } = useTranslation();
  const phrase = ([key, params = {}]) =>
    t(`settingsUi.platform.pauseRules.summary.${key}`, {
      ...params,
      ...(params.seconds !== undefined && { duration: durationLabel(t, params.seconds) })
    });
  return (
    <ul className="divide-y divide-border overflow-hidden rounded-[10px] border border-border">
      {rules.map((rule, index) => {
        const { when, then } = describePauseRule(rule);
        return (
          <li key={`${rule.name}-${index}`} className="grid gap-x-4 gap-y-0.5 px-3 py-2 text-[13px] sm:grid-cols-[10rem_1fr]">
            <span className="truncate font-mono" title={rule.name}>
              {rule.name || '—'}
            </span>
            <span className="min-w-0 text-muted-foreground">
              {when.map(phrase).join(' · ')}
              <span aria-hidden> → </span>
              <span className="text-foreground">{then.map(phrase).join(' · ')}</span>
            </span>
          </li>
        );
      })}
    </ul>
  );
}

PauseRuleList.propTypes = { rules: PropTypes.array.isRequired };

export function SupplyPauseRulesCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const pr = (k) => t(`settingsUi.platform.pauseRules.${k}`);
  const error = pauseRulesError(inputs.PauseRules);
  const validate = (i) => (pauseRulesError(i.PauseRules) ? pr(pauseRulesError(i.PauseRules)) : null);
  const defaults = useDefaultPauseRules();
  const empty = !String(inputs.PauseRules ?? '').trim();
  // Only fills the editor; the footer saves it like any other edit.
  const fillDefaults = () => setField('PauseRules', pauseRulesText(defaults));

  return (
    <OptionCard ctx={ctx} id="supply-pause-rules" enableKey="PauseRulesEnabled" options={{ validate }}>
      <TextRow
        id="PauseRules"
        label={pr('label')}
        value={inputs.PauseRules}
        onChange={(v) => setField('PauseRules', v)}
        placeholder={PAUSE_RULES_EXAMPLE}
        description={pr('description')}
        error={error ? pr(error) : ''}
        multiline
        rows={10}
      />
      {defaults.length > 0 &&
        (empty ? (
          <div className="space-y-2">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-[13px] font-medium">{pr('builtIn')}</p>
              <Button type="button" variant="outline" size="sm" className="rounded-[10px]" onClick={fillDefaults}>
                {pr('useDefaults')}
              </Button>
            </div>
            <PauseRuleList rules={defaults} />
            <p className="text-xs text-muted-foreground">{pr('useDefaultsHint')}</p>
          </div>
        ) : (
          <details className="group space-y-2">
            <summary className="flex cursor-pointer list-none items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground [&::-webkit-details-marker]:hidden">
              <ChevronRight className="size-3.5 transition-transform group-open:rotate-90" />
              {pr('showBuiltIn')}
            </summary>
            <PauseRuleList rules={defaults} />
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-xs text-muted-foreground">{pr('replaceHint')}</p>
              <Button type="button" variant="outline" size="sm" className="rounded-[10px]" onClick={fillDefaults}>
                {pr('replaceWithDefaults')}
              </Button>
            </div>
          </details>
        ))}
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

// Policies live on the auto routing page; this card keeps the billing switch for decision calls
// and whether organizations may manage their own policies (组织 → 自动路由), with a cap per
// organization.
export function AutoRoutingCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
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
        <ToggleRow
          id="UserRoutingPolicyEnabled"
          label={gs('userRoutingPolicy')}
          description={gs('userRoutingPolicyDescription')}
          {...bindSwitch(ctx, 'UserRoutingPolicyEnabled')}
        />
      </OptionList>
      <TextRow
        id="PersonalRoutingPolicyMaxCount"
        type="number"
        label={gs('personalRoutingPolicyMaxCount')}
        value={inputs.PersonalRoutingPolicyMaxCount}
        onChange={(v) => setField('PersonalRoutingPolicyMaxCount', v)}
        placeholder="20"
        description={gs('personalRoutingPolicyMaxCountDescription')}
      />
      <TextRow
        id="OrganizationRoutingPolicyMaxCount"
        type="number"
        label={gs('orgRoutingPolicyMaxCount')}
        value={inputs.OrganizationRoutingPolicyMaxCount}
        onChange={(v) => setField('OrganizationRoutingPolicyMaxCount', v)}
        placeholder="20"
        description={gs('orgRoutingPolicyMaxCountDescription')}
      />
    </OptionCard>
  );
}
