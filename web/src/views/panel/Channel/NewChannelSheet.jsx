import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, Loader2, Pencil, Play } from 'lucide-react';

import { configText } from 'i18n/configText';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { toast } from '@/components/ui/sonner';
import { createChannel, fetchUpstreamModelList, manageChannel } from './channelApi';
import { buildModelSuggestions, fetchPriceList } from './modelSuggestions';
import { usageNumbers, usageOptions, usagePeers } from './channelUsage';
import {
  KEY_PART_MODES,
  applyBlockedModels,
  applyMappingToModels,
  autoChannelName,
  buildPartsKey,
  countKeys,
  isValidWildcard,
  maskInput,
  normalizeKeyInput,
  resolveAllowedModels,
  toWildcard
} from './newChannel';
import { BASE_URL_REQUIRED, BASE_URL_SHOWN, credentialKind, typeLabels } from './providerDefaults';
import { Dot, Field, ObjectSheet, Segmented, SheetBar } from '@/components/objects';
import { ChipInput, GroupOptions, KeyLink, ProviderTile, UsageOptions, useProviderName } from './channelParts';
import { ProviderSelect } from './ProviderPicker';
import ChannelOAuth from './ChannelOAuth';
import ModelMappingInput from './ModelMappingInput';
import pluginConfig from './pluginConfig';

// Fields the create call needs besides what the sheet asks for (same defaults as the earlier
// full form). Plugins start from their declared defaults.
function createDefaults(type) {
  const plugins = pluginConfig[String(type)];
  const plugin = {};
  if (plugins) {
    for (const [pluginId, cfg] of Object.entries(plugins)) {
      plugin[pluginId] = {};
      for (const [paramId, param] of Object.entries(cfg.params)) {
        plugin[pluginId][paramId] = param.type === 'bool' ? false : (param.default ?? '');
      }
    }
  }
  return {
    status: 1,
    proxy: '',
    remark: '',
    tag: '',
    model_headers: '',
    header_override: '',
    custom_parameter: '',
    need2response_models: '',
    plugin,
    only_chat: false,
    balance_threshold: 0,
    pre_cost: 1,
    disabled_stream: [],
    compatible_response: false,
    allow_extra_body: false,
    pass_through_body: false,
    allow_service_tier: false,
    unified_request_response_model: false
  };
}

// API versions the backend expects when none is given (as the earlier form filled in).
const DEFAULT_OTHER = { 3: '2024-05-01-preview', 18: 'v2.1' };

const fresh = (type, name) => ({
  type,
  name,
  nameEdited: false,
  key: '',
  keyEditing: true,
  base_url: '',
  other: '',
  partsMode: 'apiKey',
  parts: {},
  allow: ['*'],
  allowMode: 'model',
  block: [],
  blockMode: 'model',
  mapping: '',
  groups: ['default'],
  usage: null,
  custom: { priority: '0', weight: '1' }
});

function parseMapping(raw) {
  try {
    const m = raw ? JSON.parse(raw) : {};
    return m && typeof m === 'object' && !Array.isArray(m) ? m : {};
  } catch {
    return {};
  }
}

