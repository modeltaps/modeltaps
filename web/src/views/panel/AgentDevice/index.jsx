import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { useSearchParams } from 'react-router';
import { ChevronDown, Loader2, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select';
import { Field, OptionList, OptionRow } from '@/components/objects';
import QuotaInput from '@/components/QuotaInput';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { renderQuota, showError } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';
import { agentDevice, workbench } from 'routes/paths';
import ModelLimitSelector, { toLimitModels } from 'views/panel/Token/ModelLimitSelector';
import { DEFAULT_SETTING, EXPIRY_PRESETS, expiryFromPreset, quotaPatch } from 'views/panel/Token/tokenData';
import ApprovedStatus from './ApprovedStatus';
import CodeCells, { CODE_LENGTH, codeLetters } from './CodeCells';
import { Heading, Outcome, Shell } from './DeviceShell';
import { Handshake, RequestFacts } from './RequestParts';
import { recognizeClient } from './agentClient';

// ==============================|| AGENT DEVICE AUTHORIZATION (AGENT-7) ||============================== //
// The page an agent's device authorization (RFC 8628) sends the user to: `/device?user_code=...`.
// A focused page outside the console chrome, like other consent screens: the brand and the signed-in
// account (with a way to switch) on top, one step in the middle.
//
//   enter    type the code the agent shows (skipped when the link carries it)
//   confirm  the agent and the account it connects to; the code to compare with the agent's; the
//            client, network and time of the request; whose key it is and what it may do
//   done     approved: follows the request until the agent has its key; denied; or it ran out
//
// The key is created on approval; the agent receives its secret on its next poll, so it never
// shows up here.

const NAME_ERRORS = new Set(['token_name_taken', 'token_name_reserved', 'token_name_too_long']);
const PERSONAL = 'personal';
const NO_PARENTS = [];
// Approve stays off for a moment after the request shows, so a click meant for something else
// (or a page layered over this one) cannot land on it.
export const APPROVE_DELAY_MS = 1000;

// "abcd efgh" → "ABCD-EFGH"; anything else is passed on as typed and refused by the server.
export function formatUserCode(raw) {
  const letters = codeLetters(raw);
  return letters.length === CODE_LENGTH ? `${letters.slice(0, 4)}-${letters.slice(4)}` : String(raw || '').trim();
}

const fetchRequest = (value) => API.get(`/api/agent/device/${encodeURIComponent(formatUserCode(value))}`);

// The clock while a request is open (ms), ticking each second.
function useNow(active) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!active) return undefined;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [active]);
  return now;
}

// True once `delay` has passed since `key` (non-empty) last changed.
function useArmed(key, delay) {
  const [armedKey, setArmedKey] = useState(null);
  useEffect(() => {
    if (!key) return undefined;
    const id = setTimeout(() => setArmedKey(key), delay);
    return () => clearTimeout(id);
  }, [key, delay]);
  return Boolean(key) && armedKey === key;
}

// One line of the "what the agent gets" summary.
function SummaryRow({ label, children }) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-2.5 text-sm [&+&]:border-t [&+&]:border-border/60">
      <span className="shrink-0 text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate text-right">{children}</span>
    </div>
  );
}

SummaryRow.propTypes = { label: PropTypes.node, children: PropTypes.node };

