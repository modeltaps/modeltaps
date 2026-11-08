// Detail page URL: overview by default, `config` (optionally on one section) for editing.
export function channelUrl(id, tab, section) {
  const params = new URLSearchParams();
  if (tab && tab !== 'overview') params.set('tab', tab);
  if (section && section !== 'basic') params.set('section', section);
  const q = params.toString();
  return `/panel/channel/${id}${q ? `?${q}` : ''}`;
}