// New channel sheet: connect to a provider with the fewest fields, defaults for the rest.
// Provider and name, allowed and blocked models, model mapping, the credential the provider
// needs (with a link to get it), the address only when required, groups and usage. Test checks
// the unsaved credential against the provider's model list; save creates, then tests.
export default function NewChannelSheet({ initialType, allChannels, groups, onClose, onCreated }) {
  const { t } = useTranslation();
  const providerName = useProviderName();
  const names = useMemo(() => allChannels.map((c) => c.name), [allChannels]);
  const [d, setD] = useState(() => fresh(initialType, autoChannelName(providerName(initialType), names)));
  const [test, setTest] = useState({ status: 'idle' });
  const [upstream, setUpstream] = useState([]);
  const [prices, setPrices] = useState([]);
  const [saving, setSaving] = useState(false);
  const [mapVersion, setMapVersion] = useState(0);

  useEffect(() => {
    fetchPriceList().then(setPrices);
  }, []);

  const labels = typeLabels(d.type);
  const kind = credentialKind(d.type);
  const set = (patch) => setD((prev) => ({ ...prev, ...patch }));
  const resetTest = () => {
    setTest({ status: 'idle' });
    setUpstream([]);
  };

  const key = kind === 'parts' ? buildPartsKey(d.partsMode, d.parts) : normalizeKeyInput(d.key);
  const keyCount = kind === 'key' ? countKeys(d.key) : key ? 1 : 0;
  const keys = key ? key.split('\n') : [];
  const showBase = BASE_URL_SHOWN.has(d.type);
  const baseRequired = BASE_URL_REQUIRED.has(d.type);
  const authOk = keyCount > 0;
  const baseOk = !baseRequired || d.base_url.trim();
  const ready = authOk && baseOk && d.name.trim() && d.allow.length > 0;
  const busy = saving || test.status === 'running';
  const missing = [
    !authOk &&
      (kind === 'oauth'
        ? t('channels.new.missing.account')
        : kind === 'json'
          ? t('channels.new.missing.json')
          : t('channels.new.missing.key')),
    !baseOk && t('channels.new.missing.baseUrl'),
    !d.name.trim() && t('channels.new.missing.name'),
    !d.allow.length && t('channels.new.missing.models')
  ].filter(Boolean);

  const suggestions = useMemo(() => (upstream.length ? upstream : buildModelSuggestions(d.type, prices)), [upstream, d.type, prices]);
  const peers = usagePeers({ type: d.type, groups: d.groups }, allChannels);
  const usage = usageOptions({ priority: 0, weight: 1 }, peers);
  const usageKey = usage.options.some((o) => o.key === d.usage) ? d.usage : usage.options[0].key;

  const pickType = (type) => {
    if (type === d.type) return;
    setD((prev) => ({
      ...fresh(type, prev.nameEdited ? prev.name : autoChannelName(providerName(type), names)),
      nameEdited: prev.nameEdited,
      allow: prev.allow.includes('*') ? ['*'] : [],
      groups: prev.groups
    }));
    setMapVersion((v) => v + 1);
    resetTest();
  };

  const requestClose = () => {
    const touched = d.key || d.base_url || Object.values(d.parts).some(Boolean);
    if (!touched || window.confirm(t('channels.new.discardConfirm'))) onClose();
  };

  const probe = { id: 0, type: d.type, key: keys[0] || '', base_url: d.base_url.trim(), other: d.other.trim() };

  const runTest = async () => {
    setTest({ status: 'running' });
    const started = performance.now();
    try {
      const list = await fetchUpstreamModelList(probe);
      setUpstream(list);
      setTest({ status: 'ok', ms: Math.round(performance.now() - started), count: list.length });
      return list;
    } catch (e) {
      setTest({ status: 'fail', message: e.message });
      return null;
    }
  };

  const allowChange = (list) => {
    if (list.length > d.allow.length && list[list.length - 1] === '*') set({ allow: ['*'] });
    else set({ allow: list.length > 1 ? list.filter((m) => m !== '*') : list });
  };
  const entryTransform = (mode) => (v) => {
    if (v === '*') return mode === 'allow' ? '*' : '';
    const value = (mode === 'allow' ? d.allowMode : d.blockMode) === 'wildcard' ? toWildcard(v) : v;
    if (value.includes('*') && !isValidWildcard(value)) return '';
    return value;
  };

  const saveAndTest = async () => {
    if (!ready || busy) return;
    setSaving(true);
    try {
      let list = upstream;
      if (d.allow.includes('*') && !list.length && labels.canFetchModels) {
        // A failed connection is not "no models": stop and say so instead of creating nothing.
        const fetched = await runTest();
        if (!fetched) {
          toast.error(t('channels.new.connectFailed'));
          return;
        }
        list = fetched;
      }
      const mapping = parseMapping(d.mapping);
      const resolved = resolveAllowedModels({ allow: d.allow, upstream: list, defaults: labels.defaultModels });
      const mapped = applyMappingToModels(resolved.models, mapping);
      const { models, driftIgnoreRules } = applyBlockedModels(mapped, d.block, '');
      if (!models.length) {
        toast.error(t('channels.new.noModelList'));
        return;
      }
      const nums = usageNumbers(usageKey, { priority: 0, weight: 1 }, peers, d.custom);
      const base = d.base_url.trim();
      const body = {
        ...createDefaults(d.type),
        name: d.name.trim(),
        type: d.type,
        key,
        base_url: base.endsWith('/') ? base.slice(0, -1) : base,
        other: d.other.trim() || DEFAULT_OTHER[d.type] || '',
        models: models.join(','),
        group: d.groups.join(','),
        priority: nums.priority,
        weight: nums.weight,
        model_mapping: Object.keys(mapping).length ? JSON.stringify(mapping) : '',
        drift_auto_add: resolved.driftAutoAdd,
        drift_ignore_rules: driftIgnoreRules,
        test_model: models.includes(labels.testModel) ? labels.testModel : ''
      };
      const res = await createChannel(body);
      if (!res.success) {
        toast.error(res.message || t('channels.new.createFailed'));
        return;
      }
      // Test every created channel; a failure is reported, the channel stays.
      const failed = [];
      for (const id of res.ids) {
        const r = await manageChannel(id, 'test');
        if (!r?.success) failed.push(r?.message || `#${id}`);
      }
      if (failed.length) toast.warning(t('channels.new.testFailed', { count: failed.length, message: failed[0] }));
      else toast.success(t('channels.new.created', { name: body.name, count: res.ids.length }));

      const usageLabel = t(`channels.usage.${usageKey}`);
      const summary = [
        [
          d.allow.includes('*')
            ? t('channels.created.allModels', { count: models.length })
            : t('channels.created.models', { count: models.length }),
          d.block.length ? t('channels.created.blocked', { count: d.block.length }) : ''
        ]
          .filter(Boolean)
          .join(t('channels.created.comma')),
        t('channels.created.groups', { groups: d.groups.join(t('channels.created.listSep')) }),
        usage.top && usageKey !== 'custom' ? t('channels.created.relative', { usage: usageLabel, name: usage.top.name }) : usageLabel,
        res.supplies ? t('channels.created.supplied', { count: res.supplies }) : ''
      ]
        .filter(Boolean)
        .join(' · ');
      onCreated({ ids: res.ids, type: d.type, name: body.name, summary });
    } catch (e) {
      toast.error(e.message);
    } finally {
      setSaving(false);
    }
  };

  const keyPlaceholder = configText(t, labels.prompt.key) || t('channels.new.keyPlaceholder');
  let credential;
  if (kind === 'oauth') {
    credential = (
      <div className="space-y-2">
        <div className="overflow-hidden rounded-lg border border-border">
          <div className="flex min-h-11 items-center gap-3 px-3 text-[13px]">
            {key ? (
              <span className="inline-flex items-center gap-1.5 text-success">
                <Check className="size-4" />
                <span className="text-foreground">{t('channels.new.authorized')}</span>
              </span>
            ) : (
              <span className="text-muted-foreground">{t('channels.new.authorizeHint')}</span>
            )}
          </div>
          <div className="border-t border-border px-3 py-2.5">
            <ChannelOAuth
              key={d.type}
              type={d.type}
              channelId={0}
              getValues={(name) => ({ other: d.other, proxy: '' })[name] ?? ''}
              onCredentials={(credentials) => {
                set({ key: credentials, keyEditing: false });
                resetTest();
              }}
            />
          </div>
        </div>
        <Textarea
          rows={2}
          className="font-mono text-[13px]"
          value={d.key}
          placeholder={t('channels.new.pasteCredentials')}
          onChange={(e) => set({ key: e.target.value })}
        />
      </div>
    );
  } else if (kind === 'parts') {
    credential = (
      <div className="space-y-3">
        <div className="grid grid-cols-2 gap-3">
          {KEY_PART_MODES[d.partsMode].map((f) => (
            <label key={f} className={cn('space-y-1.5 text-xs text-muted-foreground', (f === 'token' || f === 'roleArn') && 'col-span-2')}>
              <span>
                {t(`channels.new.parts.${f}`)}
                {f === 'sessionToken' && <span className="ml-1">{t('objects.optional')}</span>}
              </span>
              <Input
                className="font-mono text-[13px] text-foreground"
                type={f === 'region' || f === 'accessKey' || f === 'roleArn' ? 'text' : 'password'}
                autoComplete="off"
                value={d.parts[f] || ''}
                placeholder={f === 'region' ? 'us-east-1' : f === 'roleArn' ? 'arn:aws:iam::123456789012:role/name' : ''}
                onChange={(e) => {
                  set({ parts: { ...d.parts, [f]: e.target.value } });
                  resetTest();
                }}
              />
            </label>
          ))}
        </div>
      </div>
    );
  } else if (d.keyEditing || !keys.length) {
    credential = (
      <Textarea
        id="nc-key"
        rows={kind === 'json' ? 5 : 3}
        className="font-mono text-[13px]"
        value={d.key}
        placeholder={kind === 'json' ? t('channels.new.jsonPlaceholder') : keyPlaceholder}
        onChange={(e) => {
          set({ key: e.target.value, keyEditing: true });
          resetTest();
        }}
        onBlur={() => d.key.trim() && set({ keyEditing: false })}
      />
    );
  } else {
    credential = (
      <div className="overflow-hidden rounded-lg border border-border">
        {keys.slice(0, 3).map((k, i) => (
          <div key={i} className="flex min-h-11 items-center gap-3 border-b border-border px-3 text-[13px]">
            <Dot tone="off" />
            <span className="font-mono">{kind === 'json' ? t('channels.new.jsonSet') : maskInput(k)}</span>
          </div>
        ))}
        <div className="flex min-h-11 items-center justify-between bg-muted/50 px-2 pl-3">
          <span className="text-xs text-muted-foreground">
            {keyCount > 1 ? t('channels.new.keysCount', { count: keyCount }) : t('channels.new.keyHidden')}
          </span>
          <Button variant="ghost" size="sm" className="h-[30px] rounded-lg" onClick={() => set({ keyEditing: true })}>
            <Pencil />
            {t('channels.new.editKey')}
          </Button>
        </div>
      </div>
    );
  }

  const credLabel =
    kind === 'oauth'
      ? t('channels.conn.account')
      : kind === 'json'
        ? t('channels.new.serviceAccount')
        : kind === 'parts'
          ? t('channels.new.credentials')
          : configText(t, labels.label.key);

  let status;
  if (test.status === 'ok') {
    status = (
      <span className="inline-flex items-center gap-1 text-success">
        <Check className="size-4" />
        <span>{t('channels.new.testOk', { ms: test.ms, count: test.count })}</span>
      </span>
    );
  } else if (test.status === 'fail') {
    status = <span className="text-destructive">{t('channels.new.testFail', { message: test.message })}</span>;
  } else if (ready) {
    status = labels.canFetchModels ? t('channels.new.readyHint') : t('channels.new.readyNoTest');
  } else {
    status = t('channels.new.missingList', { items: missing.join(t('channels.created.listSep')) });
  }

  const allowHint = d.allow.includes('*')
    ? t('channels.new.allowAllHint')
    : d.allowMode === 'wildcard'
      ? t('channels.new.allowWildcardHint')
      : t('channels.new.allowSomeHint', { count: d.allow.length });

  return (
    <ObjectSheet
      titleId="new-channel-title"
      size="md"
      icon={<ProviderTile type={d.type} size="lg" />}
      title={t('channels.newChannel')}
      subtitle={t('channels.new.subtitle', { provider: providerName(d.type) })}
      onClose={requestClose}
      bodyClassName="[&>*:first-child]:pt-0"
      footer={
        <SheetBar status={status}>
          <Button variant="outline" className="rounded-lg" onClick={requestClose}>
            {t('common.cancel')}
          </Button>
          <Button
            variant="outline"
            className="rounded-lg"
            disabled={!authOk || !baseOk || busy || !labels.canFetchModels}
            title={labels.canFetchModels ? undefined : t('channels.new.readyNoTest')}
            onClick={runTest}
          >
            {test.status === 'running' && !saving ? <Loader2 className="animate-spin" /> : <Play />}
            {t('channels.action.test')}
          </Button>
          <Button className="rounded-lg" disabled={!ready || busy} onClick={saveAndTest}>
            {saving && <Loader2 className="animate-spin" />}
            {saving ? t('channels.new.saving') : t('channels.new.saveAndTest')}
          </Button>
        </SheetBar>
      }
    >
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label={t('channels.new.provider')} htmlFor="nc-type" className="pt-0">
          <ProviderSelect id="nc-type" value={d.type} onChange={pickType} />
        </Field>
        <Field label={t('channels.new.name')} htmlFor="nc-name" className="!border-t-0 pt-0">
          <Input
            id="nc-name"
            className="font-mono text-[13px]"
            value={d.name}
            onChange={(e) => set({ name: e.target.value, nameEdited: true })}
          />
        </Field>
      </div>

      <Field
        label={t('channels.new.allowed')}
        htmlFor="nc-allow"
        extra={
          <Segmented
            size="mini"
            label={t('channels.new.matchMode')}
            value={d.allowMode}
            onChange={(v) => set({ allowMode: v })}
            options={[
              { value: 'model', label: t('channels.new.modeModels') },
              { value: 'wildcard', label: t('channels.new.modeWildcard') }
            ]}
          />
        }
        hint={allowHint}
      >
        <ChipInput
          id="nc-allow"
          values={d.allow}
          onChange={allowChange}
          suggestions={d.allowMode === 'model' ? suggestions : []}
          transform={entryTransform('allow')}
          onReject={() => toast.error(t('channels.new.wildcardInvalid'))}
          chipLabel={(v) => (v === '*' ? <span className="font-sans">{t('channels.new.allModels')}</span> : v)}
          placeholder={d.allowMode === 'wildcard' ? t('channels.new.wildcardPlaceholder') : t('channels.new.allowEmpty')}
        />
      </Field>

      <Field
        label={t('channels.new.blocked')}
        htmlFor="nc-block"
        optional
        extra={
          <Segmented
            size="mini"
            label={t('channels.new.matchMode')}
            value={d.blockMode}
            onChange={(v) => set({ blockMode: v })}
            options={[
              { value: 'model', label: t('channels.new.modeModels') },
              { value: 'wildcard', label: t('channels.new.modeWildcard') }
            ]}
          />
        }
        hint={t('channels.new.blockedHint')}
      >
        <ChipInput
          id="nc-block"
          values={d.block}
          onChange={(block) => set({ block })}
          suggestions={d.blockMode === 'model' ? suggestions : []}
          transform={entryTransform('block')}
          onReject={() => toast.error(t('channels.new.wildcardInvalid'))}
          placeholder={d.blockMode === 'wildcard' ? t('channels.new.wildcardPlaceholder') : t('channels.new.blockedEmpty')}
        />
      </Field>

      <Field label={t('channels.new.mapping')} optional hint={t('channels.new.mappingHint')}>
        <ModelMappingInput key={mapVersion} value={d.mapping} models={suggestions} onChange={(v) => set({ mapping: v })} />
      </Field>

      {kind === 'parts' && (
        <Field label={t('channels.new.authMethod')}>
          <Segmented
            size="wide"
            label={t('channels.new.authMethod')}
            value={d.partsMode}
            onChange={(v) => {
              set({ partsMode: v });
              resetTest();
            }}
            options={Object.keys(KEY_PART_MODES).map((m) => ({ value: m, label: t(`channels.new.parts.mode.${m}`) }))}
          />
        </Field>
      )}

      <Field
        label={credLabel}
        htmlFor={kind === 'key' || kind === 'json' ? 'nc-key' : undefined}
        extra={kind !== 'oauth' && <KeyLink type={d.type} />}
        hint={
          kind === 'oauth'
            ? t('channels.new.oauthHint')
            : kind === 'parts'
              ? t('channels.new.partsHint')
              : kind === 'json'
                ? t('channels.new.jsonHint')
                : t('channels.new.keyHint')
        }
      >
        {credential}
        {keyCount > 1 && (
          <p className="mt-2 rounded-lg bg-muted px-3 py-2 text-[13px] text-muted-foreground">
            {t('channels.new.multiKey', { count: keyCount, first: d.name.trim() || '…', second: `${d.name.trim() || '…'}_2` })}
          </p>
        )}
      </Field>

      {labels.hasOther && (
        <Field label={configText(t, labels.label.other)} htmlFor="nc-other" hint={configText(t, labels.prompt.other)}>
          <Input
            id="nc-other"
            className="font-mono text-[13px]"
            value={d.other}
            onChange={(e) => {
              set({ other: e.target.value });
              resetTest();
            }}
          />
        </Field>
      )}

      {showBase && (
        <Field
          label={t('channels.conn.baseUrl')}
          htmlFor="nc-base"
          optional={!baseRequired}
          hint={configText(t, labels.prompt.base_url) || t('channels.new.baseUrlHint')}
        >
          <Input
            id="nc-base"
            className="font-mono text-[13px]"
            value={d.base_url}
            placeholder={d.type === 3 ? 'https://your-resource.openai.azure.com' : 'https://your-host/v1'}
            onChange={(e) => {
              set({ base_url: e.target.value });
              resetTest();
            }}
          />
        </Field>
      )}

      <Field label={t('channels.groups')} extra={t('channels.groupsHint')}>
        <GroupOptions groups={groups} value={d.groups} onChange={(g) => set({ groups: g })} />
      </Field>

      <Field label={t('channels.usage.label')} extra={usage.top ? t('channels.usage.peerExisting', { name: usage.top.name }) : null}>
        <UsageOptions
          usage={usage}
          value={usageKey}
          onChange={(k) => set({ usage: k })}
          custom={d.custom}
          onCustomChange={(custom) => set({ custom })}
        />
      </Field>
    </ObjectSheet>
  );
}

NewChannelSheet.propTypes = {
  initialType: PropTypes.number.isRequired,
  allChannels: PropTypes.array.isRequired,
  groups: PropTypes.array.isRequired,
  onClose: PropTypes.func.isRequired,
  onCreated: PropTypes.func.isRequired
};
