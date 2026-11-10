import { useCallback, useContext, useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Check, PlugZap, Power, Trash2 } from 'lucide-react';

import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { LoadStatusContext } from 'contexts/StatusContext';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { toast } from '@/components/ui/sonner';
import { CollectionCard, OptionList, StatusDot } from '@/components/settings/cards';
import { TextRow, ToggleRow } from '../parts';
import { INVITE_CODE_URL } from '../registry';
import OidcProviderSheet from '../OidcProviderSheet';
import OptionCard, { bindSwitch, cardText } from './OptionCard';

// ==============================|| PLATFORM SETTINGS — 登录与注册 ||============================== //
// The account system (built-in accounts or an external identity provider) decides which cards
// apply: built-in shows the site's own login methods and the social logins, external shows the
// OIDC providers and the admin emergency login. The choice follows the saved value, so an unsaved
// switch does not reshuffle the page; saving it asks for confirmation.

const tk = (k) => `setting_index.loginMethods.${k}`;
const sysKey = (k) => `setting_index.systemSettings.${k}`;

export const isExternalSaved = (ctx) => ctx.origin.AccountSystem === 'external';

// Confirmation dialog driven by a promise: `ask()` resolves true when the user confirms.
function useConfirm() {
  const [state, setState] = useState(null);
  const resolver = useRef(null);
  const ask = useCallback(
    (content) =>
      new Promise((resolve) => {
        resolver.current = resolve;
        setState(content);
      }),
    []
  );
  const close = (ok) => {
    resolver.current?.(ok);
    resolver.current = null;
    setState(null);
  };
  return { state, ask, close };
}

