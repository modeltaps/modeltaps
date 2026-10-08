import { useTranslation } from 'react-i18next';

import LoginMethodSettings from '../LoginMethodSettings';
import { EmailDomainCard } from '../EmailSettings';
import InviteCodeSettings from '../InviteCodeSettings';
import { SettingsCard } from '../parts';

// ==============================|| SYSTEM SETTINGS — 登录与注册 ||============================== //
// 登录方式(账号体系二选一 / 身份提供方 / 管理员应急登录 / 本站账号与注册开关 / 社交登录 / 人机验证)
// 由 LoginMethodSettings 整页承担;这里再接邮箱域名白名单与邀请码表。

export default function AuthTheme({ ctx }) {
  const { t } = useTranslation();

  return (
    <div className="space-y-6">
      <LoginMethodSettings ctx={ctx} />
      <EmailDomainCard ctx={ctx} />
      {/* 邀请码注册开关在上面的「本站账号」卡里,这里只放邀请码表。 */}
      <SettingsCard title={t('setting_index.inviteCodeSettings.title')} highlight="invite-code">
        <InviteCodeSettings />
      </SettingsCard>
    </div>
  );
}
