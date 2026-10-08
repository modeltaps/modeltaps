import { useCallback, useEffect, useState } from 'react';
import PropTypes from 'prop-types';

import { API } from 'utils/api';

// ==============================|| SETTINGS — ORG SECTION HOST ||============================== //
// 组织分组内容区的取数外壳:按 URL 中的 orgId 拉一次组织详情,交给当前 section 组件。
// orgId 变化时先清空 detail,避免上一个组织的资料在新组织表单里闪现。

export default function OrgSectionHost({ orgId, role, component: Component }) {
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

  return <Component key={orgId} orgId={orgId} role={role} detail={detail} onRefresh={fetchDetail} />;
}

OrgSectionHost.propTypes = {
  orgId: PropTypes.number.isRequired,
  role: PropTypes.string,
  component: PropTypes.elementType.isRequired
};
