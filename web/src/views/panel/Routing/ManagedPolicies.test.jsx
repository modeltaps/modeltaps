// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import ManagedPolicies from './ManagedPolicies';

afterEach(cleanup);

describe('ManagedPolicies', () => {
  it('shows the page empty state when the platform has no managed policy', async () => {
    const api = { listManaged: vi.fn().mockResolvedValue({ success: true, data: [] }) };
    render(<ManagedPolicies api={api} />);
    expect(await screen.findByRole('heading', { name: i18n.t('pageEmpty.managedRouting.title') })).toBeTruthy();
    expect(screen.getByText(i18n.t('pageEmpty.managedRouting.description'))).toBeTruthy();
  });
});
