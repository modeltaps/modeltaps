import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { matchRoutes, Outlet, useLocation } from 'react-router';

import AuthGuard from 'utils/route-guard/AuthGuard';
import { cn } from '@/lib/utils';
import useTheme from 'hooks/useTheme';
import { setDocumentTitlePrefix } from 'utils/documentTitle';
import { useIsAdmin } from 'utils/common';
import { trackSettingsReturnPath } from 'utils/adminPlane';
import { useOrg } from 'contexts/OrgContext';
import PageHeader from '@/components/chrome/PageHeader';
import { SidebarToggle } from '@/components/chrome/SidebarControls';
import { PageActionsContext } from '@/components/chrome/PageActions';
import { PageTitleExtraContext } from '@/components/chrome/PageTitleExtra';
import Sidebar from '@/components/chrome/Sidebar';
import AdminSidebar from '@/components/chrome/AdminSidebar';
import SettingsSidebar from '@/components/chrome/SettingsSidebar';
import CommandMenu from '@/components/chrome/CommandMenu';
import { findActiveItem, isAdminPath, navLabel } from '@/components/chrome/nav-config';
import { ACCOUNT_SECTIONS, ORG_SECTIONS, isSettingsPath, parseSettingsPath } from 'views/panel/Settings/sections';
import MainRoutes from 'routes/MainRoutes';
import MobilePushFrame from './MobilePushFrame';

// ==============================|| MAIN LAYOUT (shadcn app chrome) ||============================== //
// Routes are unchanged (/panel/*): the plane is derived from the current path.
// Admins on an admin route get the dedicated admin chrome (AdminSidebar);
// everyone else gets the user chrome. Non-admins on admin routes keep today's
// behavior (user chrome + AdminGuard 404 in place).
// /panel/settings/* is a third plane: the whole sidebar becomes the settings nav
// and the PageHeader shows the current section (no in-page second column).
// /panel/setting/* (system settings) is part of the admin plane: its themes are
// the last group of the admin sidebar.
// Plane order: user settings → admin → user.

export default function MainLayout() {
  // Initialize the theme (applies the .dark class on <html> from localStorage).
  useTheme();

  const { t } = useTranslation();
  const { pathname } = useLocation();
  const isAdmin = useIsAdmin();
  const { organizations } = useOrg();
  const settingsPlane = isSettingsPath(pathname);
  const adminPlane = !settingsPlane && isAdmin && isAdminPath(pathname);
  const SidebarComponent = settingsPlane ? SettingsSidebar : adminPlane ? AdminSidebar : Sidebar;
  // 系统设置主题页的页面标题 = 当前主题名(主题项在 navSections 里登记,findActiveItem 可解析)。
  const activeItem = findActiveItem(pathname);

  // 标签页标题只在管理面带上面名,便于多标签间辨认;品牌名由站点状态那侧写入。
  const titlePrefix = adminPlane ? t('admin_console.enter') : null;
  useEffect(() => {
    setDocumentTitlePrefix(titlePrefix);
    return () => setDocumentTitlePrefix(null);
  }, [titlePrefix]);

  // 设置面外的每个路径都是潜在来源,「返回」回到原处;刷新/直达时没有来源,回用户面首页。
  trackSettingsReturnPath(pathname, settingsPlane);

  // 设置面的页面标题 = 当前 section 名;组织分组再带上组织名作副标题。
  const settingsHeader = useMemo(() => {
    if (!settingsPlane) return null;
    const { orgScope, orgId, section } = parseSettingsPath(pathname);
    const entry = (orgScope ? ORG_SECTIONS : ACCOUNT_SECTIONS).find((item) => item.id === section);
    if (!entry) return null;
    const org = orgScope ? organizations.find((o) => o.id === orgId) : null;
    return { title: t(entry.labelKey, { defaultValue: entry.fallback }), subtitle: org?.name || '' };
  }, [settingsPlane, pathname, organizations, t]);
  // 未登记的 /panel/* 落到 MainRoutes 末尾的 '*' 兜底 404。此时不能沿用
  // findActiveItem 的前缀匹配（/panel/settings/xxx 会误配到「设置」），标题改为「页面不存在」。
  const isNotFound = useMemo(() => {
    const matched = matchRoutes(MainRoutes.children, pathname, MainRoutes.path);
    return !matched || matched[matched.length - 1].route.path === '*';
  }, [pathname]);

  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [commandOpen, setCommandOpen] = useState(false);
  // DOM node of the PageHeader action slot; pages portal their primary actions
  // into it via <PageActions>. Set through a callback ref so consumers re-render
  // once the slot mounts.
  const [actionsSlot, setActionsSlot] = useState(null);
  // DOM node of the PageHeader inline title-extra slot; pages portal controls
  // that sit left-aligned next to the title into it via <PageTitleExtra>.
  const [titleExtraSlot, setTitleExtraSlot] = useState(null);

  const toggleCollapse = () => setCollapsed((v) => !v);
  const openCommand = () => setCommandOpen(true);
  // 抽屉里点搜索:先收起抽屉再开搜索弹窗。
  const openCommandFromDrawer = () => {
    setMobileOpen(false);
    setCommandOpen(true);
  };
  // 没有顶部栏:展开按钮(电脑端收起时)与抽屉菜单按钮(手机端)放在页面标题左侧。
  const toggle = <SidebarToggle collapsed={collapsed} onOpenMobile={() => setMobileOpen(true)} onExpand={toggleCollapse} />;
  const pageHeader = isNotFound
    ? { title: t('common.pageNotFound') }
    : settingsPlane
      ? settingsHeader
      : activeItem && {
          title: navLabel(t, activeItem),
          subtitle: t(`nav_subtitles.${activeItem.id}`, { defaultValue: '' })
        };

  return (
    <AuthGuard>
      <div className="flex h-screen flex-col overflow-hidden bg-background text-foreground">
        <MobilePushFrame
          open={mobileOpen}
          onClose={() => setMobileOpen(false)}
          sidebar={
            /* Desktop rail (full height) */
            <aside
              className={cn('hidden shrink-0 border-r border-border transition-[width] duration-200 md:block', collapsed ? 'w-24' : 'w-64')}
            >
              <SidebarComponent collapsed={collapsed} onToggleCollapse={toggleCollapse} onOpenCommand={openCommand} />
            </aside>
          }
          drawer={
            <SidebarComponent
              collapsed={false}
              onItemClick={() => setMobileOpen(false)}
              onToggleCollapse={() => setMobileOpen(false)}
              onOpenCommand={openCommandFromDrawer}
            />
          }
        >
          {/* Content column (pushed right while the mobile drawer is open) */}
          <main className="flex-1 overflow-y-auto px-4 pb-4 pt-3 sm:px-6 sm:pb-6">
            {pageHeader ? (
              <PageHeader
                title={pageHeader.title}
                subtitle={pageHeader.subtitle}
                leading={toggle}
                actionsRef={setActionsSlot}
                titleExtraRef={setTitleExtraSlot}
              />
            ) : (
              /* 当前页没有标题时按钮单独占一行,电脑端侧栏展开时整行隐藏 */
              <div className={cn('mb-4 flex', !collapsed && 'md:hidden')}>{toggle}</div>
            )}
            <PageActionsContext.Provider value={actionsSlot}>
              <PageTitleExtraContext.Provider value={titleExtraSlot}>
                <Outlet />
              </PageTitleExtraContext.Provider>
            </PageActionsContext.Provider>
          </main>
        </MobilePushFrame>

        <CommandMenu open={commandOpen} setOpen={setCommandOpen} />
      </div>
    </AuthGuard>
  );
}
