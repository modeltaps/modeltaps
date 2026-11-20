// Off-peak pricing of a price (model.OffPeakPricing): `ratio` is the share charged outside the
// peak windows, each { days: [0 = Sunday … 6], from: 'HH:MM', to: 'HH:MM' } in UTC.

const WEEKDAYS = [1, 2, 3, 4, 5];

const sameDays = (a, b) => a.length === b.length && a.every((d, i) => d === b[i]);

// The weekdays of a window, as "weekdays" or the day names in the UI language.
function daysLabel(days, t, locale) {
  const sorted = [...days].sort((a, b) => a - b);
  if (sameDays(sorted, WEEKDAYS)) return t('pricing_edit.offPeak.weekdays');
  const format = new Intl.DateTimeFormat(locale, { weekday: 'short', timeZone: 'UTC' });
  // 2023-01-01 was a Sunday.
  return sorted.map((d) => format.format(new Date(Date.UTC(2023, 0, 1 + d)))).join(', ');
}

// The peak hours as one line, windows on the same days grouped: "Mon–Fri 01:00–04:00, 06:00–10:00".
export function peakHoursText(offPeak, t, locale = 'en') {
  const groups = [];
  for (const w of offPeak?.peak || []) {
    const days = [...(w.days || [])].sort((a, b) => a - b);
    const group = groups.find((g) => sameDays(g.days, days));
    const span = `${w.from}–${w.to}`;
    if (group) group.spans.push(span);
    else groups.push({ days, spans: [span] });
  }
  return groups.map((g) => `${daysLabel(g.days, t, locale)} ${g.spans.join(', ')}`).join('; ');
}

// Whether the schedule is one the server applies (model.OffPeakPricing.Valid, in short).
export function hasOffPeak(offPeak) {
  return Boolean(offPeak && offPeak.ratio > 0 && offPeak.ratio < 1 && offPeak.peak?.length);
}
