import { lazy } from 'react';
import { Navigate, useLocation } from 'react-router';

// project imports
import Loadable from 'ui-component/Loadable';
import AdminGuard from 'utils/route-guard/AdminGuard';
import RootGuard from 'utils/route-guard/RootGuard';
import { LEGACY_CATALOG_REDIRECTS } from 'views/panel/Pricing/modelCatalog';

const MainLayout = Loadable(lazy(() => import('layout/MainLayout')));
const NotFoundView = Loadable(lazy(() => import('views/public/Error')));
const Token = Loadable(lazy(() => import('views/panel/Token')));
const Log = Loadable(lazy(() => import('views/panel/Log')));
const Billing = Loadable(lazy(() => import('views/panel/Billing')));
const Dashboard = Loadable(lazy(() => import('views/panel/Dashboard')));
const Analytics = Loadable(lazy(() => import('views/panel/Analytics')));
const Channel = Loadable(lazy(() => import('views/panel/Channel')));
const User = Loadable(lazy(() => import('views/panel/User')));
const UserGroup = Loadable(lazy(() => import('views/panel/UserGroup')));
const Setting = Loadable(lazy(() => import('views/panel/Setting')));
const SystemInfo = Loadable(lazy(() => import('views/panel/SystemInfo')));
const MultiUserStats = Loadable(lazy(() => import('views/panel/MultiUserStats')));
const ApiCatalog = Loadable(lazy(() => import('views/panel/ApiCatalog')));
const ApiConsole = Loadable(lazy(() => import('views/panel/Playground')));
const ApiCatalogOverview = Loadable(lazy(() => import('views/panel/ApiCatalog/Overview')));
const ModelPrice = Loadable(lazy(() => import('views/public/ModelPrice')));
const Payment = Loadable(lazy(() => import('views/panel/Payment')));
const Redemption = Loadable(lazy(() => import('views/panel/Redemption')));
const Pricing = Loadable(lazy(() => import('views/panel/Pricing')));
const Usage = Loadable(lazy(() => import('views/panel/Usage')));
const OrgAdmin = Loadable(lazy(() => import('views/panel/OrgAdmin')));
const Organization = Loadable(lazy(() => import('views/panel/Organization')));
const Settings = Loadable(lazy(() => import('views/panel/Settings')));

// 旧的 /panel/profile 收口到统一设置页,保留 query / hash。日志详情的
// `?highlight=log-io` 指向留存开关,单独落到「数据与隐私」。
const ProfileRedirect = () => {
  const { search, hash } = useLocation();
  const section = new URLSearchParams(search).get('highlight') === 'log-io' ? 'privacy' : 'profile';
  return <Navigate to={`/panel/settings/account/${section}${search}${hash}`} replace />;
};

// 旧的 Playground 入口收口到 API 控制台(`/panel/api/chat`),保留 query(如 `?model=`)。
const PlaygroundRedirect = () => {
  const { search, hash } = useLocation();
  return <Navigate to={`/panel/api/chat${search}${hash}`} replace />;
};

// ==============================|| MAIN ROUTING (authenticated panel) ||============================== //

const MainRoutes = {
  path: '/panel',
  element: <MainLayout />,
  children: [
    {
      index: true,
      element: <Navigate to="dashboard" replace />
    },
    {
      path: 'dashboard',
      element: <Dashboard />
    },
    {
      path: 'analytics',
      element: (
        <AdminGuard>
          <Analytics />
        </AdminGuard>
      )
    },
    {
      path: 'usage',
      element: <Usage />
    },
    {
      path: 'log',
      element: <Log />
    },
    {
      path: 'billing',
      element: <Billing />
    },
    {
      // 旧路由:账户流水已并入账单页 Tab
      path: 'ledger',
      element: <Navigate to="/panel/billing?tab=ledger" replace />
    },
    {
      // 旧路由:异步任务记录已并入日志页 Tab
      path: 'task',
      element: <Navigate to="/panel/log?tab=task" replace />
    },
    {
      path: 'token',
      element: <Token />
    },
    {
      path: 'channel',
      element: (
        <AdminGuard>
          <Channel />
        </AdminGuard>
      )
    },
    {
      path: 'user',
      element: (
        <AdminGuard>
          <User />
        </AdminGuard>
      )
    },
    {
      path: 'user_group',
      element: (
        <AdminGuard>
          <UserGroup />
        </AdminGuard>
      )
    },
    {
      path: 'setting',
      element: (
        <AdminGuard>
          <Setting />
        </AdminGuard>
      )
    },
    {
      path: 'setting/:section',
      element: (
        <AdminGuard>
          <Setting />
        </AdminGuard>
      )
    },
    {
      path: 'profile',
      element: <ProfileRedirect />
    },
    {
      path: 'settings',
      element: <Settings />
    },
    {
      path: 'settings/account/:section',
      element: <Settings />
    },
    {
      path: 'settings/org/:orgId/:section',
      element: <Settings />
    },
    {
      // 旧路由:Playground 已并入 API 控制台
      path: 'playground',
      element: <PlaygroundRedirect />
    },
    {
      path: 'playground/:modality',
      element: <PlaygroundRedirect />
    },
    {
      path: 'api',
      element: <ApiCatalogOverview />
    },
    {
      path: 'api/docs/:modality',
      element: <ApiCatalog />
    },
    {
      path: 'api/:modality',
      element: <ApiConsole />
    },
    {
      path: 'model_price',
      element: <ModelPrice embedded />
    },
    {
      path: 'pricing',
      element: (
        <AdminGuard>
          <Pricing />
        </AdminGuard>
      )
    },
    {
      path: 'system_info',
      element: (
        <RootGuard>
          <SystemInfo />
        </RootGuard>
      )
    },
    {
      path: 'multi_user_stats',
      element: (
        <AdminGuard>
          <MultiUserStats />
        </AdminGuard>
      )
    },
    {
      // 旧路由:Midjourney 记录已并入日志页 Tab
      path: 'midjourney',
      element: <Navigate to="/panel/log?tab=midjourney" replace />
    },
    {
      path: 'payment',
      element: (
        <AdminGuard>
          <Payment />
        </AdminGuard>
      )
    },
    {
      path: 'redemption',
      element: (
        <AdminGuard>
          <Redemption />
        </AdminGuard>
      )
    },
    {
      // 旧路由:Telegram Bot 已并入系统设置「集成」主题页
      path: 'telegram',
      element: <Navigate to="/panel/setting/integrations?highlight=telegram" replace />
    },
    {
      // 旧路由:模型归属 / 模型详情已并入模型页 Tab
      path: 'model_ownedby',
      element: <Navigate to={LEGACY_CATALOG_REDIRECTS.model_ownedby} replace />
    },
    {
      path: 'model_info',
      element: <Navigate to={LEGACY_CATALOG_REDIRECTS.model_info} replace />
    },
    {
      path: 'org_admin',
      element: (
        <AdminGuard>
          <OrgAdmin />
        </AdminGuard>
      )
    },
    {
      // 旧路由:月度账单已并入账单页 Tab
      path: 'invoice',
      element: <Navigate to="/panel/billing?tab=invoice" replace />
    },
    {
      // 旧路由:充值已改为账单页余额卡上的弹层
      path: 'topup',
      element: <Navigate to="/panel/billing?topup=1" replace />
    },
    {
      path: 'organization',
      element: <Organization />
    },
    {
      path: '*',
      element: <NotFoundView />
    }
  ]
};

export default MainRoutes;
