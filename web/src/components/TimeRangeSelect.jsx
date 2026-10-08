import PropTypes from 'prop-types';
import { useEffect, useMemo, useRef, useState } from 'react';
import dayjs from 'dayjs';
import { Check, ChevronDown, SlidersHorizontal } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import DateRangeCalendar from '@/components/ui/date-range-calendar';

// ==============================|| SHARED — TIME RANGE SELECTOR (OpenRouter 风格) ||============================== //
// 触发器显示当前档位缩写徽标 + 具体时间范围文本;下拉三段(相对区间 / 日历区间两列 / 自定义弹层)。
// 受控:选择任一档位或应用自定义后回调 onApply(range, presetId),由父组件更新 timeRange 驱动列表/直方图/URL。
// Logs(/panel/log)与用量分析(/panel/usage)共用本组件,行为与外观完全一致。

// 相对区间:start = now - N,end = now(+缓冲,时钟偏差容错,与旧行为一致)。徽标为缩写。
const RELATIVE_PRESETS = [
  { id: '15m', badge: '15m', labelKey: 'rel15m', amount: 15, unit: 'minute' },
  { id: '30m', badge: '30m', labelKey: 'rel30m', amount: 30, unit: 'minute' },
  { id: '1h', badge: '1h', labelKey: 'rel1h', amount: 1, unit: 'hour' },
  { id: '3h', badge: '3h', labelKey: 'rel3h', amount: 3, unit: 'hour' },
  { id: '1d', badge: '1d', labelKey: 'rel1d', amount: 1, unit: 'day' },
  { id: '2d', badge: '2d', labelKey: 'rel2d', amount: 2, unit: 'day' },
  { id: '1w', badge: '1w', labelKey: 'rel1w', amount: 1, unit: 'week' },
  { id: '1mo', badge: '1mo', labelKey: 'rel1mo', amount: 1, unit: 'month' },
  { id: '1y', badge: '1y', labelKey: 'rel1y', amount: 1, unit: 'year' }
];

// 日历区间两列:当前期(offset 0,end = now)/上一期(offset 1,end = 期末)。
// 徽标动态:当前期显示已过时长(elapsedUnit + badgeUnit),上一期显示完整期长(fullBadge)。
const CALENDAR_PRESETS = [
  { id: 'today', labelKey: 'today', unit: 'day', offset: 0, elapsedUnit: 'hour', badgeUnit: 'h' },
  { id: 'yesterday', labelKey: 'yesterday', unit: 'day', offset: 1, fullBadge: '24h' },
  { id: 'thisWeek', labelKey: 'thisWeek', unit: 'week', offset: 0, elapsedUnit: 'day', badgeUnit: 'd' },
  { id: 'lastWeek', labelKey: 'lastWeek', unit: 'week', offset: 1, fullBadge: '7d' },
  { id: 'thisMonth', labelKey: 'thisMonth', unit: 'month', offset: 0, elapsedUnit: 'day', badgeUnit: 'd' },
  { id: 'lastMonth', labelKey: 'lastMonth', unit: 'month', offset: 1, fullBadge: '30d' },
  { id: 'thisYear', labelKey: 'thisYear', unit: 'year', offset: 0, elapsedUnit: 'month', badgeUnit: 'mo' },
  { id: 'lastYear', labelKey: 'lastYear', unit: 'year', offset: 1, fullBadge: '1y' }
];

const clampStart = (start, minTimestamp) => (minTimestamp && start < minTimestamp ? minTimestamp : start);

// 依预设 id 计算 {start,end}。相对/当前期 end = now+缓冲;上一期 end = 期末。未知 id 返回 null。
export function computeRange(presetId, minTimestamp = 0) {
  const now = dayjs();
  const rel = RELATIVE_PRESETS.find((p) => p.id === presetId);
  if (rel) {
    return { start_timestamp: clampStart(now.subtract(rel.amount, rel.unit).unix(), minTimestamp), end_timestamp: now.unix() + 3600 };
  }
  const cal = CALENDAR_PRESETS.find((p) => p.id === presetId);
  if (cal) {
    if (cal.offset === 0) {
      return { start_timestamp: clampStart(now.startOf(cal.unit).unix(), minTimestamp), end_timestamp: now.unix() + 3600 };
    }
    const ref = now.subtract(cal.offset, cal.unit);
    return { start_timestamp: clampStart(ref.startOf(cal.unit).unix(), minTimestamp), end_timestamp: ref.endOf(cal.unit).unix() };
  }
  return null;
}

// 初始预设反推:仅匹配起点稳定的日历区间(相对区间起点随 now 漂移,刷新后按自定义处理)。
export function detectPreset(range, minTimestamp = 0) {
  if (!range) return 'custom';
  for (const p of CALENDAR_PRESETS) {
    const r = computeRange(p.id, minTimestamp);
    if (r && r.start_timestamp === range.start_timestamp && (p.offset === 0 || r.end_timestamp === range.end_timestamp)) return p.id;
  }
  return 'custom';
}

