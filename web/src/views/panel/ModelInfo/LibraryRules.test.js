import { describe, expect, it } from 'vitest';

import { formToRule, ruleToForm } from './LibraryRules';

describe('inclusion rule form', () => {
  it('round-trips a rule', () => {
    const rule = {
      name: 'Publish new',
      labs: ['anthropic', 'openai'],
      patterns: ['*-preview*'],
      released_within_days: 60,
      supplied: true,
      deprecated: false,
      endpoints: ['chat'],
      action: 'publish'
    };
    expect(formToRule(ruleToForm(rule))).toEqual(rule);
  });

  it('drops empty conditions and keeps "any" unset', () => {
    const form = { ...ruleToForm({ action: 'draft' }), labs: ' , ', days: '0' };
    expect(formToRule(form)).toEqual({ name: '', action: 'draft' });
  });
});
