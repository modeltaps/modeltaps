import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { Link, useSearchParams } from 'react-router';
import { Check, ChevronDown, Loader2, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select';
import { Field } from '@/components/objects';
import { BrandLockup } from '@/components/chrome/Logo';
import UserAvatar, { accountDisplayName } from '@/components/chrome/avatar';
import QuotaInput from '@/components/QuotaInput';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { renderQuota, showError } from 'utils/common';
import { brandName } from 'utils/brand';
import { useOrg } from 'contexts/OrgContext';
import { workbench } from 'routes/paths';
import ModelLimitSelector, { toLimitModels } from 'views/panel/Token/ModelLimitSelector';
import { DEFAULT_SETTING, EXPIRY_PRESETS, expiryFromPreset, quotaPatch } from 'views/panel/Token/tokenData';
import CodeCells, { CODE_LENGTH, codeLetters } from './CodeCells';

// ==============================|| AGENT DEVICE AUTHORIZATION (AGENT-7) ||============================== //
// The page an agent's device authorization (RFC 8628) sends the user to: `/device?user_code=...`.
// A focused page outside the console chrome, like other consent screens: the brand and the signed-in
// account on top, one step in the middle.
//
//   enter    type the code the agent shows (skipped when the link carries it)
//   confirm  compare the code, see who is asking, choose whose key it is and what it may do
//   done     approved or denied: go back to the agent; or the request ran out
//
// The key is created on approval; the agent receives its secret on its next poll, so it never
// shows up here.

const NAME_ERRORS = new Set(['token_name_taken', 'token_name_reserved', 'token_name_too_long']);
const PERSONAL = 'personal';
const NO_PARENTS = [];

// "abcd efgh" → "ABCD-EFGH"; anything else is passed on as typed and refused by the server.
export function formatUserCode(raw) {
  const letters = codeLetters(raw);
  return letters.length === CODE_LENGTH ? `${letters.slice(0, 4)}-${letters.slice(4)}` : String(raw || '').trim();
}

const fetchRequest = (value) => API.get(`/api/agent/device/${encodeURIComponent(formatUserCode(value))}`);

// Seconds left until `expiresAt` (Unix seconds), ticking while the request is open.
function useSecondsLeft(expiresAt) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!expiresAt) return undefined;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [expiresAt]);
  return expiresAt ? Math.max(0, Math.round(expiresAt - now / 1000)) : null;
}

const clock = (s) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;

function Shell({ children }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const user = useSelector((state) => state.account.user);
  const name = brandName(siteInfo.system_name);
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="flex h-14 shrink-0 items-center justify-between gap-4 px-4 sm:px-6">
        <Link to={workbench.dashboard()} className="rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          {siteInfo.logo ? (
            <img src={siteInfo.logo} alt={name} className="h-6 object-contain" />
          ) : (
            <BrandLockup text={name} wordmark className="text-foreground" />
          )}
        </Link>
        {user && (
          <span className="flex min-w-0 items-center gap-2 text-sm text-muted-foreground" data-testid="device-account">
            <span className="hidden sm:inline">{t('agentDevice.signedInAs')}</span>
            <UserAvatar user={user} className="size-6" textClass="text-[10px]" />
            <span className="truncate font-medium text-foreground">{accountDisplayName(user)}</span>
          </span>
        )}
      </header>
      <main className="flex flex-1 justify-center px-4 pb-16 pt-8 sm:pt-[12vh]">
        <div className="w-full max-w-[440px]">{children}</div>
      </main>
    </div>
  );
}

Shell.propTypes = { children: PropTypes.node };

function Heading({ title, children }) {
  return (
    <div className="mb-6 text-center">
      <h1 className="text-xl font-medium tracking-[-0.01em]">{title}</h1>
      {children && <p className="mt-2 text-sm text-muted-foreground">{children}</p>}
    </div>
  );
}

Heading.propTypes = { title: PropTypes.node, children: PropTypes.node };

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

