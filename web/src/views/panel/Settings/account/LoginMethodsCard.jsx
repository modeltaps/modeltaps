import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Fingerprint, KeyRound, Mail, ShieldCheck } from 'lucide-react';

import { API } from 'utils/api';
import { getWebAuthnCredentials, showError, showSuccess } from 'utils/common';
import { emailCodeLoginEnabled, isExternalAccountSystem, passkeyLoginEnabled, passwordLoginEnabled } from 'utils/authAvailability';
import { MethodRow, SecurityCard } from './MethodRow';
import PasswordDialog from './PasswordDialog';
import PasskeyDialog from './PasskeyDialog';

// ==============================|| SETTINGS — SIGN-IN VERIFICATION CARD ||============================== //
// 这张卡只装「证明是你本人」的手段:内置账号下是密码 / 邮箱验证码 / 通行密钥,动作在本站完成。
// 已关联的第三方账号是指向这个账号的标识,归「账号标识」卡(IdentifiersCard),不在这里。
// 外部身份提供方:同一套行,但动作跳到登录服务的设置页深链(后台没填的行不显示);
// 界面上不出现任何供应商名,也不出现「Modeltaps 账号」这种把两个系统分开的说法。

const tk = (k) => `settingsPage.security.${k}`;

// 外部模式下承担登录的提供方:优先用户已关联的本站身份行,没有(如 root 用密码登录)则退回站点下发的提供方。
export const externalIdentityLinks = (siteInfo, inputs) => {
  const identities = Array.isArray(inputs?.oidc_identities) ? inputs.oidc_identities : [];
  const providers = Array.isArray(siteInfo?.oidc_providers) ? siteInfo.oidc_providers : [];
  return identities.find((identity) => identity.first_party) || providers.find((provider) => provider.first_party) || providers[0] || null;
};

// 深链一个都没填时的兜底出口:提供方 issuer 的站点根地址(只留 scheme + host[+port]),
// 解析不出 http(s) 地址就当没有,卡片退回纯说明。
export const issuerOrigin = (siteInfo) => {
  const providers = Array.isArray(siteInfo?.oidc_providers) ? siteInfo.oidc_providers : [];
  const provider = providers.find((item) => item.first_party) || providers[0] || {};
  if (!provider.issuer) return '';
  try {
    const url = new URL(provider.issuer);
    return url.protocol === 'http:' || url.protocol === 'https:' ? url.origin : '';
  } catch {
    return '';
  }
};

