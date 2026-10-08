// ==============================|| ORG USAGE — CSV EXPORT ||============================== //
// Streams the aggregated org-usage CSV from /api/org/:id/analytics/export and triggers a
// browser download. Mirrors Log/logExport.js download mechanics (raw fetch + Bearer token)
// so the export shares the exact auth of the page's /analytics data interface. The time
// window passed here matches the on-screen dimension tables (口径一致).

export async function exportOrgUsageCsv({ orgId, startTimestamp, endTimestamp, memberId }) {
  const params = {};
  if (startTimestamp) params.start_timestamp = startTimestamp;
  if (endTimestamp) params.end_timestamp = endTimestamp;
  if (memberId) params.member_id = memberId;

  const queryString = new URLSearchParams(params).toString();
  const url = `/api/org/${orgId}/analytics/export${queryString ? `?${queryString}` : ''}`;
  const token = localStorage.getItem('token');
  const response = await fetch(url, {
    method: 'GET',
    headers: { Authorization: token ? `Bearer ${token}` : '' }
  });

  if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

  const contentDisposition = response.headers.get('Content-Disposition');
  let filename = `org_usage_export_${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.csv`;
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
