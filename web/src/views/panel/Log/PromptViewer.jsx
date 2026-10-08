import PropTypes from 'prop-types';
import { Fragment, useEffect, useMemo, useRef, useState } from 'react';
import { ChevronLeft, ChevronRight, Code2, Copy, Image as ImageIcon, Search, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import CodeBlock from '@/components/ui/code-block';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';

// ==============================|| LOG — PROMPT MESSAGE BROWSER (FULLSCREEN) ||========== //
// Full-screen layer over LogDetailDialog reproducing OpenRouter's Prompt view: a
// per-message token-distribution bar + by-role stats on top, a searchable/role-filtered
// message list on the left, and a reading pane on the right that decomposes multimodal
// content and tool calls. Per-message tokens are not stored server-side, so the bar and
// by-role shares are estimated from each message's serialized character weight.
// Invalid / truncated request bodies fall back to a full raw JSON view with a badge.

const IO_CHUNK = 16 * 1024;

// Reading-pane split (md+ only): persisted left-column width fraction of the list + reading
// area. Default ~1/3 left / ~2/3 right (OpenRouter's Prompt view); clamped so neither side
// collapses. Below md the layout stacks vertically and the splitter is not rendered.
const SPLIT_STORAGE_KEY = 'promptViewer.splitRatio';
const DEFAULT_SPLIT_RATIO = 1 / 3;
const MIN_SPLIT_RATIO = 0.2;

// Known chat roles, ordered for the by-role table + legend. Any other role buckets to
// `other` (default color).
const ROLE_ORDER = ['system', 'user', 'assistant', 'tool'];

// role → dot / soft-badge classes; kept in one place so list badges, legend and the
// distribution bar stay color-consistent across light/dark.
const ROLE_STYLE = {
  system: { dot: 'bg-slate-500', badge: 'bg-slate-500/10 text-slate-600 dark:text-slate-400', text: 'text-slate-600 dark:text-slate-400' },
  user: { dot: 'bg-blue-500', badge: 'bg-blue-500/10 text-blue-600 dark:text-blue-400', text: 'text-blue-600 dark:text-blue-400' },
  assistant: { dot: 'bg-emerald-500', badge: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400', text: 'text-emerald-600 dark:text-emerald-400' },
  tool: { dot: 'bg-amber-500', badge: 'bg-amber-500/10 text-amber-600 dark:text-amber-400', text: 'text-amber-600 dark:text-amber-400' },
  other: { dot: 'bg-muted-foreground', badge: 'bg-muted text-muted-foreground', text: 'text-muted-foreground' }
};

const roleKey = (role) => (ROLE_ORDER.includes(role) ? role : 'other');
const roleStyle = (role) => ROLE_STYLE[roleKey(role)];
const roleLabel = (role, t) => t(`logPage.promptViewer.roles.${roleKey(role)}`);

// Pretty-print JSON when possible; otherwise return the value as-is.
function prettyJson(value) {
  if (typeof value !== 'string') {
    try {
      return JSON.stringify(value, null, 2);
    } catch {
      return String(value ?? '');
    }
  }
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

// Flatten a message `content` (string or multimodal array) to plain text for previews.
function extractText(content) {
  if (typeof content === 'string') return content;
  if (Array.isArray(content)) {
    return content
      .map((part) => (typeof part === 'string' ? part : part?.text || ''))
      .filter(Boolean)
      .join(' ');
  }
  return '';
}

// tool_calls[].function.name list (assistant tool-call preview summary).
function toolCallNames(msg) {
  if (!Array.isArray(msg?.tool_calls)) return [];
  return msg.tool_calls.map((tc) => tc?.function?.name || tc?.name || '').filter(Boolean);
}

// One-line preview for the message list. Assistant tool-call messages summarize as
// "N tool call: name"; otherwise the flattened content with whitespace collapsed.
function messagePreview(msg, t) {
  const names = toolCallNames(msg);
  if (names.length > 0) {
    return t('logPage.promptViewer.toolCallSummary', { count: names.length, names: names.join(', ') });
  }
  const text = extractText(msg?.content).replace(/\s+/g, ' ').trim();
  return text || t('logPage.promptViewer.emptyContent');
}

// Parse request_body → messages[]. Returns { messages, parsed }; messages is null when the
// body is missing / not JSON / has no messages[] array (callers then fall back to raw).
export function parsePromptMessages(rawBody) {
  if (!rawBody) return { messages: null, parsed: null };
  let parsed;
  try {
    parsed = JSON.parse(rawBody);
  } catch {
    return { messages: null, parsed: null };
  }
  const messages = parsed?.messages;
  if (!Array.isArray(messages) || messages.length === 0) return { messages: null, parsed };
  return { messages, parsed };
}

// Small role text tag (colored label, no filled block) for the message list, reading-pane
// header and by-role table — matches OpenRouter's compact role labels.
function RoleTag({ role, t }) {
  return <span className={cn('truncate text-[11px] font-semibold', roleStyle(role).text)}>{roleLabel(role, t)}</span>;
}

RoleTag.propTypes = { role: PropTypes.string, t: PropTypes.func.isRequired };

// Horizontal token-distribution bar (OpenRouter style): one width-proportional segment per
// message colored by role, over an overview brush track. Dragging the window handles (or
// box-selecting on the track) zooms the main bar to a message range and filters the list;
// clicking a segment selects that message. Per-message tokens are estimated from char weight.
function DistributionBar({ full, visible, totalWeight, selectedIdx, onSelect, range, onRange, onReset, t }) {
  const trackRef = useRef(null);
  const dragRef = useRef(null);
  const win = range || { start: 0, end: 1 };

  const fracAt = (clientX) => {
    const el = trackRef.current;
    if (!el) return 0;
    const rect = el.getBoundingClientRect();
    return Math.min(1, Math.max(0, (clientX - rect.left) / rect.width));
  };
  const apply = (a, b) => {
    if (b - a >= 0.999) onReset();
    else onRange({ start: Math.max(0, a), end: Math.min(1, b) });
  };
  const beginDrag = (mode, e, extra) => {
    e.stopPropagation();
    trackRef.current?.setPointerCapture?.(e.pointerId);
    dragRef.current = { mode, ...extra };
  };
  const onTrackDown = (e) => {
    trackRef.current?.setPointerCapture?.(e.pointerId);
    dragRef.current = { mode: 'new', anchor: fracAt(e.clientX) };
  };
  const onMove = (e) => {
    const d = dragRef.current;
    if (!d) return;
    const f = fracAt(e.clientX);
    if (d.mode === 'new') apply(Math.min(d.anchor, f), Math.max(d.anchor, f));
    else if (d.mode === 'left') apply(Math.min(f, win.end - 0.01), win.end);
    else if (d.mode === 'right') apply(win.start, Math.max(f, win.start + 0.01));
    else if (d.mode === 'move') {
      const a = Math.min(Math.max(f - d.grab, 0), 1 - d.width);
      apply(a, a + d.width);
    }
  };
  const onUp = () => {
    dragRef.current = null;
  };
  // pointercancel (e.g. gesture interrupted / touch cancelled) must also clear the drag.
  const onCancel = () => {
    dragRef.current = null;
  };

  return (
    <div className="select-none">
      <div className="flex h-11 w-full overflow-hidden rounded-md bg-muted/40">
        {visible.map((s) => (
          <button
            key={s.index}
            type="button"
            onClick={() => onSelect(s.index)}
            title={t('logPage.promptViewer.barSegmentTooltip', {
              index: s.index + 1,
              role: roleLabel(s.role, t),
              share: ((s.weight / totalWeight) * 100).toFixed(1)
            })}
            style={{ flexGrow: s.weight, flexBasis: 0 }}
            className={cn(
              'h-full min-w-0 transition-opacity hover:opacity-80',
              roleStyle(s.role).dot,
              s.index === selectedIdx && 'ring-2 ring-inset ring-foreground'
            )}
          />
        ))}
      </div>
      <div
        ref={trackRef}
        onPointerDown={onTrackDown}
        onPointerMove={onMove}
        onPointerUp={onUp}
        onPointerCancel={onCancel}
        className="relative mt-1.5 h-4 w-full cursor-crosshair overflow-hidden rounded bg-muted/30"
      >
        <div className="absolute inset-0 flex opacity-50">
          {full.map((s) => (
            <span key={s.index} style={{ flexGrow: s.weight, flexBasis: 0 }} className={cn('h-full', roleStyle(s.role).dot)} />
          ))}
        </div>
        <div className="absolute inset-y-0 left-0 bg-background/70" style={{ width: `${win.start * 100}%` }} />
        <div className="absolute inset-y-0 right-0 bg-background/70" style={{ width: `${(1 - win.end) * 100}%` }} />
        <div
          onPointerDown={(e) => beginDrag('move', e, { width: win.end - win.start, grab: fracAt(e.clientX) - win.start })}
          className="absolute inset-y-0 cursor-grab border-x-2 border-foreground/70 bg-foreground/5"
          style={{ left: `${win.start * 100}%`, width: `${(win.end - win.start) * 100}%` }}
        />
        <div
          onPointerDown={(e) => beginDrag('left', e)}
          className="absolute inset-y-0 w-2 -translate-x-1/2 cursor-ew-resize"
          style={{ left: `${win.start * 100}%` }}
        />
        <div
          onPointerDown={(e) => beginDrag('right', e)}
          className="absolute inset-y-0 w-2 -translate-x-1/2 cursor-ew-resize"
          style={{ left: `${win.end * 100}%` }}
        />
      </div>
    </div>
  );
}

DistributionBar.propTypes = {
  full: PropTypes.array.isRequired,
  visible: PropTypes.array.isRequired,
  totalWeight: PropTypes.number.isRequired,
  selectedIdx: PropTypes.number,
  onSelect: PropTypes.func.isRequired,
  range: PropTypes.object,
  onRange: PropTypes.func.isRequired,
  onReset: PropTypes.func.isRequired,
  t: PropTypes.func.isRequired
};

// Small role chip (colored dot + label) reused by the list, legend and by-role table.
function RoleChip({ role, t }) {
  return (
    <span className={cn('inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium', roleStyle(role).badge)}>
      <span className={cn('size-1.5 rounded-full', roleStyle(role).dot)} />
      {roleLabel(role, t)}
    </span>
  );
}

RoleChip.propTypes = { role: PropTypes.string, t: PropTypes.func.isRequired };

// Top-right stats: Tokens (X of Y, brush-aware), Prompt / Cached breakdown, Cost, Model, and a
// by-role table (estimated char share + message count). Per-message tokens aren't stored, so
// shares are estimated from serialized char weight; Prompt/Cached shares split promptTokens.
function StatsPanel({ promptTokens, cachedTokens, visibleEst, cost, model, roleAggList, roleAgg, total, onCopy, t }) {
  const pt = promptTokens || 0;
  const cached = cachedTokens || 0;
  const promptFresh = Math.max(pt - cached, 0);
  const freshPct = pt > 0 ? ((promptFresh / pt) * 100).toFixed(1) : '0.0';
  const cachedPct = pt > 0 ? ((cached / pt) * 100).toFixed(1) : '0.0';
  return (
    <div className="grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-3 lg:w-80 lg:shrink-0 lg:grid-cols-1">
      <div className="flex flex-col">
        <span className="text-[11px] uppercase tracking-wide text-muted-foreground">{t('logPage.promptViewer.tokensTotal')}</span>
        <span className="text-sm font-semibold tabular-nums text-foreground">
          {t('logPage.promptViewer.tokensOfTotal', { shown: (visibleEst || 0).toLocaleString(), total: pt.toLocaleString() })}
        </span>
      </div>
      <div className="flex flex-col">
        <span className="text-[11px] uppercase tracking-wide text-muted-foreground">{t('logPage.promptViewer.prompt')}</span>
        <span className="text-sm font-semibold tabular-nums text-foreground">
          {promptFresh.toLocaleString()}
          <span className="ml-1 text-xs font-normal text-muted-foreground">{freshPct}%</span>
        </span>
      </div>
      <div className="flex flex-col">
        <span className="text-[11px] uppercase tracking-wide text-muted-foreground">{t('logPage.promptViewer.cached')}</span>
        <span className="text-sm font-semibold tabular-nums text-foreground">
          {cached.toLocaleString()}
          <span className="ml-1 text-xs font-normal text-muted-foreground">{cachedPct}%</span>
        </span>
      </div>
      {cost && (
        <div className="flex flex-col">
          <span className="text-[11px] uppercase tracking-wide text-muted-foreground">{t('logPage.promptViewer.cost')}</span>
          <span className="text-sm font-semibold text-foreground">{cost}</span>
        </div>
      )}
      {model && (
        <div className="flex flex-col">
          <span className="text-[11px] uppercase tracking-wide text-muted-foreground">{t('logPage.promptViewer.model')}</span>
          <span className="flex items-center gap-1">
            <span className="truncate font-mono text-sm font-medium text-foreground" title={model}>
              {model}
            </span>
            <Button
              variant="ghost"
              size="icon"
              className="size-6 shrink-0"
              aria-label={t('logPage.promptViewer.copy')}
              onClick={() => onCopy(model)}
            >
              <Copy className="size-3" />
            </Button>
          </span>
        </div>
      )}
      <div className="col-span-2 sm:col-span-3 lg:col-span-1">
        <div className="mb-1 grid grid-cols-[1fr_3rem_2rem] gap-x-2 text-[11px] uppercase tracking-wide text-muted-foreground">
          <span>{t('logPage.promptViewer.byRole')}</span>
          <span className="text-right">%</span>
          <span className="text-right">#</span>
        </div>
        <div className="grid grid-cols-[1fr_3rem_2rem] items-center gap-x-2 gap-y-1">
          {roleAggList.map((k) => (
            <Fragment key={k}>
              <RoleTag role={k} t={t} />
              <span className="text-right text-xs tabular-nums text-muted-foreground">
                {((roleAgg[k].weight / total) * 100).toFixed(1)}%
              </span>
              <span className="text-right text-xs tabular-nums text-muted-foreground">{roleAgg[k].count}</span>
            </Fragment>
          ))}
        </div>
      </div>
    </div>
  );
}

StatsPanel.propTypes = {
  promptTokens: PropTypes.number,
  cachedTokens: PropTypes.number,
  visibleEst: PropTypes.number,
  cost: PropTypes.string,
  model: PropTypes.string,
  roleAggList: PropTypes.array.isRequired,
  roleAgg: PropTypes.object.isRequired,
  total: PropTypes.number.isRequired,
  onCopy: PropTypes.func.isRequired,
  t: PropTypes.func.isRequired
};

// Decompose a single message into ordered blocks: text / image placeholder / other-json,
// followed by any tool calls (name + arguments). Empty messages show a muted note.
function MessageContent({ msg, t }) {
  const blocks = [];
  const content = msg?.content;
  if (typeof content === 'string') {
    if (content) blocks.push({ kind: 'text', text: content });
  } else if (Array.isArray(content)) {
    content.forEach((part, i) => {
      if (typeof part === 'string') {
        if (part) blocks.push({ kind: 'text', text: part, key: `s${i}` });
        return;
      }
      const type = part?.type;
      if (type === 'image_url' || part?.image_url) {
        const url = typeof part.image_url === 'string' ? part.image_url : part.image_url?.url || '';
        blocks.push({ kind: 'image', url, key: `i${i}` });
      } else if (type === 'text' || typeof part?.text === 'string') {
        if (part.text) blocks.push({ kind: 'text', text: part.text, key: `t${i}` });
      } else {
        blocks.push({ kind: 'json', value: part, key: `j${i}` });
      }
    });
  }
  const tools = Array.isArray(msg?.tool_calls) ? msg.tool_calls : [];
  const isEmpty = blocks.length === 0 && tools.length === 0;

  return (
    <div className="space-y-3">
      {blocks.map((b, i) =>
        b.kind === 'image' ? (
          <div key={b.key || i} className="flex items-start gap-2 rounded-lg border border-dashed border-border bg-muted/30 p-3">
            <ImageIcon className="size-4 shrink-0 text-muted-foreground" />
            <div className="min-w-0">
              <div className="text-xs font-medium text-foreground">{t('logPage.promptViewer.imageBlock')}</div>
              {b.url && <div className="mt-0.5 break-all text-xs text-muted-foreground">{b.url}</div>}
            </div>
          </div>
        ) : b.kind === 'json' ? (
          <CodeBlock key={b.key || i} language="json" wrap showCopy={false} maxHeightClass="max-h-[40vh]" code={prettyJson(b.value)} />
        ) : (
          <div key={b.key || i} className="whitespace-pre-wrap break-words text-sm leading-relaxed text-foreground">
            {b.text}
          </div>
        )
      )}
      {tools.length > 0 && (
        <div className="space-y-2">
          <div className="text-xs font-semibold text-muted-foreground">{t('logPage.promptViewer.toolCalls')}</div>
          {tools.map((tc, i) => {
            const fn = tc.function || tc;
            const args = fn.arguments;
            const argsStr = typeof args === 'string' ? prettyJson(args) : args == null ? '' : JSON.stringify(args, null, 2);
            return (
              <div key={tc.id || i} className="overflow-hidden rounded-lg border border-border">
                <div className="border-b border-border bg-muted/40 px-2 py-1 font-mono text-xs font-medium text-foreground">
                  {fn.name || ''}
                </div>
                <CodeBlock language="json" wrap showCopy={false} maxHeightClass="max-h-[40vh]" code={argsStr} />
              </div>
            );
          })}
        </div>
      )}
      {isEmpty && <div className="text-sm text-muted-foreground">{t('logPage.promptViewer.emptyContent')}</div>}
    </div>
  );
}

MessageContent.propTypes = { msg: PropTypes.object, t: PropTypes.func.isRequired };

// Full-screen prompt message browser. Rendered by LogDetailDialog on top of the detail
// panel; Esc handling / open state lives in the parent so Esc returns to the panel.
export default function PromptViewer({ requestBody, truncated, promptTokens, cachedTokens, cost, model, onCopy, onClose, t }) {
  const { messages, parsed } = useMemo(() => parsePromptMessages(requestBody), [requestBody]);
  const rows = useMemo(
    () =>
      (messages || []).map((msg, index) => ({
        index,
        role: msg?.role || 'other',
        weight: Math.max((JSON.stringify(msg) || '').length, 1),
        msg,
        preview: messagePreview(msg, t)
      })),
    [messages, t]
  );
  const totalWeight = useMemo(() => rows.reduce((s, r) => s + r.weight, 0) || 1, [rows]);
  const { roleAgg, roleAggList } = useMemo(() => {
    const agg = {};
    rows.forEach((r) => {
      const k = roleKey(r.role);
      agg[k] = agg[k] || { count: 0, weight: 0 };
      agg[k].count += 1;
      agg[k].weight += r.weight;
    });
    return { roleAgg: agg, roleAggList: [...ROLE_ORDER, 'other'].filter((k) => agg[k]) };
  }, [rows]);
  // Cumulative char-weight fraction [start, end] per message index; the distribution bar and
  // its brush window operate in this 0..1 space so the window maps back to a message range.
  const cumFrac = useMemo(() => {
    const cum = new Array(rows.length);
    let acc = 0;
    rows.forEach((r) => {
      const start = acc / totalWeight;
      acc += r.weight;
      cum[r.index] = { start, end: acc / totalWeight };
    });
    return cum;
  }, [rows, totalWeight]);

  const [search, setSearch] = useState('');
  const [roleFilter, setRoleFilter] = useState('all');
  const [selectedIdx, setSelectedIdx] = useState(0);
  const [showRequest, setShowRequest] = useState(false);
  const [rawMsg, setRawMsg] = useState(false);
  // Brush window over the distribution bar as { start, end } fractions; null = full range.
  const [range, setRange] = useState(null);

  // Resizable split between the message list (left) and reading pane (right), md+ only.
  // splitRatio is the left column's width fraction; persisted to localStorage, double-click resets.
  const splitAreaRef = useRef(null);
  const splitDragRef = useRef(false);
  const [isWide, setIsWide] = useState(() => typeof window !== 'undefined' && window.matchMedia('(min-width: 768px)').matches);
  const [splitRatio, setSplitRatio] = useState(() => {
    try {
      const raw = parseFloat(window.localStorage.getItem(SPLIT_STORAGE_KEY));
      if (Number.isFinite(raw)) return Math.min(1 - MIN_SPLIT_RATIO, Math.max(MIN_SPLIT_RATIO, raw));
    } catch {
      /* ignore */
    }
    return DEFAULT_SPLIT_RATIO;
  });

  useEffect(() => {
    const mq = window.matchMedia('(min-width: 768px)');
    const onChange = (e) => setIsWide(e.matches);
    setIsWide(mq.matches);
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, []);

  useEffect(() => {
    try {
      window.localStorage.setItem(SPLIT_STORAGE_KEY, String(splitRatio));
    } catch {
      /* ignore */
    }
  }, [splitRatio]);

  const clampRatio = (v) => Math.min(1 - MIN_SPLIT_RATIO, Math.max(MIN_SPLIT_RATIO, v));
  const onSplitDown = (e) => {
    e.preventDefault();
    e.currentTarget.setPointerCapture?.(e.pointerId);
    splitDragRef.current = true;
  };
  const onSplitMove = (e) => {
    if (!splitDragRef.current) return;
    const el = splitAreaRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    if (rect.width <= 0) return;
    setSplitRatio(clampRatio((e.clientX - rect.left) / rect.width));
  };
  const onSplitUp = () => {
    splitDragRef.current = false;
  };
  const resetSplit = () => setSplitRatio(DEFAULT_SPLIT_RATIO);

  useEffect(() => setRawMsg(false), [selectedIdx]);

  // Messages whose weight span overlaps the brush window (zoomed set for the top bar + list).
  const visible = useMemo(
    () =>
      rows.filter((r) => {
        if (!range) return true;
        const c = cumFrac[r.index];
        return c && c.end > range.start && c.start < range.end;
      }),
    [rows, range, cumFrac]
  );
  const visibleSet = useMemo(() => new Set(visible.map((r) => r.index)), [visible]);
  // Estimated tokens inside the brush window: promptTokens split by visible char-weight share.
  const visibleEst = useMemo(() => {
    const w = visible.reduce((s, r) => s + r.weight, 0);
    return Math.round((promptTokens || 0) * (w / totalWeight));
  }, [visible, promptTokens, totalWeight]);

  const q = search.trim().toLowerCase();
  const filtered = rows.filter((r) => {
    if (!visibleSet.has(r.index)) return false;
    if (roleFilter !== 'all' && roleKey(r.role) !== roleFilter) return false;
    if (q) return r.preview.toLowerCase().includes(q);
    return true;
  });
  const activePos = filtered.findIndex((r) => r.index === selectedIdx);
  const realPos = activePos >= 0 ? activePos : 0;
  const activeRow = filtered[realPos];

  const selectFromBar = (idx) => {
    setSearch('');
    setRoleFilter('all');
    setSelectedIdx(idx);
  };
  const goPrev = () => realPos > 0 && setSelectedIdx(filtered[realPos - 1].index);
  const goNext = () => realPos < filtered.length - 1 && setSelectedIdx(filtered[realPos + 1].index);
  const copyActive = () => {
    if (!activeRow) return;
    const text = extractText(activeRow.msg?.content);
    onCopy(text || JSON.stringify(activeRow.msg, null, 2));
  };

  const fallback = !messages;
  const roleOptions = ['all', ...roleAggList];

  return (
    // Modal over LogDetailDialog: backdrop + a window inset a fixed gap from every edge, so it
    // scales with the browser window while the gap stays constant. Esc handling lives in the parent.
    <div className="fixed inset-0 z-[1300]">
      <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={onClose} aria-hidden="true" />
      <div
        role="dialog"
        aria-modal="true"
        className="absolute inset-4 flex flex-col overflow-hidden rounded-lg border border-border bg-background shadow-2xl sm:inset-8"
      >
        {/* Header */}
        <div className="flex shrink-0 items-center justify-between gap-3 border-b border-border px-5 py-3">
          <div className="min-w-0">
            <h2 className="flex flex-wrap items-center gap-x-2 text-base font-semibold text-foreground">
              <span>{t('logPage.promptViewer.title')}</span>
              {truncated && (
                <span className="rounded bg-amber-500/15 px-1.5 py-0.5 text-[11px] font-medium text-amber-600 dark:text-amber-400">
                  {t('logPage.promptViewer.truncated')}
                </span>
              )}
              <span className="text-sm font-normal text-muted-foreground">
                · {t('logPage.promptViewer.headerSummary', { count: messages ? messages.length : 0, tokens: promptTokens })}
              </span>
            </h2>
          </div>
          <div className="flex items-center gap-4">
            {!fallback && (
              <label className="flex cursor-pointer items-center gap-2 text-sm text-foreground">
                <Switch checked={showRequest} onCheckedChange={setShowRequest} />
                <span className="hidden sm:inline">{t('logPage.promptViewer.showRequest')}</span>
              </label>
            )}
            <Button variant="ghost" size="icon" aria-label={t('logPage.promptViewer.close')} onClick={onClose} className="shrink-0">
              <X className="size-5" />
            </Button>
          </div>
        </div>

        {fallback ? (
          // Unparseable / no messages[]: full raw request only (json badge + Copy).
          <div className="min-h-0 flex-1 overflow-y-auto p-5">
            <p className="mb-2 text-sm text-muted-foreground">{t('logPage.promptViewer.parseFallback')}</p>
            <CodeBlock
              language="json"
              wrap
              showCopy
              copyLabel={t('logPage.promptViewer.copy')}
              copiedLabel={t('logPage.io.copied')}
              onCopy={onCopy}
              maxHeightClass="max-h-none"
              progressiveChunkSize={IO_CHUNK}
              showAllLabel={t('logPage.io.showAll')}
              code={prettyJson(parsed ?? requestBody)}
            />
          </div>
        ) : (
          <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
            {/* Chart + stats — kept visible in both message-list and show-request modes */}
            <div className="flex shrink-0 flex-col gap-4 border-b border-border px-5 py-4 lg:flex-row">
              <div className="min-w-0 flex-1">
                <div className="mb-2 flex items-center gap-2 text-[11px] uppercase tracking-wide text-muted-foreground">
                  <span>{t('logPage.promptViewer.tokensPerMessage')}</span>
                  <span className="rounded bg-muted px-1.5 py-0.5 normal-case tracking-normal text-muted-foreground">
                    {t('logPage.promptViewer.estimated')}
                  </span>
                  {range && (
                    <button
                      type="button"
                      onClick={() => setRange(null)}
                      className="ml-auto rounded px-1.5 py-0.5 normal-case tracking-normal text-foreground hover:bg-muted"
                    >
                      {t('logPage.promptViewer.resetZoom')}
                    </button>
                  )}
                </div>
                <DistributionBar
                  full={rows}
                  visible={visible}
                  totalWeight={totalWeight}
                  selectedIdx={selectedIdx}
                  onSelect={selectFromBar}
                  range={range}
                  onRange={setRange}
                  onReset={() => setRange(null)}
                  t={t}
                />
                <div className="mt-2 flex flex-wrap gap-2">
                  {roleAggList.map((k) => (
                    <RoleChip key={k} role={k} t={t} />
                  ))}
                </div>
              </div>
              <StatsPanel
                promptTokens={promptTokens}
                cachedTokens={cachedTokens}
                visibleEst={visibleEst}
                cost={cost}
                model={model}
                roleAggList={roleAggList}
                roleAgg={roleAgg}
                total={totalWeight}
                onCopy={onCopy}
                t={t}
              />
            </div>

            {showRequest ? (
              // Show request: full raw request JSON below the chart (json badge + Copy).
              <div className="min-h-0 flex-1 overflow-y-auto p-5">
                <CodeBlock
                  language="json"
                  wrap
                  showCopy
                  copyLabel={t('logPage.promptViewer.copy')}
                  copiedLabel={t('logPage.io.copied')}
                  onCopy={onCopy}
                  maxHeightClass="max-h-none"
                  progressiveChunkSize={IO_CHUNK}
                  showAllLabel={t('logPage.io.showAll')}
                  code={prettyJson(parsed ?? requestBody)}
                />
              </div>
            ) : (
              <div ref={splitAreaRef} className="flex min-h-0 flex-1 flex-col overflow-hidden md:flex-row">
                <div
                  className="flex max-h-[38vh] shrink-0 flex-col gap-2 border-b border-border p-3 md:max-h-none md:border-b-0"
                  style={isWide ? { width: `${splitRatio * 100}%`, flexBasis: `${splitRatio * 100}%` } : undefined}
                >
                  <div className="relative">
                    <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                      placeholder={t('logPage.promptViewer.searchPlaceholder')}
                      className="h-9 pl-9"
                    />
                  </div>
                  <Select value={roleFilter} onValueChange={setRoleFilter}>
                    <SelectTrigger className="h-9">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {roleOptions.map((k) => (
                        <SelectItem key={k} value={k}>
                          {k === 'all' ? t('logPage.promptViewer.allRoles') : roleLabel(k, t)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <div className="grid grid-cols-[2rem_4.5rem_1fr] gap-2 px-2 pb-1 text-[11px] uppercase tracking-wide text-muted-foreground">
                    <span className="text-right">#</span>
                    <span>{t('logPage.promptViewer.colRole')}</span>
                    <span>{t('logPage.promptViewer.colPreview')}</span>
                  </div>
                  <div className="min-h-0 flex-1 space-y-0.5 overflow-y-auto">
                    {filtered.length === 0 ? (
                      <div className="px-2 py-6 text-center text-sm text-muted-foreground">{t('logPage.promptViewer.noMessages')}</div>
                    ) : (
                      filtered.map((r) => (
                        <button
                          key={r.index}
                          type="button"
                          onClick={() => setSelectedIdx(r.index)}
                          className={cn(
                            'grid w-full grid-cols-[2rem_4.5rem_1fr] items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors',
                            r.index === selectedIdx ? 'bg-muted ring-1 ring-inset ring-border' : 'hover:bg-muted'
                          )}
                        >
                          <span className="text-right text-xs tabular-nums text-muted-foreground">{r.index + 1}</span>
                          <RoleTag role={r.role} t={t} />
                          <span className="truncate text-xs text-muted-foreground">{r.preview}</span>
                        </button>
                      ))
                    )}
                  </div>
                </div>

                {isWide && (
                  // Draggable divider (col-resize, hover-highlight); double-click restores default ratio.
                  <div
                    role="separator"
                    aria-orientation="vertical"
                    onPointerDown={onSplitDown}
                    onPointerMove={onSplitMove}
                    onPointerUp={onSplitUp}
                    onPointerCancel={onSplitUp}
                    onDoubleClick={resetSplit}
                    className="group relative flex w-2 shrink-0 cursor-col-resize touch-none select-none items-stretch justify-center self-stretch"
                  >
                    <span className="w-px bg-border transition-all group-hover:w-0.5 group-hover:bg-primary/50" />
                  </div>
                )}

                {/* Reading pane */}
                <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
                  {activeRow ? (
                    <>
                      <div className="flex shrink-0 items-center justify-between gap-2 border-b border-border px-4 py-2">
                        <div className="flex items-center gap-1">
                          <Button variant="ghost" size="sm" className="h-7 gap-1.5 px-2" onClick={copyActive}>
                            <Copy className="size-3.5" />
                            {t('logPage.promptViewer.copy')}
                          </Button>
                          <Button
                            variant={rawMsg ? 'secondary' : 'ghost'}
                            size="sm"
                            className="h-7 gap-1.5 px-2"
                            onClick={() => setRawMsg((v) => !v)}
                          >
                            <Code2 className="size-3.5" />
                            {rawMsg ? t('logPage.promptViewer.hideRaw') : t('logPage.promptViewer.viewRaw')}
                          </Button>
                        </div>
                        <div className="flex items-center gap-2">
                          <RoleTag role={activeRow.role} t={t} />
                          <span className="text-xs font-medium text-muted-foreground">
                            {t('logPage.promptViewer.messageIndex', { current: realPos + 1, total: filtered.length })}
                          </span>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-7"
                            aria-label={t('logPage.promptViewer.prev')}
                            disabled={realPos === 0}
                            onClick={goPrev}
                          >
                            <ChevronLeft className="size-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-7"
                            aria-label={t('logPage.promptViewer.next')}
                            disabled={realPos === filtered.length - 1}
                            onClick={goNext}
                          >
                            <ChevronRight className="size-4" />
                          </Button>
                        </div>
                      </div>
                      <div className="min-h-0 flex-1 overflow-y-auto p-4">
                        {rawMsg ? (
                          <CodeBlock language="json" wrap showCopy={false} maxHeightClass="max-h-none" code={prettyJson(activeRow.msg)} />
                        ) : (
                          <MessageContent msg={activeRow.msg} t={t} />
                        )}
                      </div>
                    </>
                  ) : (
                    <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
                      {t('logPage.promptViewer.noMessages')}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

PromptViewer.propTypes = {
  requestBody: PropTypes.string,
  truncated: PropTypes.bool,
  promptTokens: PropTypes.number,
  cachedTokens: PropTypes.number,
  cost: PropTypes.string,
  model: PropTypes.string,
  onCopy: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
  t: PropTypes.func.isRequired
};
