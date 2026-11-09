// Channel page URL. Without a view it is the detail page; `settings` opens the full form,
// optionally on one of its sections.
export function channelUrl(id, view, section) {
  const params = new URLSearchParams();
  if (view === 'settings') params.set('view', 'settings');
  if (view === 'settings' && section && section !== 'basic') params.set('section', section);
  const q = params.toString();
  return `/panel/channel/${id}${q ? `?${q}` : ''}`;
}