function ConfirmDialog({ state, close }) {
  const { t } = useTranslation();
  return (
    <Dialog open={!!state} onOpenChange={(o) => !o && close(false)}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{state?.title}</DialogTitle>
          <DialogDescription>{state?.description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => close(false)}>
            {t('common.cancel')}
          </Button>
          <Button variant={state?.destructive ? 'destructive' : 'default'} onClick={() => close(true)}>
            {t('common.ok')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

ConfirmDialog.propTypes = { state: PropTypes.object, close: PropTypes.func.isRequired };

function AccountSystemChoice({ value, current, title, description, onSelect }) {
  const active = current === value;
  return (
    <button
      type="button"
      role="radio"
      aria-checked={active}
      onClick={() => !active && onSelect(value)}
      className={cn('flex flex-1 items-start gap-3 px-4 py-3.5 text-left transition-colors', active ? 'bg-muted' : 'hover:bg-muted/60')}
    >
      <span
        className={cn(
          'mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-full border',
          active ? 'border-foreground bg-foreground text-background' : 'border-muted-foreground'
        )}
      >
        {active && <Check className="size-3" />}
      </span>
      <span className="space-y-1">
        <span className="block text-sm font-medium">{title}</span>
        <span className="block text-xs leading-relaxed text-muted-foreground">{description}</span>
      </span>
    </button>
  );
}

AccountSystemChoice.propTypes = {
  value: PropTypes.string,
  current: PropTypes.string,
  title: PropTypes.node,
  description: PropTypes.node,
  onSelect: PropTypes.func
};

export function AccountSystemCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const confirm = useConfirm();
  const value = inputs.AccountSystem === 'external' ? 'external' : 'builtin';
  const external = isExternalSaved(ctx);

  const beforeSave = () =>
    confirm.ask(
      value === 'external'
        ? { title: t(tk('switchToExternal')), description: t(tk('switchToExternalDesc')) }
        : { title: t(tk('switchToBuiltin')), description: t(tk('switchToBuiltinDesc')) }
    );

  return (
    <OptionCard
      ctx={ctx}
      id="account-system"
      title={t(tk('accountSystem'))}
      description={t(tk('accountSystemDesc'))}
      options={{ beforeSave }}
    >
      <div
        role="radiogroup"
        aria-label={t(tk('accountSystem'))}
        className="divide-y divide-border overflow-hidden rounded-xl border border-border"
      >
        <AccountSystemChoice
          value="builtin"
          current={value}
          title={t(tk('builtin'))}
          description={t(tk('builtinDesc'))}
          onSelect={(v) => setField('AccountSystem', v)}
        />
        <AccountSystemChoice
          value="external"
          current={value}
          title={t(tk('external'))}
          description={t(tk('externalDesc'))}
          onSelect={(v) => setField('AccountSystem', v)}
        />
      </div>
      {external && (
        <p className="text-xs text-muted-foreground">
          {t(tk('managedByProvider'))}: {t(tk('managedByProviderHint'))}
        </p>
      )}
      <ConfirmDialog state={confirm.state} close={confirm.close} />
    </OptionCard>
  );
}

export function LoginMethodsCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, origin } = ctx;
  const confirm = useConfirm();
  const sys = (k) => t(sysKey(k));
  const smtpConfigured = Boolean(inputs.SMTPServer && inputs.SMTPAccount);
  if (isExternalSaved(ctx)) return null;

  // Turning password login off locks ordinary users out of it; ask before saving that.
  const beforeSave = () =>
    origin.PasswordLoginEnabled === 'true' && inputs.PasswordLoginEnabled !== 'true'
      ? confirm.ask({ title: t(tk('passwordOffTitle')), description: t(tk('passwordOffDesc')), destructive: true })
      : true;

  const row = (key, label, description, extra) => (
    <ToggleRow id={key} label={label} description={description} {...bindSwitch(ctx, key)}>
      {extra}
    </ToggleRow>
  );

  return (
    <OptionCard ctx={ctx} id="login-methods" options={{ beforeSave }}>
      <OptionList>
        {row('PasswordLoginEnabled', sys('configureLoginRegister.passwordLogin'), sys('configureLoginRegister.passwordLoginDescription'))}
        {row(
          'EmailCodeLoginEnabled',
          t(tk('emailCodeLogin')),
          smtpConfigured ? t(tk('emailCodeLoginDesc')) : t(tk('emailCodeLoginNeedsSmtp'))
        )}
        {row('PasskeyLoginEnabled', t(tk('passkeyLogin')), t(tk('passkeyLoginDesc')))}
        {row('RegisterEnabled', sys('configureLoginRegister.registerEnabled'), sys('configureLoginRegister.registerEnabledDescription'))}
        {row(
          'PasswordRegisterEnabled',
          sys('configureLoginRegister.passwordRegister'),
          sys('configureLoginRegister.passwordRegisterDescription')
        )}
        {row(
          'EmailVerificationEnabled',
          sys('configureLoginRegister.emailVerification'),
          sys('configureLoginRegister.emailVerificationDescription')
        )}
        {row(
          'InviteCodeRegisterEnabled',
          sys('configureLoginRegister.inviteCodeRegister'),
          <>
            {sys('configureLoginRegister.inviteCodeRegisterDescription')}{' '}
            <Link to={INVITE_CODE_URL} className="text-primary hover:underline">
              {t('settingsUi.platform.actions.manageInviteCodes')}
            </Link>
          </>
        )}
        {row(
          'TurnstileCheckEnabled',
          sys('configureLoginRegister.turnstileCheck'),
          sys('configureLoginRegister.turnstileCheckDescription')
        )}
      </OptionList>
      <ConfirmDialog state={confirm.state} close={confirm.close} />
    </OptionCard>
  );
}

