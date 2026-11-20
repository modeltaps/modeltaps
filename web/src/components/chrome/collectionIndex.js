import { API } from 'utils/api';
import { gatewayTypeLabel } from 'views/panel/Payment/paymentApi';
import { settingsGroupUrl } from 'views/panel/Setting/registry';

// ==============================|| CHROME — COMMAND PALETTE: COLLECTION ITEMS ||============================== //
// The items of the platform settings cards that hold a list saved item by item (OIDC providers,
// payment gateways, mail services, notification channels, Telegram commands), so the palette finds
// "Stripe" or a provider's name and opens the card that holds it. They are read when the palette
// opens, for platform admins (most lists for the root user only, like their endpoints); a list that
// cannot be read is left out.

const LIMIT = 100;

const listOf = (data) => (Array.isArray(data) ? data : Array.isArray(data?.data) ? data.data : []);

// group / card: where the item is set; name: what it is called; words: what else finds it.
export const COLLECTIONS = [
  {
    group: 'auth',
    card: 'oidc-providers',
    url: '/api/oidc_provider/',
    name: (item) => item.display_name || item.slug,
    words: (item) => [item.slug, item.issuer, 'oidc', 'sso'],
    rootOnly: true
  },
  {
    group: 'payment',
    card: 'payment-gateways',
    url: '/api/payment/',
    params: { page: 1, size: LIMIT },
    name: (item) => item.name,
    words: (item, t) => [item.type, gatewayTypeLabel(item.type, t), item.currency]
  },
  {
    group: 'mail',
    card: 'smtp',
    url: '/api/mail_service/',
    name: (item) => item.name || item.from || item.server,
    words: (item) => [item.provider, item.server, item.from, 'smtp'],
    rootOnly: true
  },
  {
    group: 'notify',
    card: 'notify-channels',
    url: '/api/notify_channel/',
    name: (item, t) => item.name || t(`settingsUi.platform.notifyChannels.types.${item.type}`),
    words: (item, t) => [item.type, t(`settingsUi.platform.notifyChannels.types.${item.type}`)],
    rootOnly: true
  },
  {
    group: 'entries',
    card: 'telegram',
    url: '/api/option/telegram/',
    params: { page: 1, size: LIMIT, keyword: '', order: '-id' },
    name: (item) => item.command,
    words: (item) => [item.description, 'telegram'],
    rootOnly: true
  }
];

// Reads every collection the user may see: [{ id, group, card, href, item, collection }].
export async function loadCollectionItems({ isRoot = false } = {}) {
  const lists = await Promise.all(
    COLLECTIONS.filter((c) => !c.rootOnly || isRoot).map(async (collection) => {
      try {
        const res = await API.get(collection.url, { params: collection.params });
        if (!res.data?.success) return [];
        return listOf(res.data.data)
          .slice(0, LIMIT)
          .map((item) => ({
            id: `${collection.card}:${item.id}`,
            group: collection.group,
            card: collection.card,
            href: `${settingsGroupUrl(collection.group)}?highlight=${collection.card}`,
            item,
            collection
          }));
      } catch {
        return [];
      }
    })
  );
  return lists.flat();
}
