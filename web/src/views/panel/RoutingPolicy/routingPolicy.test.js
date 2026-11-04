import { describe, expect, it } from 'vitest';

import { EMPTY_POLICY, normalizePolicy } from './routingPolicyApi';
import { POLICY_TEMPLATES, applyTemplate } from './templates';

describe('normalizePolicy', () => {
  it('trims strings, converts numbers and drops blank route rows', () => {
    const out = normalizePolicy({
      ...EMPTY_POLICY,
      name: ' auto/coder ',
      decision_model: ' small ',
      fallback_model: 'big',
      threshold: '0.7',
      pin_ttl_seconds: '0',
      max_input_chars: '',
      routes: [
        { label: ' a ', description: '', target_model: ' m1 ' },
        { label: '', description: '', target_model: '' },
        { label: 'b', description: 'desc', target_model: 'm2' }
      ]
    });
    expect(out.name).toBe('auto/coder');
    expect(out.decision_model).toBe('small');
    expect(out.threshold).toBe(0.7);
    expect(out.pin_ttl_seconds).toBe(0);
    expect(out.max_input_chars).toBe(0);
    expect(out.routes).toEqual([
      { label: 'a', description: '', target_model: 'm1' },
      { label: 'b', description: 'desc', target_model: 'm2' }
    ]);
  });
});

describe('applyTemplate', () => {
  it('replaces question and routes but keeps targets already chosen by position', () => {
    const form = { ...EMPTY_POLICY, question: 'old', routes: [{ label: 'x', description: '', target_model: 'keep-me' }] };
    const out = applyTemplate(form, POLICY_TEMPLATES[0]);
    expect(out.question).toBe(POLICY_TEMPLATES[0].question);
    expect(out.routes).toHaveLength(POLICY_TEMPLATES[0].routes.length);
    expect(out.routes[0].target_model).toBe('keep-me');
    expect(out.routes[1].target_model).toBe('');
    expect(out.routes[0].label).toBe(POLICY_TEMPLATES[0].routes[0].label);
  });

  it('every template has 2-16 distinct labels', () => {
    for (const template of POLICY_TEMPLATES) {
      const labels = template.routes.map((r) => r.label);
      expect(labels.length).toBeGreaterThanOrEqual(2);
      expect(labels.length).toBeLessThanOrEqual(16);
      expect(new Set(labels).size).toBe(labels.length);
    }
  });
});
