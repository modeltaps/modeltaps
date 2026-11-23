// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import { IntakeRuleBoard } from './IntakeRules';

afterEach(cleanup);

const RULES = [
  { name: 'Previews wait for review', patterns: ['*-preview*'], action: 'pending' },
  { name: 'Chat models join at once', endpoints: ['chat'], action: 'enable' }
];

describe('IntakeRuleBoard', () => {
  it('lists the rules and the fallback with their counts and marks the deciding one', () => {
    const stats = {
      total: 10,
      rules: [
        { index: 0, count: 2, examples: ['a-preview'] },
        { index: 1, count: 7, examples: [] },
        { index: -1, count: 1, examples: [] }
      ]
    };
    render(createElement(IntakeRuleBoard, { rules: RULES, stats, matched: 1 }));
    const rows = screen.getAllByTestId('intake-board-row');
    expect(rows).toHaveLength(3);
    expect(rows[0].textContent).toContain(i18n.t('intakeRules.board.count', { count: 2 }));
    expect(rows[2].textContent).toContain(i18n.t('intakeRules.board.fallback'));
    expect(rows[1].getAttribute('aria-current')).toBe('true');
    expect(rows[0].getAttribute('aria-current')).toBeNull();
  });
});
