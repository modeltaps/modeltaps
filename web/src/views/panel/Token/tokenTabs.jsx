import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Copy, Eye, EyeOff } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select';
import { Field, InfoRow, InfoRows, Section, Segmented, StatRow, ToggleField } from '@/components/objects';
import ConnectInfo, { useApiAddresses } from './ConnectInfo';
import { Skeleton } from '@/components/ui/skeleton';
import QuotaInput from '@/components/QuotaInput';
import { API } from 'utils/api';
import { renderQuota, SpendAmount, timestamp2string } from 'utils/common';
import { isPlaygroundToken } from 'utils/playgroundToken';
import { maskSecret } from 'utils/maskSecret';
import ModelLimitSelector from './ModelLimitSelector';
import ModelAccessField, { accessPayload, accessValueFrom } from '@/components/model/ModelAccessField';
import { GroupBadge, nextResetTime, relativeTime, statusInfo } from './TokenTable';
import { EXPIRY_PRESETS, RESET_PERIODS, expiryFromPreset, quotaDraft, quotaPatch } from './tokenData';

// Tabs of the API key panel (see components/objects/ObjectPanel), laid out like a gateway's
// virtual-key details: Overview is read-only (the key, what it spent, what it can reach); Basics
// (name, expiry, limit), Access (groups, models, IPs) and Advanced each save on their own.

const bodyProps = { object: PropTypes.object, draft: PropTypes.object, setDraft: PropTypes.func, ctx: PropTypes.object };

// The key itself: masked until revealed, with copy. Keys the playground manages are not shown.
function KeyRow({ token, onCopy }) {
  const { t } = useTranslation();
  const [shown, setShown] = useState(false);
  if (isPlaygroundToken(token)) {
    return (
      <Badge variant="secondary" className="w-fit">
        {t('token_index.playgroundManaged')}
      </Badge>
    );
  }
  const full = `sk-${token.key || ''}`;
  return (
    <div className="flex h-10 items-center gap-1 rounded-lg border border-input bg-field pl-3 pr-1">
      <code className="min-w-0 flex-1 truncate font-mono text-[13px]">{shown ? full : maskSecret(full)}</code>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="size-8 rounded-md"
        aria-label={t(shown ? 'apiKey.panel.hide' : 'apiKey.panel.reveal')}
        onClick={() => setShown((v) => !v)}
      >
        {shown ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
      </Button>
      <Button type="button" variant="ghost" size="icon" className="size-8 rounded-md" aria-label={t('token_index.copy')} onClick={onCopy}>
        <Copy className="size-4" />
      </Button>
    </div>
  );
}

KeyRow.propTypes = { token: PropTypes.object.isRequired, onCopy: PropTypes.func };

// What the key can call with its saved settings: groups ∩ model limit ∩ organization member
// allowlist, the same answer as /v1/models.
const AVAILABLE_PREVIEW = 24;

function AvailableModels({ tokenId }) {
  const { t } = useTranslation();
  const [models, setModels] = useState(null);
  const [expanded, setExpanded] = useState(false);
  useEffect(() => {
    let alive = true;
    API.get(`/api/token/${tokenId}/models`)
      .then((res) => alive && setModels(res.data.success && Array.isArray(res.data.data) ? res.data.data : []))
      .catch(() => alive && setModels([]));
    return () => {
      alive = false;
    };
  }, [tokenId]);

  const shown = models && !expanded ? models.slice(0, AVAILABLE_PREVIEW) : models;
  return (
    <Section title={t('token_index.availableModels', { count: models?.length ?? 0 })} extra={t('apiKey.panel.availableHint')}>
      <div className="py-2" data-testid="token-available-models">
        {models === null ? (
          <div className="flex flex-wrap gap-1.5" aria-busy="true">
            {[88, 120, 64, 96, 72].map((w) => (
              <Skeleton key={w} className="h-6 rounded-md" style={{ width: w }} />
            ))}
          </div>
        ) : models.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t('token_index.availableModelsNone')}</p>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {shown.map((m) => (
              <Badge key={m} variant="secondary" className="font-mono text-[11px] font-normal">
                {m}
              </Badge>
            ))}
            {models.length > AVAILABLE_PREVIEW && (
              <button type="button" className="text-xs text-muted-foreground hover:text-foreground" onClick={() => setExpanded((v) => !v)}>
                {expanded
                  ? t('token_index.availableModelsLess')
                  : t('token_index.availableModelsMore', { count: models.length - AVAILABLE_PREVIEW })}
              </button>
            )}
          </div>
        )}
      </div>
    </Section>
  );
}

