// @vitest-environment jsdom
import { createElement, useState } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import { GroupOptions } from './channelParts';

const GROUPS = [{ symbol: 'default' }, { symbol: 'vip' }];

function Harness({ initial, onValue }) {
  const [value, setValue] = useState(initial);
  return createElement(GroupOptions, {
    groups: GROUPS,
    value,
    onChange: (v) => {
      setValue(v);
      onValue(v);
    }
  });
}

const row = (name) => screen.getByRole('checkbox', { name: new RegExp(name) });

describe('GroupOptions', () => {
  afterEach(cleanup);

  it('leads with All groups and locks the single groups while it is on', async () => {
    await i18n.changeLanguage('en');
    let last = null;
    render(createElement(Harness, { initial: ['*'], onValue: (v) => (last = v) }));
    expect(row('All groups').getAttribute('aria-checked')).toBe('true');
    expect(row('^vip').disabled).toBe(true);

    // Turning All groups off selects every current group.
    fireEvent.click(row('All groups'));
    expect(last).toEqual(['default', 'vip']);
    expect(row('^vip').disabled).toBe(false);

    fireEvent.click(row('^vip'));
    expect(last).toEqual(['default']);
    fireEvent.click(row('All groups'));
    expect(last).toEqual(['*']);
  });
});