function Outcome({ icon: Icon, tone, title, children, testId }) {
  return (
    <div className="text-center" data-testid={testId}>
      <span
        className={cn(
          'mx-auto mb-5 flex size-12 items-center justify-center rounded-full',
          tone === 'good' ? 'bg-success/10 text-success' : 'bg-muted text-muted-foreground'
        )}
      >
        <Icon className="size-6" strokeWidth={1.75} />
      </span>
      <h1 className="text-xl font-medium tracking-[-0.01em]">{title}</h1>
      <div className="mt-2 text-sm text-muted-foreground">{children}</div>
    </div>
  );
}

Outcome.propTypes = {
  icon: PropTypes.elementType.isRequired,
  tone: PropTypes.oneOf(['good']),
  title: PropTypes.node,
  children: PropTypes.node,
  testId: PropTypes.string
};

export default function AgentDevice() {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const { orgEnabled, organizations } = useOrg();
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
  const secondsLeft = useSecondsLeft(request?.expires_at);
  const expired = request && secondsLeft === 0;

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
          <Outcome icon={Check} tone="good" title={t('agentDevice.approvedTitle')} testId="device-done">
            <p>{t('agentDevice.approvedHint', { name: done.name })}</p>
            <p className="mt-1">{t('agentDevice.closeHint')}</p>
            <Button asChild variant="outline" className="mt-6">
              <Link to={keysPath}>{t('agentDevice.openKeys')}</Link>
            </Button>
          </Outcome>
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
  const source = [request.request_ip, request.request_user_agent].filter(Boolean).join(' · ');
  const quotaText = form.quota !== '' ? renderQuota(form.quota) : t('token_index.noLimit');
  const modelsText = form.models.length ? t('agentDevice.modelsCount', { count: form.models.length }) : t('agentDevice.allModels');

  return (
    <Shell>
      <Heading title={t('agentDevice.confirmTitle', { client })} />

      <section className="rounded-xl border border-border bg-card p-5 sm:p-6" data-testid="device-request">
        <div className="text-center">
          <CodeCells readOnly label={t('agentDevice.codeLabel')} value={request.user_code} />
          <p className="mt-3 text-sm text-muted-foreground">{t('agentDevice.compareHint')}</p>
        </div>

        <dl className="mt-5 space-y-1 border-t border-border/60 pt-4 text-xs text-muted-foreground">
          <div className="flex justify-between gap-4">
            <dt>{t('agentDevice.source')}</dt>
            <dd className="min-w-0 truncate font-mono" title={source}>
              {source || '—'}
            </dd>
          </div>
          <div className="flex justify-between gap-4">
            <dt>{t('agentDevice.openFor')}</dt>
            <dd className="font-mono tabular-nums">{secondsLeft !== null ? clock(secondsLeft) : '—'}</dd>
          </div>
        </dl>
      </section>

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
            {owners.length > 1 && <SummaryRow label={t('agentDevice.owner')}>{owner.label}</SummaryRow>}
            <SummaryRow label={t('token_index.name')}>{form.name.trim() || client}</SummaryRow>
            <SummaryRow label={t('apiKey.new.limit')}>{quotaText}</SummaryRow>
            <SummaryRow label={t('apiKey.new.expiry')}>{t(`apiKey.expiry.${form.expiry}`)}</SummaryRow>
            <SummaryRow label={t('agentDevice.models')}>{modelsText}</SummaryRow>
          </div>
        ) : (
          <div className="pb-1">
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
        )}
      </section>

      <p className="mt-4 px-1 text-xs leading-relaxed text-muted-foreground">{t('agentDevice.warning')}</p>

      <div className="mt-6 grid grid-cols-2 gap-3">
        <Button type="button" variant="outline" className="h-10" onClick={() => answer(false)} disabled={Boolean(busy)}>
          {busy === 'deny' && <Loader2 className="size-4 animate-spin" />}
          {t('agentDevice.deny')}
        </Button>
        <Button type="button" className="h-10" onClick={() => answer(true)} disabled={Boolean(busy)}>
          {busy === 'approve' && <Loader2 className="size-4 animate-spin" />}
          {t('agentDevice.approve')}
        </Button>
      </div>
    </Shell>
  );
}
