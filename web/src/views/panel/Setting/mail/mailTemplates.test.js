import { describe, expect, it } from 'vitest';

import { isTemplateCustom, parseTemplates, resetTemplate, serializeTemplates, setTemplateField, templateText } from './mailTemplates';

describe('mail templates value', () => {
  it('stores only what the admin changed, per language', () => {
    let raw = setTemplateField('', 'verification', 'zh_CN', 'subject', '【站】验证码', 'Default subject');
    expect(JSON.parse(raw)).toEqual({ verification: { zh_CN: { subject: '【站】验证码', body: '' } } });
    expect(templateText(raw, 'verification', 'zh_CN', 'body', 'Default body')).toBe('Default body');
    expect(templateText(raw, 'verification', 'zh_CN', 'subject', 'Default subject')).toBe('【站】验证码');
    // Other languages keep their built-in text.
    expect(templateText(raw, 'verification', 'ja_JP', 'subject', '既定')).toBe('既定');
    expect(isTemplateCustom(raw, 'verification', 'zh_CN')).toBe(true);
    expect(isTemplateCustom(raw, 'verification', 'en_US')).toBe(false);
    expect(isTemplateCustom(raw, 'verification')).toBe(true);

    // Typing the default back, or restoring, drops the language and then the template.
    raw = setTemplateField(raw, 'verification', 'zh_CN', 'subject', 'Default subject', 'Default subject');
    expect(raw).toBe('');
    raw = setTemplateField('', 'quota_low', 'en_US', 'body', '<p>x</p>', 'd');
    raw = setTemplateField(raw, 'quota_low', 'ja_JP', 'body', '<p>y</p>', 'd');
    raw = resetTemplate(raw, 'quota_low', 'en_US');
    expect(JSON.parse(raw)).toEqual({ quota_low: { ja_JP: { subject: '', body: '<p>y</p>' } } });
    expect(resetTemplate(raw, 'quota_low', 'ja_JP')).toBe('');
  });

  it('reads a template stored before the languages as the site default language', () => {
    const raw = JSON.stringify({ verification: { subject: 'Old', body: '' } });
    expect(parseTemplates(raw, 'zh_CN')).toEqual({ verification: { zh_CN: { subject: 'Old', body: '' } } });
    expect(templateText(raw, 'verification', 'zh_CN', 'subject', 'd', 'zh_CN')).toBe('Old');
  });

  it('treats a broken value as no templates', () => {
    expect(parseTemplates('{bad')).toEqual({});
    expect(parseTemplates('[]')).toEqual({});
    expect(serializeTemplates({ a: { en_US: { subject: ' ', body: '' } } })).toBe('');
  });
});
