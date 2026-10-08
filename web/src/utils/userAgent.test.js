import { describe, expect, it } from 'vitest';

import { describeUserAgent } from './userAgent';

describe('describeUserAgent', () => {
  it('常见桌面与移动端 UA 压成「系统 · 浏览器」', () => {
    expect(
      describeUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36')
    ).toBe('macOS · Chrome');
    expect(describeUserAgent('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/128.0 Safari/537.36 Edg/128.0')).toBe(
      'Windows · Edge'
    );
    expect(
      describeUserAgent(
        'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1'
      )
    ).toBe('iOS · Safari');
    expect(describeUserAgent('Mozilla/5.0 (X11; Linux x86_64; rv:129.0) Gecko/20100101 Firefox/129.0')).toBe('Linux · Firefox');
  });

  it('认不出来时返回空串,由调用方回退到「未知设备」', () => {
    expect(describeUserAgent('')).toBe('');
    expect(describeUserAgent(undefined)).toBe('');
    expect(describeUserAgent('curl/8.4.0')).toBe('');
  });
});
