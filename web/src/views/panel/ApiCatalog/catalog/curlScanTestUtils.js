// ==============================|| CURL 示例扫描（测试件）||============================== //
// 「复制即可跑」的结构扫描：把 curl 文本按 POSIX 词法切开，再挑出 --data 里的请求体。
// 目录页示例(catalog/examples.test.js)与 Playground 代码生成器(shared/codegen.test.js)
// 共用同一套断言，换文案、加模态都不必重写。

// 最小 POSIX 词法切分：单引号串原样、双引号串处理反斜杠转义、裸反斜杠转义下一字符
// （行尾反斜杠为续行）。引号没闭合就抛错 —— 这正是「复制到 shell 会卡住」的那种坏示例。
export function shellTokens(input) {
  const tokens = [];
  let current = '';
  let started = false;
  let i = 0;

  while (i < input.length) {
    const ch = input[i];

    if (ch === "'") {
      const end = input.indexOf("'", i + 1);
      if (end === -1) throw new Error('unterminated single quote');
      current += input.slice(i + 1, end);
      started = true;
      i = end + 1;
    } else if (ch === '"') {
      i += 1;
      let closed = false;
      while (i < input.length) {
        if (input[i] === '\\' && i + 1 < input.length) {
          current += input[i + 1];
          i += 2;
        } else if (input[i] === '"') {
          closed = true;
          i += 1;
          break;
        } else {
          current += input[i];
          i += 1;
        }
      }
      if (!closed) throw new Error('unterminated double quote');
      started = true;
    } else if (ch === '\\') {
      if (i + 1 >= input.length) throw new Error('dangling backslash');
      if (input[i + 1] !== '\n') {
        current += input[i + 1];
        started = true;
      }
      i += 2;
    } else if (/\s/.test(ch)) {
      if (started) {
        tokens.push(current);
        current = '';
        started = false;
      }
      i += 1;
    } else {
      current += ch;
      started = true;
      i += 1;
    }
  }

  if (started) tokens.push(current);
  return tokens;
}

const DATA_FLAGS = ['--data', '--data-raw', '--data-binary', '-d'];

export const dataValues = (tokens) =>
  tokens.flatMap((token, i) => {
    if (DATA_FLAGS.includes(token)) return [tokens[i + 1]];
    const flag = DATA_FLAGS.find((f) => token.startsWith(`${f}=`));
    return flag ? [token.slice(flag.length + 1)] : [];
  });

export const dataFlagCount = (code) => (code.match(/(^|\s)(--data(-raw|-binary)?|-d)(\s|=)/g) || []).length;

// 切分 + 请求体校验合一：断言交给调用方的 expect（本文件不 import vitest，便于任何测试复用）。
export function scanCurl(code) {
  const tokens = shellTokens(code);
  const bodies = dataValues(tokens);
  return { tokens, bodies, expectedBodies: dataFlagCount(code) };
}
