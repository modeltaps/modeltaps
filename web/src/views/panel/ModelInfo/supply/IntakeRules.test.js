import { describe, expect, it } from 'vitest';

import { describeIntakeRule, intakeFormToRule, intakeRuleToForm, parseIntakeRules } from './IntakeRules';

const t = (key, vars) => (vars ? `${key}:${JSON.stringify(vars)}` : key);

describe('intake rules form', () => {
  it('round-trips a rule and drops empty conditions', () => {
    const rule = { name: 'claude', families: ['claude-*'], endpoints: ['chat'], supplied: true, action: 'enable', placement: 'last' };
    expect(intakeFormToRule(intakeRuleToForm(rule))).toEqual(rule);
    expect(intakeFormToRule(intakeRuleToForm({ name: 'all', action: 'pending' }))).toEqual({ name: 'all', action: 'pending' });
  });
  it('keeps the automatic placement implicit and none for skipped models', () => {
    expect(intakeFormToRule({ ...intakeRuleToForm({ action: 'enable' }), placement: 'auto' }).placement).toBeUndefined();
    expect(intakeFormToRule({ ...intakeRuleToForm({ action: 'skip' }), placement: 'last' }).placement).toBeUndefined();
  });
  it('splits comma lists', () => {
    const form = { ...intakeRuleToForm({ action: 'enable' }), patterns: '*-preview*, *-exp*,' };
    expect(intakeFormToRule(form).patterns).toEqual(['*-preview*', '*-exp*']);
  });
  it('parses stored rules', () => {
    expect(parseIntakeRules('')).toBeNull();
    expect(parseIntakeRules('{')).toBeNull();
    expect(parseIntakeRules('[]')).toEqual([]);
  });
  it('describes a rule in words', () => {
    const { when, then } = describeIntakeRule(t, { endpoints: ['images'], supplied: true, action: 'enable' });
    expect(when).toContain('intakeRules.when.endpoints');
    expect(when).toContain('intakeRules.when.supplied');
    expect(then).toBe('intakeRules.actions.enable · intakeRules.placements.auto');
    expect(describeIntakeRule(t, { action: 'skip' })).toEqual({ when: 'intakeRules.when.any', then: 'intakeRules.actions.skip' });
  });
});
