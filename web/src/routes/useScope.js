import { useCallback } from 'react';
import { useLocation, useNavigate } from 'react-router';

import { orgSlugOf, rescope } from './paths';

// The organization slug of the current URL (org-scoped workbench pages), or null in the personal
// workbench, settings and platform admin.
export function useOrgSlug() {
  const { pathname } = useLocation();
  return orgSlugOf(pathname);
}

// `inScope(path)`: a personal workbench path moved into the scope of the current URL, so in-page
// links stay in the organization the user is in. Build the path with `workbench.*` (personal form)
// and pass it through: `inScope(workbench.logs('tasks'))`.
export function useInScope() {
  const orgSlug = useOrgSlug();
  return useCallback((path) => (orgSlug ? rescope(path, orgSlug) : path), [orgSlug]);
}

// `navigate` for workbench paths (personal form), kept in the current scope.
export function useScopedNavigate() {
  const navigate = useNavigate();
  const inScope = useInScope();
  return useCallback((path, options) => navigate(inScope(path), options), [navigate, inScope]);
}