export function OidcProvidersCard({ ctx }) {
  const { t } = useTranslation();
  const loadStatus = useContext(LoadStatusContext);
  const external = isExternalSaved(ctx);
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const ok = (k) => `setting_index.oidcProviders.${k}`;

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/oidc_provider/');
      const { success, message, data } = res.data;
      if (success) setItems(data || []);
      else showError(message);
    } catch (error) {
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (external) fetchData();
  }, [external, fetchData]);

  if (!external) return null;

  const afterChange = async () => {
    await fetchData();
    if (loadStatus) await loadStatus();
  };

  const toggleStatus = async (item) => {
    try {
      const res = await API.put(`/api/oidc_provider/${item.id}/status`, { enabled: !item.enabled });
      if (res.data?.success) await afterChange();
      else showError(res.data?.message);
    } catch (error) {
      /* surfaced by the API interceptor */
    }
  };

  const test = async (item) => {
    try {
      const res = await API.post(`/api/oidc_provider/${item.id}/test`);
      if (res.data?.success) toast.success(res.data.message || t(ok('testOk')));
      else showError(res.data?.message);
    } catch (error) {
      /* the discovery error is surfaced by the API interceptor */
    }
  };

  // A 409 (identities still bound to the provider) is surfaced by the interceptor; the dialog stays.
  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      const res = await API.delete(`/api/oidc_provider/${deleteTarget.id}`);
      if (res.data?.success) {
        toast.success(t('common.deleteSuccess'));
        setDeleteTarget(null);
        await afterChange();
      } else {
        showError(res.data?.message);
      }
    } catch (error) {
      /* surfaced by the API interceptor */
    } finally {
      setDeleting(false);
    }
  };

  const open = (item) => {
    setEditing(item);
    setSheetOpen(true);
  };

  return (
    <>
      <CollectionCard
        title={cardText(t, 'oidc-providers', 'title')}
        description={t(tk('providersDesc'))}
        highlight="oidc-providers"
        addLabel={t(ok('createButton'))}
        onAdd={() => open(null)}
        loading={loading}
        empty={t(ok('emptyHint'))}
        rows={items}
        onRowClick={open}
        columns={[
          { key: 'display_name', header: t(ok('headLabels.displayName')), cell: (item) => item.display_name || item.slug },
          {
            key: 'slug',
            header: t(ok('headLabels.slug')),
            cell: (item) => <span className="font-mono text-xs text-muted-foreground">{item.slug}</span>
          },
          {
            key: 'issuer',
            header: t(ok('headLabels.issuer')),
            className: 'max-w-[18rem]',
            cell: (item) => (
              <span className="block truncate font-mono text-xs text-muted-foreground" title={item.issuer}>
                {item.issuer || '-'}
              </span>
            )
          },
          { key: 'status', header: t(ok('headLabels.status')), cell: (item) => <StatusDot on={!!item.enabled} /> }
        ]}
        menu={(item) => [
          { label: t(ok('test')), icon: PlugZap, onClick: () => test(item) },
          {
            label: item.enabled ? t('settingsUi.collection.disable') : t('settingsUi.collection.enable'),
            icon: Power,
            onClick: () => toggleStatus(item)
          },
          { label: t('common.delete'), icon: Trash2, destructive: true, separator: true, onClick: () => setDeleteTarget(item) }
        ]}
      />
      <OidcProviderSheet
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        editing={editing}
        serverAddress={ctx.inputs.ServerAddress || ''}
        onSaved={async () => {
          setSheetOpen(false);
          setEditing(null);
          await afterChange();
        }}
      />
      <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('common.delete')}</DialogTitle>
            <DialogDescription>
              {t('common.deleteConfirm', { title: `"${deleteTarget?.display_name || deleteTarget?.slug}"` })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={confirmDelete} disabled={deleting}>
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

export function AdminLoginCard({ ctx }) {
  const { t } = useTranslation();
  if (!isExternalSaved(ctx)) return null;
  const url = `${(ctx.inputs.ServerAddress || '').replace(/\/+$/, '')}/login/admin`;
  return (
    <OptionCard ctx={ctx} id="admin-login" title={t(tk('adminLogin'))} description={t(tk('adminLoginDesc'))}>
      <OptionList>
        <ToggleRow
          id="AdminLoginEnabled"
          label={t(tk('adminLoginToggle'), { url })}
          description={t(tk('adminLoginToggleDesc'))}
          {...bindSwitch(ctx, 'AdminLoginEnabled')}
        />
      </OptionList>
    </OptionCard>
  );
}

// Social login cards: header switch, the provider console URLs to copy, then the credentials.
function SocialCard({ ctx, id, enableKey, titleKey, subtitleKey, required, path, urlLabels, children }) {
  const { t } = useTranslation();
  const { inputs } = ctx;
  if (isExternalSaved(ctx)) return null;
  const notConfigured = inputs[enableKey] === 'true' && required.some((k) => !inputs[k]);
  return (
    <OptionCard ctx={ctx} id={id} title={t(sysKey(titleKey))} description={t(sysKey(subtitleKey))} enableKey={enableKey}>
      {notConfigured && (
        <p className="text-xs text-amber-600 dark:text-amber-500">{t(sysKey('configureLoginRegister.providerNotConfigured'))}</p>
      )}
      {path && (
        <div className="grid gap-5 sm:grid-cols-2">
          <TextRow id={`${id}-homepage`} label={urlLabels[0]} value={inputs.ServerAddress || ''} onChange={() => {}} readOnly copyable />
          <TextRow
            id={`${id}-callback`}
            label={urlLabels[1]}
            value={`${inputs.ServerAddress || ''}${path}`}
            onChange={() => {}}
            readOnly
            copyable
          />
        </div>
      )}
      {children}
    </OptionCard>
  );
}

SocialCard.propTypes = {
  ctx: PropTypes.object.isRequired,
  id: PropTypes.string.isRequired,
  enableKey: PropTypes.string,
  titleKey: PropTypes.string.isRequired,
  subtitleKey: PropTypes.string.isRequired,
  required: PropTypes.arrayOf(PropTypes.string),
  path: PropTypes.string,
  urlLabels: PropTypes.arrayOf(PropTypes.node),
  children: PropTypes.node
};

const useCredentialField = (ctx) => {
  const { inputs, secrets, setField } = ctx;
  return (key, label, placeholder, type = 'text') => (
    <TextRow
      id={key}
      type={type}
      label={label}
      value={inputs[key]}
      onChange={(v) => setField(key, v)}
      placeholder={placeholder}
      secret={secrets?.[key]}
    />
  );
};

export function GitHubOAuthCard({ ctx }) {
  const { t } = useTranslation();
  const sys = (k) => t(sysKey(k));
  const field = useCredentialField(ctx);
  return (
    <SocialCard
      ctx={ctx}
      id="github-oauth"
      enableKey="GitHubOAuthEnabled"
      titleKey="configureGitHubOAuthApp.title"
      subtitleKey="configureGitHubOAuthApp.subTitle"
      required={['GitHubClientId']}
      path="/oauth/github"
      urlLabels={[sys('configureGitHubOAuthApp.homepageUrl'), sys('configureGitHubOAuthApp.callbackUrl')]}
    >
      <div className="grid gap-5 sm:grid-cols-2">
        {field('GitHubClientId', sys('configureGitHubOAuthApp.clientId'), sys('configureGitHubOAuthApp.clientIdPlaceholder'))}
        {field('GitHubClientSecret', sys('configureGitHubOAuthApp.clientSecret'), sys('configureGitHubOAuthApp.clientSecretPlaceholder'))}
      </div>
      <OptionList>
        <ToggleRow
          id="GitHubOldIdCloseEnabled"
          label={sys('configureLoginRegister.gitHubOldIdClose')}
          description={sys('configureLoginRegister.gitHubOldIdCloseDescription')}
          {...bindSwitch(ctx, 'GitHubOldIdCloseEnabled')}
        />
      </OptionList>
    </SocialCard>
  );
}

export function WeChatAuthCard({ ctx }) {
  const { t } = useTranslation();
  const sys = (k) => t(sysKey(k));
  const field = useCredentialField(ctx);
  return (
    <SocialCard
      ctx={ctx}
      id="wechat-auth"
      enableKey="WeChatAuthEnabled"
      titleKey="configureWeChatServer.title"
      subtitleKey="configureWeChatServer.subTitle"
      required={['WeChatServerAddress']}
    >
      <div className="grid gap-5 sm:grid-cols-2">
        {field('WeChatServerAddress', sys('configureWeChatServer.serverAddress'), sys('configureWeChatServer.serverAddressPlaceholder'))}
        {field('WeChatServerToken', sys('configureWeChatServer.accessToken'), sys('configureWeChatServer.accessTokenPlaceholder'))}
        {field(
          'WeChatAccountQRCodeImageURL',
          sys('configureWeChatServer.qrCodeImage'),
          sys('configureWeChatServer.qrCodeImagePlaceholder')
        )}
      </div>
    </SocialCard>
  );
}

export function LarkAuthCard({ ctx }) {
  const { t } = useTranslation();
  const sys = (k) => t(sysKey(k));
  const field = useCredentialField(ctx);
  return (
    <SocialCard
      ctx={ctx}
      id="lark-auth"
      enableKey="LarkAuthEnabled"
      titleKey="configureFeishuAuthorization.title"
      subtitleKey="configureFeishuAuthorization.subTitle"
      required={['LarkClientId']}
      path="/oauth/lark"
      urlLabels={[sys('configureFeishuAuthorization.homepageUrl'), sys('configureFeishuAuthorization.callbackUrl')]}
    >
      <div className="grid gap-5 sm:grid-cols-2">
        {field('LarkClientId', sys('configureFeishuAuthorization.appId'), sys('configureFeishuAuthorization.appIdPlaceholder'))}
        {field('LarkClientSecret', sys('configureFeishuAuthorization.appSecret'), sys('configureFeishuAuthorization.appSecretPlaceholder'))}
      </div>
    </SocialCard>
  );
}

export function LinuxDoOAuthCard({ ctx }) {
  const { t } = useTranslation();
  const sys = (k) => t(sysKey(k));
  const field = useCredentialField(ctx);
  return (
    <SocialCard
      ctx={ctx}
      id="linuxdo-oauth"
      enableKey="LinuxDoOAuthEnabled"
      titleKey="configureLinuxDoOAuthApp.title"
      subtitleKey="configureLinuxDoOAuthApp.subTitle"
      required={['LinuxDoClientId']}
      path="/oauth/linuxdo"
      urlLabels={[sys('configureLinuxDoOAuthApp.homepageUrl'), sys('configureLinuxDoOAuthApp.callbackUrl')]}
    >
      <div className="grid gap-5 sm:grid-cols-2">
        {field('LinuxDoClientId', sys('configureLinuxDoOAuthApp.clientId'), sys('configureLinuxDoOAuthApp.clientIdPlaceholder'))}
        {field(
          'LinuxDoClientSecret',
          sys('configureLinuxDoOAuthApp.clientSecret'),
          sys('configureLinuxDoOAuthApp.clientSecretPlaceholder')
        )}
        {field(
          'LinuxDoOAuthLowestTrustLevel',
          sys('configureLinuxDoOAuthApp.lowestTrustLevel'),
          sys('configureLinuxDoOAuthApp.lowestTrustLevelPlaceholder')
        )}
      </div>
      <OptionList>
        <ToggleRow
          id="LinuxDoOAuthTrustLevelEnabled"
          label={sys('configureLoginRegister.linuxDoOAuthTrustLevel')}
          description={sys('configureLoginRegister.linuxDoOAuthTrustLevelDescription')}
          {...bindSwitch(ctx, 'LinuxDoOAuthTrustLevelEnabled')}
        />
        <ToggleRow
          id="LinuxDoOAuthDynamicTrustLevel"
          label={sys('configureLoginRegister.linuxDoOAuthDynamicTrustLevel')}
          description={sys('configureLoginRegister.linuxDoOAuthDynamicTrustLevelDescription')}
          {...bindSwitch(ctx, 'LinuxDoOAuthDynamicTrustLevel')}
        />
      </OptionList>
    </SocialCard>
  );
}

export function TurnstileCard({ ctx }) {
  const { t } = useTranslation();
  const sys = (k) => t(sysKey(k));
  const field = useCredentialField(ctx);
  if (isExternalSaved(ctx)) return null;
  return (
    <OptionCard ctx={ctx} id="turnstile" title={sys('configureTurnstile.title')} description={sys('configureTurnstile.subTitle')}>
      <div className="grid gap-5 sm:grid-cols-2">
        {field('TurnstileSiteKey', sys('configureTurnstile.siteKey'), sys('configureTurnstile.siteKeyPlaceholder'))}
        {field('TurnstileSecretKey', sys('configureTurnstile.secretKey'), sys('configureTurnstile.secretKeyPlaceholder'), 'password')}
      </div>
    </OptionCard>
  );
}

export function EmailDomainCard({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField } = ctx;
  const sys = (k) => t(sysKey(k));
  // Stored comma-joined, edited one domain per line.
  const whitelistText = (() => {
    const v = inputs.EmailDomainWhitelist;
    if (Array.isArray(v)) return v.join('\n');
    return (v || '').split(',').filter(Boolean).join('\n');
  })();

  return (
    <OptionCard
      ctx={ctx}
      id="email-domain"
      title={sys('configureEmailDomainWhitelist.title')}
      description={sys('configureEmailDomainWhitelist.subTitle')}
      enableKey="EmailDomainRestrictionEnabled"
    >
      <TextRow
        id="EmailDomainWhitelist"
        label={sys('configureEmailDomainWhitelist.allowedEmailDomains')}
        value={whitelistText}
        onChange={(v) =>
          setField(
            'EmailDomainWhitelist',
            v
              .split('\n')
              .map((s) => s.trim())
              .filter(Boolean)
              .join(',')
          )
        }
        placeholder="example.com"
        multiline
        rows={5}
      />
    </OptionCard>
  );
}
