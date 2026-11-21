import { describe, expect, it } from 'vitest';

import { rankSentence, sentenceScore, sentenceTerms } from './commandScore';
import { isSentence } from './CommandMenu';

describe('sentence queries', () => {
  it('reads a long query as a sentence', () => {
    expect(isSentence('上游老是 429，怎么让它少重试一点')).toBe(true);
    expect(isSentence('upstream keeps returning 429')).toBe(true);
    expect(isSentence('重试次数')).toBe(false);
    expect(isSentence('retry times')).toBe(false);
  });

  it('splits it into words, CJK pairs and the console words of its synonyms', () => {
    const terms = sentenceTerms('上游老是 429，少重试', { 故障转移: ['重试'], 老是: ['always'] });
    expect(terms).toEqual(expect.arrayContaining(['上游', '游老', '老是', '429', '少重', '重试', 'always']));
    expect(terms).not.toContain('，');
  });

  it('ranks a setting by where its terms are: name, then other words, then the rest', () => {
    const terms = sentenceTerms('上游老是 429，怎么让它少重试一点');
    const retries = sentenceScore(terms, { primary: ['重试次数'], secondary: ['RetryTimes'], tertiary: ['请求失败后最多再换几个渠道试'] });
    const cooldown = sentenceScore(terms, {
      primary: ['限流冷却'],
      secondary: ['RetryCooldownSeconds', '429'],
      tertiary: ['渠道对某个模型返回 429 限流后暂停']
    });
    const mail = sentenceScore(terms, { primary: ['SMTP 服务器'], secondary: ['SMTPServer'], tertiary: ['发信用的服务器'] });
    expect(retries).toBe(3);
    expect(cooldown).toBe(2);
    expect(mail).toBe(0);
  });
});

describe('rankSentence', () => {
  it('counts rare terms more than common ones and drops the weak matches', () => {
    const entries = [
      { primary: ['上游新模型接入规则'], tertiary: ['上游新增模型时怎么处理'] },
      { primary: ['重试次数'], tertiary: ['上游失败后最多再换几个渠道试'] },
      { primary: ['限流冷却'], secondary: ['429'], tertiary: ['上游返回 429 限流后暂停'] },
      { primary: ['上游地址'], tertiary: ['上游的地址'] },
      { primary: ['SMTP 服务器'] }
    ];
    const scores = rankSentence('上游老是 429，怎么让它少重试一点', entries);
    const order = entries
      .map((e, i) => ({ name: e.primary[0], score: scores[i] }))
      .filter((x) => x.score > 0)
      .sort((a, b) => b.score - a.score);
    expect(order.map((x) => x.name).slice(0, 2).sort()).toEqual(['重试次数', '限流冷却'].sort());
    expect(order.map((x) => x.name)).not.toContain('SMTP 服务器');
    expect(order.map((x) => x.name)).not.toContain('上游地址');
  });
});
