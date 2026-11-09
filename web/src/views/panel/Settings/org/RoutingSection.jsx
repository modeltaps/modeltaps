import { useMemo } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { RoutingPolicyManager } from 'views/panel/RoutingPolicy';
import { orgRoutingPolicyApi } from 'views/panel/RoutingPolicy/routingPolicyApi';

// ==============================|| SETTINGS — ORG AUTO ROUTING ||============================== //
// The organization's own routing policies (Owner / Admin; the settings page only lists organizations
// the user manages). Same editor as the platform console, scoped to /api/org/:id/routing_policy/.

export default function RoutingSection({ orgId }) {
  const { t } = useTranslation();
  const api = useMemo(() => orgRoutingPolicyApi(orgId), [orgId]);
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">{t('routingPolicy.orgIntro')}</p>
      <RoutingPolicyManager api={api} inlineToolbar />
    </div>
  );
}

RoutingSection.propTypes = {
  orgId: PropTypes.number.isRequired
};