AvailableModels.propTypes = { tokenId: PropTypes.number.isRequired };

const isPast = (ts) => ts <= Math.floor(Date.now() / 1000);

function limitSummary(t, token) {
  const qr = token.setting?.quota_reset;
  if (qr?.period && qr.limit > 0) {
    return {
      value: renderQuota(Math.max(0, qr.limit - (token.period_used || 0))),
      hint: t('apiKey.panel.leftOfPeriod', { limit: renderQuota(qr.limit), period: t(`token_index.quotaReset_${qr.period}`) })
    };
  }
  if (token.unlimited_quota) return { value: t('token_index.noLimit') };
  return { value: renderQuota(token.remain_quota || 0), hint: t('apiKey.panel.leftOfTotal') };
}

function OverviewBody({ object: token, ctx }) {
  const { t } = useTranslation();
  const apiAddresses = useApiAddresses();
  const rtf = new Intl.RelativeTimeFormat((ctx.locale || 'en').replace('_', '-'), { numeric: 'auto' });
  const limit = limitSummary(t, token);
  const info = statusInfo(t, token.status);
  const qr = token.setting?.quota_reset;
  const nextReset = qr?.period ? nextResetTime(qr.period, ctx.resetTimezone, ctx.resetWeekStart) : null;
  const expiry =
    token.expired_time === -1 ? (
      t('token_index.expiryNever')
    ) : (
      <span className={isPast(token.expired_time) ? 'text-destructive' : undefined}>{timestamp2string(token.expired_time)}</span>
    );

  return (
    <>
      <Field label={t('apiKey.panel.key')}>
        <KeyRow token={token} onCopy={() => ctx.onCopyKey(token)} />
      </Field>
      <StatRow
        items={[
          { label: t('apiKey.panel.used'), value: <SpendAmount quota={token.used_quota || 0} /> },
          ...(ctx.todayUsage
            ? [{ label: t('token_index.usageToday'), value: <SpendAmount quota={ctx.todayUsage[token.name] || 0} /> }]
            : []),
          { label: t('apiKey.panel.left'), value: limit.value, hint: limit.hint }
        ]}
      />
      <Section title={t('apiKey.panel.details')}>
        <InfoRows>
          <InfoRow label={t('token_index.status')}>{info.label}</InfoRow>
          <InfoRow label={t('token_index.userGroup')}>
            <GroupBadge item={token} userGroup={ctx.userGroup} />
          </InfoRow>
          <InfoRow label={t('token_index.expiryTime')}>{expiry}</InfoRow>
          {nextReset && <InfoRow label={t('token_index.quotaResetNext')}>{timestamp2string(nextReset)}</InfoRow>}
          <InfoRow label={t('token_index.lastUsed')}>
            {token.accessed_time > 0 ? (
              <span title={timestamp2string(token.accessed_time)}>{relativeTime(rtf, token.accessed_time)}</span>
            ) : (
              <span className="text-muted-foreground">{t('token_index.neverUsed')}</span>
            )}
          </InfoRow>
          <InfoRow label={t('token_index.createdTime')}>{timestamp2string(token.created_time)}</InfoRow>
          {ctx.orgMemberNames && token.created_by > 0 && (
            <InfoRow label={t('org.creator')}>{ctx.orgMemberNames[token.created_by] || `#${token.created_by}`}</InfoRow>
          )}
          {ctx.adminMode && (
            <InfoRow label={t('apiKey.panel.owner')} mono>
              #{token.user_id}
            </InfoRow>
          )}
        </InfoRows>
      </Section>
      {/* Where this key sends requests, so it can be copied right next to the key itself. */}
      <Section title={t('apiKey.list.connect')}>
        <ConnectInfo addresses={apiAddresses} onCopy={ctx.onCopyText} />
      </Section>
      <AvailableModels tokenId={token.id} />
    </>
  );
}