export default function LoginMethodsCard({ siteInfo, inputs, reloadUser }) {
  const { t } = useTranslation();
  const external = isExternalAccountSystem(siteInfo);
  const isRoot = inputs?.role === 100;
  const passwordSite = passwordLoginEnabled(siteInfo);
  const emailCodeSite = emailCodeLoginEnabled(siteInfo);
  const passkeySite = passkeyLoginEnabled(siteInfo);
  const hasPassword = inputs?.has_password !== false;

  const [passwordOpen, setPasswordOpen] = useState(false);
  const [passkeyOpen, setPasskeyOpen] = useState(false);
  const [passkeyCount, setPasskeyCount] = useState(null);
  const [emailCodeAllowed, setEmailCodeAllowed] = useState(null);

  useEffect(() => {
    if (external || !passkeySite) return;
    getWebAuthnCredentials().then((list) => setPasskeyCount(list.length));
  }, [external, passkeySite]);

  useEffect(() => {
    if (external || !emailCodeSite) return;
    API.get('/api/user/setting')
      .then((res) => {
        if (res.data?.success) setEmailCodeAllowed(res.data.data?.email_code_login !== false);
      })
      .catch(() => {});
  }, [external, emailCodeSite]);

  const toggleEmailCode = async () => {
    const next = !(emailCodeAllowed !== false);
    try {
      const res = await API.put('/api/user/setting', { email_code_login: next });
      if (res.data?.success) {
        setEmailCodeAllowed(next);
        showSuccess(t('profilePage.updateSuccess'));
      } else {
        showError(res.data?.message);
      }
    } catch (err) {
      showError(err.message);
    }
  };

  // ---------- 外部身份提供方:同一套行,动作跳到登录服务的设置页 ----------
  if (external) {
    const links = externalIdentityLinks(siteInfo, inputs) || {};
    const rows = [
      {
        key: 'password',
        icon: KeyRound,
        title: t(tk('password')),
        subtitle: t(tk('passwordHintExternal')),
        href: links.password_url,
        action: t(tk('change'))
      },
      { key: 'mfa', icon: ShieldCheck, title: t(tk('mfa')), subtitle: t(tk('mfaHint')), href: links.mfa_url, action: t(tk('set')) },
      {
        key: 'passkey',
        icon: Fingerprint,
        title: t(tk('passkey')),
        subtitle: t(tk('passkeyHint')),
        href: links.passkey_url,
        action: t(tk('manage'))
      }
    ].filter((row) => row.href);
    // 深链全空:先退到账号设置页,再退到登录服务站点根地址,都没有才只剩一句说明。
    const fallbackHref = rows.length === 0 ? links.account_settings_url || issuerOrigin(siteInfo) : '';
    const fallbackByIssuer = !!fallbackHref && !links.account_settings_url;

    return (
      <SecurityCard
        title={t(tk('loginVerification'))}
        description={t(tk(rows.length > 0 ? 'loginVerificationHintExternal' : 'loginVerificationHintManaged'))}
      >
        {rows.map((row) => (
          <MethodRow key={row.key} icon={row.icon} title={row.title} subtitle={row.subtitle} href={row.href} action={row.action} />
        ))}
        {fallbackHref && (
          <MethodRow
            icon={ShieldCheck}
            title={t(tk('accountSettings'))}
            subtitle={t(tk(fallbackByIssuer ? 'accountSettingsHintIssuer' : 'accountSettingsHint'))}
            href={fallbackHref}
            action={t(tk('manage'))}
          />
        )}
        {rows.length === 0 && !fallbackHref && <p className="py-3 text-sm text-muted-foreground">{t(tk('managedExternally'))}</p>}
      </SecurityCard>
    );
  }

  // ---------- 内置账号 ----------
  const showPassword = passwordSite || isRoot;
  const emailCodeOn = emailCodeAllowed !== false;
  const emailCodeSubtitle = !inputs?.email
    ? t(tk('emailCodeNeedsEmail'))
    : emailCodeOn
      ? t(tk('emailCodeOn'), { email: inputs.email })
      : t(tk('emailCodeOff'));
  const passkeySubtitle =
    passkeyCount === null ? t(tk('passkeyHint')) : passkeyCount > 0 ? t(tk('passkeyCount'), { count: passkeyCount }) : t(tk('passkeyNone'));

  return (
    <SecurityCard title={t(tk('loginVerification'))} description={t(tk('loginVerificationHintBuiltin'))}>
      {showPassword && (
        <MethodRow
          icon={KeyRound}
          title={t(tk('password'))}
          subtitle={hasPassword ? t(tk('passwordSet')) : t(tk('passwordUnset'))}
          action={hasPassword ? t(tk('change')) : t(tk('set'))}
          onAction={() => setPasswordOpen(true)}
        />
      )}
      {emailCodeSite && (
        <MethodRow
          icon={Mail}
          title={t(tk('emailCode'))}
          subtitle={emailCodeSubtitle}
          action={emailCodeOn ? t(tk('disable')) : t(tk('enable'))}
          onAction={toggleEmailCode}
          disabled={!inputs?.email || emailCodeAllowed === null}
        />
      )}
      {passkeySite && (
        <MethodRow
          icon={Fingerprint}
          title={t(tk('passkey'))}
          subtitle={passkeySubtitle}
          action={t(tk('manage'))}
          onAction={() => setPasskeyOpen(true)}
        />
      )}

      <PasswordDialog open={passwordOpen} onOpenChange={setPasswordOpen} hasPassword={hasPassword} inputs={inputs} onSaved={reloadUser} />
      <PasskeyDialog
        open={passkeyOpen}
        onOpenChange={setPasskeyOpen}
        onChanged={() => getWebAuthnCredentials().then((list) => setPasskeyCount(list.length))}
      />
    </SecurityCard>
  );
}

LoginMethodsCard.propTypes = {
  siteInfo: PropTypes.object,
  inputs: PropTypes.object,
  reloadUser: PropTypes.func
};
