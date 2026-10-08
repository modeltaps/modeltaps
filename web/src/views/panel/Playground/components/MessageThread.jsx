import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { AlertCircle } from 'lucide-react';

import { Alert, AlertDescription } from '@/components/ui/alert';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { copySnippet } from './CodeView';
import MessageMeta from './MessageMeta';

// ==============================|| CONSOLE — MESSAGE THREAD ||============================== //
// 一段对话的消息列表：用户消息是右对齐的浅灰圆角块，助手回复是纯正文，思考过程默认折叠，
// 下面一行常显「tokens · ms + 该条代码 / 复制 / 重跑」。对话页与对比页的每一列共用。
// 只吃 props：轮次、运行态与回调都由上层给；缺哪个回调就不画哪个图标。
// error 给了就在列表末尾画一条错误提示（对比页每列各自报错；对话页的错误放在输入框上方）。

export const seconds = (ms) => (typeof ms === 'number' && ms >= 0 ? `${(ms / 1000).toFixed(2)}s` : '');

// 思考过程默认折叠，标题上先给段数，让人知道值不值得展开。
export const reasoningParts = (text) =>
  String(text || '')
    .split(/\n+/)
    .filter((line) => line.trim() !== '').length;

// 元信息行只报一个 token 数：优先用上游给的合计，没有合计就把进出相加。
export function turnTokens(turn) {
  const usage = turn?.usage;
  if (!usage) return null;
  if (typeof usage.total_tokens === 'number') return usage.total_tokens;
  const input = usage.prompt_tokens ?? 0;
  const output = usage.completion_tokens ?? 0;
  return input + output ? input + output : null;
}

// 「消息变了」的最小标识：轮数 + 末轮正文 / 思考过程长度，流式每来一段都会变。
export function threadSignal(turns = []) {
  const last = turns[turns.length - 1] || {};
  return `${turns.length}:${String(last.content || '').length}:${String(last.reasoning || '').length}`;
}

export default function MessageThread({ turns = [], running = false, error = null, onViewCode, onRegenerate, className }) {
  const { t } = useTranslation();

  return (
    <div className={className ?? 'space-y-6'}>
      {turns.map((turn, index) =>
        turn.role === 'user' ? (
          <div key={index} className="flex justify-end">
            <div className="max-w-[85%] whitespace-pre-wrap rounded-2xl bg-muted px-4 py-2.5 text-sm [overflow-wrap:anywhere]">
              {turn.content}
            </div>
          </div>
        ) : (
          <article key={index} className="min-w-0 space-y-2">
            {turn.reasoning && (
              <CollapsibleSection
                plain
                title={t('playgroundConsole.chat.reasoning', { parts: reasoningParts(turn.reasoning) })}
                className="text-xs"
              >
                <p className="whitespace-pre-wrap text-xs text-muted-foreground [overflow-wrap:anywhere]">{turn.reasoning}</p>
              </CollapsibleSection>
            )}

            <div className="whitespace-pre-wrap text-sm leading-relaxed [overflow-wrap:anywhere]">
              {turn.content}
              {turn.pending &&
                running &&
                (turn.content ? (
                  <span className="ml-0.5 inline-block h-3.5 w-1.5 animate-pulse bg-foreground align-middle" aria-hidden="true" />
                ) : (
                  <span className="text-muted-foreground">{t('playgroundConsole.chat.waiting')}</span>
                ))}
            </div>

            {turn.pending && running ? (
              <p className="text-xs text-muted-foreground">
                {t('playgroundConsole.chat.meta.generating')}
                {turn.firstTokenMs !== null && ` · ${t('playgroundConsole.chat.meta.firstToken', { value: seconds(turn.firstTokenMs) })}`}
              </p>
            ) : (
              <MessageMeta
                tokens={turnTokens(turn)}
                durationMs={turn.interrupted ? null : turn.durationMs}
                tokensLabel={t('playgroundConsole.chat.meta.tokensUnit')}
                status={turn.interrupted ? t('playgroundConsole.chat.meta.interrupted') : undefined}
                onViewCode={onViewCode ? () => onViewCode(index) : undefined}
                onCopy={() => copySnippet(turn.content)}
                onRegenerate={onRegenerate && index === turns.length - 1 && !running ? onRegenerate : undefined}
                viewCodeLabel={t('playgroundConsole.chat.actions.code')}
                copyLabel={t('playgroundConsole.chat.actions.copy')}
                regenerateLabel={t('playgroundConsole.chat.actions.rerun')}
              />
            )}
          </article>
        )
      )}
      {error && (
        <Alert variant="error">
          <AlertCircle />
          <AlertDescription>
            {t(error.i18nKey)}
            {error.message ? ` — ${error.message}` : ''}
          </AlertDescription>
        </Alert>
      )}
    </div>
  );
}

MessageThread.propTypes = {
  turns: PropTypes.array,
  running: PropTypes.bool,
  error: PropTypes.object,
  onViewCode: PropTypes.func,
  onRegenerate: PropTypes.func,
  className: PropTypes.string
};
