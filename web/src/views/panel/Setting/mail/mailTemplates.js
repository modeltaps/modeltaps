// ==============================|| MAIL SENDING — USER EMAIL TEMPLATES ||============================== //
// MailTemplates is a JSON object keyed by template id ({ verification: { subject, body }, ... }).
// Only what the admin changed is stored: an empty subject or body means the default, so the
// defaults can improve with an upgrade and "Restore default" is just clearing the field.

export const TEMPLATE_IDS = ['verification', 'password_reset', 'quota_warning'];

export function parseTemplates(raw) {
  if (!raw || !String(raw).trim()) return {};
  try {
    const value = JSON.parse(raw);
    return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
  } catch {
    return {};
  }
}

// The stored value: templates with neither field changed are dropped, and nothing changed at all
// is ''.
export function serializeTemplates(templates) {
  const out = {};
  for (const id of Object.keys(templates).sort()) {
    const subject = (templates[id]?.subject || '').trim() ? templates[id].subject : '';
    const body = (templates[id]?.body || '').trim() ? templates[id].body : '';
    if (subject || body) out[id] = { subject, body };
  }
  return Object.keys(out).length ? JSON.stringify(out) : '';
}

// Writes one field. Typing the default text back counts as using the default.
export function setTemplateField(raw, id, field, value, defaultValue) {
  const templates = parseTemplates(raw);
  const next = { ...templates, [id]: { subject: '', body: '', ...templates[id] } };
  next[id][field] = value === defaultValue ? '' : value;
  return serializeTemplates(next);
}

export function resetTemplate(raw, id) {
  const templates = { ...parseTemplates(raw) };
  delete templates[id];
  return serializeTemplates(templates);
}

// The text shown in the editor: the admin's version, else the default.
export function templateText(raw, id, field, defaultValue) {
  const value = parseTemplates(raw)[id]?.[field];
  return value && String(value).trim() ? value : defaultValue;
}

export const isTemplateCustom = (raw, id) => Boolean(parseTemplates(raw)[id]);
