import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { matchRoutes, Outlet, useLocation } from 'react-router';

import AuthGuard from 'utils/route-guard/AuthGuard';
import { cn } from '@/lib/utils';
import useTheme from 'hooks/useTheme';
import useMediaQuery from 'hooks/useMediaQuery';
import { setDocumentTitlePrefix } from 'utils/documentTitle';
import { useIsAdmin } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';
import PageHeader from '@/components/chrome/PageHeader';
import { PageHeaderOverrideContext } from '@/components/chrome/PageHeaderOverride';
import { ContentMinWidthContext } from '@/components/chrome/ContentWidth';
import { SidebarToggle } from '@/components/chrome/SidebarControls';
import { PageActionsContext } from '@/components/chrome/PageActions';
import { PageTitleExtraContext } from '@/components/chrome/PageTitleExtra';
import Sidebar from '@/components/chrome/Sidebar';
import CommandMenu from '@/components/chrome/CommandMenu';
import { deriveContext, findActiveItem, navLabel } from '@/components/chrome/nav-config';
import MainRoutes from 'routes/MainRoutes';
import MobilePushFrame from './MobilePushFrame';

// ==============================|| MAIN LAYOUT (shadcn app chrome) ||============================== //
// One sidebar shell for every panel page. Routes are unchanged (/panel/*): the context
// (personal / org / platform admin) is derived from the path and the active organization
// (nav-config `deriveContext`). Non-admins on admin routes keep the workbench chrome and the
// route guard's 404 in place. Settings pages (personal, organization, platform) render inside
// the shell with an in-page subnav; their titles come from the nav item like any other page.

export default function MainLayout() {
  // Initialize the theme (applies the .dark class on <html> from localStorage).
  useTheme();

  const { t } = useTranslation();
  const { pathname } = useLocation();
  const isAdmin = useIsAdmin();
  const { currentOrgId } = useOrg();
  const context = deriveContext({ pathname, isAdmin, orgId: currentOrgId });
  const activeItem = findActiveItem(pathname);

  // The tab title names the space only in platform admin, to tell tabs apart; the brand part
  // is written from the site status (utils/documentTitle composes both).
  const titlePrefix = context === 'admin' ? t('nav.context.admin') : null;
  useEffect(() => {
    setDocumentTitlePrefix(titlePrefix);
    return () => setDocumentTitlePrefix(null);
  }, [titlePrefix]);

  // Unknown /panel/* paths fall through to the trailing '*' 404 route; title it "page not found"
  // instead of whatever a prefix match would give.
  const isNotFound = useMemo(() => {
    const matched = matchRoutes(MainRoutes.children, pathname, MainRoutes.path);
    return !matched || matched[matched.length - 1].route.path === '*';
  }, [pathname]);

  // The rail collapses to icons on its own below the lg breakpoint (1024px), and also as soon as
  // the expanded rail would squeeze a page below the width it asked for (useContentMinWidth, e.g.
  // a table's natural width). The user's own toggle wins until that automatic choice changes.
  const belowLg = useMediaQuery('(max-width: 1023px)');
  const [contentMinWidth, setContentMinWidth] = useState(null);
  const [viewportWidth, setViewportWidth] = useState(() => (typeof window === 'undefined' ? 1440 : window.innerWidth));
  useEffect(() => {
    const onResize = () => setViewportWidth(window.innerWidth);
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);
  // Content width beside the expanded rail: the viewport minus the w-64 rail, its border and the
  // main area's horizontal padding (px-6).
  const roomBesideRail = viewportWidth - 256 - 1 - 48;
  const autoCollapsed = belowLg || (contentMinWidth != null && roomBesideRail < contentMinWidth);
  const [collapseOverride, setCollapseOverride] = useState(null);
  useEffect(() => {
    setCollapseOverride(null);
  }, [autoCollapsed]);
  const collapsed = collapseOverride ?? autoCollapsed;
  const [mobileOpen, setMobileOpen] = useState(false);
  const [commandOpen, setCommandOpen] = useState(false);
  // DOM node of the PageHeader action slot; pages portal their primary actions
  // into it via <PageActions>. Set through a callback ref so consumers re-render
  // once the slot mounts.
  const [actionsSlot, setActionsSlot] = useState(null);
  // DOM node of the PageHeader inline title-extra slot; pages portal controls
  // that sit left-aligned next to the title into it via <PageTitleExtra>.
  const [titleExtraSlot, setTitleExtraSlot] = useState(null);
  // Title set by a detail page through usePageHeader; replaces the nav-derived one.
  const [headerOverride, setHeaderOverride] = useState(null);

  const toggleCollapse = () => setCollapseOverride(!collapsed);
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
    : headerOverride
      ? headerOverride
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
              <Sidebar context={context} collapsed={collapsed} onToggleCollapse={toggleCollapse} onOpenCommand={openCommand} />
            </aside>
          }
          drawer={
            <Sidebar
              context={context}
              collapsed={false}
              onItemClick={() => setMobileOpen(false)}
              onToggleCollapse={() => setMobileOpen(false)}
              onOpenCommand={openCommandFromDrawer}
            />
          }
        >
          {/* Content column (pushed right while the mobile drawer is open) */}
          <main className="flex-1 overflow-y-auto px-4 pb-4 pt-4 sm:px-6 sm:pb-6">
            {pageHeader ? (
              <PageHeader
                title={pageHeader.title}
                subtitle={pageHeader.subtitle}
                back={pageHeader.back}
                leading={toggle}
                actionsRef={setActionsSlot}
                titleExtraRef={setTitleExtraSlot}
              />
            ) : (
              /* 当前页没有标题时按钮单独占一行,电脑端侧栏展开时整行隐藏 */
              <div className={cn('mb-4 flex', !collapsed && 'md:hidden')}>{toggle}</div>
            )}
            <PageHeaderOverrideContext.Provider value={setHeaderOverride}>
              <PageActionsContext.Provider value={actionsSlot}>
                <PageTitleExtraContext.Provider value={titleExtraSlot}>
                  <ContentMinWidthContext.Provider value={setContentMinWidth}>
                    <Outlet />
                  </ContentMinWidthContext.Provider>
                </PageTitleExtraContext.Provider>
              </PageActionsContext.Provider>
            </PageHeaderOverrideContext.Provider>
          </main>
        </MobilePushFrame>

        <CommandMenu open={commandOpen} setOpen={setCommandOpen} />
      </div>
    </AuthGuard>
  );
}
