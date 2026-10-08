import PropTypes from 'prop-types';
import { useMemo, useState } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';

import { cn } from '@/lib/utils';

// ==============================|| SHARED — DOUBLE MONTH RANGE CALENDAR ||============================== //
// 双月并排、成对翻页(左右月锁定相邻),周一为周首;点第一下定起点、第二下定终点(点反自动交换),
// 未闭合时用 hover 预览另一端。反色带用一层连续背景(端点才切圆角),避免分格背景在单元格间留 1px 断点。
// 纯展示:草稿由调用方持有(应用/取消的决定权在它那儿)。

const pad2 = (n) => String(n).padStart(2, '0');

export const isoOf = (d) => `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;

/** 月首的锚(day=1,规避 31 号加月溢出)。 */
export const monthAnchor = (d) => new Date(d.getFullYear(), d.getMonth(), 1);

export const shiftMonth = (anchor, delta) => new Date(anchor.getFullYear(), anchor.getMonth() + delta, 1);

/** 点一天之后的草稿:第一下定起点;第二下定终点并按大小归位;已成范围时再点从头开始。 */
export function nextDraft(draft, iso) {
  if (!draft.start || draft.end) return { start: iso, end: null };
  return draft.start <= iso ? { start: draft.start, end: iso } : { start: iso, end: draft.start };
}

/** 一个月的 6×7 网格(周一为周首);非本月的格子给 null。 */
export function monthMatrix(anchor) {
  const year = anchor.getFullYear();
  const month = anchor.getMonth();
  const lead = (new Date(year, month, 1).getDay() + 6) % 7;
  const days = new Date(year, month + 1, 0).getDate();
  const cells = [];
  for (let i = 0; i < lead; i += 1) cells.push(null);
  for (let d = 1; d <= days; d += 1) cells.push(isoOf(new Date(year, month, d)));
  while (cells.length % 7 !== 0) cells.push(null);
  const weeks = [];
  for (let i = 0; i < cells.length; i += 7) weeks.push(cells.slice(i, i + 7));
  return weeks;
}

/** 一天在草稿范围里的位置:none / start / end / single / middle。 */
export function dayPosition(draft, iso, hover) {
  const { start } = draft;
  if (!start) return 'none';
  const other = draft.end ?? hover ?? start;
  const lo = start <= other ? start : other;
  const hi = start <= other ? other : start;
  if (iso < lo || iso > hi) return 'none';
  if (lo === hi) return 'single';
  if (iso === lo) return 'start';
  if (iso === hi) return 'end';
  return 'middle';
}

const bandClass = (pos) => {
  if (pos === 'none') return '';
  if (pos === 'single') return 'bg-foreground/10 rounded-md';
  if (pos === 'start') return 'bg-foreground/10 rounded-l-md';
  if (pos === 'end') return 'bg-foreground/10 rounded-r-md';
  return 'bg-foreground/10';
};

const toBcp47 = (locale) => (locale ? locale.replace('_', '-') : 'en-US');

// 周首为周一的短星期名:2024-01-01 是周一,顺推 7 天。
const weekdayNames = (locale) => {
  const fmt = new Intl.DateTimeFormat(toBcp47(locale), { weekday: 'short' });
  return Array.from({ length: 7 }, (_, i) => fmt.format(new Date(2024, 0, 1 + i)));
};

const monthLabel = (anchor, locale) => new Intl.DateTimeFormat(toBcp47(locale), { year: 'numeric', month: 'long' }).format(anchor);

function DayCell({ iso, pos, today, disabled, onPick, onHover }) {
  if (!iso) return <div className="h-8" />;
  const endpoint = pos === 'start' || pos === 'end' || pos === 'single';
  return (
    <div className={cn('flex h-8 items-center justify-center', !disabled && bandClass(pos))}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => onPick(iso)}
        onMouseEnter={() => onHover(iso)}
        onMouseLeave={() => onHover(null)}
        className={cn(
          'flex size-8 items-center justify-center rounded-md text-[12.5px] tabular-nums transition-colors',
          disabled && 'cursor-not-allowed text-muted-foreground/40',
          !disabled && endpoint && 'bg-foreground font-medium text-background',
          !disabled && !endpoint && 'text-foreground/85 hover:bg-accent',
          !disabled && !endpoint && iso === today && 'font-medium underline decoration-dotted underline-offset-4'
        )}
      >
        {Number(iso.slice(8))}
      </button>
    </div>
  );
}

DayCell.propTypes = {
  iso: PropTypes.string,
  pos: PropTypes.string.isRequired,
  today: PropTypes.string.isRequired,
  disabled: PropTypes.bool,
  onPick: PropTypes.func.isRequired,
  onHover: PropTypes.func.isRequired
};

function MonthView({ anchor, draft, hover, today, minIso, weekdays, onPick, onHover }) {
  return (
    <div className="w-[196px] shrink-0">
      <div className="grid grid-cols-7">
        {weekdays.map((w) => (
          <div key={w} className="pb-1 text-center text-[11px] text-muted-foreground">
            {w}
          </div>
        ))}
      </div>
      {monthMatrix(anchor).map((week) => (
        <div key={week.find((d) => d) ?? `blank-${anchor.getMonth()}`} className="grid grid-cols-7">
          {week.map((iso, i) => (
            <DayCell
              key={iso ?? `pad-${i}`}
              iso={iso}
              pos={iso ? dayPosition(draft, iso, hover) : 'none'}
              today={today}
              disabled={Boolean(iso && minIso && iso < minIso)}
              onPick={onPick}
              onHover={onHover}
            />
          ))}
        </div>
      ))}
    </div>
  );
}

MonthView.propTypes = {
  anchor: PropTypes.instanceOf(Date).isRequired,
  draft: PropTypes.object.isRequired,
  hover: PropTypes.string,
  today: PropTypes.string.isRequired,
  minIso: PropTypes.string,
  weekdays: PropTypes.array.isRequired,
  onPick: PropTypes.func.isRequired,
  onHover: PropTypes.func.isRequired
};

export default function DateRangeCalendar({ draft, onDraftChange, initialMonth, locale, minIso, prevLabel, nextLabel }) {
  const [anchor, setAnchor] = useState(() => monthAnchor(initialMonth ?? new Date()));
  const [hover, setHover] = useState(null);
  const today = isoOf(new Date());
  const weekdays = useMemo(() => weekdayNames(locale), [locale]);
  const months = [anchor, shiftMonth(anchor, 1)];

  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <button
          type="button"
          aria-label={prevLabel}
          onClick={() => setAnchor(shiftMonth(anchor, -1))}
          className="flex size-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent"
        >
          <ChevronLeft className="size-4" />
        </button>
        <div className="flex flex-1 justify-around text-[12.5px] font-medium">
          {months.map((m) => (
            <span key={`${m.getFullYear()}-${m.getMonth()}`}>{monthLabel(m, locale)}</span>
          ))}
        </div>
        <button
          type="button"
          aria-label={nextLabel}
          onClick={() => setAnchor(shiftMonth(anchor, 1))}
          className="flex size-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent"
        >
          <ChevronRight className="size-4" />
        </button>
      </div>
      <div className="flex gap-3 overflow-x-auto">
        {months.map((m) => (
          <MonthView
            key={`${m.getFullYear()}-${m.getMonth()}`}
            anchor={m}
            draft={draft}
            hover={hover}
            today={today}
            minIso={minIso}
            weekdays={weekdays}
            onPick={(iso) => onDraftChange(nextDraft(draft, iso))}
            onHover={setHover}
          />
        ))}
      </div>
    </div>
  );
}

DateRangeCalendar.propTypes = {
  draft: PropTypes.shape({ start: PropTypes.string, end: PropTypes.string }).isRequired,
  onDraftChange: PropTypes.func.isRequired,
  initialMonth: PropTypes.instanceOf(Date),
  locale: PropTypes.string,
  minIso: PropTypes.string,
  prevLabel: PropTypes.string,
  nextLabel: PropTypes.string
};
