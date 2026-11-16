import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { TextSearch } from 'lucide-react';

import { cn } from '@/lib/utils';
import { formatFullTimestamp, formatTimestamp } from './logHelpers';

// ==============================|| SYSTEM LOGS — ROWS ||============================== //
// One grid for the list and the context dialog: time, level, source, message, row action.
// Only errors and warnings carry color; info and debug stay in the text ramp.

const GRID = 'grid grid-cols-[104px_56px_120px_minmax(0,1fr)_32px] items-start gap-x-3 px-3';

// Request ids start with a timestamp, so their tail is the part that tells them apart.
const shortSource = (source) => (source.length > 16 ? `…${source.slice(-10)}` : source);

const LEVEL_TEXT = {
  error: 'text-destructive',
  warn: 'text-warning',
  info: 'text-muted-foreground',
  debug: 'text-muted-foreground/60'
};

export function LogHeader() {
  const { t } = useTranslation();
  return (
    <div
      className={cn(GRID, 'sticky top-0 z-[1] h-8 items-center border-b border-border bg-card text-xs font-medium text-muted-foreground')}
    >
      <span>{t('systemLogs.time')}</span>
      <span>{t('systemLogs.level')}</span>
      <span>{t('systemLogs.source')}</span>
      <span>{t('systemLogs.message')}</span>
      <span />
    </div>
  );
}

export function LogRow({ log, target = false, onSource, onContext }) {
  const { t } = useTranslation();
  return (
    <div
      className={cn(
        GRID,
        'group relative py-1.5 font-mono text-xs leading-5 transition-colors hover:bg-muted/50',
        target && 'bg-muted hover:bg-muted before:absolute before:inset-y-0 before:left-0 before:w-0.5 before:bg-foreground'
      )}
    >
      <time className="tabular-nums text-muted-foreground" title={formatFullTimestamp(log.timestamp)}>
        {formatTimestamp(log.timestamp)}
      </time>
      <span className={cn('font-sans font-medium', LEVEL_TEXT[log.level])}>{t(`systemLogs.${log.level}`)}</span>
      {log.source && onSource ? (
        <button
          type="button"
          onClick={() => onSource(log.source)}
          title={`${log.source}\n${t('systemLogs.filterSource', { source: shortSource(log.source) })}`}
          className="truncate text-left text-muted-foreground underline-offset-2 hover:text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {shortSource(log.source)}
        </button>
      ) : (
        <span className="truncate text-muted-foreground" title={log.source}>
          {shortSource(log.source)}
        </span>
      )}
      <span className="whitespace-pre-wrap break-words text-foreground [overflow-wrap:anywhere]">{log.text}</span>
      {onContext ? (
        <button
          type="button"
          onClick={() => onContext(log)}
          title={t('systemLogs.context')}
          aria-label={t('systemLogs.context')}
          className="flex size-6 items-center justify-center rounded-md text-muted-foreground opacity-0 transition-[opacity,color] hover:bg-muted hover:text-foreground focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring group-hover:opacity-100"
        >
          <TextSearch className="size-4" strokeWidth={1.5} />
        </button>
      ) : (
        <span />
      )}
    </div>
  );
}

const logShape = PropTypes.shape({
  timestamp: PropTypes.string,
  level: PropTypes.string.isRequired,
  source: PropTypes.string,
  text: PropTypes.string
});

LogRow.propTypes = {
  log: logShape.isRequired,
  target: PropTypes.bool,
  onSource: PropTypes.func,
  onContext: PropTypes.func
};
