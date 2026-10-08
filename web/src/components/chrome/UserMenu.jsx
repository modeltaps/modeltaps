import { useNavigate } from 'react-router';
import { useSelector } from 'react-redux';
import { CreditCard, KeyRound, LayoutDashboard, LogOut, Settings, ShieldUser } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator
} from '@/components/ui/dropdown-menu';
import { useIsAdmin } from 'utils/common';
import { saveUserReturnPath } from 'utils/adminPlane';
import useLogin from 'hooks/useLogin';
import UserAvatar, { accountDisplayName } from './avatar';

// ==============================|| CHROME — USER MENU (public header avatar) ||============================== //
// 只用于公开页(落地页)头部的头像下拉:控制台、API Key、充值、设置、管理后台、退出登录。
// 面板内没有用户菜单:身份与切换在侧栏顶部账号行,主题 / 语言 / 退出在页脚「更多」。

export default function UserMenu() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const user = useSelector((state) => state.account.user);
  const isAdmin = useIsAdmin();
  const { logout } = useLogin();

  const displayName = accountDisplayName(user);
  // Secondary identifier under the name: email if set, else @username (so the
  // real username still shows even when a display name is set).
  const secondary =
    user?.email && user.email !== displayName ? user.email : user?.display_name && user?.username ? `@${user.username}` : '';

  const enterAdminConsole = () => {
    saveUserReturnPath('/panel/dashboard');
    navigate('/panel/analytics');
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={displayName}
          className="rounded-full transition-opacity hover:opacity-90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <UserAvatar user={user} className="size-9" textClass="text-sm" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-56">
        <div className="flex items-center gap-2.5 px-2 py-1.5">
          <UserAvatar user={user} className="size-9" textClass="text-sm" />
          <div className="min-w-0">
            <p className="truncate text-sm font-medium text-foreground">{displayName}</p>
            {secondary && <p className="truncate text-xs text-muted-foreground">{secondary}</p>}
          </div>
        </div>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => navigate('/panel/dashboard')}>
          <LayoutDashboard />
          {t('home.cta.console')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => navigate('/panel/token')}>
          <KeyRound />
          {t('api_token')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => navigate('/panel/billing?topup=1')}>
          <CreditCard />
          {t('topup', { defaultValue: 'Top-up' })}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => navigate('/panel/settings')}>
          <Settings />
          {t('userMenu.settings', { defaultValue: 'Settings' })}
        </DropdownMenuItem>
        {isAdmin && (
          <DropdownMenuItem onClick={enterAdminConsole}>
            <ShieldUser />
            {t('admin_console.enter')}
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={logout}>
          <LogOut />
          {t('common.logout', { defaultValue: 'Log out' })}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
