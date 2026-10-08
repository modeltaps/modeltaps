import PropTypes from 'prop-types';
import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { useNavigate } from 'react-router';
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CircleDollarSign,
  Clock,
  Copy,
  Expand,
  GitBranch,
  Hash,
  Loader2,
  Timer,
  X,
  Zap
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import CodeBlock from '@/components/ui/code-block';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import ModelIcon from '@/components/brand/ModelIcon';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { resolveBrandLabel } from '@/components/brand/brandIcons';
import { cn } from '@/lib/utils';
import { API } from 'utils/api';
import { renderQuota, showError, showSuccess, timestamp2string } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';
import { orgSettingsUrl } from '../Settings/sections';
import { badgeClass, calculatePrice, calculateOriginalQuota, calculateTokens, deriveDuration, finishReasonMeta } from './logHelpers';
import PromptViewer, { parsePromptMessages } from './PromptViewer';
import AppFavicon from './AppFavicon';

// ==============================|| LOG — ROW DETAIL PANEL ||============================== //
// Wide right-side slide-over (OpenRouter "Generation details" style) for reading log
// detail: header chips → metric cards → overview → billing → request/response bodies →
// metadata. Fixed ~640px width (does not scale with viewport), full screen on narrow
// viewports; Esc / overlay close. Portaled to <body> so the fixed overlay is anchored to
// the viewport and stays flush to the right edge on resize, independent of the scrollable
// <main> ancestor (a fixed element trapped in an overflow/transformed ancestor drifts).

// Progressive-render threshold for request/response bodies (may reach 64KB). Opening
// the panel only highlights the first chunk; "show all" reveals the rest as plain text.
const IO_CHUNK = 16 * 1024;

