// ==============================|| MAIL SENDING — USER EMAIL TEMPLATES ||============================== //
// MailTemplates is a JSON object keyed by template id, then by language:
// { verification: { zh_CN: { subject, body }, ... }, ... }. Only what the admin changed is stored:
// an empty subject or body means the built-in text of that language, so the built-in texts can
// improve with an upgrade and "Restore default" is just clearing the field. A template stored
// before the languages ({ subject, body } directly) counts as the site default language, as on the
// server (common/stmp/mail_templates.go).

export const TEMPLATE_IDS = ['verification', 'password_reset', 'quota_low', 'quota_exhausted'];

const isText = (v) => Boolean(v) && String(v).trim() !== '';

export function parseTemplates(raw, legacyLang = 'en_US') {
  if (!raw || !String(raw).trim()) return {};
  let value;
  try {
    value = JSON.parse(raw);
  } catch {
    return {};
  }
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
  const out = {};
  for (const [id, inner] of Object.entries(value)) {
    if (!inner || typeof inner !== 'object') continue;
    out[id] = 'subject' in inner || 'body' in inner ? { [legacyLang]: inner } : inner;
  }
  return out;
}

// The stored value: languages with neither field changed are dropped, then templates with no
// language left, and nothing changed at all is ''.
export function serializeTemplates(templates) {
  const out = {};
  for (const id of Object.keys(templates).sort()) {
    const byLang = {};
    for (const lng of Object.keys(templates[id] || {}).sort()) {
      const t = templates[id][lng] || {};
      const subject = isText(t.subject) ? t.subject : '';
      const body = isText(t.body) ? t.body : '';
      if (subject || body) byLang[lng] = { subject, body };
    }
    if (Object.keys(byLang).length) out[id] = byLang;
  }
  return Object.keys(out).length ? JSON.stringify(out) : '';
}

// Writes one field of one language. Typing the built-in text back counts as using it.
export function setTemplateField(raw, id, lng, field, value, defaultValue, legacyLang) {
  const templates = parseTemplates(raw, legacyLang);
  const byLang = { ...templates[id] };
  byLang[lng] = { subject: '', body: '', ...byLang[lng], [field]: value === defaultValue ? '' : value };
  return serializeTemplates({ ...templates, [id]: byLang });
}

export function resetTemplate(raw, id, lng, legacyLang) {
  const templates = parseTemplates(raw, legacyLang);
  const byLang = { ...templates[id] };
  delete byLang[lng];
  return serializeTemplates({ ...templates, [id]: byLang });
}

// The text shown in the editor: the admin's version in that language, else the built-in one.
export function templateText(raw, id, lng, field, defaultValue, legacyLang) {
  const value = parseTemplates(raw, legacyLang)[id]?.[lng]?.[field];
  return isText(value) ? value : defaultValue;
}

// Whether the admin changed a template in a language, or (without lng) in any language.
export function isTemplateCustom(raw, id, lng, legacyLang) {
  const byLang = parseTemplates(raw, legacyLang)[id] || {};
  const changed = (t) => isText(t?.subject) || isText(t?.body);
  return lng ? changed(byLang[lng]) : Object.values(byLang).some(changed);
}
