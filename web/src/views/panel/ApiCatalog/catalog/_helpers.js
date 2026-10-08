// ==============================|| API CATALOG — EXAMPLE HELPERS ||============================== //
// 四个模态目录共用的示例构造件:JSON 缩进、shell 安全的 curl 片段、三语言客户端前缀。
// 示例是按所选模型即时生成的,文案随时会改,所以 curl 的 header 与请求体一律走
// shellQuote —— 否则文案里的撇号(What's ...)会提前闭合单引号字符串,复制到 shell 就报错。

export const indent = (obj) => JSON.stringify(obj, null, 2).split('\n').join('\n  ');

// POSIX shell 的单引号字符串内无法转义任何字符,只能先闭合、拼一个转义引号、再开新串:
//   ' -> '\''
export const shellQuote = (value) => `'${String(value).replace(/'/g, `'\\''`)}'`;

// Python 字面量:JSON 的 true / false / null 在 Python 里写作 True / False / None,
// 直接 JSON.stringify 插到 Python 示例里会产出语法错误的代码。
export function pyLiteral(value, level = 0) {
  const pad = (n) => ' '.repeat(n * 4);

  if (value === null || value === undefined) return 'None';
  if (value === true) return 'True';
  if (value === false) return 'False';
  if (typeof value === 'number') return String(value);
  if (typeof value === 'string') return JSON.stringify(value);

  if (Array.isArray(value)) {
    if (value.length === 0) return '[]';
    const items = value.map((item) => `${pad(level + 1)}${pyLiteral(item, level + 1)}`);
    return `[\n${items.join(',\n')}\n${pad(level)}]`;
  }

  const entries = Object.entries(value);
  if (entries.length === 0) return '{}';
  const items = entries.map(([key, item]) => `${pad(level + 1)}${JSON.stringify(key)}: ${pyLiteral(item, level + 1)}`);
  return `{\n${items.join(',\n')}\n${pad(level)}}`;
}

// 统一的 curl 片段:首行方法与地址,其余为 header / flag / 请求体。
export const curlPost = (url, lines) => ['curl --request POST', `  --url ${url}`, ...lines.map((line) => `  ${line}`)].join(' \\\n');

// JSON 请求体的 curl:headers 为 "Name: value" 字符串数组,extra 为额外的 curl flag。
export const curlJson = (url, headers, body, extra = []) =>
  curlPost(url, [...headers.map((h) => `--header ${shellQuote(h)}`), ...extra, `--data ${shellQuote(indent(body))}`]);

export const curlOpenAI = (baseUrl, apiKey, path, body, extra = []) =>
  curlJson(`${baseUrl}${path}`, [`Authorization: Bearer ${apiKey}`, 'Content-Type: application/json'], body, extra);

export const pyClient = (baseUrl, apiKey) => `from openai import OpenAI

client = OpenAI(api_key="${apiKey}", base_url="${baseUrl}/v1")
`;

export const jsClient = (baseUrl, apiKey) => `import OpenAI from 'openai';

const client = new OpenAI({ apiKey: '${apiKey}', baseURL: '${baseUrl}/v1' });
`;
