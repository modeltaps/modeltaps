import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Code2, Link as LinkIcon, Mail, MessageCircle, Smartphone } from 'lucide-react';

import { API } from 'utils/api';
import { getOAuthState, onGitHubOAuthClicked, onLarkOAuthClicked, onLinuxDoOAuthClicked, showError, showSuccess } from 'utils/common';
import { isExternalAccountSystem } from 'utils/authAvailability';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { MethodRow, RowGroupLabel, SecurityCard } from './MethodRow';
import EmailBindModal from './EmailBindModal';
import { externalIdentityLinks } from './LoginMethodsCard';

// ==============================|| SETTINGS — ACCOUNT IDENTIFIERS CARD ||============================== //
// 这张卡装的是「指向这个账号的东西」:邮箱、手机号,以及已关联的第三方账号 —— 它们不是联系方式,
// 用其中任何一个登录进的都是同一个账号。内置账号:邮箱在本站验证与更换(需要 SMTP),第三方账号
// 在本站关联 / 取消关联;外部身份提供方:三者都来自登录服务,只读,动作跳到登录服务的设置页 ——
// 邮箱、手机号已有值时是「更换」,还没有时是「添加」。界面上不出现任何供应商名。

const tk = (k) => `settingsPage.security.${k}`;

// 邮箱行的「已验证 / 未验证」按后端记下的真实来源显示(users.email_verified):验证码绑定与
// 登录服务下发的邮箱才算已验证,管理员在后台代填的不算。手机号只经登录服务的已验证声明写入,
// 没有这个问题,故恒按已验证展示。
export const emailSubtitle = (t, inputs) =>
  inputs?.email ? `${inputs.email} · ${t(tk(inputs?.email_verified ? 'verified' : 'notVerified'))}` : t(tk('notSet'));

