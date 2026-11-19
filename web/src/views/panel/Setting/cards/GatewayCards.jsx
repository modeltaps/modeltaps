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
import { FormField } from '@/components/ui/form-field';
import { FieldSection, OptionList } from '@/components/settings/cards';
import { ModifiedDot } from '@/components/settings/SettingHelp';
import { SelectRow, TextRow, ToggleRow, useSettingCopy } from '../parts';
import { settingEnumLabel } from '@/components/settings/catalog';
import { SettingHelpLine } from '@/components/settings/SettingHelp';
import OptionCard, { bindSwitch, cardText } from './OptionCard';
import { describePauseRule, pauseRulesError, pauseRulesText } from './pauseRules';
import { admin } from 'routes/paths';
import IntakeRulesEditor, { IntakeRuleList, intakeRulesJSON, parseIntakeRules } from 'views/panel/ModelInfo/supply/IntakeRules';
import { fetchIntakeRules, previewSupplyMigration } from 'views/panel/ModelInfo/supply/supplyApi';

// ==============================|| PLATFORM SETTINGS — 网关 ||============================== //
// Retry, error handling, channel auto disable / recover, session affinity, model name matching,
// native protocols, prompt limits, image proxy and the auto routing options. Field labels, hints
// and help come from the settings catalog (common/settings/gateway*.go).

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
// locally while editing (so a half-typed code is not lost) and written back as JSON. The label,
// hint and help are the catalog's for `id`.
function StatusCooldownEditor({ id, value, onChange }) {
  const { t } = useTranslation();
  const copy = useSettingCopy(id);
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
        <TextRow id={id} value={value} onChange={onChange} multiline rows={3} />
      </div>
    );
  }
  return (
    <FormField
      id={id}
      data-setting-key={id}
      data-highlight={id}
      label={
        <>
          {copy.label}
          {copy.modified && <ModifiedDot defaultValue={copy.modified.defaultValue} />}
        </>
      }
      hint={copy.description}
      help={copy.help}
    >
      {rows.length > 0 && (
        <div className="space-y-2">
          {rows.map((row, i) => (
            <div key={i} className="flex items-center gap-2">
              <Input
                id={i === 0 ? id : undefined}
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
    </FormField>
  );
}

StatusCooldownEditor.propTypes = {
  id: PropTypes.string.isRequired,
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired
};

export function RetryCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const gs = (k) => t(gsKey(k));
  const num = (key, name) => (
    <TextRow id={key} value={inputs[key]} onChange={(v) => setField(key, v)} placeholder={gs(`${name}.placeholder`)} />
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
        value={inputs.RetryCooldownPerStatus}
        onChange={(v) => setField('RetryCooldownPerStatus', v)}
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
        <ToggleRow id="ChannelFailErrorWrapEnabled" {...bindSwitch(ctx, 'ChannelFailErrorWrapEnabled')} />
      </OptionList>
      <TextRow
        id="ChannelFailErrorMessage"
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
  const row = (key) => <ToggleRow id={key} {...bindSwitch(ctx, key)} />;

  return (
    <OptionCard ctx={ctx} id="channel-auto">
      <OptionList>
        {row('AutomaticDisableChannelEnabled')}
        {row('AutomaticEnableChannelEnabled')}
      </OptionList>
      <div className="grid gap-5 sm:grid-cols-2">
        <SelectRow
          id="ChannelTestMode"
          value={inputs.ChannelTestMode || 'recovery'}
          onChange={(v) => setField('ChannelTestMode', v)}
          options={['recovery', 'all', 'off'].map((value) => ({ value, label: settingEnumLabel(t, 'ChannelTestMode', value) }))}
        />
        <TextRow
          id="ChannelTestIntervalMinutes"
          type="number"
          value={inputs.ChannelTestIntervalMinutes}
          onChange={(v) => setField('ChannelTestIntervalMinutes', v)}
          placeholder="10"
          disabled={inputs.ChannelTestMode === 'off'}
        />
        <TextRow
          id="ChannelCredentialRefreshMinutes"
          type="number"
          value={inputs.ChannelCredentialRefreshMinutes}
          onChange={(v) => setField('ChannelCredentialRefreshMinutes', v)}
          placeholder="30"
        />
        <TextRow
          id="ChannelDisableThreshold"
          type="number"
          value={inputs.ChannelDisableThreshold}
          onChange={(v) => setField('ChannelDisableThreshold', v)}
          placeholder={ms('channelDisableThreshold.placeholder')}
        />
      </div>
      <TextRow
        id="DisableChannelKeywords"
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
    <OptionCard ctx={ctx} id="supply-pause-rules" options={{ validate }}>
      <TextRow
        id="PauseRules"
        value={inputs.PauseRules}
        onChange={(v) => setField('PauseRules', v)}
        placeholder={PAUSE_RULES_EXAMPLE}
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

// What happens to a model a channel's upstream starts listing (for channels that add new
// upstream models on their own). Empty means the built-in rules; editing them saves a copy that
// replaces them as a whole.
export function SupplyIntakeRulesCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const [defaults, setDefaults] = useState(null);
  // Bumped to reload the editor after "use the built-in rules".
  const [version, setVersion] = useState(0);
  useEffect(() => {
    let alive = true;
    fetchIntakeRules()
      .then((res) => alive && res.success && setDefaults(res.data?.defaults || []))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);
  const saved = parseIntakeRules(inputs.SupplyIntakeRules);
  const custom = saved !== null;
  const rules = saved || defaults;
  if (!rules) return <OptionCard ctx={ctx} id="supply-intake-rules" />;
  return (
    <OptionCard ctx={ctx} id="supply-intake-rules">
      <div className="space-y-3">
        <SettingHelpLine settingKey="SupplyIntakeRules" />
        <p className="text-[13px] text-muted-foreground">{t(custom ? 'intakeRules.customNote' : 'intakeRules.defaultNote')}</p>
        <IntakeRulesEditor
          key={`${version}-${custom}`}
          rules={rules}
          onChange={(next) => setField('SupplyIntakeRules', intakeRulesJSON(next))}
        />
        {custom && defaults && (
          <details className="group space-y-2">
            <summary className="flex cursor-pointer list-none items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground [&::-webkit-details-marker]:hidden">
              <ChevronRight className="size-3.5 transition-transform group-open:rotate-90" />
              {t('intakeRules.showBuiltIn')}
            </summary>
            <IntakeRuleList rules={defaults} />
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="rounded-[10px]"
              onClick={() => {
                setField('SupplyIntakeRules', '');
                setVersion((v) => v + 1);
              }}
            >
              {t('intakeRules.useBuiltIn')}
            </Button>
          </details>
        )}
      </div>
    </OptionCard>
  );
}

SupplyIntakeRulesCard.propTypes = { ctx: PropTypes.object.isRequired };

// What saving the routing switch will do: turning it on migrates the channels to supplies first
// (the dry run says how much), turning it off goes back to the channels' model lists.
function SupplyRoutingNote({ turningOn }) {
  const { t } = useTranslation();
  const [report, setReport] = useState(null);
  useEffect(() => {
    if (!turningOn) return undefined;
    let live = true;
    setReport(null);
    previewSupplyMigration()
      .then((res) => live && setReport(res.success ? res.data : { error: res.message || true }))
      .catch(() => live && setReport({ error: true }));
    return () => {
      live = false;
    };
  }, [turningOn]);
  const text = (part, values) => t(`settingsUi.platform.cards.supply-mode.${part}`, values);
  let line;
  if (!turningOn) line = text('offNote');
  else if (!report) line = text('migrateChecking');
  else if (report.error) line = text('migrateUnknown');
  else line = report.supplies_planned ? text('migrateNote', { supplies: report.supplies_planned }) : text('migrateNothing');
  // Models a group can request today and could not after the switch (no usable supply).
  const lost =
    report && !report.error ? new Set((report.routing_differences || []).filter((d) => d.kind === 'lost').map((d) => d.model)).size : 0;
  return (
    <p className="text-xs text-muted-foreground">
      {line}
      {lost > 0 && <span className="text-warning"> {text('migrateLost', { count: lost })}</span>}
    </p>
  );
}

SupplyRoutingNote.propTypes = { turningOn: PropTypes.bool.isRequired };

// Whether requests route by supplies, how a model that follows the default picks a supply within
// one priority tier, and whether a supply's cost reorders the tiers (with the saving and success
// rate a cheaper supply needs to be put ahead). Labels, hints, the values' descriptions and examples come from the settings catalog.
export function SupplyModeCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, origin, setField } = ctx;
  const routing = inputs.ModelSupplyEnabled === 'true';
  const switching = routing !== (origin?.ModelSupplyEnabled === 'true');
  const value = inputs.SupplyModeDefault === 'failover' ? 'failover' : 'balance';
  const costOrder = inputs.SupplyCostOrder === 'cost' ? 'cost' : 'priority';
  const enumOptions = (key, values) => values.map((v) => ({ value: v, label: settingEnumLabel(t, key, v) }));
  return (
    <OptionCard ctx={ctx} id="supply-mode">
      <OptionList>
        <ToggleRow id="ModelSupplyEnabled" {...bindSwitch(ctx, 'ModelSupplyEnabled')}>
          {switching && <SupplyRoutingNote turningOn={routing} />}
        </ToggleRow>
      </OptionList>
      <SelectRow
        id="SupplyModeDefault"
        value={value}
        onChange={(v) => setField('SupplyModeDefault', v)}
        options={enumOptions('SupplyModeDefault', ['balance', 'failover'])}
      />
      <SelectRow
        id="SupplyCostOrder"
        value={costOrder}
        onChange={(v) => setField('SupplyCostOrder', v)}
        options={enumOptions('SupplyCostOrder', ['priority', 'cost'])}
      />
      {/* The two guards only mean something while cost orders supplies. */}
      {costOrder === 'cost' && (
        <div className="grid gap-5 sm:grid-cols-2">
          <TextRow
            id="SupplyCostMinSavingPercent"
            type="number"
            unit="%"
            value={inputs.SupplyCostMinSavingPercent}
            onChange={(v) => setField('SupplyCostMinSavingPercent', v)}
            placeholder="5"
          />
          <TextRow
            id="SupplyCostMinSuccessPercent"
            type="number"
            unit="%"
            value={inputs.SupplyCostMinSuccessPercent}
            onChange={(v) => setField('SupplyCostMinSuccessPercent', v)}
            placeholder="95"
          />
        </div>
      )}
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
        <ToggleRow id="SessionAffinityEnabled" {...bindSwitch(ctx, 'SessionAffinityEnabled')} />
        <ToggleRow id="SessionAffinityStrict" {...bindSwitch(ctx, 'SessionAffinityStrict')} />
      </OptionList>
      <TextRow
        id="SessionAffinityTTLSeconds"
        type="number"
        value={inputs.SessionAffinityTTLSeconds}
        onChange={(v) => setField('SessionAffinityTTLSeconds', v)}
        placeholder={gs('sessionAffinityTTL.placeholder')}
      />
    </OptionCard>
  );
}

export function ModelNameMatchCard({ ctx }) {
  return (
    <OptionCard ctx={ctx} id="model-name-match">
      <OptionList>
        <ToggleRow id="ModelNameCaseInsensitiveEnabled" {...bindSwitch(ctx, 'ModelNameCaseInsensitiveEnabled')} />
        <ToggleRow id="UnifiedRequestResponseModelEnabled" {...bindSwitch(ctx, 'UnifiedRequestResponseModelEnabled')} />
      </OptionList>
    </OptionCard>
  );
}

export function NativeProtocolCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const op = (k) => t(`setting_index.operationSettings.${k}`);
  const row = (key) => <ToggleRow id={key} {...bindSwitch(ctx, key)} />;
  const validate = (i) => {
    if (i.ClaudeDefaultMaxTokens && !verifyJSON(i.ClaudeDefaultMaxTokens)) return op('claudeSettings.invalidJson');
    if (i.GeminiOpenThink && !verifyJSON(i.GeminiOpenThink)) return op('geminiSettings.invalidJson');
    return null;
  };

  return (
    <OptionCard ctx={ctx} id="native-protocol" options={{ validate }}>
      <OptionList>
        {row('ClaudeAPIEnabled')}
        {row('GeminiAPIEnabled')}
        {row('ClaudePromptCachingEnabled')}
        {row('FingerprintPassThroughEnabled')}
      </OptionList>
      <FieldSection title={op('claudeSettings.title')}>
        <TextRow
          id="ClaudeBudgetTokensPercentage"
          type="number"
          value={inputs.ClaudeBudgetTokensPercentage}
          onChange={(v) => setField('ClaudeBudgetTokensPercentage', v)}
          placeholder={op('claudeSettings.budgetTokensPercentage.placeholder')}
        />
        <TextRow
          id="ClaudeDefaultMaxTokens"
          value={inputs.ClaudeDefaultMaxTokens}
          onChange={(v) => setField('ClaudeDefaultMaxTokens', v)}
          placeholder={op('claudeSettings.defaultMaxTokens.placeholder')}
          multiline
        />
      </FieldSection>
      <FieldSection title={op('geminiSettings.title')}>
        <TextRow
          id="GeminiOpenThink"
          value={inputs.GeminiOpenThink}
          onChange={(v) => setField('GeminiOpenThink', v)}
          placeholder={op('geminiSettings.geminiOpenThink.placeholder')}
          multiline
        />
      </FieldSection>
    </OptionCard>
  );
}

export function PromptLimitsCard({ ctx }) {
  const { inputs, setField } = ctx;
  return (
    <OptionCard ctx={ctx} id="prompt-limits">
      <TextRow id="MaxPromptTokens" type="number" value={inputs.MaxPromptTokens} onChange={(v) => setField('MaxPromptTokens', v)} />
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
        value={inputs.ChatImageRequestProxy}
        onChange={(v) => setField('ChatImageRequestProxy', v)}
        placeholder={op('chatImageRequestProxy.placeholder')}
      />
      <div className="grid gap-5 sm:grid-cols-2">
        <TextRow id="CFWorkerImageUrl" value={inputs.CFWorkerImageUrl} onChange={(v) => setField('CFWorkerImageUrl', v)} />
        <TextRow
          id="CFWorkerImageKey"
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
  return (
    <OptionCard
      ctx={ctx}
      id="auto-routing"
      description={
        <>
          {cardText(t, 'auto-routing', 'description')}{' '}
          <Link to={admin.routing()} className="text-primary hover:underline">
            {t('settingsUi.platform.actions.openRouting')}
          </Link>
        </>
      }
    >
      <OptionList>
        <ToggleRow id="RoutingDecisionBillingEnabled" {...bindSwitch(ctx, 'RoutingDecisionBillingEnabled')} />
        <ToggleRow id="UserRoutingPolicyEnabled" {...bindSwitch(ctx, 'UserRoutingPolicyEnabled')} />
      </OptionList>
      <TextRow
        id="PersonalRoutingPolicyMaxCount"
        type="number"
        value={inputs.PersonalRoutingPolicyMaxCount}
        onChange={(v) => setField('PersonalRoutingPolicyMaxCount', v)}
        placeholder="20"
      />
      <TextRow
        id="OrganizationRoutingPolicyMaxCount"
        type="number"
        value={inputs.OrganizationRoutingPolicyMaxCount}
        onChange={(v) => setField('OrganizationRoutingPolicyMaxCount', v)}
        placeholder="20"
      />
    </OptionCard>
  );
}

export { default as ImageStorageCard } from './ImageStorageCard';
