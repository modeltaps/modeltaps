import { useCallback, useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { Check, Copy } from 'lucide-react';

import { Button } from '@/components/ui/button';
import CodeBlock from '@/components/ui/code-block';
import { cn } from '@/lib/utils';

// ==============================|| PLAYGROUND — CODE VIEW ||============================== //
// 工作区里与「对话 / 表单」同级的代码视图：三语言 tab + 一键复制 + 顶部一行当前模型与接口。
// 只吃 props(snippets / meta)，不感知任何业务 state；代码由 shared/codegen.js 生成。
// 上次选的语言记在 localStorage，切模态、刷新后仍是同一门语言。

export const LANGUAGE_TABS = [
  { id: 'curl', label: 'curl', highlight: 'bash' },
  { id: 'python', label: 'Python', highlight: 'python' },
  { id: 'javascript', label: 'JavaScript', highlight: 'javascript' }
];

const STORAGE_KEY = 'playground.codeLanguage';

// node 环境(单测)与隐私模式下没有可用的 Storage，读写都不抛，回落默认语言。
export function readStoredLanguage() {
  try {
    const stored = typeof localStorage !== 'undefined' ? localStorage.getItem(STORAGE_KEY) : null;
    return LANGUAGE_TABS.some((tab) => tab.id === stored) ? stored : LANGUAGE_TABS[0].id;
  } catch {
    return LANGUAGE_TABS[0].id;
  }
}

export function storeLanguage(language) {
  try {
    if (typeof localStorage !== 'undefined') localStorage.setItem(STORAGE_KEY, language);
  } catch {
    /* 存不下就只在本次会话里生效 */
  }
}

// 复制：写剪贴板成功才回调 onCopy，失败走 onCopyError —— 两个分支都不抛给调用方。
export async function copySnippet(text, { clipboard, onCopy, onCopyError } = {}) {
  const target = clipboard ?? (typeof navigator !== 'undefined' ? navigator.clipboard : null);
  try {
    if (!target) throw new Error('clipboard unavailable');
    await target.writeText(text ?? '');
    onCopy?.(text);
    return true;
  } catch (err) {
    onCopyError?.(err, text);
    return false;
  }
}

export default function CodeView({ snippets, meta, labels, language, onLanguageChange, onCopy, onCopyError, className }) {
  const [internal, setInternal] = useState(readStoredLanguage);
  const active = language ?? internal;
  const [copied, setCopied] = useState(false);
  const timer = useRef();

  useEffect(() => () => clearTimeout(timer.current), []);

  const select = useCallback(
    (next) => {
      storeLanguage(next);
      if (language === undefined) setInternal(next);
      onLanguageChange?.(next);
    },
    [language, onLanguageChange]
  );

  const tab = LANGUAGE_TABS.find((item) => item.id === active) ?? LANGUAGE_TABS[0];
  const code = snippets?.[tab.id] ?? '';

  const handleCopy = useCallback(async () => {
    const ok = await copySnippet(code, { onCopy, onCopyError });
    if (!ok) return;
    setCopied(true);
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), 2000);
  }, [code, onCopy, onCopyError]);

  return (
    <div className={cn('flex min-h-0 flex-1 flex-col gap-2.5 p-4', className)}>
      <div className="flex items-center gap-2">
        <div role="tablist" aria-label="code-language" className="inline-flex gap-1">
          {LANGUAGE_TABS.map((item) => (
            <button
              key={item.id}
              type="button"
              role="tab"
              aria-selected={item.id === tab.id}
              onClick={() => select(item.id)}
              className={cn(
                'rounded-md px-2.5 py-1 text-xs transition-colors',
                item.id === tab.id ? 'bg-secondary font-semibold text-foreground' : 'text-muted-foreground hover:bg-accent'
              )}
            >
              {item.label}
            </button>
          ))}
        </div>
        <div className="flex-1" />
        <Button variant="outline" size="sm" className="h-7 gap-1.5 px-2" onClick={handleCopy}>
          {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
          {copied ? labels?.copied : labels?.copy}
        </Button>
      </div>
      {/* 顶部一行：当前模型 · 接口 · 「随运行设置实时同步」提示。 */}
      <p className="truncate text-xs text-muted-foreground">{[meta?.model, meta?.endpoint, labels?.sync].filter(Boolean).join(' · ')}</p>
      <CodeBlock
        code={code}
        language={tab.highlight}
        showCopy={false}
        wrap
        maxHeightClass="max-h-none"
        className="min-h-0 flex-1 overflow-auto text-[12px]"
      />
      {labels?.keyHint && <span className="text-xs text-muted-foreground">{labels.keyHint}</span>}
    </div>
  );
}

CodeView.propTypes = {
  snippets: PropTypes.shape({ curl: PropTypes.string, python: PropTypes.string, javascript: PropTypes.string }),
  meta: PropTypes.shape({ model: PropTypes.string, endpoint: PropTypes.string }),
  labels: PropTypes.shape({
    copy: PropTypes.string,
    copied: PropTypes.string,
    sync: PropTypes.string,
    keyHint: PropTypes.string
  }),
  language: PropTypes.oneOf(LANGUAGE_TABS.map((tab) => tab.id)),
  onLanguageChange: PropTypes.func,
  onCopy: PropTypes.func,
  onCopyError: PropTypes.func,
  className: PropTypes.string
};
