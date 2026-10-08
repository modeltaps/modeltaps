import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { SettingsCard, TextRow, ToggleRow, HeaderSwitch, SaveButton } from './parts';
import OidcProviderSettings from './OidcProviderSettings';

// ==============================|| ADMIN — SIGN-IN METHODS ||============================== //
// 账号体系二选一是这一页的总开关:
//   内置账号 —— 本站账号开关(密码 / 邮箱验证码 / 通行密钥 / 注册 / 人机验证)+ 社交登录各家配置;
//   外部身份提供方 —— 提供方列表(承担登录的那个要标「本站身份」)+ 管理员应急登录;
//     本站账号与社交登录整组停用并置灰,全部在提供方后台配置。
// 第三方登录只在管账号的那一方配置,这就是「两边都能配社交登录」重合问题的答案。

const tk = (k) => `setting_index.loginMethods.${k}`;

function AccountSystemChoice({ value, current, title, description, disabled, onSelect }) {
  const active = current === value;
  return (
    <button
      type="button"
      role="radio"
      aria-checked={active}
      disabled={disabled}
      onClick={() => !active && onSelect(value)}
      className={cn(
        'flex flex-1 items-start gap-3 rounded-lg border p-4 text-left transition-colors',
        active ? 'border-foreground bg-muted' : 'border-border hover:bg-muted/60',
        disabled && 'opacity-60'
      )}
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
        <span className="block text-sm font-semibold">{title}</span>
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
  disabled: PropTypes.bool,
  onSelect: PropTypes.func
};

export default function LoginMethodSettings({ ctx }) {
  const { t } = useTranslation();
  const { inputs, setField, toggle, saveKeys, loading, isDirty } = ctx;
  const [pendingSystem, setPendingSystem] = useState('');
  const [pwWarn, setPwWarn] = useState(false);
  const is = (k) => inputs[k] === 'true';
  const sys = (k) => t(`setting_index.systemSettings.${k}`);
  const accountSystem = inputs.AccountSystem === 'external' ? 'external' : 'builtin';
  const external = accountSystem === 'external';

  // 切换账号体系走一次确认:切到外部会立刻让所有本站登录方式失效,切回内置会让提供方登录失效。
  const confirmSwitch = async () => {
    const next = pendingSystem;
    setPendingSystem('');
    setField('AccountSystem', next);
    const ok = await saveKeys(['AccountSystem']);
    if (!ok) setField('AccountSystem', accountSystem);
  };

  const onTogglePasswordLogin = () => {
    if (is('PasswordLoginEnabled')) {
      setPwWarn(true);
      return;
    }
    toggle('PasswordLoginEnabled');
  };

  const field = (key, labelKey, phKey) => (
    <TextRow id={key} label={labelKey} value={inputs[key]} onChange={(v) => setField(key, v)} placeholder={phKey} disabled={loading} />
  );
  const toggleRow = (labelKey, key, descriptionKey) => (
    <ToggleRow
      label={sys(labelKey)}
      description={descriptionKey ? sys(descriptionKey) : undefined}
      checked={is(key)}
      onCheckedChange={() => toggle(key)}
      disabled={loading}
    />
  );
  const enableSwitch = (key) => <HeaderSwitch id={key} checked={is(key)} onCheckedChange={() => toggle(key)} disabled={loading} />;
  const notConfigured = (enabledKey, requiredKeys) =>
    is(enabledKey) && requiredKeys.some((k) => !inputs[k]) ? (
      <p className="text-xs text-amber-600 dark:text-amber-500">{sys('configureLoginRegister.providerNotConfigured')}</p>
    ) : null;
  // Read-only URLs to paste into the provider console, with one-click copy.
  const urlRows = (provider, path, homepageLabel, callbackLabel) => (
    <div className="grid gap-4 sm:grid-cols-2">
      <TextRow
        id={`${provider}HomepageUrl`}
        label={homepageLabel}
        value={inputs.ServerAddress || ''}
        onChange={() => {}}
        readOnly
        copyable
      />
      <TextRow
        id={`${provider}CallbackUrl`}
        label={callbackLabel}
        value={`${inputs.ServerAddress || ''}${path}`}
        onChange={() => {}}
        readOnly
        copyable
      />
    </div>
  );

  const smtpConfigured = Boolean(inputs.SMTPServer && inputs.SMTPAccount);

  return (
    <div className="space-y-6">
      <SettingsCard title={t(tk('accountSystem'))} description={t(tk('accountSystemDesc'))} highlight="login-methods">
        <div role="radiogroup" className="flex flex-col gap-3 md:flex-row">
          <AccountSystemChoice
            value="builtin"
            current={accountSystem}
            title={t(tk('builtin'))}
            description={t(tk('builtinDesc'))}
            disabled={loading}
            onSelect={setPendingSystem}
          />
          <AccountSystemChoice
            value="external"
            current={accountSystem}
            title={t(tk('external'))}
            description={t(tk('externalDesc'))}
            disabled={loading}
            onSelect={setPendingSystem}
          />
        </div>
      </SettingsCard>

      {external ? (
        <>
          <SettingsCard title={t('setting_index.oidcProviders.title')} description={t(tk('providersDesc'))}>
            <OidcProviderSettings serverAddress={inputs.ServerAddress || ''} />
          </SettingsCard>

          <SettingsCard title={t(tk('adminLogin'))} description={t(tk('adminLoginDesc'))}>
            <ToggleRow
              label={t(tk('adminLoginToggle'), { url: `${(inputs.ServerAddress || '').replace(/\/+$/, '')}/login/admin` })}
              description={t(tk('adminLoginToggleDesc'))}
              checked={is('AdminLoginEnabled')}
              onCheckedChange={() => toggle('AdminLoginEnabled')}
              disabled={loading}
            />
          </SettingsCard>

          <SettingsCard title={t(tk('managedByProvider'))} description={t(tk('managedByProviderDesc'))} className="opacity-70">
            <p className="text-sm text-muted-foreground">{t(tk('managedByProviderHint'))}</p>
          </SettingsCard>
        </>
      ) : (
        <>
          <SettingsCard title={t(tk('builtinTitle'))} description={t(tk('builtinTitleDesc'))}>
            <div className="grid gap-3">
              <ToggleRow
                label={sys('configureLoginRegister.passwordLogin')}
                description={sys('configureLoginRegister.passwordLoginDescription')}
                checked={is('PasswordLoginEnabled')}
                onCheckedChange={onTogglePasswordLogin}
                disabled={loading}
              />
              <ToggleRow
                label={t(tk('emailCodeLogin'))}
                description={smtpConfigured ? t(tk('emailCodeLoginDesc')) : t(tk('emailCodeLoginNeedsSmtp'))}
                checked={is('EmailCodeLoginEnabled')}
                onCheckedChange={() => toggle('EmailCodeLoginEnabled')}
                disabled={loading}
              />
              <ToggleRow
                label={t(tk('passkeyLogin'))}
                description={t(tk('passkeyLoginDesc'))}
                checked={is('PasskeyLoginEnabled')}
                onCheckedChange={() => toggle('PasskeyLoginEnabled')}
                disabled={loading}
              />
              {toggleRow('configureLoginRegister.registerEnabled', 'RegisterEnabled', 'configureLoginRegister.registerEnabledDescription')}
              {toggleRow(
                'configureLoginRegister.passwordRegister',
                'PasswordRegisterEnabled',
                'configureLoginRegister.passwordRegisterDescription'
              )}
              {toggleRow(
                'configureLoginRegister.emailVerification',
                'EmailVerificationEnabled',
                'configureLoginRegister.emailVerificationDescription'
              )}
              {toggleRow(
                'configureLoginRegister.inviteCodeRegister',
                'InviteCodeRegisterEnabled',
                'configureLoginRegister.inviteCodeRegisterDescription'
              )}
              {toggleRow(
                'configureLoginRegister.turnstileCheck',
                'TurnstileCheckEnabled',
                'configureLoginRegister.turnstileCheckDescription'
              )}
            </div>
          </SettingsCard>

          <SettingsCard
            title={sys('configureGitHubOAuthApp.title')}
            description={sys('configureGitHubOAuthApp.subTitle')}
            headerAction={enableSwitch('GitHubOAuthEnabled')}
          >
            <div className="grid gap-3">
              {toggleRow(
                'configureLoginRegister.gitHubOldIdClose',
                'GitHubOldIdCloseEnabled',
                'configureLoginRegister.gitHubOldIdCloseDescription'
              )}
            </div>
            {notConfigured('GitHubOAuthEnabled', ['GitHubClientId'])}
            {urlRows('GitHub', '/oauth/github', sys('configureGitHubOAuthApp.homepageUrl'), sys('configureGitHubOAuthApp.callbackUrl'))}
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {field('GitHubClientId', sys('configureGitHubOAuthApp.clientId'), sys('configureGitHubOAuthApp.clientIdPlaceholder'))}
              {field(
                'GitHubClientSecret',
                sys('configureGitHubOAuthApp.clientSecret'),
                sys('configureGitHubOAuthApp.clientSecretPlaceholder')
              )}
            </div>
            <SaveButton
              loading={loading}
              disabled={!isDirty(['GitHubClientId', 'GitHubClientSecret'])}
              onClick={() => saveKeys(['GitHubClientId', 'GitHubClientSecret'])}
            >
              {t('common.save')}
            </SaveButton>
          </SettingsCard>

          <SettingsCard
            title={sys('configureWeChatServer.title')}
            description={sys('configureWeChatServer.subTitle')}
            headerAction={enableSwitch('WeChatAuthEnabled')}
          >
            {notConfigured('WeChatAuthEnabled', ['WeChatServerAddress'])}
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {field(
                'WeChatServerAddress',
                sys('configureWeChatServer.serverAddress'),
                sys('configureWeChatServer.serverAddressPlaceholder')
              )}
              {field('WeChatServerToken', sys('configureWeChatServer.accessToken'), sys('configureWeChatServer.accessTokenPlaceholder'))}
              {field(
                'WeChatAccountQRCodeImageURL',
                sys('configureWeChatServer.qrCodeImage'),
                sys('configureWeChatServer.qrCodeImagePlaceholder')
              )}
            </div>
            <SaveButton
              loading={loading}
              disabled={!isDirty(['WeChatServerAddress', 'WeChatServerToken', 'WeChatAccountQRCodeImageURL'])}
              onClick={() => saveKeys(['WeChatServerAddress', 'WeChatServerToken', 'WeChatAccountQRCodeImageURL'])}
            >
              {t('common.save')}
            </SaveButton>
          </SettingsCard>

          <SettingsCard
            title={sys('configureFeishuAuthorization.title')}
            description={sys('configureFeishuAuthorization.subTitle')}
            headerAction={enableSwitch('LarkAuthEnabled')}
          >
            {notConfigured('LarkAuthEnabled', ['LarkClientId'])}
            {urlRows(
              'Lark',
              '/oauth/lark',
              sys('configureFeishuAuthorization.homepageUrl'),
              sys('configureFeishuAuthorization.callbackUrl')
            )}
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {field('LarkClientId', sys('configureFeishuAuthorization.appId'), sys('configureFeishuAuthorization.appIdPlaceholder'))}
              {field(
                'LarkClientSecret',
                sys('configureFeishuAuthorization.appSecret'),
                sys('configureFeishuAuthorization.appSecretPlaceholder')
              )}
            </div>
            <SaveButton
              loading={loading}
              disabled={!isDirty(['LarkClientId', 'LarkClientSecret'])}
              onClick={() => saveKeys(['LarkClientId', 'LarkClientSecret'])}
            >
              {t('common.save')}
            </SaveButton>
          </SettingsCard>

          <SettingsCard
            title={sys('configureLinuxDoOAuthApp.title')}
            description={sys('configureLinuxDoOAuthApp.subTitle')}
            headerAction={enableSwitch('LinuxDoOAuthEnabled')}
          >
            <div className="grid gap-3">
              {toggleRow(
                'configureLoginRegister.linuxDoOAuthTrustLevel',
                'LinuxDoOAuthTrustLevelEnabled',
                'configureLoginRegister.linuxDoOAuthTrustLevelDescription'
              )}
              {toggleRow(
                'configureLoginRegister.linuxDoOAuthDynamicTrustLevel',
                'LinuxDoOAuthDynamicTrustLevel',
                'configureLoginRegister.linuxDoOAuthDynamicTrustLevelDescription'
              )}
            </div>
            {notConfigured('LinuxDoOAuthEnabled', ['LinuxDoClientId'])}
            {urlRows('LinuxDo', '/oauth/linuxdo', sys('configureLinuxDoOAuthApp.homepageUrl'), sys('configureLinuxDoOAuthApp.callbackUrl'))}
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
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
            <SaveButton
              loading={loading}
              disabled={!isDirty(['LinuxDoClientId', 'LinuxDoClientSecret', 'LinuxDoOAuthLowestTrustLevel'])}
              onClick={() => saveKeys(['LinuxDoClientId', 'LinuxDoClientSecret', 'LinuxDoOAuthLowestTrustLevel'])}
            >
              {t('common.save')}
            </SaveButton>
          </SettingsCard>

          <SettingsCard title={sys('configureTurnstile.title')} description={sys('configureTurnstile.subTitle')}>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {field('TurnstileSiteKey', sys('configureTurnstile.siteKey'), sys('configureTurnstile.siteKeyPlaceholder'))}
              <TextRow
                id="TurnstileSecretKey"
                type="password"
                label={sys('configureTurnstile.secretKey')}
                value={inputs.TurnstileSecretKey}
                onChange={(v) => setField('TurnstileSecretKey', v)}
                placeholder={sys('configureTurnstile.secretKeyPlaceholder')}
                disabled={loading}
              />
            </div>
            <SaveButton
              loading={loading}
              disabled={!isDirty(['TurnstileSiteKey', 'TurnstileSecretKey'])}
              onClick={() => saveKeys(['TurnstileSiteKey', 'TurnstileSecretKey'])}
            >
              {t('common.save')}
            </SaveButton>
          </SettingsCard>
        </>
      )}

      <Dialog open={!!pendingSystem} onOpenChange={(o) => !o && setPendingSystem('')}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{pendingSystem === 'external' ? t(tk('switchToExternal')) : t(tk('switchToBuiltin'))}</DialogTitle>
            <DialogDescription>
              {pendingSystem === 'external' ? t(tk('switchToExternalDesc')) : t(tk('switchToBuiltinDesc'))}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPendingSystem('')}>
              {t('common.cancel')}
            </Button>
            <Button onClick={confirmSwitch} disabled={loading}>
              {t('common.ok')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={pwWarn} onOpenChange={setPwWarn}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t(tk('passwordOffTitle'))}</DialogTitle>
            <DialogDescription>{t(tk('passwordOffDesc'))}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPwWarn(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                setPwWarn(false);
                toggle('PasswordLoginEnabled');
              }}
            >
              {t('common.ok')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

LoginMethodSettings.propTypes = {
  ctx: PropTypes.object
};