const LABEL_KEY_BY_ID = Object.fromEntries([...RELATIVE_PRESETS, ...CALENDAR_PRESETS].map((p) => [p.id, p.labelKey]));

// 日历档位徽标:当前期显示自期初至今的已过时长(向下取整,对齐 OpenRouter;至少 1 单位兜底);上一期显示完整期长。
const calendarBadge = (cal) => {
  if (cal.offset !== 0) return cal.fullBadge;
  const now = dayjs();
  const elapsed = Math.max(1, Math.floor(now.diff(now.startOf(cal.unit), cal.elapsedUnit, true)));
  return `${elapsed}${cal.badgeUnit}`;
};

// 单行/触发器徽标:相对档位为静态缩写,日历档位为动态时长。
const badgeForPreset = (p) => (p.badge != null ? p.badge : calendarBadge(p));

// 触发器徽标:相对/日历返回徽标文本,custom 或未知返回 null(触发器改用具体时间文本)。
const triggerBadge = (presetId) => {
  const rel = RELATIVE_PRESETS.find((p) => p.id === presetId);
  if (rel) return rel.badge;
  const cal = CALENDAR_PRESETS.find((p) => p.id === presetId);
  if (cal) return calendarBadge(cal);
  return null;
};

const formatMoment = (unix, locale) => {
  const d = dayjs.unix(unix);
  if (!locale || locale === 'en_US') return d.format('MMM D, h:mm A');
  // Other languages get their own month/day order and 24-hour clock from Intl.
  return new Intl.DateTimeFormat(locale.replace('_', '-'), {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23'
  }).format(d.toDate());
};

// 触发器范围文本:end 超过 now 时按 now 显示(避免相对/当前期缓冲导致的未来时间)。
const formatRange = (range, locale) => {
  if (!range?.start_timestamp) return '';
  const nowUnix = dayjs().unix();
  const end = Math.min(range.end_timestamp || nowUnix, nowUnix);
  return `${formatMoment(range.start_timestamp, locale)} – ${formatMoment(end, locale)}`;
};

// 自定义范围草稿:日期由双月日历定(本地 'YYYY-MM-DD' 闭区间),时刻由日历下方两个 time 输入定。
const DEFAULT_START_TIME = '00:00';
const DEFAULT_END_TIME = '23:59';
const toIso = (unix) => (unix ? dayjs.unix(unix).format('YYYY-MM-DD') : null);
const toTime = (unix, fallback) => (unix ? dayjs.unix(unix).format('HH:mm') : fallback);
const composeUnix = (iso, time) => dayjs(`${iso}T${time || DEFAULT_START_TIME}`).unix();

