import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Fingerprint, KeyRound, TriangleAlert } from 'lucide-react';

import { getWebAuthnCredentials } from 'utils/common';
import { MethodRow, SecurityCard } from './MethodRow';
import PasswordDialog from './PasswordDialog';
import PasskeyDialog from './PasskeyDialog';

// ==============================|| SETTINGS — ADMIN EMERGENCY LOGIN CARD ||============================== //
// 外部身份提供方模式下只对 root 显示:登录服务不可用时从 /login/admin 用这里的密码或通行密钥进后台。
// 应急密码与应急通行密钥只用于这个入口,与登录服务里的密码、通行密钥无关。

const tk = (k) => `settingsPage.security.${k}`;

export default function AdminEmergencyCard({ siteInfo, inputs, reloadUser }) {
  const { t } = useTranslation();
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [passkeyOpen, setPasskeyOpen] = useState(false);
  const [passkeyCount, setPasskeyCount] = useState(null);
  const hasPassword = inputs?.has_password !== false;
  const url = `${(siteInfo?.server_address || '').replace(/\/+$/, '')}/login/admin`;

  const loadPasskeys = () => getWebAuthnCredentials().then((list) => setPasskeyCount(list.length));
  useEffect(() => {
    loadPasskeys();
  }, []);

  return (
    <SecurityCard title={t(tk('emergency'))} description={t(tk('emergencyHint'))}>
      <div className="mb-2 flex items-start gap-3 rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm">
        <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-500" aria-hidden="true" />
        <span>{t(tk('emergencyWarning'), { url })}</span>
      </div>
      <MethodRow
        icon={KeyRound}
        title={t(tk('emergencyPassword'))}
        subtitle={hasPassword ? t(tk('passwordSet')) : t(tk('passwordUnset'))}
        action={hasPassword ? t(tk('rotate')) : t(tk('set'))}
        onAction={() => setPasswordOpen(true)}
      />
      <MethodRow
        icon={Fingerprint}
        title={t(tk('emergencyPasskey'))}
        subtitle={
          passkeyCount === null
            ? t(tk('emergencyPasskeyHint'))
            : passkeyCount > 0
              ? t(tk('passkeyCount'), { count: passkeyCount })
              : t(tk('passkeyNone'))
        }
        action={t(tk('manage'))}
        onAction={() => setPasskeyOpen(true)}
      />
      <PasswordDialog
        open={passwordOpen}
        onOpenChange={setPasswordOpen}
        hasPassword={hasPassword}
        inputs={inputs}
        emergency
        onSaved={reloadUser}
      />
      <PasskeyDialog open={passkeyOpen} onOpenChange={setPasskeyOpen} emergency onChanged={loadPasskeys} />
    </SecurityCard>
  );
}

AdminEmergencyCard.propTypes = {
  siteInfo: PropTypes.object,
  inputs: PropTypes.object,
  reloadUser: PropTypes.func
};
