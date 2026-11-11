import { useMemo } from 'react';
import { Navigate, useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { useOrg } from 'contexts/OrgContext';

import { Segmented } from '@/components/objects';
import { DASHBOARD_URL } from '@/components/chrome/nav-config';
import { RoutingPolicyList } from 'views/panel/RoutingPolicy';
import { orgRoutingPolicyApi, personalRoutingPolicyApi } from 'views/panel/RoutingPolicy/routingPolicyApi';
import ManagedPolicies from './ManagedPolicies';

// ==============================|| WORKBENCH — ROUTING POLICIES ||============================== //
// 路由策略: the policies the current context's API keys can call, one tab per policy type.
//   托管 (managed)   the platform's policies, called by their bare name; read-only.
//   自动路由 (auto)   the context's own Auto policies, called as policy/<name>: the user's personal
//                    ones, or the organization's (Owner / Admin manage them, members only see them).
// The tab is kept in the URL (?type=managed); the default is auto.

const TYPES = ['managed', 'auto'];

export default function Routing() {
  const { t } = useTranslation();
  const { routingPolicyEnabled, orgsLoaded, currentOrgId, orgRole } = useOrg();
  const [searchParams, setSearchParams] = useSearchParams();
  const type = searchParams.get('type') === 'managed' ? 'managed' : 'auto';
  const isOrgAdmin = orgRole === 'owner' || orgRole === 'admin';
  const api = useMemo(
    () => (currentOrgId ? orgRoutingPolicyApi(currentOrgId, { readOnly: !isOrgAdmin }) : personalRoutingPolicyApi),
    [currentOrgId, isOrgAdmin]
  );

  // Site info and the organization list decide the switch and the context; wait for both.
  if (!orgsLoaded) return null;
  if (!routingPolicyEnabled) return <Navigate to={DASHBOARD_URL} replace />;

  const filter = (
    <Segmented
      label={t('routingPolicy.types.label')}
      value={type}
      onChange={(next) => setSearchParams(next === 'auto' ? {} : { type: next })}
      options={TYPES.map((value) => ({ value, label: t(`routingPolicy.types.${value}`) }))}
    />
  );

  if (type === 'managed') {
    return <ManagedPolicies key={currentOrgId || 'personal'} api={api} filter={filter} />;
  }
  let intro = t('routingPolicy.personalOwnIntro');
  if (currentOrgId) intro = isOrgAdmin ? t('routingPolicy.orgIntro') : t('routingPolicy.orgMemberIntro');
  return <RoutingPolicyList key={`${currentOrgId || 'personal'}-${isOrgAdmin}`} api={api} filter={filter} intro={intro} />;
}
