import { describe, expect, it } from 'vitest';

import { EMPTY_POLICY, modelPickerOptions, normalizePolicy } from './routingPolicyApi';
import { POLICY_TEMPLATES, applyTemplate, pickDecisionModel, pickTierModel, templateName } from './templates';

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
    expect(out.scoring).toBe('auto');
  });

  it('keeps the chosen scoring and defaults a missing one to auto', () => {
    expect(normalizePolicy({ ...EMPTY_POLICY, scoring: 'logprobs' }).scoring).toBe('logprobs');
    expect(normalizePolicy({ ...EMPTY_POLICY, scoring: undefined }).scoring).toBe('auto');
  });

  it('names a described route without a short name after its position', () => {
    const out = normalizePolicy({
      ...EMPTY_POLICY,
      routes: [
        { label: '', description: 'small edits', target_model: 'm1' },
        { label: 'route_2', description: 'x', target_model: 'm2' },
        { label: '', description: 'big changes', target_model: 'm3' },
        { label: '', description: '', target_model: 'm4' }
      ]
    });
    expect(out.routes.map((r) => r.label)).toEqual(['route_1', 'route_2', 'route_3', '']);
  });
});

describe('modelPickerOptions', () => {
  it('lists recommended decision models first', () => {
    const { classifiers, targets } = modelPickerOptions({
      models: ['aaa-decider', 'd1', 'gpt', 'jev-1.14.0'],
      kinds: { 'aaa-decider': 'decision', d1: 'decision', 'jev-1.14.0': 'decision' }
    });
    expect(classifiers.decision).toEqual(['jev-1.14.0', 'd1', 'aaa-decider']);
    expect(targets).toEqual(['gpt']);
  });

  it('suggests only published models when the list says which are published', () => {
    const { suggest, targets } = modelPickerOptions({
      models: ['a', 'b', 'jev'],
      kinds: { jev: 'decision' },
      published: ['b', 'jev']
    });
    expect(targets).toEqual(['a', 'b']);
    expect(suggest.models).toEqual(['b']);
    expect(suggest.classifiers.decision).toEqual(['jev']);
  });
});

describe('templates', () => {
  const models = [
    'claude-haiku-5-5',
    'claude-sonnet-5-5',
    'claude-opus-5-5',
    'deepseek-v4.1-flash',
    'deepseek-v4.1-flash-1017',
    'gpt-5-mini'
  ];

  it('picks the model of each tier, falling back to the nearest tier', () => {
    expect(pickTierModel(models, 'fast')).toBe('claude-haiku-5-5');
    expect(pickTierModel(models, 'balanced')).toBe('claude-sonnet-5-5');
    expect(pickTierModel(models, 'strongest')).toBe('claude-opus-5-5');
    expect(pickTierModel(['deepseek-v4.1-flash-1017', 'deepseek-v4.1-flash', 'deepseek-v4-flash'], 'fast')).toBe('deepseek-v4.1-flash');
    expect(pickTierModel(['claude-haiku-5.5-batch', 'claude-haiku-4.5', 'claude-haiku-5-5'], 'fast')).toBe('claude-haiku-5-5');
    expect(pickTierModel(['claude-sonnet-5-5'], 'strongest')).toBe('claude-sonnet-5-5');
    expect(pickTierModel(['some-model'], 'fast')).toBe('');
  });

  it('picks a decision model by the template kind, then any decision model, then a fast chat model', () => {
    const coding = POLICY_TEMPLATES.find((t) => t.decision === 'coding');
    const general = POLICY_TEMPLATES.find((t) => t.decision === 'general');
    const decision = ['d1', 'gemini-decision-1', 'jev-1.14.0'];
    expect(pickDecisionModel(coding, { decision })).toBe('jev-1.14.0');
    expect(pickDecisionModel(general, { decision })).toBe('gemini-decision-1');
    expect(pickDecisionModel(general, { decision: ['other-decider'] })).toBe('other-decider');
    expect(pickDecisionModel(general, { decision: [], chat: models })).toBe('claude-haiku-5-5');
  });

  it('replaces question and routes, keeps targets already chosen and fills the rest', () => {
    const template = POLICY_TEMPLATES[0];
    const form = {
      ...EMPTY_POLICY,
      question: 'old',
      decision_model: 'mine',
      routes: [{ label: 'x', description: '', target_model: 'keep-me' }]
    };
    const out = applyTemplate(form, template, { models, classifiers: { decision: [], chat: models } });
    expect(out.question).toBe(template.question);
    expect(out.routes).toHaveLength(template.routes.length);
    expect(out.routes[0].target_model).toBe('keep-me');
    expect(out.routes[1].target_model).toBe(pickTierModel(models, template.routes[1].tier));
    expect(out.routes[0].label).toBe(template.routes[0].label);
    expect(out.decision_model).toBe('mine');
    expect(out.fallback_model).toBe('claude-opus-5-5');
  });

  it('suggests auto/<name> for platform policies and the bare name for policy/<name>', () => {
    expect(templateName(POLICY_TEMPLATES[0], '')).toBe('auto/coder');
    expect(templateName(POLICY_TEMPLATES[0], 'policy/')).toBe('coder');
  });

  it('every template has 2-16 distinct labels, a tier per route, a name and a decision kind', () => {
    const names = new Set();
    for (const template of POLICY_TEMPLATES) {
      const labels = template.routes.map((r) => r.label);
      expect(labels.length).toBeGreaterThanOrEqual(2);
      expect(labels.length).toBeLessThanOrEqual(16);
      expect(new Set(labels).size).toBe(labels.length);
      for (const route of template.routes) expect(['fast', 'balanced', 'strongest']).toContain(route.tier);
      expect(['coding', 'general']).toContain(template.decision);
      expect(names.has(template.name)).toBe(false);
      names.add(template.name);
    }
  });
});
