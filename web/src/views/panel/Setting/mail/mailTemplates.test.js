import { describe, expect, it } from 'vitest';

import { isTemplateCustom, parseTemplates, resetTemplate, serializeTemplates, setTemplateField, templateText } from './mailTemplates';

describe('mail templates value', () => {
  it('stores only what the admin changed', () => {
    let raw = setTemplateField('', 'verification', 'subject', '【站】验证码', 'Default subject');
    expect(JSON.parse(raw)).toEqual({ verification: { subject: '【站】验证码', body: '' } });
    expect(templateText(raw, 'verification', 'body', 'Default body')).toBe('Default body');
    expect(templateText(raw, 'verification', 'subject', 'Default subject')).toBe('【站】验证码');

    // Typing the default back, or restoring, drops the template.
    raw = setTemplateField(raw, 'verification', 'subject', 'Default subject', 'Default subject');
    expect(raw).toBe('');
    raw = setTemplateField('', 'quota_warning', 'body', '<p>x</p>', 'd');
    expect(isTemplateCustom(raw, 'quota_warning')).toBe(true);
    expect(resetTemplate(raw, 'quota_warning')).toBe('');
  });

  it('treats a broken value as no templates', () => {
    expect(parseTemplates('{bad')).toEqual({});
    expect(parseTemplates('[]')).toEqual({});
    expect(serializeTemplates({ a: { subject: ' ', body: '' } })).toBe('');
  });
});
