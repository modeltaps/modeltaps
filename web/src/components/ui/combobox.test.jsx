// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import { Combobox } from './combobox';

beforeAll(() => {
  // cmdk measures and scrolls its list; jsdom implements neither.
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  Element.prototype.scrollIntoView ??= () => {};
});

afterEach(cleanup);

const groups = [
  {
    key: 'common',
    options: [
      { value: '1', label: 'OpenAI' },
      { value: '3', label: 'Azure OpenAI' }
    ]
  },
  {
    key: 'others',
    options: [
      { value: '24', label: 'Azure Speech' },
      { value: '28', label: 'Deepseek' }
    ]
  }
];

const optionLabels = () => screen.queryAllByRole('option').map((o) => o.textContent);

describe('Combobox', () => {
  it('renders groups in order with a separator between them', () => {
    const { container } = render(<Combobox value="1" groups={groups} />);
    fireEvent.click(screen.getByRole('combobox'));
    expect(optionLabels()).toEqual(['OpenAI', 'Azure OpenAI', 'Azure Speech', 'Deepseek']);
    expect(container.querySelectorAll('[cmdk-group]')).toHaveLength(2);
    expect(container.querySelectorAll('[cmdk-separator]')).toHaveLength(1);
  });

  it('searches across every group', () => {
    const { container } = render(<Combobox value="1" groups={groups} />);
    fireEvent.click(screen.getByRole('combobox'));
    fireEvent.change(container.querySelector('[cmdk-input]'), { target: { value: 'azure' } });
    expect(optionLabels()).toEqual(['Azure OpenAI', 'Azure Speech']);
    fireEvent.change(container.querySelector('[cmdk-input]'), { target: { value: 'deep' } });
    expect(optionLabels()).toEqual(['Deepseek']);
  });

  it('selects from a group and shows the selected label', () => {
    const onValueChange = vi.fn();
    const { rerender } = render(<Combobox value="1" groups={groups} onValueChange={onValueChange} />);
    expect(screen.getByRole('combobox').textContent).toBe('OpenAI');
    fireEvent.click(screen.getByRole('combobox'));
    fireEvent.click(screen.getByText('Deepseek'));
    expect(onValueChange).toHaveBeenCalledWith('28');
    rerender(<Combobox value="28" groups={groups} onValueChange={onValueChange} />);
    expect(screen.getByRole('combobox').textContent).toBe('Deepseek');
  });

  it('keeps the flat options list working', () => {
    const { container } = render(
      <Combobox
        value="b"
        options={[
          { value: 'a', label: 'Alpha' },
          { value: 'b', label: 'Beta' }
        ]}
      />
    );
    expect(screen.getByRole('combobox').textContent).toBe('Beta');
    fireEvent.click(screen.getByRole('combobox'));
    expect(optionLabels()).toEqual(['Alpha', 'Beta']);
    expect(container.querySelector('[cmdk-separator]')).toBeNull();
  });
});
