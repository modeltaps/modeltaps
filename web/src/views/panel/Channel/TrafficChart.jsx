import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Area, CartesianGrid, ComposedChart, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';

import { cn } from '@/lib/utils';

// Neutral greys for the lines that are not in focus; the focused channel gets the accent.
const GREYS = [
  'color-mix(in srgb, var(--foreground) 42%, transparent)',
  'color-mix(in srgb, var(--foreground) 26%, transparent)',
  'color-mix(in srgb, var(--foreground) 16%, transparent)'
];
const LEAD = 'var(--info)';
const AXIS_TICK = { fill: 'var(--muted-foreground)', fontSize: 11, fontFamily: 'var(--font-mono, monospace)' };

function label(ts, bucket, long = false) {
  const d = new Date(ts * 1000);
  const hh = String(d.getHours()).padStart(2, '0');
  const md = `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  if (bucket >= 86400) return md;
  if (bucket <= 3600 && !long) return `${hh}:00`;
  return `${md} ${hh}:00`;
}

const compact = (v) => (v >= 1000 ? `${(v / 1000).toFixed(v % 1000 ? 1 : 0)}k` : String(Math.round(v)));

function ChartTooltip({ active, payload, label: ts, bucket, channels }) {
  if (!active || !payload?.length) return null;
  const values = payload[0].payload.values;
  return (
    <div className="min-w-44 rounded-lg border border-border bg-popover px-2.5 py-2 text-xs shadow-md">
      <p className="mb-1 font-mono text-muted-foreground">{label(ts, bucket, true)}</p>
      {channels.map((c) => (
        <p key={c.id} className="flex justify-between gap-4 tabular-nums">
          <span className="truncate font-mono">{c.name}</span>
          <span>{(values[c.id] || 0).toLocaleString()}</span>
        </p>
      ))}
    </div>
  );
}

ChartTooltip.propTypes = {
  active: PropTypes.bool,
  payload: PropTypes.array,
  label: PropTypes.any,
  bucket: PropTypes.number,
  channels: PropTypes.array
};

// One line per channel of a provider (requests per bucket). The lead channel (hovered row, or
// the busiest one) is drawn in the accent colour with a light fill; the others in greys.
export default function TrafficChart({ points, channels, leadId, bucket, loading }) {
  const { t } = useTranslation();
  const lead = channels.some((c) => c.id === leadId) ? leadId : channels[0]?.id;
  let grey = 0;
  const colors = new Map(channels.map((c) => [c.id, c.id === lead ? LEAD : GREYS[Math.min(grey++, GREYS.length - 1)]]));
  const empty = !loading && channels.length === 0;

  return (
    <>
      <div className="flex flex-wrap gap-x-3.5 gap-y-1 px-1 pb-2 text-xs text-muted-foreground">
        {empty ? (
          <span>{t('channels.chart.empty')}</span>
        ) : (
          channels.map((c) => (
            <span key={c.id} className={cn('inline-flex items-center gap-1.5', c.id === lead && 'text-foreground')}>
              <i className="inline-block h-0.5 w-3 rounded-full" style={{ background: colors.get(c.id) }} />
              <span className="font-mono">{c.name}</span>
            </span>
          ))
        )}
      </div>
      <div className="h-[220px]" role="img" aria-label={t('channels.chart.aria')}>
        {!empty && (
          <ResponsiveContainer width="100%" height="100%">
            <ComposedChart data={points} margin={{ top: 6, right: 8, left: 0, bottom: 0 }}>
              <CartesianGrid stroke="var(--border)" vertical={false} />
              <XAxis
                dataKey="ts"
                tickFormatter={(v) => label(v, bucket)}
                tick={AXIS_TICK}
                axisLine={false}
                tickLine={false}
                minTickGap={28}
              />
              <YAxis width={44} allowDecimals={false} tickFormatter={compact} tick={AXIS_TICK} axisLine={false} tickLine={false} />
              <Tooltip
                content={<ChartTooltip bucket={bucket} channels={channels} />}
                cursor={{ stroke: 'var(--muted-foreground)', strokeDasharray: '2 3' }}
                isAnimationActive={false}
              />
              {channels
                .filter((c) => c.id !== lead)
                .map((c) => (
                  <Line
                    key={c.id}
                    type="linear"
                    dataKey={(p) => p.values[c.id] || 0}
                    stroke={colors.get(c.id)}
                    strokeWidth={1.5}
                    dot={false}
                    activeDot={false}
                    isAnimationActive={false}
                  />
                ))}
              {lead != null && (
                <Area
                  type="linear"
                  dataKey={(p) => p.values[lead] || 0}
                  stroke={LEAD}
                  strokeWidth={2}
                  fill="color-mix(in srgb, var(--info) 8%, transparent)"
                  dot={false}
                  activeDot={{ r: 3 }}
                  isAnimationActive={false}
                />
              )}
            </ComposedChart>
          </ResponsiveContainer>
        )}
      </div>
    </>
  );
}

TrafficChart.propTypes = {
  points: PropTypes.array.isRequired,
  channels: PropTypes.array.isRequired,
  leadId: PropTypes.number,
  bucket: PropTypes.number.isRequired,
  loading: PropTypes.bool
};
