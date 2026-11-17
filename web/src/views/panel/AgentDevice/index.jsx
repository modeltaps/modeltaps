import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useSearchParams } from 'react-router';
import { Bot, Check, Loader2, ShieldAlert, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select';
import { Field, InfoRow, InfoRows } from '@/components/objects';
import QuotaInput from '@/components/QuotaInput';
import { API } from 'utils/api';
import { showError, timestamp2string } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';
import { workbench } from 'routes/paths';
import ModelLimitSelector, { toLimitModels } from 'views/panel/Token/ModelLimitSelector';
import { DEFAULT_SETTING, EXPIRY_PRESETS, expiryFromPreset, quotaPatch } from 'views/panel/Token/tokenData';

// ==============================|| AGENT DEVICE AUTHORIZATION (AGENT-7) ||============================== //
// The page an agent's device authorization (RFC 8628) sends the user to: `/device?user_code=...`.
// The signed-in user checks which client is asking (name, IP, User-Agent as the agent sent them),
// chooses whose key it gets (personal or an organization they belong to), its spending limit,
// expiry and models, and approves or denies. The key is created on approval; the agent receives
// its secret on its next poll, so it never shows up here.

const NAME_ERRORS = new Set(['token_name_taken', 'token_name_reserved', 'token_name_too_long']);
const PERSONAL = 'personal';
const NO_PARENTS = [];

// "abcd efgh" → "ABCD-EFGH"; anything else is passed on as typed and refused by the server.
export function formatUserCode(raw) {
  const letters = String(raw || '')
    .toUpperCase()
    .replace(/[^A-Z]/g, '');
  return letters.length === 8 ? `${letters.slice(0, 4)}-${letters.slice(4)}` : String(raw || '').trim();
}

const fetchRequest = (value) => API.get(`/api/agent/device/${encodeURIComponent(formatUserCode(value))}`);

export default function AgentDevice() {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const { orgEnabled, organizations } = useOrg();
  const linkCode = searchParams.get('user_code') || '';
  const [code, setCode] = useState(() => formatUserCode(linkCode));
  const [request, setRequest] = useState(null);
  const [lookupError, setLookupError] = useState('');
  const [looking, setLooking] = useState(Boolean(linkCode));
  const [form, setForm] = useState({ owner: PERSONAL, name: '', quota: '', expiry: 'never', models: [] });
  const [nameError, setNameError] = useState('');
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(null); // { approved: bool, name }
  const [models, setModels] = useState([]);
  const [orgLimits, setOrgLimits] = useState({ owner: null, parents: [] });

  const owners = useMemo(
    () => [
      { value: PERSONAL, label: t('agentDevice.personal') },
      ...(orgEnabled ? organizations.map((o) => ({ value: String(o.id), label: o.name })) : [])
    ],
    [t, orgEnabled, organizations]
  );
  const owner = owners.find((o) => o.value === form.owner) || owners[0];

  // The answer to a lookup; state is only set from here, after the request.
  const showLookup = useCallback((res) => {
    const { success, message, data } = res.data;
    if (success) {
      setRequest(data);
      setForm((f) => ({ ...f, name: f.name || (data.client_name || '').slice(0, 30) }));
    } else {
      setRequest(null);
      setLookupError(message);
    }
  }, []);

  const lookup = async (value) => {
    if (!formatUserCode(value)) return;
    setLooking(true);
    setLookupError('');
    try {
      showLookup(await fetchRequest(value));
    } catch {
      // surfaced globally
    } finally {
      setLooking(false);
    }
  };

  // A link from the agent carries the code: look it up right away.
  useEffect(() => {
    if (!linkCode) return;
    fetchRequest(linkCode)
      .then(showLookup)
      .catch(() => {})
      .finally(() => setLooking(false));
  }, [linkCode, showLookup]);

  useEffect(() => {
    let alive = true;
    API.get('/api/available_model')
      .then((res) => alive && setModels(res.data.success ? toLimitModels(res.data.data) : []))
      .catch(() => alive && setModels([]));
    return () => {
      alive = false;
    };
  }, []);

  // An organization key stays inside the organization's approved models and the member's layer.
  useEffect(() => {
    if (form.owner === PERSONAL) return undefined;
    let alive = true;
    const owner = form.owner;
    API.get(`/api/org/${owner}/model_access`)
      .then((res) => {
        const scope = res.data.success ? res.data.data?.scope : null;
        if (alive) setOrgLimits({ owner, parents: [scope?.approved_models, scope?.effective_member].filter((l) => l?.length) });
      })
      .catch(() => alive && setOrgLimits({ owner, parents: [] }));
    return () => {
      alive = false;
    };
  }, [form.owner]);
  const parents = form.owner !== PERSONAL && orgLimits.owner === form.owner ? orgLimits.parents : NO_PARENTS;

  const answer = async (approve) => {
    if (!request || busy) return;
    const userCode = request.user_code;
    let url = `/api/agent/device/${encodeURIComponent(userCode)}/deny`;
    let body;
    if (approve) {
      const quota = quotaPatch({ quota: form.quota, period: '' }, t);
      if (quota.error) {
        showError(quota.error);
        return;
      }
      const setting = structuredClone(DEFAULT_SETTING);
      setting.limits.limit_model_setting = { enabled: form.models.length > 0, models: form.models };
      body = {
        name: form.name.trim(),
        expired_time: expiryFromPreset(form.expiry),
        unlimited_quota: quota.unlimited_quota,
        remain_quota: quota.remain_quota,
        group: '',
        backup_group: '',
        setting
      };
      url =
        form.owner === PERSONAL
          ? `/api/agent/device/${encodeURIComponent(userCode)}/approve`
          : `/api/org/${form.owner}/agent/device/${encodeURIComponent(userCode)}/approve`;
    }
    setBusy(true);
    try {
      const res = await API.post(url, body);
      const { success, message, code: errorCode, data } = res.data;
      if (success) setDone({ approved: approve, name: data?.name || body?.name || '' });
      else if (NAME_ERRORS.has(errorCode)) setNameError(message);
      else showError(message);
    } catch {
      // surfaced globally
    } finally {
      setBusy(false);
    }
  };

  if (done) {
    const keysPath =
      form.owner === PERSONAL ? workbench.keys() : workbench.keys(organizations.find((o) => String(o.id) === form.owner)?.slug);
    return (
      <div className="mx-auto max-w-lg">
        <Card className="p-6" data-testid="device-done">
          <div className="flex items-center gap-2">
            {done.approved ? <Check className="size-5 text-success" /> : <X className="size-5 text-muted-foreground" />}
            <h2 className="text-base font-semibold">{done.approved ? t('agentDevice.approvedTitle') : t('agentDevice.deniedTitle')}</h2>
          </div>
          <p className="mt-2 text-sm text-muted-foreground">
            {done.approved ? t('agentDevice.approvedHint', { name: done.name }) : t('agentDevice.deniedHint')}
          </p>
          {done.approved && (
            <Button asChild variant="outline" className="mt-4 rounded-lg">
              <Link to={keysPath}>{t('agentDevice.openKeys')}</Link>
            </Button>
          )}
        </Card>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-lg space-y-4">
      <Card className="p-6">
        <div className="flex items-center gap-2">
          <Bot className="size-5 text-muted-foreground" />
          <h2 className="text-base font-semibold">{t('agentDevice.title')}</h2>
        </div>
        <p className="mt-1 text-sm text-muted-foreground">{t('agentDevice.description')}</p>
        <form
          className="mt-4 flex flex-col gap-2 sm:flex-row"
          onSubmit={(e) => {
            e.preventDefault();
            lookup(code);
          }}
        >
          <Input
            id="device-user-code"
            aria-label={t('agentDevice.codeLabel')}
            autoComplete="off"
            spellCheck={false}
            className="font-mono uppercase tracking-widest"
            placeholder="XXXX-XXXX"
            value={code}
            aria-invalid={Boolean(lookupError)}
            onChange={(e) => {
              setLookupError('');
              setCode(e.target.value);
            }}
          />
          <Button type="submit" variant="outline" disabled={looking || !code.trim()}>
            {looking && <Loader2 className="size-4 animate-spin" />}
            {t('agentDevice.lookup')}
          </Button>
        </form>
        {lookupError && (
          <p className="mt-2 text-sm text-destructive" data-testid="device-lookup-error">
            {lookupError}
          </p>
        )}
      </Card>

      {request && (
        <Card className="p-6" data-testid="device-request">
          <h3 className="text-sm font-semibold">{t('agentDevice.requestTitle')}</h3>
          <InfoRows className="mt-2">
            <InfoRow label={t('agentDevice.client')}>
              {request.client_name || <span className="text-muted-foreground">{t('agentDevice.unnamed')}</span>}
            </InfoRow>
            <InfoRow label={t('agentDevice.ip')} mono>
              {request.request_ip || '—'}
            </InfoRow>
            <InfoRow label={t('agentDevice.userAgent')} mono>
              {request.request_user_agent || '—'}
            </InfoRow>
            <InfoRow label={t('agentDevice.requestedAt')}>{timestamp2string(request.created_time)}</InfoRow>
            <InfoRow label={t('agentDevice.openUntil')}>{timestamp2string(request.expires_at)}</InfoRow>
          </InfoRows>
          <p
            role="note"
            className="mt-3 flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/[0.06] px-3 py-2 text-xs text-foreground"
          >
            <ShieldAlert className="mt-0.5 size-3.5 shrink-0 text-amber-600 dark:text-amber-500" aria-hidden="true" />
            <span>{t('agentDevice.warning')}</span>
          </p>

          <div className="mt-2">
            {owners.length > 1 && (
              <Field label={t('agentDevice.owner')} htmlFor="device-owner">
                <Select value={owner.value} onValueChange={(v) => setForm((f) => ({ ...f, owner: v, models: [] }))}>
                  <SelectTrigger id="device-owner" data-testid="device-owner-trigger">
                    <span className="truncate">{owner.label}</span>
                  </SelectTrigger>
                  <SelectContent>
                    {owners.map((o) => (
                      <SelectItem key={o.value} value={o.value}>
                        {o.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
            )}
            <Field label={t('token_index.name')} htmlFor="device-name" error={nameError}>
              <Input
                id="device-name"
                autoComplete="off"
                maxLength={30}
                value={form.name}
                placeholder={t('apiKey.new.namePlaceholder')}
                aria-invalid={Boolean(nameError)}
                onChange={(e) => {
                  setNameError('');
                  setForm((f) => ({ ...f, name: e.target.value }));
                }}
              />
            </Field>
            <div className="grid grid-cols-2 gap-3">
              <Field label={t('apiKey.new.limit')} optional className="[&+&]:border-t-0">
                <QuotaInput
                  name="device-quota"
                  value={form.quota}
                  placeholder={t('token_index.noLimit')}
                  allowEmpty
                  onChange={(v) => setForm((f) => ({ ...f, quota: v }))}
                />
              </Field>
              <Field label={t('apiKey.new.expiry')} htmlFor="device-expiry" className="[&+&]:border-t-0">
                <Select value={form.expiry} onValueChange={(v) => setForm((f) => ({ ...f, expiry: v }))}>
                  <SelectTrigger id="device-expiry">
                    <span className="truncate">{t(`apiKey.expiry.${form.expiry}`)}</span>
                  </SelectTrigger>
                  <SelectContent>
                    {EXPIRY_PRESETS.map((p) => (
                      <SelectItem key={p} value={p}>
                        {t(`apiKey.expiry.${p}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
            </div>
            <Field label={t('agentDevice.models')} htmlFor="device-models" hint={t('agentDevice.modelsHint')}>
              <ModelLimitSelector
                id="device-models"
                models={models}
                value={form.models}
                parents={parents}
                onChange={(ids) => setForm((f) => ({ ...f, models: ids }))}
              />
            </Field>
          </div>

          <div className="mt-4 flex justify-end gap-2">
            <Button type="button" variant="outline" className="rounded-lg" onClick={() => answer(false)} disabled={busy}>
              {t('agentDevice.deny')}
            </Button>
            <Button type="button" className="rounded-lg" onClick={() => answer(true)} disabled={busy}>
              {busy && <Loader2 className="size-4 animate-spin" />}
              {t('agentDevice.approve')}
            </Button>
          </div>
        </Card>
      )}
    </div>
  );
}
