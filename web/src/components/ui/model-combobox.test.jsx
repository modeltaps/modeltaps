// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ModelCombobox } from './model-combobox';

afterEach(cleanup);

const groups = [
  { label: 'Decision models', options: ['router-decision-1'] },
  { label: 'Chat models', options: ['claude-haiku-5-5', 'gpt-5-mini'] }
];

describe('ModelCombobox groups', () => {
  it('lists each group under its heading and leaves out a group with no match', () => {
    const onChange = vi.fn();
    const { rerender } = render(<ModelCombobox id="m" value="" onChange={onChange} groups={groups} />);
    fireEvent.focus(screen.getByRole('combobox'));
    const names = screen.getAllByRole('button').map((b) => b.textContent);
    expect(names).toEqual(['router-decision-1', 'claude-haiku-5-5', 'gpt-5-mini']);
    expect(screen.getByText('Decision models')).toBeTruthy();
    expect(screen.getByText('Chat models')).toBeTruthy();

    // Typing filters; the parent then passes the typed text back as the value.
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'haiku' } });
    rerender(<ModelCombobox id="m" value="haiku" onChange={onChange} groups={groups} />);
    expect(screen.queryByText('Decision models')).toBeNull();
    fireEvent.mouseDown(screen.getByText('claude-haiku-5-5'));
    expect(onChange).toHaveBeenCalledWith('claude-haiku-5-5');
  });

  it('opening a filled picker lists every option, the current one marked', () => {
    render(<ModelCombobox id="m" value="claude-haiku-5-5" onChange={vi.fn()} groups={groups} />);
    fireEvent.focus(screen.getByRole('combobox'));
    const names = screen.getAllByRole('button').map((b) => b.textContent);
    expect(names).toEqual(['router-decision-1', 'claude-haiku-5-5', 'gpt-5-mini']);
  });
});
