import { useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { Check, Copy } from 'lucide-react';
import hljs from 'highlight.js/lib/core';
import bash from 'highlight.js/lib/languages/bash';
import javascript from 'highlight.js/lib/languages/javascript';
import json from 'highlight.js/lib/languages/json';
import python from 'highlight.js/lib/languages/python';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import './code-block.css';

// 只注册需要的语言(core + bash + json + python + javascript),避免打包整个 highlight.js。stock bash 语法
// 在 `curl ... -H ...` 这类命令下不会给命令名/flag 上色,这里扩展出 title(命令)与
// params(短/长 flag)两类,配合 code-block.css 让「命令 / flag / 字符串」三者可区分。
let registered = false;
function ensureLanguages() {
  if (registered) return;
  hljs.registerLanguage('bash', (instance) => {
    const def = bash(instance);
    return {
      ...def,
      contains: [
        { scope: 'title.function_', begin: /^\s*[A-Za-z][\w./-]*/, relevance: 0 },
        { scope: 'params', begin: /(^|\s)-{1,2}[A-Za-z][\w-]*/, relevance: 0 },
        ...def.contains
      ]
    };
  });
  hljs.registerLanguage('json', json);
  // API 目录页的多语言示例(openai / anthropic / google SDK)需要 python 与 javascript。
  hljs.registerLanguage('python', python);
  hljs.registerLanguage('javascript', javascript);
  registered = true;
}

// 大内容渲染阈值:对话/日志明细可达 64KB,对超大文本跑 hljs 全量高亮会明显卡顿。
// 超过此字节数的实际渲染文本直接走纯文本(React 转义),不再高亮。
const HIGHLIGHT_MAX_CHARS = 32 * 1024;

// 共享代码块:顶部工具条(语言标签 + 可选复制按钮),下方 hljs 高亮代码区。
// 复制走 onCopy/onCopyError 回调,由调用方决定 toast 等反馈;按钮自身提供 2s
// 的「已复制」状态切换。
// 大内容防护(默认关闭,不影响既有调用点):
//   - 实际渲染文本超过 HIGHLIGHT_MAX_CHARS 时跳过高亮,改纯文本渲染;
//   - progressiveChunkSize>0 且内容更长时,先渲染首段,点「显示全部」再渲染剩余。
export default function CodeBlock({
  code,
  language = 'bash',
  showCopy = true,
  wrap = false,
  maxHeightClass = 'max-h-72',
  copyLabel = 'Copy',
  copiedLabel = 'Copied',
  onCopy,
  onCopyError,
  className,
  progressiveChunkSize = 0,
  showAllLabel = 'Show all'
}) {
  const [copied, setCopied] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const timer = useRef();

  useEffect(() => () => clearTimeout(timer.current), []);

  const value = code ?? '';
  const progressive = progressiveChunkSize > 0 && value.length > progressiveChunkSize;
  // 折叠时先渲染首段(渐进渲染),打开弹窗即使内容极大也只处理一小段。
  useEffect(() => {
    setExpanded(false);
  }, [code]);
  const rendered = progressive && !expanded ? value.slice(0, progressiveChunkSize) : value;
  const skipHighlight = rendered.length > HIGHLIGHT_MAX_CHARS;

  const html = useMemo(() => {
    if (skipHighlight) return null;
    ensureLanguages();
    if (language && hljs.getLanguage(language)) {
      return hljs.highlight(rendered, { language, ignoreIllegals: true }).value;
    }
    return hljs.highlightAuto(rendered).value;
  }, [rendered, language, skipHighlight]);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(code ?? '');
      setCopied(true);
      clearTimeout(timer.current);
      timer.current = setTimeout(() => setCopied(false), 2000);
      onCopy?.(code);
    } catch (err) {
      onCopyError?.(err, code);
    }
  };

  return (
    <div className={cn('code-block-hl overflow-hidden rounded-lg border border-border', className)}>
      <div className="flex items-center justify-between gap-2 border-b border-border bg-muted px-3 py-1.5">
        <span className="font-mono text-[11px] font-medium uppercase tracking-wider text-muted-foreground">{language}</span>
        {showCopy && (
          <Button variant="ghost" size="sm" className="h-7 shrink-0 gap-1.5 px-2" onClick={handleCopy}>
            {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
            {copied ? copiedLabel : copyLabel}
          </Button>
        )}
      </div>
      <pre className={cn('overflow-y-auto bg-muted/50 p-3 text-xs leading-relaxed', maxHeightClass)}>
        {skipHighlight ? (
          <code className={cn('hljs block font-mono', wrap ? 'whitespace-pre-wrap break-all' : 'overflow-x-auto whitespace-pre')}>
            {rendered}
          </code>
        ) : (
          <code
            className={cn('hljs block font-mono', wrap ? 'whitespace-pre-wrap break-all' : 'overflow-x-auto whitespace-pre')}
            dangerouslySetInnerHTML={{ __html: html }}
          />
        )}
        {progressive && !expanded && (
          <Button variant="outline" size="sm" className="mt-2 h-7" onClick={() => setExpanded(true)}>
            {showAllLabel} · {Math.ceil((value.length - progressiveChunkSize) / 1024)} KB
          </Button>
        )}
      </pre>
    </div>
  );
}

CodeBlock.propTypes = {
  code: PropTypes.string,
  language: PropTypes.string,
  showCopy: PropTypes.bool,
  wrap: PropTypes.bool,
  maxHeightClass: PropTypes.string,
  copyLabel: PropTypes.node,
  copiedLabel: PropTypes.node,
  onCopy: PropTypes.func,
  onCopyError: PropTypes.func,
  className: PropTypes.string,
  progressiveChunkSize: PropTypes.number,
  showAllLabel: PropTypes.node
};
