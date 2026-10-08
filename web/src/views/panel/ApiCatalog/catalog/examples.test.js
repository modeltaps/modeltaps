import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

import { CATALOG } from './index';
import { pyLiteral, shellQuote } from './_helpers';
import { scanCurl, shellTokens } from './curlScanTestUtils';

// 示例代码回归：目录页的唯一卖点是「复制即可跑」，所以这里对生成出来的示例做结构扫描，
// 而不是逐条断言文本 —— 换文案、加能力都不该重新踩同一个坑。

const CTX = { baseUrl: 'https://api.example.com', apiKey: 'sk-YOUR_TOKEN', model: 'demo-model' };

const allCapabilities = () =>
  Object.values(CATALOG).flatMap((entry) => entry.capabilities.map((capability) => [`${entry.key}/${capability.id}`, capability]));

describe('preferredModels', () => {
  // 推荐名单是前端唯一一处写死的模型名。名单整体失效时页面会静默回落到列表首项，
  // 不会报错也不会被人发现 —— 所以直接对着后端策展种子核一遍。
  const seed = JSON.parse(readFileSync(new URL('../../../../../../model/catalog_seed.json', import.meta.url), 'utf8'));
  const known = new Set(seed.models.flatMap((m) => [m.canonical, ...(m.aliases || [])]));

  it.each(allCapabilities())('%s/%o 的推荐模型都在 catalog_seed.json 里', (label, capability) => {
    for (const name of capability.preferredModels || []) {
      expect(known, `${name} 不在策展种子里`).toContain(name);
    }
  });
});

describe('shellQuote', () => {
  it('把单引号拆成闭合 + 转义引号 + 重开，整体仍是一段合法的 shell 字符串', () => {
    expect(shellQuote("What's up")).toBe(`'What'\\''s up'`);
    expect(shellQuote('plain')).toBe(`'plain'`);
  });
});

describe('curl 示例', () => {
  it('词法切分能认出引号、转义与续行，未闭合引号直接报错', () => {
    expect(shellTokens("curl --data 'a b' --header 'x: 1'")).toEqual(['curl', '--data', 'a b', '--header', 'x: 1']);
    expect(shellTokens("curl \\\n  --data 'What'\\''s up'")).toEqual(['curl', '--data', "What's up"]);
    expect(() => shellTokens("curl --data 'unterminated")).toThrow();
  });

  it.each(allCapabilities())('%s 的 curl 能被切分完整，请求体是合法 JSON', (label, capability) => {
    const build = capability.examples?.curl;
    if (typeof build !== 'function') return;

    const { bodies, expectedBodies } = scanCurl(build(CTX));

    // 有 --data 就必须能被切出来：切不出来说明引号在中途被提前闭合了。
    expect(bodies.length, '切分结果里没有请求体').toBe(expectedBodies);
    for (const body of bodies) {
      expect(() => JSON.parse(body), `请求体不是合法 JSON: ${body}`).not.toThrow();
    }
  });
});

describe('pyLiteral', () => {
  it('把 JSON 的 true / false / null 写成 Python 字面量', () => {
    expect(pyLiteral(true)).toBe('True');
    expect(pyLiteral(false)).toBe('False');
    expect(pyLiteral(null)).toBe('None');
    expect(pyLiteral(undefined)).toBe('None');
  });

  it('字符串与数字按原样输出，空容器不展开', () => {
    expect(pyLiteral('hi')).toBe('"hi"');
    expect(pyLiteral(1024)).toBe('1024');
    expect(pyLiteral([])).toBe('[]');
    expect(pyLiteral({})).toBe('{}');
  });

  it('嵌套结构按 4 空格缩进展开', () => {
    expect(pyLiteral({ strict: true, items: [1, null] })).toBe(
      ['{', '    "strict": True,', '    "items": [', '        1,', '        None', '    ]', '}'].join('\n')
    );
  });
});

describe('Python 示例', () => {
  it.each(allCapabilities())('%s 不含 JSON 的布尔 / null 字面量', (label, capability) => {
    const build = capability.examples?.python;
    if (typeof build !== 'function') return;

    const code = build(CTX);
    // Python 里只有 True / False / None；裸的 true / false / null 一定是 JSON 直接插值漏出来的。
    expect(code).not.toMatch(/[:=[,{]\s*(true|false|null)\b/);
  });
});