OverviewBody.propTypes = bodyProps;

function BasicsBody({ draft, setDraft, ctx }) {
  const { t } = useTranslation();
  const set = (patch) => setDraft((d) => ({ ...d, ...patch }));
  const never = draft.expired_time === -1;
  // Which choice the key was last set with; an existing date matches none of them until changed.
  const [expiryChoice, setExpiryChoice] = useState(never ? 'never' : null);
  const hasPeriod = Boolean(draft.period);
  const tzNote = t('token_index.quotaResetTimezoneNote', {
    tz: ctx.resetTimezone || 'UTC',
    weekStart: t(ctx.resetWeekStart === 'sunday' ? 'token_index.weekStartSunday' : 'token_index.weekStartMonday')
  });
  return (
    <>
      <Field label={t('token_index.name')} htmlFor="tk-name">
        <Input id="tk-name" maxLength={30} value={draft.name} onChange={(e) => set({ name: e.target.value })} />
      </Field>
      <Field
        label={t('token_index.expiryTime')}
        extra={
          <Segmented
            size="mini"
            label={t('token_index.expiryTime')}
            value={expiryChoice}
            onChange={(v) => {
              setExpiryChoice(v);
              set({ expired_time: expiryFromPreset(v) });
            }}
            options={EXPIRY_PRESETS.map((p) => ({ value: p, label: t(`apiKey.expiry.${p}`) }))}
          />
        }
      >
        {/* The choice counts from now; the line below says exactly when the key stops working. */}
        <p className="text-sm text-muted-foreground">
          {never ? t('apiKey.panel.neverHint') : t('apiKey.panel.expiresOn', { date: timestamp2string(draft.expired_time) })}
        </p>
      </Field>
      <Field
        label={hasPeriod ? t('token_index.quotaPeriodLabel') : t('apiKey.panel.limit')}
        hint={hasPeriod ? `${t('token_index.quotaPeriodHint')} ${tzNote}` : t('apiKey.panel.limitHint')}
        extra={
          <Segmented
            size="mini"
            label={t('token_index.quotaResetPeriod')}
            value={draft.period || 'none'}
            onChange={(v) => set({ period: v === 'none' ? '' : v })}
            options={[
              { value: 'none', label: t('apiKey.panel.noReset') },
              ...RESET_PERIODS.map((p) => ({ value: p, label: t(`token_index.quotaReset_${p}`) }))
            ]}
          />
        }
      >
        <QuotaInput
          name="tk-quota"
          value={draft.quota}
          placeholder={hasPeriod ? '0' : t('token_index.noLimit')}
          allowEmpty={!hasPeriod}
          onChange={(v) => set({ quota: v })}
        />
      </Field>
    </>
  );
}

BasicsBody.propTypes = bodyProps;

const basicsTab = {
  key: 'basics',
  make: (tk) => ({ name: tk.name || '', expired_time: tk.expired_time ?? -1, ...quotaDraft(tk) }),
  patch: (d, tk, { t }) => {
    const name = d.name.trim();
    if (!name) return { error: t('apiKey.panel.nameRequired') };
    const quota = quotaPatch(d, t);
    if (quota.error) return quota;
    return { name, expired_time: d.expired_time, ...quota };
  },
  Body: BasicsBody
};

