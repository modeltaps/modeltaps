import { useCallback, useEffect, useState } from 'react';
import PropTypes from 'prop-types';

import { API } from 'utils/api';

// ==============================|| SETTINGS — ORG DETAIL ||============================== //
// Loads the organization detail of the URL's orgId once for the page. When orgId changes the
// detail is cleared first, so the previous organization never flashes in the new one's forms.

export function useOrgDetail(orgId) {
  const [detail, setDetail] = useState(null);

  const fetchDetail = useCallback(async () => {
    if (!orgId) return;
    try {
      const res = await API.get(`/api/org/${orgId}/`);
      const { success, data } = res.data;
      if (success) setDetail(data);
    } catch (error) {
      console.error(error);
    }
  }, [orgId]);

  useEffect(() => {
    setDetail(null);
    fetchDetail();
  }, [fetchDetail]);

  return { detail, refresh: fetchDetail };
}

// Renders one org page component with the detail (used by the members page).
export default function OrgSectionHost({ orgId, role, component: Component }) {
  const { detail, refresh } = useOrgDetail(orgId);
  return <Component key={orgId} orgId={orgId} role={role} detail={detail} onRefresh={refresh} />;
}

OrgSectionHost.propTypes = {
  orgId: PropTypes.number.isRequired,
  role: PropTypes.string,
  component: PropTypes.elementType.isRequired
};
