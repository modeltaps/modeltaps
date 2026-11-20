// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';

import PriceSourceNotice from './PriceSourceNotice';

vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key) => key }) }));

afterEach(cleanup);

describe('PriceSourceNotice', () => {
  it('shows nothing while reads succeed', () => {
    const { container } = render(<PriceSourceNotice state={{ failing_since: 0 }} onForce={() => {}} />);
    expect(container.firstChild).toBeNull();
  });

  it('shows a failed read with its error and no button', () => {
    render(
      <PriceSourceNotice
        state={{ failing_since: 1763600000, last_success_at: 1763500000, last_reason: 'read', last_error: 'status 502' }}
        onForce={() => {}}
      />
    );
    expect(screen.getByText(/priceSource.failed/)).toBeTruthy();
    expect(screen.getByText('status 502')).toBeTruthy();
    expect(screen.queryByRole('button')).toBeNull();
  });

  it('offers to take a held list', () => {
    const onForce = vi.fn();
    render(<PriceSourceNotice state={{ failing_since: 1763600000, last_reason: 'shrank' }} onForce={onForce} />);
    expect(screen.getByText(/priceSource.none/)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'channels.models.priceSource.useNew' }));
    expect(onForce).toHaveBeenCalled();
  });
});