// Pretty-print request/response bodies when they are valid JSON; otherwise render as-is.
function prettyJson(raw) {
  const value = raw || '';
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

// Compact overview field (label above value), used in the top metadata grid.
function Field({ label, value }) {
  if (value === undefined || value === null || value === '') return null;
  return (
    <div className="flex min-w-0 flex-col gap-0.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="break-words text-sm font-medium text-foreground">{value}</span>
    </div>
  );
}

Field.propTypes = { label: PropTypes.string, value: PropTypes.node };

// Label/value row for the billing breakdown card.
function Row({ label, value }) {
  if (value === undefined || value === null || value === '') return null;
  return (
    <div className="flex items-start justify-between gap-4 py-1.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium text-foreground">{value}</span>
    </div>
  );
}

Row.propTypes = { label: PropTypes.string, value: PropTypes.node };

// Compact metric card (icon + label + prominent value + optional unit suffix) for the top
// metrics grid (OpenRouter style). When `placeholder` is provided, an empty value renders it
// in a muted/faded tone instead of hiding the card — used for metrics that may lack data.
function MetricCard({ icon: Icon, label, value, unit, placeholder }) {
  const isEmpty = value === undefined || value === null || value === '';
  if (isEmpty && placeholder === undefined) return null;
  return (
    <div className="flex flex-col gap-0.5 rounded-lg border border-border bg-muted/30 px-3 py-2">
      <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
        {Icon && <Icon className="size-4 shrink-0" />}
        <span className="truncate">{label}</span>
      </div>
      <div
        className={cn('truncate text-sm font-medium', isEmpty ? 'text-muted-foreground/50' : 'text-foreground')}
        title={typeof value === 'string' ? value : undefined}
      >
        {isEmpty ? placeholder : value}
        {!isEmpty && unit && <span className="ml-0.5 text-xs text-muted-foreground">{unit}</span>}
      </div>
    </div>
  );
}

MetricCard.propTypes = {
  icon: PropTypes.elementType,
  label: PropTypes.string,
  value: PropTypes.node,
  unit: PropTypes.node,
  placeholder: PropTypes.node
};

// Collapsible request/response block: header (toggle + truncated badge + copy) over a
// large-content-safe CodeBlock that scrolls independently. Bodies are already
// truncated/redacted server-side (only request/response bodies are stored, never auth
// headers/keys — see model.LogDetail), so they render verbatim.
function IoBlock({ title, body, truncated, open, onToggle, onCopy, t }) {
  return (
    <div className="flex flex-col rounded-lg border border-border">
      <div className="flex shrink-0 items-center justify-between gap-2 px-3 py-2">
        <button type="button" onClick={onToggle} className="flex min-w-0 items-center gap-1.5 text-sm font-medium text-foreground">
          {open ? <ChevronDown className="size-4 shrink-0" /> : <ChevronRight className="size-4 shrink-0" />}
          <span className="truncate">{title}</span>
          {truncated && (
            <span className="shrink-0 rounded bg-amber-500/15 px-1.5 py-0.5 text-[11px] font-medium text-amber-600 dark:text-amber-400">
              {t('logPage.io.truncated')}
            </span>
          )}
        </button>
        <Button variant="ghost" size="icon" className="size-7 shrink-0" aria-label={t('logPage.io.copy')} onClick={onCopy}>
          <Copy className="size-3.5" />
        </Button>
      </div>
      {open && (
        <div className="border-t border-border p-2">
          <CodeBlock
            language="json"
            wrap
            showCopy={false}
            maxHeightClass="max-h-[42vh]"
            progressiveChunkSize={IO_CHUNK}
            showAllLabel={t('logPage.io.showAll')}
            code={prettyJson(body)}
          />
        </div>
      )}
    </div>
  );
}

IoBlock.propTypes = {
  title: PropTypes.string,
  body: PropTypes.string,
  truncated: PropTypes.bool,
  open: PropTypes.bool,
  onToggle: PropTypes.func.isRequired,
  onCopy: PropTypes.func.isRequired,
  t: PropTypes.func.isRequired
};

// Always-visible status pill next to the IO section title. `statusKey` is derived from
// the fetch outcome (see computeIoStatus) and maps to a localized label + tone.
const IO_STATUS_TONE = {
  emerald: 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
  amber: 'bg-amber-500/15 text-amber-600 dark:text-amber-400',
  muted: 'bg-muted text-muted-foreground',
  red: 'bg-red-500/15 text-red-600 dark:text-red-400'
};

const IO_STATUS_META = {
  stored: { labelKey: 'logPage.io.statusStored', tone: 'emerald' },
  siteOff: { labelKey: 'logPage.io.statusSiteOff', tone: 'amber' },
  settingOff: { labelKey: 'logPage.io.statusSettingOff', tone: 'amber' },
  expired: { labelKey: 'logPage.io.statusExpired', tone: 'muted' },
  forbidden: { labelKey: 'logPage.io.statusForbidden', tone: 'muted' },
  error: { labelKey: 'logPage.io.statusError', tone: 'red' }
};

function computeIoStatus(io, siteLogIOEnabled) {
  if (io.status === 'forbidden') return 'forbidden';
  if (io.status === 'error') return 'error';
  if (io.status === 'ok') {
    if (io.data.has_detail) return 'stored';
    if (io.data.reason === 'expired') return 'expired';
    return siteLogIOEnabled ? 'settingOff' : 'siteOff';
  }
  return null;
}

function IoStatusBadge({ statusKey, t }) {
  const meta = statusKey ? IO_STATUS_META[statusKey] : null;
  if (!meta) return null;
  return (
    <span className={cn('shrink-0 rounded px-1.5 py-0.5 text-[11px] font-medium normal-case', IO_STATUS_TONE[meta.tone])}>
      {t(meta.labelKey)}
    </span>
  );
}

IoStatusBadge.propTypes = { statusKey: PropTypes.string, t: PropTypes.func.isRequired };

// Key-value row for Overview/Request sections (OpenRouter style): fixed 10rem label column
// + left-aligned value. Copyable values render as a button (mono text + hover-revealed copy
// icon) so the whole value is clickable to copy.
function KeyValueRow({ label, value, mono, copyable, onCopy, t }) {
  if (value === undefined || value === null || value === '') return null;
  return (
    <div className="grid grid-cols-[10rem_minmax(0,1fr)] items-center gap-3">
      <span className="text-sm text-muted-foreground">{label}</span>
      {copyable ? (
        <button
          type="button"
          onClick={onCopy}
          aria-label={t('logPage.io.copy')}
          className="group/copy flex min-w-0 items-center gap-1.5 justify-self-start text-left"
        >
          <span className={cn('min-w-0 truncate text-sm text-foreground', mono && 'font-mono')}>{value}</span>
          <Copy className="size-3 shrink-0 text-muted-foreground md:invisible md:group-hover/copy:visible" />
        </button>
      ) : (
        <span className={cn('min-w-0 justify-self-start break-words text-sm text-foreground', mono && 'font-mono')}>{value}</span>
      )}
    </div>
  );
}

KeyValueRow.propTypes = {
  label: PropTypes.string,
  value: PropTypes.node,
  mono: PropTypes.bool,
  copyable: PropTypes.bool,
  onCopy: PropTypes.func,
  t: PropTypes.func
};

// Provider Responses timeline chart (OpenRouter style).
function ProviderResponsesChart({ item, t }) {
  const metadata = item.metadata || {};
  const { requestTime } = deriveDuration(item);
  const completionTokens = item.completion_tokens || 0;

  // W9-M6: OpenRouter generation 统计(provider 侧真实时长)。有真实数据时优先展示,
  // 否则回退到本地测量(first_response + request_time),非 OpenRouter 日志行为不变。
  const orLatency = Number(metadata.or_latency) || 0; // provider 侧 TTFT(ms)
  const orGenTime = Number(metadata.or_generation_time) || 0; // provider 侧生成耗时(ms)
  const hasProviderData = Boolean(metadata.or_provider) && (orLatency > 0 || orGenTime > 0);

  let ttftSec;
  let generationSec;
  let totalSec;
  let providerName;
  if (hasProviderData) {
    ttftSec = orLatency / 1000;
    generationSec = orGenTime / 1000;
    totalSec = ttftSec + generationSec || requestTime;
    providerName = metadata.or_provider;
  } else {
    const ttftMs = metadata.first_response || 0;
    ttftSec = ttftMs / 1000;
    totalSec = requestTime;
    generationSec = Math.max(totalSec - ttftSec, 0);
    // Provider brand name from channel type (icon-source consistent), fallback to channel name.
    providerName =
      resolveBrandLabel(item.channel?.type, item.channel?.name) || item.channel?.name || t('logPage.providerResponses.provider');
  }

  // No timing data at all
  if (totalSec <= 0) return null;

  const hasTTFT = ttftSec > 0;
  const ttftPct = Math.min((ttftSec / totalSec) * 100, 100);
  const genPct = Math.min((generationSec / totalSec) * 100, 100);
  const secStr = (v) => t('logPage.providerResponses.seconds', { value: v.toFixed(1) });

  return (
    <div className="rounded-lg border border-border p-3">
      <h3 className="mb-2 text-sm font-semibold text-foreground">{t('logPage.providerResponses.title')}</h3>
      <div className="space-y-1.5">
        {/* Timeline rows share one grid so the auto-width label column (provider name / */}
        {/* "generation") is never truncated and both rows' bars stay vertically aligned. */}
        <div className="grid grid-cols-[max-content_1fr_auto] items-center gap-x-3 gap-y-1.5">
          {/* Row 1: provider label (name + 200) | timeline | seconds */}
          <div className="flex items-center gap-1.5">
            <span className="text-sm font-semibold text-foreground">{providerName}</span>
            <span className="shrink-0 rounded bg-emerald-500/15 px-1.5 py-0.5 text-xs font-medium text-emerald-600 dark:text-emerald-400">
              200
            </span>
          </div>
          <div className="relative h-5 overflow-hidden rounded bg-muted">
            <div className="absolute inset-y-0 left-0 rounded-l bg-emerald-500" style={{ width: `${hasTTFT ? ttftPct : 100}%` }} />
          </div>
          <span className="text-right text-sm font-medium text-foreground">{secStr(hasTTFT ? ttftSec : totalSec)}</span>

          {/* Row 2: Generation — same grid columns keep bars aligned with row 1 */}
          {hasTTFT && (
            <>
              <span className="text-sm text-muted-foreground">{t('logPage.providerResponses.generation')}</span>
              <div className="relative h-5 overflow-hidden rounded bg-muted">
                <div className="absolute inset-y-0 flex items-center bg-blue-500" style={{ left: `${ttftPct}%`, width: `${genPct}%` }}>
                  {completionTokens > 0 && generationSec > 0 && (
                    <span className="truncate px-2 text-xs font-medium text-white">
                      {t('logPage.providerResponses.tokensPerSecond', {
                        count: completionTokens,
                        rate: (completionTokens / generationSec).toFixed(1)
                      })}
                    </span>
                  )}
                </div>
              </div>
              <span className="text-right text-sm font-medium text-foreground">{secStr(generationSec)}</span>
            </>
          )}
        </div>

        {/* Total */}
        <div className="flex justify-end pt-1">
          <span className="text-sm font-medium text-muted-foreground">
            {t('logPage.providerResponses.totalDuration', { value: totalSec.toFixed(1) })}
          </span>
        </div>
      </div>
    </div>
  );
}

ProviderResponsesChart.propTypes = { item: PropTypes.object, t: PropTypes.func.isRequired };

// Collapsible section container (Usage, Prompt, Completion, Generation Data).
function CollapsibleSection({ title, summary, open, onToggle, children }) {
  return (
    <div className="rounded-lg border border-border">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-center justify-between gap-2 px-3 py-2.5 text-left transition-colors hover:bg-muted/50"
      >
        <div className="flex min-w-0 items-center gap-2">
          {open ? <ChevronDown className="size-4 shrink-0" /> : <ChevronRight className="size-4 shrink-0" />}
          <span className="truncate text-sm font-semibold text-foreground">{title}</span>
          {summary && <span className="shrink-0 text-sm text-muted-foreground">{summary}</span>}
        </div>
      </button>
      {open && <div className="border-t border-border px-3 py-2.5">{children}</div>}
    </div>
  );
}

CollapsibleSection.propTypes = {
  title: PropTypes.string,
  summary: PropTypes.string,
  open: PropTypes.bool,
  onToggle: PropTypes.func.isRequired,
  children: PropTypes.node
};

// Parse a non-streaming response body into decomposed choices. Returns null when the body
// is missing, not valid JSON, or lacks a choices[] array — callers then fall back to raw.
function parseCompletionChoices(rawBody) {
  if (!rawBody) return null;
  let parsed;
  try {
    parsed = JSON.parse(rawBody);
  } catch {
    return null;
  }
  const choices = parsed?.choices;
  if (!Array.isArray(choices) || choices.length === 0) return null;
  return choices.map((choice, index) => {
    const message = choice.message || choice.delta || {};
    const rawContent = message.content;
    const content = typeof rawContent === 'string' ? rawContent : rawContent == null ? '' : JSON.stringify(rawContent, null, 2);
    return {
      index: choice.index ?? index,
      finishReason: choice.finish_reason || null,
      content,
      toolCalls: Array.isArray(message.tool_calls) ? message.tool_calls : []
    };
  });
}

// Decomposed presentation of a single choice: finish reason, tool calls (name + args), or
// plain text content; empty message renders a "no content" note.
function ChoiceView({ choice, t }) {
  const hasTools = choice.toolCalls.length > 0;
  return (
    <div className="space-y-3 rounded-lg border border-border p-3">
      {choice.finishReason && (
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">{t('logPage.completion.finishReason')}</span>
          <span className="rounded bg-muted px-1.5 py-0.5 text-xs font-medium text-foreground">{choice.finishReason}</span>
        </div>
      )}
      {hasTools ? (
        <div className="space-y-2">
          <div className="text-xs font-semibold text-muted-foreground">{t('logPage.completion.toolCalls')}</div>
          {choice.toolCalls.map((tc, i) => {
            const fn = tc.function || tc;
            const name = fn.name || '';
            const args = fn.arguments;
            const argsStr = typeof args === 'string' ? prettyJson(args) : args == null ? '' : JSON.stringify(args, null, 2);
            return (
              <div key={tc.id || i} className="overflow-hidden rounded border border-border">
                <div className="border-b border-border bg-muted/40 px-2 py-1 font-mono text-xs font-medium text-foreground">{name}</div>
                <CodeBlock language="json" wrap showCopy maxHeightClass="max-h-[42vh]" code={argsStr} />
              </div>
            );
          })}
        </div>
      ) : choice.content ? (
        <div className="space-y-1">
          <div className="text-xs font-semibold text-muted-foreground">{t('logPage.completion.text')}</div>
          <CodeBlock
            language="text"
            wrap
            showCopy
            maxHeightClass="max-h-[42vh]"
            progressiveChunkSize={IO_CHUNK}
            showAllLabel={t('logPage.io.showAll')}
            code={choice.content}
          />
        </div>
      ) : (
        <div className="text-sm text-muted-foreground">{t('logPage.completion.noContent')}</div>
      )}
    </div>
  );
}

ChoiceView.propTypes = { choice: PropTypes.object.isRequired, t: PropTypes.func.isRequired };

// Completion body renderer. Non-streaming responses are decomposed into choices with
// per-choice text / tool-call presentation and 1/N pagination; a parsed/raw toggle lets
// users inspect the underlying JSON. Streaming responses (plain SSE text) render as-is,
// and parse-failed / truncated bodies fall back to raw text with a badge.
function CompletionBody({ io, isStream, t }) {
  const rawBody = io.data.response_body || '';
  const truncated = Boolean(io.data.response_truncated);
  const [view, setView] = useState('parsed');
  const [choiceIdx, setChoiceIdx] = useState(0);

  if (isStream) {
    return (
      <div className="space-y-2">
        <div className="text-xs font-semibold text-muted-foreground">{t('logPage.completion.streamedText')}</div>
        <CodeBlock
          language="text"
          wrap
          showCopy
          maxHeightClass="max-h-[42vh]"
          progressiveChunkSize={IO_CHUNK}
          showAllLabel={t('logPage.io.showAll')}
          code={rawBody}
        />
        {truncated && <div className="text-xs text-amber-600 dark:text-amber-400">{t('logPage.completion.truncatedWarning')}</div>}
      </div>
    );
  }

  const choices = truncated ? null : parseCompletionChoices(rawBody);
  const canParse = Array.isArray(choices) && choices.length > 0;
  const showRaw = !canParse || view === 'raw';
  const activeIdx = canParse ? Math.min(choiceIdx, choices.length - 1) : 0;

  return (
    <div className="space-y-3">
      {canParse && (
        <div className="flex items-center justify-between gap-2">
          <div className="flex gap-2">
            {['parsed', 'raw'].map((v) => (
              <button
                key={v}
                type="button"
                onClick={() => setView(v)}
                className={cn(
                  'rounded px-2 py-1 text-xs font-medium transition-colors',
                  view === v ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                )}
              >
                {t(`logPage.completion.${v}`)}
              </button>
            ))}
          </div>
          {view === 'parsed' && choices.length > 1 && (
            <div className="flex items-center gap-1">
              <Button
                variant="ghost"
                size="icon"
                className="size-6"
                aria-label="Previous choice"
                disabled={activeIdx === 0}
                onClick={() => setChoiceIdx((i) => Math.max(0, i - 1))}
              >
                <ChevronLeft className="size-4" />
              </Button>
              <span className="text-xs font-medium text-muted-foreground">
                {t('logPage.completion.choice', { current: activeIdx + 1, total: choices.length })}
              </span>
              <Button
                variant="ghost"
                size="icon"
                className="size-6"
                aria-label="Next choice"
                disabled={activeIdx === choices.length - 1}
                onClick={() => setChoiceIdx((i) => Math.min(choices.length - 1, i + 1))}
              >
                <ChevronRight className="size-4" />
              </Button>
            </div>
          )}
        </div>
      )}

      {showRaw ? (
        <>
          <CodeBlock
            language="json"
            wrap
            showCopy
            maxHeightClass="max-h-[42vh]"
            progressiveChunkSize={IO_CHUNK}
            showAllLabel={t('logPage.io.showAll')}
            code={prettyJson(rawBody)}
          />
          {truncated ? (
            <div className="text-xs text-amber-600 dark:text-amber-400">{t('logPage.completion.truncatedWarning')}</div>
          ) : (
            !canParse && rawBody && <div className="text-xs text-amber-600 dark:text-amber-400">{t('logPage.completion.parseError')}</div>
          )}
        </>
      ) : (
        <ChoiceView choice={choices[activeIdx]} t={t} />
      )}
    </div>
  );
}

CompletionBody.propTypes = { io: PropTypes.object.isRequired, isStream: PropTypes.bool, t: PropTypes.func.isRequired };

// Empty state for `has_detail=false && reason=not_enabled`: a status description plus a
// role-aware guide toward the page where retention can be turned on. The site-wide gate
// (siteLogIOEnabled) short-circuits everything: when it is off, only a platform admin
// sees an actionable button; otherwise the request just missed the org / account default.
// Guide URLs carry `?highlight=log-io` so the target page scrolls to and highlights the
// matching control (anchor mechanism implemented by the sibling deep-link task).
function NotEnabledEmpty({ t, siteLogIOEnabled, platformIsAdmin, isOrgContext, orgIsAdmin, orgId, onNavigate }) {
  let description;
  let showHint = true;
  const actions = [];
  if (!siteLogIOEnabled) {
    if (platformIsAdmin) {
      description = t('logPage.io.emptyNotEnabledSiteAdmin');
      actions.push({ label: t('logPage.io.gotoSiteSetting'), to: '/panel/setting/privacy?highlight=log-io' });
    } else {
      description = t('logPage.io.emptyNotEnabledSiteUser');
      showHint = false;
    }
  } else if (isOrgContext) {
    if (orgIsAdmin) {
      description = t('logPage.io.emptyNotEnabledOrgAdmin');
      actions.push({ label: t('logPage.io.gotoOrgSetting'), to: `${orgSettingsUrl(orgId, 'general')}?highlight=log-io` });
    } else {
      description = t('logPage.io.emptyNotEnabledOrgMember');
    }
  } else {
    description = t('logPage.io.emptyNotEnabledPersonal');
    actions.push({ label: t('logPage.io.gotoProfile'), to: '/panel/settings/account/privacy?highlight=log-io' });
  }

  return (
    <div className="space-y-2">
      <p className="text-sm text-muted-foreground">{description}</p>
      {showHint && <p className="text-xs text-muted-foreground">{t('logPage.io.emptyNotEnabledHint')}</p>}
      {actions.length > 0 && (
        <div className="flex flex-wrap gap-2 pt-1">
          {actions.map((a) => (
            <Button key={a.to} variant="outline" size="sm" onClick={() => onNavigate(a.to)}>
              {a.label}
            </Button>
          ))}
        </div>
      )}
    </div>
  );
}

NotEnabledEmpty.propTypes = {
  t: PropTypes.func.isRequired,
  siteLogIOEnabled: PropTypes.bool,
  platformIsAdmin: PropTypes.bool,
  isOrgContext: PropTypes.bool,
  orgIsAdmin: PropTypes.bool,
  orgId: PropTypes.number,
  onNavigate: PropTypes.func.isRequired
};

export default function LogDetailDialog({
  item,
  userGroup,
  userIsAdmin,
  siteLogIOEnabled,
  platformIsAdmin,
  isOrgContext,
  orgIsAdmin,
  t,
  onClose
}) {
  const logId = item?.id;
  const navigate = useNavigate();
  const { currentOrgId } = useOrg();
  const [io, setIo] = useState({ status: 'idle' });
  const [showUsage, setShowUsage] = useState(false);
  const [showPromptViewer, setShowPromptViewer] = useState(false);
  const [showCompletion, setShowCompletion] = useState(false);
  const [showGenerationData, setShowGenerationData] = useState(false);

  // Esc closes the prompt viewer first (returns to the detail panel), otherwise the dialog.
  useEffect(() => {
    const onKey = (e) => {
      if (e.key !== 'Escape') return;
      if (showPromptViewer) {
        setShowPromptViewer(false);
        return;
      }
      onClose?.();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose, showPromptViewer]);

  // Fetch the full request/response detail on open. /api/log/self/detail is rewritten to
  // /api/org/:id/logs/detail in an org context (utils/orgScope.js). Authorization is
  // enforced server-side via CanViewTokenLogIO; empty states carry a reason.
  useEffect(() => {
    if (!logId) return undefined;
    let cancelled = false;
    setIo({ status: 'loading' });
    const url = userIsAdmin ? `/api/log/detail/${logId}` : `/api/log/self/detail/${logId}`;
    (async () => {
      try {
        const res = await API.get(url);
        if (cancelled) return;
        const { success, data } = res.data || {};
        setIo(success && data ? { status: 'ok', data } : { status: 'error' });
      } catch (error) {
        if (cancelled) return;
        setIo({ status: error?.response?.status === 403 ? 'forbidden' : 'error' });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [logId, userIsAdmin]);

  if (!item) return null;

  const handleGuideNavigate = (to) => {
    onClose?.();
    navigate(to);
  };

  const handleCopy = (text) => {
    try {
      navigator.clipboard.writeText(text || '');
      showSuccess(t('logPage.io.copied'));
    } catch {
      showError(t('logPage.io.copyFailed'));
    }
  };

  const { totalInputTokens, totalOutputTokens } = calculateTokens(item);
  const metadata = item.metadata || {};
  // App 归因(W9-B 写入 metadata.app_name/app_domain);缺省时不渲染徽标。
  const appName = metadata.app_name;
  const appDomain = metadata.app_domain;
  // 请求 ID:新日志落独立列 item.request_id,旧日志仅存于 metadata,取列值优先。
  // upstream_request_id 由后端按角色裁剪(self 列表 Omit),前端有值即展示。
  const requestId = item.request_id || metadata.request_id;
  // finish_reason 置顶展示:旧日志无该字段时 meta 为 null,不渲染 badge(容错)。
  // native 原文仅管理员可见(平台管理员或站点管理员),放进 tooltip。
  const finishMeta = finishReasonMeta(metadata.finish_reason);
  const nativeFinishReason = metadata.native_finish_reason;
  const showNativeFinish = Boolean(userIsAdmin || platformIsAdmin);
  const groupRatio = metadata.group_ratio || 1;
  const originalQuota = calculateOriginalQuota(item);
  const quota = item.quota || 0;

  const cachedWriteTokens = Number(metadata.cached_write_tokens) || 0;
  const cachedWrite1hTokens = Number(metadata.cached_write_1h_tokens) || 0;
  const cachedReadTokens = Number(metadata.cached_read_tokens) || 0;
  const openaiCacheWriteTokens = Number(metadata.openai_cache_write_tokens) || 0;

  // Modality/extra token breakdown for the Usage section (audio/image/reasoning); only
  // rows with a positive count from metadata are shown.
  const modalityTokenRows = [
    { key: 'input_text_tokens', labelKey: 'logPage.usage.inputText' },
    { key: 'output_text_tokens', labelKey: 'logPage.usage.outputText' },
    { key: 'input_audio_tokens', labelKey: 'logPage.usage.inputAudio' },
    { key: 'output_audio_tokens', labelKey: 'logPage.usage.outputAudio' },
    { key: 'input_image_tokens', labelKey: 'logPage.usage.inputImage' },
    { key: 'output_image_tokens', labelKey: 'logPage.usage.outputImage' },
    { key: 'reasoning_tokens', labelKey: 'logPage.usage.reasoning' }
  ].filter((r) => Number(metadata[r.key]) > 0);

  const inputPrice = metadata.input_price || (metadata.input_ratio ? `$${calculatePrice(metadata.input_ratio, groupRatio, false)}` : '$0');
  const outputPrice =
    metadata.output_price || (metadata.output_ratio ? `$${calculatePrice(metadata.output_ratio, groupRatio, false)}` : '$0');

  const groupName = metadata.is_backup_group
    ? `${userGroup?.[metadata.group_name]?.name || metadata.group_name} → ${userGroup?.[metadata.backup_group_name]?.name || metadata.backup_group_name}`
    : userGroup?.[metadata.group_name]?.name || metadata.group_name;

  const ioStatusKey = computeIoStatus(io, siteLogIOEnabled);

  const { requestTime, requestTs } = deriveDuration(item);
  const hasDuration = Number(item.request_time) > 0;
  const costValue = item.type === 2 ? renderQuota(quota, 6) : null;
  const hasTokens = totalInputTokens > 0 || totalOutputTokens > 0;
  const tokensValue = hasTokens ? `${totalInputTokens} → ${totalOutputTokens}` : null;
  const hasMetrics = costValue || tokensValue || hasDuration;
  const hasDetail = io.status === 'ok' && io.data.has_detail;
  // Message count for the Prompt entry row; 0 when the body can't be parsed (viewer falls back to raw).
  const promptCount = hasDetail ? parsePromptMessages(io.data.request_body).messages?.length || 0 : 0;

  return createPortal(
    <div className="fixed inset-0 z-[1200]">
      <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={onClose} aria-hidden="true" />
      <div
        role="dialog"
        aria-modal="true"
        className="absolute inset-y-0 right-0 flex h-full w-full flex-col border-l border-border bg-card shadow-xl sm:w-[48rem] sm:min-w-[24rem] sm:max-w-[calc(100vw-3rem)]"
      >
        <div className="flex shrink-0 items-start justify-between gap-3 border-b border-border px-4 py-3">
          <div className="min-w-0">
            <h2 className="flex min-w-0 items-center gap-2 text-lg font-semibold">
              {item.model_name ? (
                <>
                  <ModelIcon model={item.model_name} className="size-5 shrink-0" />
                  <span className="truncate">{item.model_name}</span>
                </>
              ) : (
                <span className="truncate">{t('logPage.detailLabel')}</span>
              )}
            </h2>
            {(item.channel_id || finishMeta || appName || item.is_stream) && (
              <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                {item.channel_id ? (
                  <span className={badgeClass('default')}>
                    <ProviderIcon type={item.channel?.type} name={item.channel?.name} baseUrl={item.channel?.base_url} className="mr-1" />
                    {`${t('logPage.channelLabel')} ${item.channel_id}`}
                  </span>
                ) : null}
                {appName && (
                  <span className={badgeClass('default')}>
                    <AppFavicon name={appName} domain={appDomain} className="mr-1 size-3.5" />
                    {appName}
                  </span>
                )}
                {finishMeta &&
                  (showNativeFinish && nativeFinishReason ? (
                    <TooltipProvider delayDuration={150}>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span className={cn(badgeClass(finishMeta.color), 'cursor-help')}>{t(finishMeta.labelKey)}</span>
                        </TooltipTrigger>
                        <TooltipContent className="text-xs">
                          {t('logPage.finishReason.nativeTooltip', { reason: nativeFinishReason })}
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  ) : (
                    <span className={badgeClass(finishMeta.color)}>{t(finishMeta.labelKey)}</span>
                  ))}
                {item.is_stream ? <span className={badgeClass('info')}>{t('logPage.streamLabel')}</span> : null}
              </div>
            )}
          </div>
          <Button variant="ghost" size="icon" aria-label="Close" onClick={onClose} className="shrink-0">
            <X className="size-5" />
          </Button>
        </div>

        <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-4">
          {/* Top group (OpenRouter flat layout): metric cards + overview + request */}
          <div className="flex shrink-0 flex-col gap-6">
            {/* 6 Metric Cards (2×3 grid) */}
            {hasMetrics && (
              <section className="grid grid-cols-2 gap-2 sm:grid-cols-3">
                <MetricCard
                  icon={Clock}
                  label={t('logPage.metrics.providerLatency')}
                  value={hasDuration ? requestTime.toFixed(2) : null}
                  unit="s"
                  placeholder="—"
                />
                <MetricCard
                  icon={Zap}
                  label={t('logPage.metrics.throughput')}
                  value={requestTs > 0 ? requestTs.toFixed(2) : null}
                  unit="tok/s"
                  placeholder="—"
                />
                <MetricCard icon={CircleDollarSign} label={t('logPage.metrics.cost')} value={costValue} placeholder="—" />
                <MetricCard icon={Hash} label={t('logPage.metrics.tokens')} value={tokensValue} placeholder="—" />
                <MetricCard icon={GitBranch} label={t('logPage.metrics.fallbacks')} value={null} placeholder="—" />
                <MetricCard icon={Timer} label={t('logPage.metrics.fallbackLatency')} value={null} placeholder="—" />
              </section>
            )}

            {/* Overview Section */}
            <section>
              <h3 className="mb-3 text-sm font-semibold text-foreground">{t('logPage.overviewLabel')}</h3>
              <div className="space-y-3">
                <KeyValueRow label={t('logPage.modelLabel')} value={item.model_name} />
                <KeyValueRow label={t('logPage.groupLabel')} value={groupName} />
                <KeyValueRow label={t('logPage.userLabel')} value={item.username} />
                <KeyValueRow label={t('logPage.sourceIp')} value={item.source_ip} />
                <KeyValueRow label={t('logPage.timeLabel')} value={timestamp2string(item.created_at)} />
              </div>
            </section>

            {/* Request Section */}
            <section className="border-t border-border/60 pt-5">
              <h3 className="mb-3 text-sm font-semibold text-foreground">{t('logPage.requestSection.title')}</h3>
              <div className="space-y-3">
                {appName && <KeyValueRow label={t('logPage.requestSection.app')} value={appName} />}
                <KeyValueRow label={t('logPage.requestSection.token')} value={item.token_name} />
                {requestId && (
                  <KeyValueRow
                    label={t('logPage.requestSection.requestId')}
                    value={requestId}
                    mono
                    copyable
                    onCopy={() => handleCopy(requestId)}
                    t={t}
                  />
                )}
                {item.upstream_request_id && (
                  <KeyValueRow
                    label={t('logPage.requestSection.upstreamRequestId')}
                    value={item.upstream_request_id}
                    mono
                    copyable
                    onCopy={() => handleCopy(item.upstream_request_id)}
                    t={t}
                  />
                )}
                {finishMeta && <KeyValueRow label={t('logPage.requestSection.finishReason')} value={t(finishMeta.labelKey)} />}
                <KeyValueRow
                  label={t('logPage.requestSection.streaming')}
                  value={item.is_stream ? t('logPage.requestSection.yes') : t('logPage.requestSection.no')}
                />
              </div>
            </section>
          </div>

          {/* Provider Responses Chart */}
          <ProviderResponsesChart item={item} t={t} />

          {/* Usage Section */}
          {item.type === 2 && (
            <CollapsibleSection
              title={t('logPage.usage.title')}
              summary={t('logPage.usage.summary', { cost: renderQuota(quota, 6), tokens: totalInputTokens + totalOutputTokens })}
              open={showUsage}
              onToggle={() => setShowUsage((v) => !v)}
            >
              <div className="space-y-3">
                <div className="space-y-1">
                  <KeyValueRow label={t('logPage.usage.inputTokens')} value={totalInputTokens} />
                  <KeyValueRow label={t('logPage.usage.outputTokens')} value={totalOutputTokens} />
                  <KeyValueRow label={t('logPage.usage.totalTokens')} value={totalInputTokens + totalOutputTokens} />
                </div>
                {(cachedWriteTokens > 0 || cachedWrite1hTokens > 0 || cachedReadTokens > 0 || openaiCacheWriteTokens > 0) && (
                  <div className="space-y-1">
                    <div className="text-xs font-semibold text-muted-foreground">{t('logPage.usage.cachedTokensSection')}</div>
                    {cachedWriteTokens > 0 && <KeyValueRow label={t('logPage.usage.cachedWrite5m')} value={cachedWriteTokens} />}
                    {cachedWrite1hTokens > 0 && <KeyValueRow label={t('logPage.usage.cachedWrite1h')} value={cachedWrite1hTokens} />}
                    {cachedReadTokens > 0 && <KeyValueRow label={t('logPage.usage.cachedRead')} value={cachedReadTokens} />}
                    {openaiCacheWriteTokens > 0 && <KeyValueRow label={t('logPage.usage.openaiCacheWrite')} value={openaiCacheWriteTokens} />}
                  </div>
                )}
                {modalityTokenRows.length > 0 && (
                  <div className="space-y-1">
                    <div className="text-xs font-semibold text-muted-foreground">{t('logPage.usage.modalityTokensSection')}</div>
                    {modalityTokenRows.map((r) => (
                      <KeyValueRow key={r.key} label={t(r.labelKey)} value={Number(metadata[r.key])} />
                    ))}
                  </div>
                )}
                <div className="space-y-1">
                  <KeyValueRow label={t('logPage.quotaDetail.input')} value={`${inputPrice} /M`} />
                  <KeyValueRow label={t('logPage.quotaDetail.output')} value={`${outputPrice} /M`} />
                  <KeyValueRow label={t('logPage.quotaDetail.groupRatioValue')} value={groupRatio} />
                  <KeyValueRow label={t('logPage.quotaDetail.originalBilling')} value={renderQuota(originalQuota, 6)} />
                  <KeyValueRow label={t('logPage.usage.cost')} value={renderQuota(quota, 6)} />
                </div>
              </div>
            </CollapsibleSection>
          )}

          {/* Prompt Section — opens the full-screen message browser */}
          {hasDetail && (
            <button
              type="button"
              onClick={() => setShowPromptViewer(true)}
              className="group flex w-full shrink-0 items-center justify-between gap-2 rounded-lg border border-border px-3 py-2.5 text-left transition-colors hover:bg-muted/50"
            >
              <div className="flex min-w-0 items-center gap-2">
                <Expand className="size-4 shrink-0 text-muted-foreground transition-colors group-hover:text-foreground" />
                <span className="truncate text-sm font-semibold text-foreground">{t('logPage.prompt.title')}</span>
                <span className="shrink-0 text-sm text-muted-foreground">
                  {t('logPage.prompt.summary', { tokens: item.prompt_tokens || 0, count: promptCount })}
                </span>
              </div>
            </button>
          )}

          {/* Completion Section */}
          {hasDetail && (
            <CollapsibleSection
              title={t('logPage.completion.title')}
              summary={t('logPage.completion.summary', { tokens: item.completion_tokens || 0 })}
              open={showCompletion}
              onToggle={() => setShowCompletion((v) => !v)}
            >
              <CompletionBody io={io} isStream={item.is_stream} t={t} />
            </CollapsibleSection>
          )}

          {/* Generation Data Section */}
          <CollapsibleSection
            title={t('logPage.generationData.title')}
            summary={t('logPage.generationData.summary')}
            open={showGenerationData}
            onToggle={() => setShowGenerationData((v) => !v)}
          >
            <CodeBlock language="json" wrap showCopy code={JSON.stringify({ log: item, metadata: item.metadata }, null, 2)} />
          </CollapsibleSection>

          {/* IO Status Section (for non-detail or error states) */}
          {!hasDetail && (
            <section className="flex shrink-0 flex-col gap-2">
              <div className="flex items-center gap-2">
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{t('logPage.io.title')}</h3>
                <IoStatusBadge statusKey={ioStatusKey} t={t} />
              </div>

              {io.status === 'loading' && (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="size-4 animate-spin" />
                  {t('logPage.io.loading')}
                </div>
              )}
              {io.status === 'forbidden' && <p className="text-sm text-muted-foreground">{t('logPage.io.forbidden')}</p>}
              {io.status === 'error' && <p className="text-sm text-muted-foreground">{t('logPage.io.error')}</p>}
              {io.status === 'ok' &&
                !io.data.has_detail &&
                (io.data.reason === 'expired' ? (
                  <p className="text-sm text-muted-foreground">{t('logPage.io.emptyExpired')}</p>
                ) : (
                  <NotEnabledEmpty
                    t={t}
                    siteLogIOEnabled={siteLogIOEnabled}
                    platformIsAdmin={platformIsAdmin}
                    isOrgContext={isOrgContext}
                    orgIsAdmin={orgIsAdmin}
                    orgId={currentOrgId}
                    onNavigate={handleGuideNavigate}
                  />
                ))}
            </section>
          )}
        </div>
      </div>

      {showPromptViewer && hasDetail && (
        <PromptViewer
          requestBody={io.data.request_body}
          truncated={Boolean(io.data.request_truncated)}
          promptTokens={item.prompt_tokens || 0}
          cachedTokens={cachedReadTokens}
          cost={costValue}
          model={item.model_name}
          onCopy={handleCopy}
          onClose={() => setShowPromptViewer(false)}
          t={t}
        />
      )}
    </div>,
    document.body
  );
}

LogDetailDialog.propTypes = {
  item: PropTypes.object,
  userGroup: PropTypes.object,
  userIsAdmin: PropTypes.bool,
  siteLogIOEnabled: PropTypes.bool,
  platformIsAdmin: PropTypes.bool,
  isOrgContext: PropTypes.bool,
  orgIsAdmin: PropTypes.bool,
  t: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired
};
