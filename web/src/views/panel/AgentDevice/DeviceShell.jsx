import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { useSelector } from 'react-redux';
import { Link, useLocation } from 'react-router';

import { BrandLockup } from '@/components/chrome/Logo';
import UserAvatar, { accountDisplayName } from '@/components/chrome/avatar';
import { cn } from '@/lib/utils';
import { brandName } from 'utils/brand';
import { requestLogout } from 'utils/logout';
import { workbench } from 'routes/paths';
import { clearChatSession } from 'views/panel/Playground/chat/chatPersistence';

// ==============================|| AGENT DEVICE — PAGE SHELL ||============================== //
// The frame of the device authorization page: the brand and the signed-in account on top, one
// narrow column in the middle, like other consent screens. No console sidebar to wander off to.

// Signing in as someone else comes back here with the same code. With an external identity
// provider its own sign-out page takes over, as it does everywhere else.
async function switchAccount(returnTo) {
  const providerLogout = await requestLogout();
  localStorage.removeItem('user');
  clearChatSession();
  window.location.assign(providerLogout || `/login?redirect=${encodeURIComponent(returnTo)}`);
}

export function Shell({ children, returnTo }) {
  const { t } = useTranslation();
  const { pathname, search } = useLocation();
  const siteInfo = useSelector((state) => state.siteInfo);
  const user = useSelector((state) => state.account.user);
  const name = brandName(siteInfo.system_name);
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="flex h-14 shrink-0 items-center justify-between gap-4 px-4 sm:px-6">
        <Link to={workbench.dashboard()} className="rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          {siteInfo.logo ? (
            <img src={siteInfo.logo} alt={name} className="h-6 object-contain" />
          ) : (
            <BrandLockup text={name} wordmark className="text-foreground" />
          )}
        </Link>
        {user && (
          <div className="flex min-w-0 items-center gap-2 text-sm" data-testid="device-account">
            <UserAvatar user={user} className="size-6" textClass="text-[10px]" />
            <span className="truncate font-medium">{accountDisplayName(user)}</span>
            <span className="text-border">/</span>
            <button
              type="button"
              className="shrink-0 rounded-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              onClick={() => switchAccount(returnTo || pathname + search)}
            >
              {t('agentDevice.switchAccount')}
            </button>
          </div>
        )}
      </header>
      <main className="flex flex-1 justify-center px-4 pb-16 pt-6 sm:pt-[9vh]">
        <div className="w-full max-w-[480px]">{children}</div>
      </main>
    </div>
  );
}

Shell.propTypes = { children: PropTypes.node, returnTo: PropTypes.string };

export function Heading({ title, children, className }) {
  return (
    <div className={cn('mb-6 text-center', className)}>
      <h1 className="text-balance text-xl font-medium tracking-[-0.01em]">{title}</h1>
      {children && <div className="mt-2 text-sm text-muted-foreground">{children}</div>}
    </div>
  );
}

Heading.propTypes = { title: PropTypes.node, children: PropTypes.node, className: PropTypes.string };

export function Outcome({ icon: Icon, tone, title, children, testId }) {
  return (
    <div className="text-center" data-testid={testId}>
      <span
        className={cn(
          'mx-auto mb-5 flex size-12 items-center justify-center rounded-full',
          tone === 'good' ? 'bg-success/10 text-success' : 'bg-muted text-muted-foreground'
        )}
      >
        <Icon className="size-6" strokeWidth={1.75} />
      </span>
      <h1 className="text-xl font-medium tracking-[-0.01em]">{title}</h1>
      <div className="mt-2 text-sm text-muted-foreground">{children}</div>
    </div>
  );
}

Outcome.propTypes = {
  icon: PropTypes.elementType.isRequired,
  tone: PropTypes.oneOf(['good']),
  title: PropTypes.node,
  children: PropTypes.node,
  testId: PropTypes.string
};
