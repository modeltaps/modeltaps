import { admin } from 'routes/paths';

// One fact of an alert, as the page shows it: `tone` warn for what breaks requests or loses
// money, info for the rest. Items are written by the server (model/alert.go and its callers):
// "renamed:a → b", "gone:a", "stopped:m", "unsupplied:m", "held" for model drift;
// "raised|m|in|out|in|out", "held|…", "locked|…", "scheduled|…" and "review|m|day|source" for
// price changes; "cost|m|higher|lower" and "daily|higher|lower" for upstream charges that differ
// from the price list; "refresh failed" for
// credentials; a model name for model consistency.
export function describeAlertItem(t, kind, item) {
  if (kind === 'model_drift') {
    if (item === 'held') return { tone: 'warn', text: t('alerts.item.drift.held') };
    const at = item.indexOf(':');
    const tag = item.slice(0, at);
    const rest = item.slice(at + 1);
    if (['renamed', 'gone', 'stopped', 'unsupplied'].includes(tag)) {
      return { tone: 'warn', text: t(`alerts.item.drift.${tag}`, { model: rest }) };
    }
  }
  if (kind === 'price_change') {
    const parts = item.split('|');
    if (parts[0] === 'review' && parts.length === 4) {
      const [, model, day, source] = parts;
      return { tone: 'warn', text: t('alerts.item.price.review', { model, day }), href: source };
    }
    if (parts.length === 6) {
      const [tag, model, oldIn, oldOut, newIn, newOut] = parts;
      return {
        tone: tag === 'locked' ? 'warn' : 'info',
        text: t(`alerts.item.price.${tag}`, { model, oldIn, oldOut, newIn, newOut })
      };
    }
  }
  if (kind === 'cost_mismatch') {
    const parts = item.split('|');
    if (parts[0] === 'cost' && parts.length === 3) {
      return { tone: parts[2] === 'higher' ? 'warn' : 'info', text: t(`alerts.item.cost.${parts[2]}`, { model: parts[1] }) };
    }
    if (parts[0] === 'daily' && parts.length === 2) {
      return { tone: parts[1] === 'higher' ? 'warn' : 'info', text: t(`alerts.item.cost.daily_${parts[1]}`) };
    }
  }
  if (kind === 'credential') return { tone: 'warn', text: t('alerts.item.credential') };
  if (kind === 'model_consistency') return { tone: 'info', text: t('alerts.item.consistency', { model: item }) };
  return { tone: 'info', text: item };
}

// Where an alert is handled: the channel's models, connection or performance tab, or the model
// catalog.
export function alertTarget(alert) {
  if (alert.channel_id) {
    const tab = alert.kind === 'credential' ? 'connection' : alert.kind === 'cost_mismatch' ? 'performance' : 'models';
    return `${admin.providers(alert.channel_id)}?tab=${tab}`;
  }
  if (alert.kind === 'price_change' || alert.kind === 'model_consistency') return admin.models();
  return null;
}

// Price sync sources the server names (model.ApplyAutoPriceSync callers), by message key.
const SOURCES = {
  'channel price lists': 'channel_price_lists',
  'models.dev': 'models_dev',
  'official schedule': 'official_schedule'
};

// What the alert is about: the channel's name, the price source, or the catalog.
export function alertSubject(t, alert) {
  if (alert.subject.startsWith('channel:'))
    return alert.channel_name || t('alerts.subject.goneChannel', { id: alert.channel_id || alert.subject.slice(8) });
  if (alert.subject.startsWith('source:')) {
    const source = alert.subject.slice(7);
    const known = SOURCES[source];
    return t('alerts.subject.source', { source: known ? t(`alerts.sources.${known}`) : source });
  }
  if (alert.subject === 'catalog') return t('alerts.subject.catalog');
  return alert.subject;
}
