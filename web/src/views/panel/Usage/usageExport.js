import { calculateQuota } from 'utils/common';

// ==============================|| USAGE — CSV EXPORT ||============================== //
// Client-side CSV generation for the current dimension's rows (no extra request),
// mirroring the download mechanics of Log/logExport.js. Quota is exported in USD
// (calculateQuota, 6 digits) so the file matches the on-screen spend values.

function csvCell(value) {
  const s = value === null || value === undefined ? '' : String(value);
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

export function exportUsageCsv({
  dimension,
  rows,
  nameHeader,
  headers,
  nameOf,
  startDate,
  endDate,
  rangeLabel,
  selectedTokens,
  selectedModels
}) {
  const lines = [];

  // Add filter comment as first line if any filters are active
  const filterParts = [];
  if (selectedTokens && selectedTokens.length > 0) {
    filterParts.push(`tokens=${selectedTokens.join(',')}`);
  }
  if (selectedModels && selectedModels.length > 0) {
    filterParts.push(`models=${selectedModels.join(',')}`);
  }
  if (filterParts.length > 0) {
    lines.push(`# Filters: ${filterParts.join(', ')}, range=${startDate} to ${endDate}`);
  }

  lines.push([nameHeader, ...headers].map(csvCell).join(','));
  for (const r of rows) {
    lines.push(
      [
        nameOf(r),
        Number(r.request_count || 0),
        calculateQuota(r.quota || 0, 6),
        Number(r.prompt_tokens || 0),
        Number(r.completion_tokens || 0)
      ]
        .map(csvCell)
        .join(',')
    );
  }

  // \ufeff BOM so Excel opens the UTF-8 file with CJK headers intact
  const blob = new Blob(['\ufeff' + lines.join('\n')], { type: 'text/csv;charset=utf-8' });
  const link = document.createElement('a');
  link.href = window.URL.createObjectURL(blob);

  // Build filename with time range label and filter suffixes
  const parts = ['usage'];
  if (rangeLabel && rangeLabel !== 'custom') {
    parts.push(rangeLabel);
  }
  parts.push(dimension, startDate, endDate);
  if (selectedTokens && selectedTokens.length > 0) {
    parts.push(...selectedTokens.slice(0, 3).map((tk) => tk.replace(/[^a-zA-Z0-9_-]/g, '_')));
    if (selectedTokens.length > 3) parts.push('more');
  }
  if (selectedModels && selectedModels.length > 0) {
    parts.push(...selectedModels.slice(0, 3).map((m) => m.replace(/[^a-zA-Z0-9_-]/g, '_')));
    if (selectedModels.length > 3) parts.push('more');
  }
  link.download = `${parts.join('_')}.csv`;

  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(link.href);
}
