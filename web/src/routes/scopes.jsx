import { lazy, useEffect } from 'react';
import { Navigate, Outlet, useLocation, useParams } from 'react-router';

import { useOrg } from 'contexts/OrgContext';
import Loadable from 'ui-component/Loadable';
import { readStoredOrgId } from 'utils/orgScope';
import { org as orgPaths, invitations, workbench } from './paths';

const NotFoundView = Loadable(lazy(() => import('views/public/Error')));

// ==============================|| ROUTES — WORKBENCH SCOPES ||============================== //
// The URL decides whose workbench a page shows. These layout routes bring the active organization
// (OrgContext, and with it the API path rewriting) in line with the URL *before* rendering the
// page: React runs a child's effects before its parent's, so a page mounted early would fetch with
// the previous scope. Changing scope therefore remounts the page, which starts from fresh state.

// `/panel/<page>`: the personal workbench.
export function PersonalScope() {
  const { currentOrgId, switchOrg } = useOrg();
  useEffect(() => {
    if (currentOrgId !== null) switchOrg(null);
  }, [currentOrgId, switchOrg]);
  if (currentOrgId !== null) return null;
  return <Outlet />;
}

// `/panel/orgs/:orgSlug/<page>`: an organization's workbench. Unknown slugs, organizations the
// user does not belong to and sites with organizations off render the 404 page.
export function OrgScope() {
  const { orgSlug } = useParams();
  const { orgEnabled, orgsLoaded, organizations, currentOrgId, switchOrg } = useOrg();
  const target = organizations.find((o) => o.slug === orgSlug) || null;

  useEffect(() => {
    if (target && currentOrgId !== target.id) switchOrg(target.id);
  }, [target, currentOrgId, switchOrg]);

  if (!orgsLoaded) return null;
  if (!orgEnabled || !target) return <NotFoundView />;
  if (currentOrgId !== target.id) return null;
  return <Outlet />;
}

// Old org-less links to an organization-only page (`/panel/organization`): the last organization
// the user worked in when they still belong to it, otherwise their invitations. Invitation links
// (`?invite=`) always land on the invitations page.
export function LegacyOrgPageRedirect() {
  const { search, hash } = useLocation();
  const { orgEnabled, orgsLoaded, organizations } = useOrg();
  if (!orgsLoaded) return null;
  const params = new URLSearchParams(search);
  const last = organizations.find((o) => o.id === readStoredOrgId());
  if (!orgEnabled || params.has('invite') || !last) return <Navigate to={`${invitations()}${search}${hash}`} replace />;
  return <Navigate to={`${orgPaths.overview(last.slug)}${search}${hash}`} replace />;
}

// `/panel/orgs/:orgSlug` itself opens the organization's dashboard.
export function OrgIndexRedirect() {
  const { orgSlug } = useParams();
  return <Navigate to={workbench.dashboard(orgSlug)} replace />;
}
