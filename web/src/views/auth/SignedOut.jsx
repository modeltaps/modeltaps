import { useContext } from 'react';
import { Link, useNavigate } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { LogoMark } from '@/components/chrome/Logo';
import { UserContext } from 'contexts/UserContext';
import { brandName } from 'utils/brand';

// ==============================|| AUTH — SIGNED OUT ||============================== //
// 退出登录的落点：公开路由，不经 AuthGuard。IdP 结束会话后也回到这里（后端下发的
// post_logout_redirect_uri）。用回退键回到本页而会话仍在时，改成「你仍在登录状态」+ 进入控制台，
// 避免用户以为自己已经退出。

export default function SignedOut() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const siteInfo = useSelector((state) => state.siteInfo);
  const account = useSelector((state) => state.account);
  const { isUserLoaded } = useContext(UserContext) || {};
  const stillSignedIn = Boolean(isUserLoaded && account.user);
  const name = brandName(siteInfo.system_name);

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-background px-4 py-6 text-foreground">
      <Card className="w-full max-w-sm shadow-sm">
        <CardHeader className="items-center space-y-3 text-center">
          {siteInfo.logo ? (
            <img src={siteInfo.logo} alt={name} className="size-12 object-contain" />
          ) : (
            <LogoMark className="h-9 w-auto text-foreground" />
          )}
          <CardTitle className="text-2xl">{t(stillSignedIn ? 'login.stillSignedInTitle' : 'login.signedOutTitle')}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-5 pt-2 text-center">
          <p className="text-sm text-muted-foreground">
            {stillSignedIn ? t('login.stillSignedInDescription') : t('login.signedOutDescription', { name })}
          </p>
          {stillSignedIn ? (
            <Button className="h-11 w-full font-semibold" onClick={() => navigate('/panel/dashboard')}>
              {t('login.goConsole')}
            </Button>
          ) : (
            <Button className="h-11 w-full font-semibold" onClick={() => navigate('/login')}>
              {t('login.retryLogin')}
            </Button>
          )}
          <Link to="/" className="text-sm font-medium text-primary-text hover:underline">
            {t('login.backHome')}
          </Link>
        </CardContent>
      </Card>
    </div>
  );
}
