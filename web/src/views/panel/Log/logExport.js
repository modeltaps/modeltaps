import { rewriteOrgUrl } from 'utils/orgScope';
import { toSearchParams } from '@/components/filter-bar';

// ==============================|| LOG — CSV EXPORT ||============================== //
// Streams the filtered CSV from the admin/self export endpoints and triggers a
// browser download. Uses raw fetch (not the axios instance), so the org-context
// path rewrite is applied here explicitly via rewriteOrgUrl.
//
// W9-D:`params` 为 Log 页 buildFilterQuery() 产出的查询对象(含 log_type / 时间范围 /
// 级联筛选的多值数组 + exclude_ 键);上下文裁剪已在调用方按 filterFields 完成,此处只序列化。
export async function exportLogs({ params: filterParams, order, orderBy, userIsAdmin }) {
  let orderByExport = orderBy;
  if (orderByExport) orderByExport = order === 'desc' ? `-${orderByExport}` : orderByExport;

  const url = rewriteOrgUrl(userIsAdmin ? '/api/log/export' : '/api/log/self/export');
  // 数组值以重复键展开(gin 切片绑定所需),标量按原样;空值跳过。
  const queryString = toSearchParams({ order: orderByExport, ...filterParams }).toString();
  const token = localStorage.getItem('token');
  const response = await fetch(`${url}?${queryString}`, {
    method: 'GET',
    headers: { Authorization: token ? `Bearer ${token}` : '' }
  });

  if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

  const contentDisposition = response.headers.get('Content-Disposition');
  let filename = `logs_export_${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.csv`;
  if (contentDisposition) {
    const match = contentDisposition.match(/filename=(.+)/);
    if (match) filename = match[1];
  }

  const blob = await response.blob();
  const link = document.createElement('a');
  link.href = window.URL.createObjectURL(blob);
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(link.href);
}
