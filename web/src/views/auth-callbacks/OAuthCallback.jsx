import { useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useNavigate, useParams, useSearchParams } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import useLogin from 'hooks/useLogin';
import { onGitHubOAuthClicked, onLarkOAuthClicked, onOIDCAuthClicked, onLinuxDoOAuthClicked } from 'utils/common';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { LogoMark } from '@/components/chrome/Logo';
import OAuthInviteCodeDialog from '@/components/OAuthInviteCodeDialog';
import { oidcFailureExit, oidcFailureKey } from './oidcFailure';

// ==============================|| OAUTH CALLBACK (shared — github / lark / oidc / linuxdo) ||============================== //
// 回调失败后回 /login(开了「登录直达」时会再跳一次 IdP,相当于重新走登录)。
// 但本站身份是唯一登录入口时,回过去只会被原样送回 IdP,是死路 —— 此时停在终态卡片,
// 由本页给出「重新登录 / 返回首页」(UX-35)。

export default function OAuthCallback({ provider }) {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  // /oauth/oidc/:slug 的提供方；旧的 /oauth/oidc 没有 slug，后端按 slug=oidc 处理。
  const { slug: oidcSlug = '' } = useParams();
  const siteInfo = useSelector((state) => state.siteInfo);
  const { githubLogin, larkLogin, oidcLogin, linuxDoLogin } = useLogin();

  const [prompt, setPrompt] = useState(t('common.processing'));
  const [showInviteDialog, setShowInviteDialog] = useState(false);
  // 终态：停掉转圈，给出出口。出口不记进 state,按当前 siteInfo 现算 —— 挂载即发 sendCode,失败可能
  // 早于 /api/status 回来,那一刻算出的是初值(内置模式)下的「去登录」,而外部模式下回 /login 只会被
  // 原样送回 IdP,转一圈撞回同一堵墙。
  const [failed, setFailed] = useState(false);
  // 失败出口：'retry' = 死路，停在本页；'login' = 回本站登录页逃生口
  const failureExit = provider === 'oidc' ? oidcFailureExit(siteInfo, oidcSlug) : 'login';
  const terminalAction = failed ? failureExit : '';
  const deadEnd = failureExit === 'retry';
  // 异步流程里要用最新的 siteInfo:挂载时闭包里的那份可能还没加载完。
  const siteInfoRef = useRef(siteInfo);
  siteInfoRef.current = siteInfo;
  const deadEndNow = () => (provider === 'oidc' ? oidcFailureExit(siteInfoRef.current, oidcSlug) === 'retry' : false);

  // Per-provider wiring. Note: lark intentionally reuses the github error keys to
  // match the original v1 behavior (not a typo to "fix" here).
  const config = {
    github: {
      login: githubLogin,
      titleKey: 'login.githubLogin',
      errorKey: 'login.githubError',
      countErrorKey: 'login.githubCountError',
      reauth: () => onGitHubOAuthClicked(siteInfo.github_client_id)
    },
    lark: {
      login: larkLogin,
      titleKey: 'login.larkLogin',
      errorKey: 'login.githubError',
      countErrorKey: 'login.githubCountError',
      reauth: () => onLarkOAuthClicked(siteInfo.lark_client_id)
    },
    oidc: {
      login: (code, state) => oidcLogin(code, state, oidcSlug),
      titleKey: 'login.signingIn',
      errorKey: 'login.oidcError',
      countErrorKey: 'login.oidcCountError',
      reauth: () => onOIDCAuthClicked(oidcSlug)
    },
    linuxdo: {
      login: linuxDoLogin,
      titleKey: 'login.linuxDoLogin',
      errorKey: 'login.linuxDoError',
      countErrorKey: 'login.linuxDoCountError',
      reauth: () => onLinuxDoOAuthClicked(siteInfo.linuxDo_client_id)
    }
  }[provider];

  const sendCode = async (code, state, count) => {
    const { success, message } = await config.login(code, state);
    if (!success) {
      if (message && message.startsWith('NEED_INVITE_CODE:')) {
        const actualMessage = message.substring('NEED_INVITE_CODE:'.length);
        setPrompt(actualMessage || t('oauthInvite.needCode'));
        setShowInviteDialog(true);
        return;
      }

      // 身份冲突 / 不给建号 / 站点关闭注册：都不重试，用前端文案说明（后端原文可能带供应商名，不展示）
      const failureKey = oidcFailureKey(message);
      if (failureKey) {
        setPrompt(t(failureKey));
        setFailed(true);
        return;
      }

      // 后端 HTTP 错误已由 utils/api.js 响应拦截器统一弹过 toast，这里不再重复
      if (count === 0) {
        setPrompt(t(config.errorKey));
        if (deadEndNow()) {
          setFailed(true);
          return;
        }
        await new Promise((resolve) => setTimeout(resolve, 2000));
        // 等这 2 秒的过程中站点状态可能才回来：外部模式下回 /login 是死路，改为停在本页
        if (deadEndNow()) {
          setFailed(true);
          return;
        }
        navigate('/login');
        return;
      }
      count++;
      setPrompt(t(config.countErrorKey, { count }));
      await new Promise((resolve) => setTimeout(resolve, 2000));
      await sendCode(code, state, count);
    }
  };

  // 处理邀请码确认 - 重新授权
  const handleInviteCodeConfirm = () => {
    setShowInviteDialog(false);
    setPrompt(t('oauthInvite.codeSetReauth'));
    setTimeout(() => {
      config.reauth();
    }, 1000);
  };

  // 处理邀请码对话框关闭：死路时回本站登录页仍是死循环，停在本页终态给出口
  const handleInviteCodeClose = () => {
    setShowInviteDialog(false);
    if (deadEnd) {
      setPrompt(t('oauthInvite.cancelled'));
      setFailed(true);
      return;
    }
    navigate('/login');
  };

  useEffect(() => {
    const code = searchParams.get('code');
    const state = searchParams.get('state');
    sendCode(code, state, 0).then();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="flex min-h-[calc(100vh-8rem)] items-center justify-center px-4 py-6">
      <Card className="w-full max-w-md shadow-sm">
        <CardHeader className="items-center space-y-3 text-center">
          <LogoMark className="h-9 w-auto text-foreground" />
          <CardTitle className="text-2xl text-foreground">{t(terminalAction ? 'login.signInFailed' : config.titleKey)}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-5 py-6">
          {!terminalAction && <Loader2 className="size-10 animate-spin text-muted-foreground" />}
          <p className="text-center text-base font-medium">{prompt}</p>
          {terminalAction === 'login' && <Button onClick={() => navigate('/login')}>{t('login.goLogin')}</Button>}
          {terminalAction === 'retry' && (
            <div className="flex flex-wrap items-center justify-center gap-3">
              <Button onClick={() => config.reauth()}>{t('login.retryLogin')}</Button>
              <Button variant="outline" onClick={() => navigate('/')}>
                {t('login.backHome')}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <OAuthInviteCodeDialog
        open={showInviteDialog}
        onClose={handleInviteCodeClose}
        onConfirm={handleInviteCodeConfirm}
        provider={provider}
        providerSlug={oidcSlug}
      />
    </div>
  );
}

OAuthCallback.propTypes = {
  provider: PropTypes.oneOf(['github', 'lark', 'oidc', 'linuxdo']).isRequired
};
