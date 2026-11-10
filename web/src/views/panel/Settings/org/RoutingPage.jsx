import { useMemo } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { RoutingPolicyList } from 'views/panel/RoutingPolicy';
import { orgRoutingPolicyApi } from 'views/panel/RoutingPolicy/routingPolicyApi';

// ==============================|| ORG — AUTO ROUTING ||============================== //
// The organization's own routing policies (组织 → 自动路由, owners and admins, while the site
// lets organizations manage them). The platform console's list and panel, scoped to
// /api/org/:id/routing_policy/: the model pickers list the organization's models, Try it is
// billed to the organization and there is no log sample replay.

export default function RoutingPage({ orgId }) {
  const { t } = useTranslation();
  const api = useMemo(() => orgRoutingPolicyApi(orgId), [orgId]);
  return <RoutingPolicyList key={orgId} api={api} intro={t('routingPolicy.orgIntro')} />;
}

RoutingPage.propTypes = {
  orgId: PropTypes.number.isRequired
};
