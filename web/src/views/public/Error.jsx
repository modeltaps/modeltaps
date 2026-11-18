import { Link, useLocation, useNavigate } from 'react-router';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';

import { Button } from '@/components/ui/button';
import ErrorState from '@/components/ErrorState';
import { PANEL, workbench } from 'routes/paths';

// ==============================|| 404 / NOT FOUND ||============================== //
// Unknown addresses, and admin pages for non-admins (the guards render this in place so the page's
// existence is not revealed). Inside the console the page header already says "Page not found",
// so the title is left out there.

export default function NotFoundView() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { pathname, search, key } = useLocation();
  const signedIn = Boolean(useSelector((state) => state.account.user));
  const inPanel = pathname === PANEL || pathname.startsWith(`${PANEL}/`);
  const toConsole = inPanel || signedIn;
  // 'default' is the first entry of this tab: there is no in-app page to go back to.
  const canGoBack = key !== 'default';

  return (
    <ErrorState
      code="404"
      title={inPanel ? null : t('errorPage.notFoundTitle')}
      description={t('errorPage.notFoundBody')}
      path={pathname === '/404' ? null : `${pathname}${search}`}
      actions={
        <>
          {canGoBack && (
            <Button variant="outline" onClick={() => navigate(-1)}>
              {t('errorPage.goBack')}
            </Button>
          )}
          <Button asChild>
            <Link to={toConsole ? workbench.dashboard() : '/'}>
              {t(inPanel ? 'errorPage.goDashboard' : signedIn ? 'errorPage.goConsole' : 'errorPage.goHome')}
            </Link>
          </Button>
        </>
      }
    />
  );
}
