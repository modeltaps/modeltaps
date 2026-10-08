import { lazy } from 'react';
import { Navigate, useLocation } from 'react-router';

// project imports
import Loadable from 'ui-component/Loadable';
import PublicLayout from 'layout/PublicLayout';

// Auth + sandbox pages render standalone (no public chrome wrapper).
const Sandbox = Loadable(lazy(() => import('views/Sandbox')));
const Login = Loadable(lazy(() => import('views/auth/Login')));
const AdminLogin = Loadable(lazy(() => import('views/auth/AdminLogin')));
const Register = Loadable(lazy(() => import('views/auth/Register')));
const ForgetPassword = Loadable(lazy(() => import('views/auth/ForgetPassword')));
const ResetPassword = Loadable(lazy(() => import('views/auth/ResetPassword')));
const SignedOut = Loadable(lazy(() => import('views/auth/SignedOut')));

// OAuth callbacks stay at /oauth/* so external providers can redirect back.
const OAuthCallback = Loadable(lazy(() => import('views/auth-callbacks/OAuthCallback')));
const Home = Loadable(lazy(() => import('views/public/Home')));
const About = Loadable(lazy(() => import('views/public/About')));
const NotFoundView = Loadable(lazy(() => import('views/public/Error')));
const Jump = Loadable(lazy(() => import('views/public/Jump')));
const ModelPrice = Loadable(lazy(() => import('views/public/ModelPrice')));

// 公开演练场页已删除：旧书签落到面板内的 API 控制台，保留 query(如 `?model=`)。
const PlaygroundRedirect = () => {
  const { search, hash } = useLocation();
  return <Navigate to={`/panel/api/chat${search}${hash}`} replace />;
};

// ==============================|| PUBLIC ROUTING (shadcn) ||============================== //

const OtherRoutes = [
  {
    path: '/login',
    element: <Login />
  },
  {
    // 管理员应急登录:只在外部账号体系下存在,仅 root 能通过;旧的 /login?local=1 由后端 301 到这里。
    path: '/login/admin',
    element: <AdminLogin />
  },
  {
    path: '/register',
    element: <Register />
  },
  {
    path: '/forgot-password',
    element: <ForgetPassword />
  },
  {
    path: '/reset-password',
    element: <ResetPassword />
  },
  {
    // 退出登录的落点：公开路由（不经 AuthGuard），后端也把它登记为 IdP 的 post_logout_redirect_uri。
    path: '/signed-out',
    element: <SignedOut />
  },
  {
    path: '/reset',
    element: <Navigate to="/forgot-password" replace />
  },
  {
    path: '/user/reset',
    element: <Navigate to="/reset-password" replace />
  },
  {
    path: '/sandbox',
    element: <Sandbox />
  },
  {
    path: '/',
    element: <PublicLayout />,
    children: [
      {
        path: '',
        element: <Home />
      },
      {
        path: '/about',
        element: <About />
      },
      {
        path: '/oauth/github',
        element: <OAuthCallback provider="github" />
      },
      {
        // 无 slug 的旧回调保留：存量部署在 IdP 侧登记的就是它，后端等价于 slug=oidc。
        path: '/oauth/oidc',
        element: <OAuthCallback provider="oidc" />
      },
      {
        path: '/oauth/oidc/:slug',
        element: <OAuthCallback provider="oidc" />
      },
      {
        path: '/oauth/lark',
        element: <OAuthCallback provider="lark" />
      },
      {
        path: '/oauth/linuxdo',
        element: <OAuthCallback provider="linuxdo" />
      },
      {
        path: '/404',
        element: <NotFoundView />
      },
      {
        path: '/jump',
        element: <Jump />
      },
      {
        path: '/price',
        element: <ModelPrice />
      },
      {
        path: '/playground',
        element: <PlaygroundRedirect />
      },
      {
        path: '*',
        element: <NotFoundView />
      }
    ]
  }
];

export default OtherRoutes;