export default function TimeRangeSelect({ value, preset, minTimestamp = 0, onApply, t, locale }) {
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState('menu');
  const [draft, setDraft] = useState({ start: null, end: null });
  const [startTime, setStartTime] = useState(DEFAULT_START_TIME);
  const [endTime, setEndTime] = useState(DEFAULT_END_TIME);
  const rootRef = useRef(null);

  const close = () => {
    setOpen(false);
    setMode('menu');
  };

  useEffect(() => {
    if (!open) return undefined;
    const onDown = (e) => {
      if (rootRef.current && !rootRef.current.contains(e.target)) close();
    };
    const onKey = (e) => {
      if (e.key === 'Escape') close();
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const rangeText = useMemo(() => formatRange(value, locale), [value, locale]);
  const minIso = minTimestamp ? dayjs.unix(minTimestamp).format('YYYY-MM-DD') : undefined;

  // 预设档位(相对/日历):触发器显示徽标 + 档位名,不显示具体时间;custom/未知:显示具体起止时间。
  const badge = triggerBadge(preset);
  const isPreset = badge != null;
  const presetLabel = isPreset && LABEL_KEY_BY_ID[preset] ? t(`logPage.timeRangeMenu.${LABEL_KEY_BY_ID[preset]}`) : '';
  // 无起始时间(start_timestamp 为 0/空)时范围文本为空,改用「全部时间」占位,避免触发器只剩图标。
  const triggerText = isPreset ? presetLabel : rangeText || t('logPage.timeRangeMenu.allTime');

  const pick = (presetId) => {
    const r = computeRange(presetId, minTimestamp);
    if (r) onApply(r, presetId);
    close();
  };

  const openCustom = () => {
    setDraft({ start: toIso(value?.start_timestamp), end: toIso(value?.end_timestamp) });
    setStartTime(toTime(value?.start_timestamp, DEFAULT_START_TIME));
    setEndTime(toTime(value?.end_timestamp, DEFAULT_END_TIME));
    setMode('custom');
  };

  // 只点了起点即视为单日范围(终点=起点),时刻仍取两个 time 输入。
  const draftStartUnix = draft.start ? composeUnix(draft.start, startTime) : 0;
  const draftEndUnix = draft.start ? composeUnix(draft.end ?? draft.start, endTime) : 0;
  const invalidRange = Boolean(draftStartUnix && draftEndUnix && draftEndUnix <= draftStartUnix);

  const applyCustom = () => {
    if (!draft.start || invalidRange) return;
    let start = draftStartUnix;
    let end = draftEndUnix;
    if (minTimestamp && start < minTimestamp) start = minTimestamp;
    if (minTimestamp && end < minTimestamp) end = minTimestamp;
    if (end <= start) return;
    onApply({ start_timestamp: start, end_timestamp: end }, 'custom');
    close();
  };

  const initialMonth = draft.start ? new Date(`${draft.start}T00:00:00`) : new Date();

  const badgeCls =
    'inline-flex min-w-[34px] items-center justify-center rounded bg-muted px-1 py-0.5 text-[11px] font-medium text-muted-foreground';
  const rowCls = 'flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm hover:bg-muted';

  const renderRow = (p) => {
    const active = preset === p.id;
    return (
      <button key={p.id} type="button" onClick={() => pick(p.id)} className={cn(rowCls, active && 'bg-muted')}>
        <span className={badgeCls}>{badgeForPreset(p)}</span>
        <span className="flex-1 truncate">{t(`logPage.timeRangeMenu.${p.labelKey}`)}</span>
        {active && <Check className="size-4 shrink-0 text-foreground" />}
      </button>
    );
  };

  return (
    <div ref={rootRef} className="relative inline-flex">
      <button
        type="button"
        onClick={() => (open ? close() : setOpen(true))}
        aria-label={t('logPage.timeRange')}
        aria-expanded={open}
        className="inline-flex h-9 max-w-[300px] items-center gap-2 rounded-md border border-input bg-background px-2.5 text-sm text-foreground transition-colors hover:bg-muted"
      >
        <span className={cn(badgeCls, 'min-w-[30px]')}>{isPreset ? badge : <SlidersHorizontal className="size-3" />}</span>
        <span className="truncate">{triggerText}</span>
        <ChevronDown className="size-4 shrink-0 opacity-60" />
      </button>
      {open && (
        <div
          className={cn(
            'absolute right-0 top-full z-50 mt-1 max-w-[calc(100vw-1rem)] rounded-md border border-border bg-card text-card-foreground shadow-md',
            mode === 'custom' ? 'w-[440px]' : 'w-72'
          )}
        >
          {mode === 'menu' ? (
            <div className="max-h-[70vh] overflow-y-auto p-1">
              <div className="flex flex-col">{RELATIVE_PRESETS.map(renderRow)}</div>
              <div className="my-1 h-px bg-border" />
              <div className="grid grid-cols-2 gap-0.5">{CALENDAR_PRESETS.map(renderRow)}</div>
              <div className="my-1 h-px bg-border" />
              <button type="button" onClick={openCustom} className={rowCls}>
                <span className={cn(badgeCls, 'py-1')}>
                  <SlidersHorizontal className="size-3" />
                </span>
                <span className="flex-1 truncate">{t('logPage.timeRangeMenu.custom')}</span>
                {preset === 'custom' && <Check className="size-4 shrink-0 text-foreground" />}
              </button>
            </div>
          ) : (
            <div className="flex flex-col gap-3 p-3">
              <DateRangeCalendar
                draft={draft}
                onDraftChange={setDraft}
                initialMonth={initialMonth}
                locale={locale}
                minIso={minIso}
                prevLabel={t('logPage.timeRangeMenu.prevMonth')}
                nextLabel={t('logPage.timeRangeMenu.nextMonth')}
              />
              <div className="grid grid-cols-2 gap-3">
                <div className="flex flex-col gap-1">
                  <label htmlFor="log-tr-start" className="text-xs text-muted-foreground">
                    {t('tableToolBar.startTime')}
                  </label>
                  <Input id="log-tr-start" type="time" className="h-8" value={startTime} onChange={(e) => setStartTime(e.target.value)} />
                </div>
                <div className="flex flex-col gap-1">
                  <label htmlFor="log-tr-end" className="text-xs text-muted-foreground">
                    {t('tableToolBar.endTime')}
                  </label>
                  <Input id="log-tr-end" type="time" className="h-8" value={endTime} onChange={(e) => setEndTime(e.target.value)} />
                </div>
              </div>
              {invalidRange && <p className="text-xs text-destructive">{t('logPage.timeRangeMenu.invalidRange')}</p>}
              <div className="flex items-center justify-between gap-2">
                <span className="truncate text-xs tabular-nums text-muted-foreground">
                  {draft.start ? `${draft.start} – ${draft.end ?? '…'}` : t('logPage.timeRangeMenu.pickDates')}
                </span>
                <div className="flex gap-2">
                  <Button variant="ghost" size="sm" onClick={() => setMode('menu')}>
                    {t('logPage.timeRangeMenu.cancel')}
                  </Button>
                  <Button size="sm" onClick={applyCustom} disabled={!draft.start || invalidRange}>
                    {t('logPage.timeRangeMenu.apply')}
                  </Button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

TimeRangeSelect.propTypes = {
  value: PropTypes.object,
  preset: PropTypes.string,
  minTimestamp: PropTypes.number,
  onApply: PropTypes.func.isRequired,
  t: PropTypes.func.isRequired,
  locale: PropTypes.string
};
