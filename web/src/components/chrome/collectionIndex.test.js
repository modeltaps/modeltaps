import { beforeEach, describe, expect, it, vi } from 'vitest';

import { API } from 'utils/api';
import { loadCollectionItems } from './collectionIndex';
import { commandFilter } from './CommandMenu';
import { packTiers } from './commandScore';

vi.mock('utils/api', () => ({ API: { get: vi.fn() } }));

const lists = {
  '/api/oidc_provider/': [{ id: 1, slug: 'okta', display_name: 'Okta' }],
  '/api/payment/': { data: [{ id: 7, name: 'Stripe main', type: 'stripe' }], total_count: 1 },
  '/api/mail_service/': [{ id: 2, name: '', from: 'noreply@example.com', provider: 'resend' }],
  '/api/notify_channel/': [{ id: 3, name: 'Ops Slack', type: 'slack' }],
  '/api/option/telegram/': { data: [{ id: 4, command: '/balance', description: 'Show the balance' }] }
};

beforeEach(() => {
  API.get.mockReset();
  API.get.mockImplementation((url) =>
    url === '/api/mail_service/' ? Promise.reject(new Error('down')) : Promise.resolve({ data: { success: true, data: lists[url] } })
  );
});

describe('loadCollectionItems', () => {
  it('reads every collection a root user can see and links each item to its card', async () => {
    const items = await loadCollectionItems({ isRoot: true });
    expect(items.map((x) => x.id)).toEqual(['oidc-providers:1', 'payment-gateways:7', 'notify-channels:3', 'telegram:4']);
    const stripe = items.find((x) => x.card === 'payment-gateways');
    expect(stripe.href).toBe('/panel/admin/settings/payment?highlight=payment-gateways');
    expect(stripe.collection.name(stripe.item)).toBe('Stripe main');
  });

  it('reads only the payment gateways for an admin who is not root', async () => {
    const items = await loadCollectionItems({ isRoot: false });
    expect(items.map((x) => x.id)).toEqual(['payment-gateways:7']);
    expect(API.get).toHaveBeenCalledTimes(1);
  });

  it('ranks an item found by its name above one found by its card', () => {
    const value = 'collection-item:payment-gateways:7';
    const byName = commandFilter(value, 'stripe', packTiers({ primary: ['Stripe main'], secondary: ['Payment gateways'] }));
    const byCard = commandFilter(value, 'gateways', packTiers({ primary: ['Stripe main'], secondary: ['Payment gateways'] }));
    expect(byName).toBeGreaterThan(byCard);
    expect(byCard).toBeGreaterThan(0);
  });
});
