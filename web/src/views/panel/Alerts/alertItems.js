import { admin } from 'routes/paths';

// One fact of an alert, as the page shows it: `tone` warn for what breaks requests or loses
// money, info for the rest. Items are written by the server (model/alert.go and its callers):
// "renamed:a → b", "gone:a", "stopped:m", "unsupplied:m", "held" for model drift;
// "raised|m|in|out|in|out", "held|…", "locked|…" for price changes; "refresh failed" for
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
    if (parts.length === 6) {
      const [tag, model, oldIn, oldOut, newIn, newOut] = parts;
      return {
        tone: tag === 'locked' ? 'warn' : 'info',
        text: t(`alerts.item.price.${tag}`, { model, oldIn, oldOut, newIn, newOut })
      };
    }
  }
  if (kind === 'credential') return { tone: 'warn', text: t('alerts.item.credential') };
  if (kind === 'model_consistency') return { tone: 'info', text: t('alerts.item.consistency', { model: item }) };
  return { tone: 'info', text: item };
}

// Where an alert is handled: the channel's models or connection tab, or the model catalog.
export function alertTarget(alert) {
  if (alert.channel_id) {
    const tab = alert.kind === 'credential' ? 'connection' : 'models';
    return `${admin.providers(alert.channel_id)}?tab=${tab}`;
  }
  if (alert.kind === 'price_change' || alert.kind === 'model_consistency') return admin.models();
  return null;
}

// Price sync sources the server names (model.ApplyAutoPriceSync callers), by message key.
const SOURCES = { 'channel price lists': 'channel_price_lists', 'models.dev': 'models_dev' };

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