export default function IdentifiersCard({ siteInfo, inputs, reloadUser }) {
  const { t } = useTranslation();
  const [openEmail, setOpenEmail] = useState(false);
  const [unbindTarget, setUnbindTarget] = useState(null);
  const [openWechat, setOpenWechat] = useState(false);
  const [wechatCode, setWechatCode] = useState('');
  const external = isExternalAccountSystem(siteInfo);

  const confirmUnbind = async () => {
    if (!unbindTarget) return;
    try {
      const res = await API.post('/api/user/unbind', { type: unbindTarget });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('profilePage.unbindSuccess'));
        await reloadUser();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setUnbindTarget(null);
    }
  };

  const bindWeChat = async () => {
    if (!wechatCode) return;
    try {
      const state = await getOAuthState();
      if (!state) return;
      const res = await API.get(`/api/oauth/wechat/bind?code=${encodeURIComponent(wechatCode)}&state=${state}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('profilePage.wechatBindSuccess'));
        setOpenWechat(false);
        await reloadUser();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    }
  };

  if (external) {
    const links = externalIdentityLinks(siteInfo, inputs) || {};
    const href = links.identity_url || links.account_settings_url || '';
    return (
      <SecurityCard title={t(tk('identifiers'))} description={t(tk('identifiersHintExternal'))}>
        <MethodRow
          icon={Mail}
          title={t(tk('email'))}
          subtitle={emailSubtitle(t, inputs)}
          href={href || undefined}
          action={href ? t(tk(inputs?.email ? 'replace' : 'add')) : undefined}
        />
        <MethodRow
          icon={Smartphone}
          title={t(tk('phone'))}
          subtitle={inputs?.phone_number ? `${inputs.phone_number} · ${t(tk('verified'))}` : t(tk('notSet'))}
          href={href || undefined}
          action={href ? t(tk(inputs?.phone_number ? 'replace' : 'add')) : undefined}
        />
        {href && (
          <MethodRow icon={LinkIcon} title={t(tk('thirdParty'))} subtitle={t(tk('thirdPartyHint'))} href={href} action={t(tk('manage'))} />
        )}
      </SecurityCard>
    );
  }

  const canEdit = siteInfo?.smtp_configured !== false;
  const social = [
    {
      type: 'github',
      enabled: !!siteInfo?.github_oauth,
      icon: Code2,
      label: 'GitHub',
      bound: !!inputs?.github_id,
      bind: () => onGitHubOAuthClicked(siteInfo.github_client_id, true)
    },
    {
      type: 'wechat',
      enabled: !!siteInfo?.wechat_login,
      icon: MessageCircle,
      label: t(tk('wechat')),
      bound: !!inputs?.wechat_id,
      bind: () => setOpenWechat(true)
    },
    {
      type: 'lark',
      enabled: !!siteInfo?.lark_login,
      icon: LinkIcon,
      label: t(tk('lark')),
      bound: !!inputs?.lark_id,
      bind: () => onLarkOAuthClicked(siteInfo.lark_client_id)
    },
    {
      type: 'linuxdo',
      enabled: !!siteInfo?.linuxDo_oauth,
      icon: LinkIcon,
      label: 'LinuxDo',
      bound: !!inputs?.linuxdo_id,
      bind: () => onLinuxDoOAuthClicked(siteInfo.linuxDo_client_id, true)
    }
  ].filter((item) => item.enabled);

  return (
    <SecurityCard title={t(tk('identifiers'))} description={t(tk('identifiersHint'))}>
      <MethodRow
        icon={Mail}
        title={t(tk('email'))}
        subtitle={emailSubtitle(t, inputs)}
        action={canEdit ? (inputs?.email ? t(tk('replace')) : t(tk('bind'))) : undefined}
        onAction={() => setOpenEmail(true)}
      />
      {inputs?.phone_number && <MethodRow icon={Smartphone} title={t(tk('phone'))} subtitle={inputs.phone_number} />}
      {social.length > 0 && (
        <>
          <RowGroupLabel>{t(tk('thirdParty'))}</RowGroupLabel>
          {social.map((item) => (
            <MethodRow
              key={item.type}
              icon={item.icon}
              title={item.label}
              subtitle={item.bound ? t(tk('linked')) : t(tk('unlinked'))}
              action={item.bound ? t(tk('unlink')) : t(tk('link'))}
              onAction={item.bound ? () => setUnbindTarget(item.type) : item.bind}
            />
          ))}
        </>
      )}

      <EmailBindModal
        open={openEmail}
        onClose={() => {
          setOpenEmail(false);
          reloadUser();
        }}
        turnstileEnabled={!!siteInfo?.turnstile_check}
        turnstileSiteKey={siteInfo?.turnstile_site_key || ''}
      />

      <Dialog open={openWechat} onOpenChange={setOpenWechat}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t(tk('wechat'))}</DialogTitle>
            <DialogDescription>{t('login.wechatLoginInfo')}</DialogDescription>
          </DialogHeader>
          <DialogBody className="space-y-4">
            {siteInfo?.wechat_qrcode && (
              <img src={siteInfo.wechat_qrcode} alt="wechat qrcode" className="mx-auto size-48 rounded-md border border-border" />
            )}
            <Input value={wechatCode} onChange={(e) => setWechatCode(e.target.value)} placeholder={t('profilePage.code')} />
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpenWechat(false)}>
              {t('common.cancel')}
            </Button>
            <Button onClick={bindWeChat}>{t('common.submit')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!unbindTarget} onOpenChange={(o) => !o && setUnbindTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('profilePage.unbindConfirm')}</DialogTitle>
            <DialogDescription>{t('profilePage.unbindWarning')}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setUnbindTarget(null)}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={confirmUnbind}>
              {t(tk('unlink'))}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SecurityCard>
  );
}

IdentifiersCard.propTypes = {
  siteInfo: PropTypes.object,
  inputs: PropTypes.object,
  reloadUser: PropTypes.func
};
