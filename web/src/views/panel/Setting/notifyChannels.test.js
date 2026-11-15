import { describe, expect, it } from 'vitest';

import { channelDraftProblem, channelKinds, channelPayload, channelWhere } from './notifyChannels';

describe('notification channel drafts', () => {
  it('needs the required fields, a stored secret counts unless it is cleared', () => {
    expect(channelDraftProblem({ type: 'telegram', settings: { chat_id: '1' } })).toBe('bot_token');
    expect(channelDraftProblem({ type: 'telegram', settings: { chat_id: '1' } }, { bot_token: '12****ab' })).toBeNull();
    expect(channelDraftProblem({ type: 'email', settings: {} })).toBeNull();
    expect(channelDraftProblem({ type: 'sms', settings: {} })).toBe('type');
  });

  it('DingTalk and Lark need a signing secret or a keyword', () => {
    const draft = { type: 'dingtalk', settings: { token: 't' } };
    expect(channelDraftProblem(draft)).toBe('signed');
    expect(channelDraftProblem({ ...draft, settings: { token: 't', keyword: 'k' } })).toBeNull();
    expect(channelDraftProblem({ ...draft, clear: ['secret'] }, { token: 'x', secret: 'y' })).toBe('signed');
  });

  it('sends every field of the type, trimmed, and nothing else', () => {
    const body = channelPayload({ type: 'pushdeer', name: ' Phone ', settings: { pushkey: ' k ', junk: 'x' }, enabled: true, sort: '2' });
    expect(body).toEqual({
      id: 0,
      name: 'Phone',
      type: 'pushdeer',
      settings: { pushkey: 'k', url: '' },
      clear_secrets: [],
      enabled: true,
      kinds: [],
      sort: 2
    });
  });

  it('sends the notice kinds a channel takes, from a draft or a stored row', () => {
    expect(channelPayload({ type: 'slack', kinds: ['low_balance'] }).kinds).toEqual(['low_balance']);
    expect(channelPayload({ type: 'slack', kinds: 'channel_disabled,low_balance' }).kinds).toEqual(['channel_disabled', 'low_balance']);
    expect(channelKinds({ kinds: '' })).toEqual([]);
  });

  it('shows where a channel sends to', () => {
    expect(channelWhere({ type: 'email', detail: 'root@example.com' })).toBe('root@example.com');
    expect(channelWhere({ type: 'telegram', settings: { chat_id: '-100' } })).toBe('-100');
    expect(channelWhere({ type: 'wecom', settings: {} })).toBe('');
  });
});