function GroupSelect({ id, value, onChange, options, emptyLabel, exclude }) {
  const label = (v) => options.find((o) => o.value === v)?.label || v;
  return (
    <Select value={value || '__none'} onValueChange={(v) => onChange(v === '__none' ? '' : v)}>
      <SelectTrigger id={id}>
        <span className="truncate">{value ? label(value) : emptyLabel}</span>
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="__none">{emptyLabel}</SelectItem>
        {options
          .filter((o) => !exclude || o.value !== exclude)
          .map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
      </SelectContent>
    </Select>
  );
}

GroupSelect.propTypes = {
  id: PropTypes.string,
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  options: PropTypes.array.isRequired,
  emptyLabel: PropTypes.node,
  exclude: PropTypes.string
};

function AccessBody({ object, draft, setDraft, ctx }) {
  const { t } = useTranslation();
  const set = (patch) => setDraft((d) => ({ ...d, ...patch }));
  const [models, setModels] = useState([]);
  const [modelAccess, setModelAccess] = useState(null);
  const { loadModels, loadModelAccess } = ctx;
  const creatorId = object?.created_by;
  useEffect(() => {
    let alive = true;
    loadModels().then((m) => alive && setModels(m));
    if (loadModelAccess) loadModelAccess(creatorId).then((a) => alive && setModelAccess(a));
    return () => {
      alive = false;
    };
  }, [loadModels, loadModelAccess, creatorId]);
  // Organization keys (ORG-15): inside the approved models and the creator's member layer.
  const orgAccess = Boolean(loadModelAccess);
  const scope = modelAccess?.scope;
  const parents = [scope?.approved_models, scope?.effective_member].filter((l) => l?.length);
  return (
    <>
      <Section title={t('apiKey.panel.routing')}>
        <div className="grid grid-cols-1 gap-x-4 sm:grid-cols-2">
          <Field label={t('token_index.userGroup')} htmlFor="tk-group" className="[&+&]:border-t-0">
            <GroupSelect
              id="tk-group"
              value={draft.group}
              options={ctx.groupOptions}
              emptyLabel={t('token_index.followUser')}
              onChange={(v) => set({ group: v, backup_group: draft.backup_group === v ? '' : draft.backup_group })}
            />
          </Field>
          <Field label={t('token_index.userBackupGroup')} htmlFor="tk-backup" optional className="[&+&]:border-t-0">
            <GroupSelect
              id="tk-backup"
              value={draft.backup_group}
              options={ctx.groupOptions}
              emptyLabel={t('apiKey.panel.noBackup')}
              exclude={draft.group}
              onChange={(v) => set({ backup_group: v })}
            />
          </Field>
        </div>
        <p className="-mt-1 pb-3 text-xs text-muted-foreground">{t('apiKey.panel.groupHint')}</p>
        {ctx.userIsReliable && (
          <Field label={t('token_index.billingTag')} htmlFor="tk-billing" optional hint={t('token_index.billingTagHelper')}>
            <GroupSelect
              id="tk-billing"
              value={draft.billing_tag}
              options={ctx.groupOptions}
              emptyLabel="-"
              onChange={(v) => set({ billing_tag: v })}
            />
          </Field>
        )}
      </Section>
      <Section title={t('apiKey.panel.restrictions')}>
        {orgAccess && (
          <Field label={t('token_index.limits_models_switch')} hint={t('modelAccess.keyHint')}>
            {modelAccess ? (
              <ModelAccessField
                id="token-model-limit"
                value={draft.access}
                onChange={(access) => set({ access })}
                lists={modelAccess.lists || []}
                models={models}
                parents={parents}
                inheritLabel={t('modelAccess.mode.noExtra')}
                inheritHint={
                  scope?.effective_member?.length
                    ? t('modelAccess.memberLayer', { models: scope.effective_member.join(', ') })
                    : scope?.approved_models?.length
                      ? t('modelAccess.orgDefaultApproved')
                      : t('modelAccess.orgDefaultNone')
                }
              />
            ) : (
              <div className="h-9 w-full animate-pulse rounded-lg bg-muted" aria-hidden="true" />
            )}
          </Field>
        )}
        {!orgAccess && (
          <ToggleField
            id="tk-limit-models"
            label={t('token_index.limits_models_switch')}
            hint={t('token_index.limits_info')}
            checked={draft.limitModels}
            onChange={(c) => set({ limitModels: c, models: c ? draft.models : [] })}
          />
        )}
        {!orgAccess && draft.limitModels && (
          <div className="pb-3">
            <ModelLimitSelector
              id="token-model-limit"
              models={models}
              value={draft.models}
              onChange={(ids) => set({ models: ids })}
              getGroupLabel={(s) => ctx.userGroup?.[s]?.name || s}
            />
            {draft.models.length === 0 && <p className="mt-1.5 text-xs text-muted-foreground">{t('token_index.limitModelsEmptyHint')}</p>}
          </div>
        )}
        <ToggleField
          id="tk-limit-ip"
          label={t('token_index.limits_ip_whitelist_switch')}
          hint={t('token_index.limits_ip_whitelist_info')}
          checked={draft.limitIp}
          onChange={(c) => set({ limitIp: c, whitelist: c ? draft.whitelist : '' })}
        />
        {draft.limitIp && (
          <div className="pb-3">
            <Textarea
              rows={4}
              className="font-mono text-[13px]"
              value={draft.whitelist}
              onChange={(e) => set({ whitelist: e.target.value })}
              placeholder={'192.168.1.1\n10.0.0.0/8'}
            />
            <p className="mt-1.5 text-xs text-muted-foreground">{t('token_index.limits_ip_whitelist_helper')}</p>
          </div>
        )}
      </Section>
    </>
  );
}

AccessBody.propTypes = bodyProps;

// Organization key model limit: a named list, inline models, or nothing beyond the member layer.
function orgLimitPatch(access) {
  const { listId, models } = accessPayload(access);
  return { enabled: models.length > 0, models, access_list_id: listId };
}

const accessTab = {
  key: 'access',
  make: (tk) => {
    const lm = tk.setting.limits.limit_model_setting;
    const ip = tk.setting.limits.limits_ip_setting;
    return {
      group: tk.group || '',
      backup_group: tk.backup_group || '',
      billing_tag: tk.setting.billing_tag || '',
      limitModels: lm.enabled === true,
      models: lm.models || [],
      access: accessValueFrom(lm.access_list_id || 0, lm.enabled ? lm.models || [] : []),
      limitIp: ip.enabled === true,
      whitelist: (ip.whitelist || []).join('\n')
    };
  },
  patch: (d, tk, ctx) => ({
    group: d.group,
    backup_group: d.backup_group,
    setting: {
      billing_tag: d.billing_tag || null,
      limits: {
        limit_model_setting: ctx?.loadModelAccess
          ? orgLimitPatch(d.access)
          : { enabled: d.limitModels, models: d.limitModels ? d.models : [] },
        limits_ip_setting: {
          enabled: d.limitIp,
          whitelist: d.limitIp
            ? d.whitelist
                .split('\n')
                .map((s) => s.trim())
                .filter(Boolean)
            : []
        }
      }
    }
  }),
  Body: AccessBody
};

function AdvancedBody({ draft, setDraft, ctx }) {
  const { t } = useTranslation();
  const set = (patch) => setDraft((d) => ({ ...d, ...patch }));
  return (
    <>
      <ToggleField
        id="tk-heartbeat"
        label={t('token_index.heartbeat')}
        hint={t('token_index.heartbeatTip')}
        checked={draft.heartbeat}
        onChange={(c) => set({ heartbeat: c })}
      />
      {draft.heartbeat && (
        <Field label={t('token_index.heartbeatTimeout')} htmlFor="tk-hb-timeout" hint={t('token_index.heartbeatTimeoutHelperText')}>
          <Input
            id="tk-hb-timeout"
            type="number"
            className="w-40"
            value={draft.timeout}
            onChange={(e) => set({ timeout: e.target.value })}
          />
        </Field>
      )}
      {ctx.adminMode && (
        <Field label={t('token_index.transferToUser')} htmlFor="tk-owner" hint={t('token_index.transferToUserHelper')}>
          <Input id="tk-owner" type="number" className="w-40" value={draft.user_id} onChange={(e) => set({ user_id: e.target.value })} />
        </Field>
      )}
    </>
  );
}

AdvancedBody.propTypes = bodyProps;

const advancedTab = {
  key: 'advanced',
  make: (tk) => ({
    heartbeat: tk.setting.heartbeat.enabled === true,
    timeout: String(tk.setting.heartbeat.timeout_seconds ?? 30),
    user_id: String(tk.user_id ?? '')
  }),
  patch: (d, tk, { adminMode }) => ({
    ...(adminMode && d.user_id ? { user_id: parseInt(d.user_id, 10) } : {}),
    setting: { heartbeat: { enabled: d.heartbeat, timeout_seconds: parseInt(d.timeout, 10) || 30 } }
  }),
  Body: AdvancedBody
};

export const TOKEN_TABS = [{ key: 'overview', Body: OverviewBody }, basicsTab, accessTab, advancedTab];
export const TOKEN_TAB_KEYS = TOKEN_TABS.map((x) => x.key);
