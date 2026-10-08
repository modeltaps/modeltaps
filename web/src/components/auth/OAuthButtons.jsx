import { useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';

import { Button } from '@/components/ui/button';
import useLogin from 'hooks/useLogin';
import WechatModal from './WechatModal';
import { onGitHubOAuthClicked, onLarkOAuthClicked, onLinuxDoOAuthClicked } from 'utils/common';
import { socialLoginEnabled } from 'utils/authAvailability';

import Github from 'assets/images/icons/github.svg';
import Wechat from 'assets/images/icons/wechat.svg';
import Lark from 'assets/images/icons/lark.svg';
import LinuxDoIcon from 'assets/images/icons/LinuxDoIcon';

// ==============================|| AUTH — SOCIAL LOGIN BUTTONS ||============================== //
// 只渲染第三方账号登录(GitHub / 微信 / 飞书 / LinuxDo),按钮上只出现第三方账号名。
// 身份提供方(Authgear / Keycloak / Auth0 …)不是第三方登录,它就是账号体系本身:外部模式下
// /login 直接跳过去,不在这里出按钮。

const ProviderButton = ({ onClick, icon, label }) => (
  <Button type="button" variant="outline" className="h-11 w-full justify-center gap-3 font-medium" onClick={onClick}>
    <span className="flex size-[22px] items-center justify-center">{icon}</span>
    {label}
  </Button>
);

ProviderButton.propTypes = {
  onClick: PropTypes.func,
  icon: PropTypes.node,
  label: PropTypes.node
};

// divider:按钮组之后是否渲染分隔线。下方没有表单时(注册页降级态)不需要分隔。
export default function OAuthButtons({ divider = true }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const { wechatLogin } = useLogin('/panel/dashboard');
  const [openWechat, setOpenWechat] = useState(false);

  if (!socialLoginEnabled(siteInfo)) return null;

  return (
    <div className="space-y-3">
      <div className="space-y-3">
        {siteInfo.github_oauth && (
          <ProviderButton
            onClick={() => onGitHubOAuthClicked(siteInfo.github_client_id)}
            icon={<img src={Github} alt="github" width={22} height={22} />}
            label={t('login.useGithubLogin')}
          />
        )}
        {siteInfo.wechat_login && (
          <>
            <ProviderButton
              onClick={() => setOpenWechat(true)}
              icon={<img src={Wechat} alt="wechat" width={22} height={22} />}
              label={t('login.useWechatLogin')}
            />
            <WechatModal
              open={openWechat}
              handleClose={() => setOpenWechat(false)}
              wechatLogin={wechatLogin}
              qrCode={siteInfo.wechat_qrcode}
            />
          </>
        )}
        {siteInfo.lark_login && (
          <ProviderButton
            onClick={() => onLarkOAuthClicked(siteInfo.lark_client_id)}
            icon={<img src={Lark} alt="lark" width={22} height={22} />}
            label={t('login.useLarkLogin')}
          />
        )}
        {siteInfo.linuxDo_oauth && (
          <ProviderButton
            onClick={() => onLinuxDoOAuthClicked(siteInfo.linuxDo_client_id, true)}
            icon={<LinuxDoIcon size={22} />}
            label={t('login.useLinuxDoLogin')}
          />
        )}
      </div>

      {divider && (
        <div className="flex items-center gap-3 py-1">
          <span className="h-px flex-1 bg-border" />
          <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{t('login.or', { defaultValue: 'OR' })}</span>
          <span className="h-px flex-1 bg-border" />
        </div>
      )}
    </div>
  );
}

OAuthButtons.propTypes = {
  divider: PropTypes.bool
};
