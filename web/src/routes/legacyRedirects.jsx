import PropTypes from 'prop-types';
import { Navigate, useLocation, useParams } from 'react-router';

import { useOrg } from 'contexts/OrgContext';
import { LegacyOrgPageRedirect } from './scopes';
import { admin, settings, workbench } from './paths';

// ==============================|| ROUTES — LEGACY REDIRECTS ||============================== //
// Every URL the console used before the current scheme, mapped to its page today. Old bookmarks,
// emails and docs keep working; the query string and hash are kept (old `?tab=` values are turned
// into path segments by the target page, see hooks/useRouteTab). Paths are relative to `/panel`.
// Old links had no organization in them, so they open the personal workbench.

// `{ search, hash }` of the current URL merged with extra query parameters.
function withQuery(to, search, hash, extra = {}) {
  const params = new URLSearchParams(search);
  Object.entries(extra).forEach(([k, v]) => params.set(k, v));
  const query = params.toString();
  return `${to}${query ? `?${query}` : ''}${hash}`;
}

function Redirect({ to, query }) {
  const params = useParams();
  const { search, hash } = useLocation();
  const target = typeof to === 'function' ? to(params, new URLSearchParams(search)) : to;
  return <Navigate to={withQuery(target, search, hash, query)} replace />;
}

Redirect.propTypes = {
  to: PropTypes.oneOfType([PropTypes.string, PropTypes.func]).isRequired,
  query: PropTypes.object
};

// `/panel/profile`: the personal profile; `?highlight=log-io` points at the retention switch on
// 数据与隐私.
function ProfileRedirect() {
  const { search, hash } = useLocation();
  const section = new URLSearchParams(search).get('highlight') === 'log-io' ? 'privacy' : 'profile';
  return <Navigate to={withQuery(settings.account(section), search, hash)} replace />;
}

// `/panel/settings/org/:orgId/:section`: organization settings were keyed by numeric id.
function OrgSettingsByIdRedirect() {
  const { orgId, section } = useParams();
  const { search, hash } = useLocation();
  const { orgsLoaded, organizations } = useOrg();
  if (!orgsLoaded) return null;
  const target = organizations.find((o) => String(o.id) === orgId);
  return <Navigate to={withQuery(target ? settings.org(target.slug, section) : settings.root(), search, hash)} replace />;
}

const r = (path, to, query) => ({ path, element: <Redirect to={to} query={query} /> });

export const legacyRedirects = [
  // Workbench
  r('log', workbench.logs()),
  r('task', workbench.logs('tasks')),
  r('midjourney', workbench.logs('midjourney')),
  r('token', workbench.keys()),
  r('model_price', workbench.models()),
  r('routing', workbench.routingPolicies()),
  r('api', workbench.docs()),
  r('api/docs/:modality', ({ modality }) => workbench.docs(modality)),
  r('api/:modality', ({ modality }) => workbench.playground(modality)),
  r('ledger', workbench.billing()),
  r('invoice', workbench.billing('invoices')),
  r('topup', workbench.billing(), { topup: '1' }),
  { path: 'organization', element: <LegacyOrgPageRedirect /> },
  // Settings hub
  { path: 'profile', element: <ProfileRedirect /> },
  r('settings/account/:section', ({ section }) => settings.account(section)),
  { path: 'settings/org/:orgId/:section', element: <OrgSettingsByIdRedirect /> },
  // Platform admin
  r('analytics', admin.stats()),
  r('multi_user_stats', admin.statsByUser()),
  r('system_info', admin.systemLogs()),
  r('channel/:id?', ({ id }) => admin.providers(id ?? null)),
  r('allocation', admin.providers()),
  r('pricing', admin.models()),
  r('model_info', admin.models()),
  r('model_ownedby', admin.models('vendors')),
  r('routing_policy', admin.routing()),
  r('user', admin.users()),
  r('user_group', admin.userGroups()),
  r('org_admin', admin.orgs()),
  r('payment', admin.orders()),
  r('redemption', admin.redemptionCodes()),
  r('invite_code', admin.inviteCodes()),
  r('setting', admin.settings()),
  r('setting/:section', ({ section }) => admin.settings(section)),
  r('telegram', admin.settings('notify'), { highlight: 'telegram' })
];
