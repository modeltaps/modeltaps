import { lazy } from 'react';
import { Navigate, Outlet, useLocation } from 'react-router';

// project imports
import Loadable from 'ui-component/Loadable';
import AdminGuard from 'utils/route-guard/AdminGuard';
import RootGuard from 'utils/route-guard/RootGuard';
import { OrgIndexRedirect, OrgScope, PersonalScope } from './scopes';
import { legacyRedirects } from './legacyRedirects';
import { PANEL, admin } from './paths';

const MainLayout = Loadable(lazy(() => import('layout/MainLayout')));
const NotFoundView = Loadable(lazy(() => import('views/public/Error')));
const Token = Loadable(lazy(() => import('views/panel/Token')));
const Log = Loadable(lazy(() => import('views/panel/Log')));
const Billing = Loadable(lazy(() => import('views/panel/Billing')));
const Dashboard = Loadable(lazy(() => import('views/panel/Dashboard')));
const Analytics = Loadable(lazy(() => import('views/panel/Analytics')));
const Channel = Loadable(lazy(() => import('views/panel/Channel')));
const RoutingPolicy = Loadable(lazy(() => import('views/panel/RoutingPolicy')));
const Routing = Loadable(lazy(() => import('views/panel/Routing')));
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
const InviteCode = Loadable(lazy(() => import('views/panel/InviteCode')));
const Pricing = Loadable(lazy(() => import('views/panel/Pricing')));
const Usage = Loadable(lazy(() => import('views/panel/Usage')));
const OrgAdmin = Loadable(lazy(() => import('views/panel/OrgAdmin')));
const Organization = Loadable(lazy(() => import('views/panel/Organization')));
const MyInvitations = Loadable(lazy(() => import('views/panel/Organization/MyInvitations')));
const AgentDevice = Loadable(lazy(() => import('views/panel/AgentDevice')));
const Settings = Loadable(lazy(() => import('views/panel/Settings')));

// `playground` without a modality opens the chat console, keeping the query (`?model=`).
const PlaygroundIndex = () => {
  const { search, hash } = useLocation();
  return <Navigate to={`chat${search}${hash}`} replace />;
};

// ==============================|| MAIN ROUTING (authenticated panel) ||============================== //
// The URL scheme (path builders and rules in routes/paths.js):
//
//   /panel/<page>                     personal workbench
//   /panel/orgs/:orgSlug/<page>       an organization's workbench (same pages + overview)
//   /panel/settings/...               settings hub: personal sections, organizations' settings
//   /panel/admin/<page>               platform administration (admins only)
//
// Workbench pages: dashboard, usage, logs[/tasks|/midjourney], keys, models, routing-policies,
// playground/:modality, docs[/:modality], billing[/orders|/invoices].
// Old URLs redirect (routes/legacyRedirects.jsx).

const adminOnly = (element) => <AdminGuard>{element}</AdminGuard>;
const rootOnly = (element) => <RootGuard>{element}</RootGuard>;

// The pages every workbench scope has.
const workbenchPages = () => [
  { path: 'dashboard', element: <Dashboard /> },
  { path: 'usage', element: <Usage /> },
  { path: 'logs/:tab?', element: <Log /> },
  { path: 'keys', element: <Token /> },
  { path: 'models', element: <ModelPrice embedded /> },
  { path: 'routing-policies', element: <Routing /> },
  { path: 'playground', element: <PlaygroundIndex /> },
  { path: 'playground/:modality', element: <ApiConsole /> },
  { path: 'docs', element: <ApiCatalogOverview /> },
  { path: 'docs/:modality', element: <ApiCatalog /> },
  { path: 'billing/:tab?', element: <Billing /> }
];

const MainRoutes = {
  path: PANEL,
  element: <MainLayout />,
  children: [
    {
      element: <PersonalScope />,
      children: [
        { index: true, element: <Navigate to="dashboard" replace /> },
        ...workbenchPages(),
        { path: 'invitations', element: <MyInvitations /> },
        { path: 'device', element: <AgentDevice /> }
      ]
    },
    {
      path: 'orgs/:orgSlug',
      element: <OrgScope />,
      children: [{ index: true, element: <OrgIndexRedirect /> }, ...workbenchPages(), { path: 'overview', element: <Organization /> }]
    },
    // Settings hub: it never changes the workbench scope.
    { path: 'settings', element: <Settings /> },
    { path: 'settings/:section', element: <Settings /> },
    { path: 'settings/orgs/:orgSlug/:section', element: <Settings /> },
    {
      path: 'admin',
      element: adminOnly(<Outlet />),
      children: [
        { index: true, element: <Navigate to={admin.stats()} replace /> },
        { path: 'stats', element: <Analytics /> },
        { path: 'stats/users', element: <MultiUserStats /> },
        { path: 'system-logs/:tab?', element: rootOnly(<SystemInfo />) },
        // One page: `providers/:id` opens that provider's panel, `providers/new` the new provider
        // sheet, over the providers page.
        { path: 'providers/:id?', element: <Channel /> },
        { path: 'models/:tab?', element: <Pricing /> },
        { path: 'routing', element: <RoutingPolicy /> },
        { path: 'users', element: <User /> },
        { path: 'user-groups', element: <UserGroup /> },
        { path: 'orgs', element: <OrgAdmin /> },
        { path: 'orders', element: <Payment /> },
        { path: 'redemption-codes', element: <Redemption /> },
        { path: 'invite-codes', element: <InviteCode /> },
        { path: 'settings', element: <Setting /> },
        { path: 'settings/:section', element: <Setting /> }
      ]
    },
    ...legacyRedirects,
    { path: '*', element: <NotFoundView /> }
  ]
};

export default MainRoutes;
