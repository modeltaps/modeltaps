// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import BatchBar from './BatchBar';

afterEach(cleanup);

describe('BatchBar select all matching', () => {
  it('offers to select every matching row when only part of them is picked', async () => {
    await i18n.changeLanguage('en_US');
    const onSelectAll = vi.fn();
    render(<BatchBar count={20} total={877} onSelectAll={onSelectAll} onClear={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'Select all 877 matching' }));
    expect(onSelectAll).toHaveBeenCalledTimes(1);
  });

  it('hides the link once every matching row is picked or without a handler', async () => {
    await i18n.changeLanguage('en_US');
    const { rerender } = render(<BatchBar count={877} total={877} onSelectAll={() => {}} onClear={() => {}} />);
    expect(screen.queryByText(/matching/)).toBeNull();
    rerender(<BatchBar count={20} total={877} onClear={() => {}} />);
    expect(screen.queryByText(/matching/)).toBeNull();
  });

  it('shows the link in the floating phone bar too', async () => {
    await i18n.changeLanguage('en_US');
    render(<BatchBar count={3} total={40} floating onSelectAll={() => {}} onClear={() => {}} />);
    expect(screen.getByRole('button', { name: 'Select all 40 matching' })).toBeTruthy();
  });
});
