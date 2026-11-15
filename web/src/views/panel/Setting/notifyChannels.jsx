import PropTypes from 'prop-types';
import { Mail } from 'lucide-react';

import { cn } from '@/lib/utils';
import BrandIcon from '@/components/brand/BrandIcon';
import larkLogo from 'assets/images/icons/lark.svg';
import telegramLogo from 'assets/images/icons/telegram.svg';

// Notification channel types and their settings, in the order the console offers them. Mirrors
// notify.Types on the server (common/notify/build.go): `secret` fields are never sent back, only
// a masked echo; `required` ones must be set. `domain` is the official site whose icon stands
// for the type; `logo` is its official mark bundled with the console, used when there is one.
export const NOTIFY_CHANNEL_TYPES = [
  { type: 'email', fields: [{ key: 'to' }] },
  { type: 'wecom', domain: 'work.weixin.qq.com', fields: [{ key: 'webhook', secret: true, required: true }] },
  {
    type: 'dingtalk',
    domain: 'dingtalk.com',
    signed: true,
    fields: [{ key: 'token', secret: true, required: true }, { key: 'secret', secret: true }, { key: 'keyword' }]
  },
  {
    type: 'lark',
    domain: 'feishu.cn',
    logo: larkLogo,
    signed: true,
    fields: [{ key: 'token', secret: true, required: true }, { key: 'secret', secret: true }, { key: 'keyword' }]
  },
  { type: 'pushdeer', domain: 'pushdeer.com', fields: [{ key: 'pushkey', secret: true, required: true }, { key: 'url' }] },
  {
    type: 'telegram',
    domain: 'telegram.org',
    logo: telegramLogo,
    fields: [{ key: 'bot_token', secret: true, required: true }, { key: 'chat_id', required: true }, { key: 'http_proxy' }]
  }
];

export const channelTypeOf = (type) => NOTIFY_CHANNEL_TYPES.find((t) => t.type === type) || null;

// What the list shows as the place on that channel: the email recipient (the server's detail,
// which falls back to the root user's address), the Telegram chat, the self-hosted PushDeer.
export function channelWhere(channel) {
  if (channel.detail) return channel.detail;
  const settings = channel.settings || {};
  if (channel.type === 'telegram') return settings.chat_id || '';
  if (channel.type === 'pushdeer') return settings.url || '';
  if (channelTypeOf(channel.type)?.signed) return settings.keyword || '';
  return '';
}

// The request body of a channel draft: every field of its type, the secrets to clear, and the
// rest of the channel.
export function channelPayload(draft) {
  const meta = channelTypeOf(draft.type);
  const settings = {};
  for (const field of meta?.fields || []) settings[field.key] = (draft.settings?.[field.key] || '').trim();
  return {
    id: draft.id || 0,
    name: (draft.name || '').trim(),
    type: draft.type,
    settings,
    clear_secrets: draft.clear || [],
    enabled: !!draft.enabled,
    sort: Number.parseInt(draft.sort, 10) || 0
  };
}

// What is missing before the draft can be saved: required fields (a stored secret counts) and,
// for DingTalk and Lark, a signing secret or a keyword.
export function channelDraftProblem(draft, stored = {}) {
  const meta = channelTypeOf(draft.type);
  if (!meta) return 'type';
  const has = (key) => Boolean((draft.settings?.[key] || '').trim()) || (Boolean(stored[key]) && !(draft.clear || []).includes(key));
  const missing = meta.fields.find((field) => field.required && !has(field.key));
  if (missing) return missing.key;
  if (meta.signed && !has('secret') && !has('keyword')) return 'signed';
  return null;
}

export function NotifyChannelIcon({ type, className }) {
  const meta = channelTypeOf(type);
  if (meta?.logo) return <img src={meta.logo} alt="" aria-hidden className={cn('size-4 shrink-0 rounded-sm', className)} />;
  if (!meta?.domain) return <Mail className={cn('size-4 shrink-0 text-muted-foreground', className)} aria-hidden />;
  return <BrandIcon domain={meta.domain} fallbackText={type} className={cn('size-4', className)} />;
}

NotifyChannelIcon.propTypes = { type: PropTypes.string, className: PropTypes.string };