export default function AgentDevice() {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const { orgEnabled, organizations } = useOrg();
  const user = useSelector((state) => state.account.user);
  const linkCode = searchParams.get('user_code') || '';
  const [code, setCode] = useState(() => codeLetters(linkCode));
  const [request, setRequest] = useState(null);
  const [lookupError, setLookupError] = useState('');
  const [looking, setLooking] = useState(Boolean(linkCode));
  const [form, setForm] = useState({ owner: PERSONAL, name: '', quota: '', expiry: 'never', models: [] });
  const [editing, setEditing] = useState(false);
  const [nameError, setNameError] = useState('');
  const [busy, setBusy] = useState(null); // 'approve' | 'deny'
  const [done, setDone] = useState(null); // { approved: bool, name }
  const [models, setModels] = useState([]);
  const [orgLimits, setOrgLimits] = useState({ owner: null, parents: [] });
  const now = useNow(Boolean(request));
  const secondsLeft = request ? Math.max(0, Math.round(request.expires_at - now / 1000)) : null;
  const expired = request && secondsLeft === 0;
  const armed = useArmed(request?.user_code, APPROVE_DELAY_MS);

  const owners = useMemo(
    () => [
      { value: PERSONAL, label: t('agentDevice.personal') },
      ...(orgEnabled ? organizations.map((o) => ({ value: String(o.id), label: o.name })) : [])
    ],
    [t, orgEnabled, organizations]
  );

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
    if (codeLetters(value).length !== CODE_LENGTH || looking) return;
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

  const startOver = () => {
    setRequest(null);
    setCode('');
    setLookupError('');
    setEditing(false);
    setNameError('');
  };

  const answer = async (approve) => {
    if (!request || busy) return;
    const userCode = request.user_code;
    let url = `/api/agent/device/${encodeURIComponent(userCode)}/deny`;
    let body;
    if (approve) {
      const quota = quotaPatch({ quota: form.quota, period: '' }, t);
      if (quota.error) {
        setEditing(true);
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
    setBusy(approve ? 'approve' : 'deny');
    try {
      const res = await API.post(url, body);
      const { success, message, code: errorCode, data } = res.data;
      if (success) setDone({ approved: approve, name: data?.name || body?.name || '' });
      else if (NAME_ERRORS.has(errorCode)) {
        setEditing(true);
        setNameError(message);
      } else showError(message);
    } catch {
      // surfaced globally
    } finally {
      setBusy(null);
    }
  };

  // ---- done ----
  if (done) {
    const keysPath =
      form.owner === PERSONAL ? workbench.keys() : workbench.keys(organizations.find((o) => String(o.id) === form.owner)?.slug);
    return (
      <Shell>
        {done.approved ? (
          <ApprovedStatus userCode={request.user_code} keyName={done.name} keysPath={keysPath} />
        ) : (
          <Outcome icon={X} title={t('agentDevice.deniedTitle')} testId="device-done">
            <p>{t('agentDevice.deniedHint')}</p>
            <p className="mt-1">{t('agentDevice.closeHint')}</p>
          </Outcome>
        )}
      </Shell>
    );
  }

  // ---- the request ran out while open ----
  if (expired) {
    return (
      <Shell>
        <Outcome icon={X} title={t('agentDevice.expiredTitle')} testId="device-expired">
          <p>{t('agentDevice.expiredHint')}</p>
          <Button variant="outline" className="mt-6" onClick={startOver}>
            {t('agentDevice.enterAnother')}
          </Button>
        </Outcome>
      </Shell>
    );
  }

  // ---- enter ----
  if (!request) {
    const complete = codeLetters(code).length === CODE_LENGTH;
    return (
      <Shell>
        <Heading title={t('agentDevice.title')}>{t('agentDevice.description')}</Heading>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            lookup(code);
          }}
        >
          <CodeCells
            id="device-user-code"
            label={t('agentDevice.codeLabel')}
            value={code}
            invalid={Boolean(lookupError)}
            autoFocus={!linkCode}
            onChange={(v) => {
              setLookupError('');
              setCode(v);
            }}
            onComplete={lookup}
          />
          <p className={cn('mt-3 min-h-5 text-center text-sm', lookupError ? 'text-destructive' : 'text-muted-foreground')}>
            {lookupError ? <span data-testid="device-lookup-error">{lookupError}</span> : t('agentDevice.codeHint')}
          </p>
          <Button type="submit" className="mt-6 h-10 w-full" disabled={looking || !complete}>
            {looking && <Loader2 className="size-4 animate-spin" />}
            {t('agentDevice.lookup')}
          </Button>
        </form>
      </Shell>
    );
  }

  // ---- confirm ----
  const client = request.client_name || t('agentDevice.unnamed');
  const known = recognizeClient(request.client_name, request.request_user_agent);
  const ownerOrg = form.owner === PERSONAL ? null : organizations.find((o) => String(o.id) === form.owner);
  const quotaText = form.quota !== '' ? renderQuota(form.quota) : t('token_index.noLimit');
  const modelsText = form.models.length ? t('agentDevice.modelsCount', { count: form.models.length }) : t('agentDevice.allModels');
  const pickOwner = (v) => setForm((f) => (f.owner === v ? f : { ...f, owner: v, models: [] }));

  return (
    <Shell returnTo={`${agentDevice()}?user_code=${encodeURIComponent(request.user_code)}`}>
      <Handshake client={known} user={user} org={ownerOrg} />
      <Heading title={t('agentDevice.confirmTitle', { client })}>
        <span data-testid="device-identity">
          {known
            ? t('agentDevice.recognized', { name: [known.name, known.version].filter(Boolean).join(' ') })
            : t('agentDevice.unrecognized')}
        </span>
      </Heading>

      <section className="rounded-xl border border-border bg-card p-5 sm:p-6" data-testid="device-request">
        <div className="text-center">
          <CodeCells readOnly label={t('agentDevice.codeLabel')} value={request.user_code} />
          <p className="mt-3 text-sm text-muted-foreground">{t('agentDevice.compareHint')}</p>
        </div>
        <div className="mt-5 border-t border-border/60 pt-4">
          <RequestFacts request={request} secondsLeft={secondsLeft} now={now} />
        </div>
      </section>

      {owners.length > 1 && (
        <section className="mt-4">
          <h2 className="mb-2 px-1 text-sm font-medium">{t('agentDevice.owner')}</h2>
          <OptionList label={t('agentDevice.owner')}>
            {owners.map((o) => (
              <OptionRow
                key={o.value}
                kind="radio"
                selected={form.owner === o.value}
                title={o.label}
                meta={o.value === PERSONAL ? t('agentDevice.personalHint') : t('agentDevice.orgHint')}
                onClick={() => pickOwner(o.value)}
              />
            ))}
          </OptionList>
        </section>
      )}

      <section className="mt-4 rounded-xl border border-border bg-card px-5 py-3 sm:px-6">
        <div className="flex items-center justify-between gap-4 py-2">
          <h2 className="text-sm font-medium">{t('agentDevice.grantTitle')}</h2>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="-mr-2 text-muted-foreground"
            aria-expanded={editing}
            onClick={() => setEditing((v) => !v)}
          >
            {t(editing ? 'agentDevice.doneEditing' : 'agentDevice.adjust')}
            <ChevronDown className={cn('transition-transform', editing && 'rotate-180')} />
          </Button>
        </div>

        {!editing ? (
          <div data-testid="device-summary">
            <SummaryRow label={t('token_index.name')}>{form.name.trim() || client}</SummaryRow>
            <SummaryRow label={t('apiKey.new.limit')}>{quotaText}</SummaryRow>
            <SummaryRow label={t('apiKey.new.expiry')}>{t(`apiKey.expiry.${form.expiry}`)}</SummaryRow>
            <SummaryRow label={t('agentDevice.models')}>{modelsText}</SummaryRow>
          </div>
        ) : (
          <div className="pb-1">
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
        )}
      </section>

      <p className="mt-4 px-1 text-xs leading-relaxed text-muted-foreground">{t('agentDevice.warning')}</p>

      <div className="mt-6 grid grid-cols-2 gap-3">
        <Button type="button" variant="outline" className="h-10" onClick={() => answer(false)} disabled={Boolean(busy)}>
          {busy === 'deny' && <Loader2 className="size-4 animate-spin" />}
          {t('agentDevice.deny')}
        </Button>
        <Button type="button" className="h-10" onClick={() => answer(true)} disabled={Boolean(busy) || !armed}>
          {busy === 'approve' && <Loader2 className="size-4 animate-spin" />}
          {t('agentDevice.approve')}
        </Button>
      </div>
    </Shell>
  );
}
