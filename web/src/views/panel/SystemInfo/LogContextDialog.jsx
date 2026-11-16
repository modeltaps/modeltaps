import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Button } from '@/components/ui/button';
import { Dialog, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Skeleton } from '@/components/ui/skeleton';
import { Segmented } from '@/components/objects';
import { API } from 'utils/api';
import { LogRow } from './LogRows';
import { processLogEntry } from './logHelpers';

// ==============================|| SYSTEM LOGS — CONTEXT DIALOG ||============================== //
// The lines written around one entry, unfiltered, from the in-memory history
// (`/api/system_info/log/context`). The entry itself is marked with a bar on the left.

const LINES = [5, 10, 20];

export default function LogContextDialog({ log, onClose }) {
  const { t } = useTranslation();
  const [lines, setLines] = useState(5);
  const [state, setState] = useState({ loading: true, logs: [], target: -1, error: '' });

  useEffect(() => {
    let cancelled = false;
    API.post('/api/system_info/log/context', {
      timestamp: log.timestamp,
      message: log.message,
      context_lines: lines
    })
      .then((res) => {
        if (cancelled) return;
        if (!res.data.success) {
          setState({ loading: false, logs: [], target: -1, error: t('systemLogs.contextFailed', { message: res.data.message }) });
          return;
        }
        const result = res.data.data || {};
        setState({ loading: false, logs: (result.logs || []).map(processLogEntry), target: result.target_position ?? -1, error: '' });
      })
      .catch((err) => {
        if (!cancelled) setState({ loading: false, logs: [], target: -1, error: t('systemLogs.contextFailed', { message: err.message }) });
      });
    return () => {
      cancelled = true;
    };
  }, [log, lines, t]);

  const missing = !state.loading && !state.error && state.target < 0;

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-5xl">
        <DialogHeader className="flex-row items-center justify-between gap-3 space-y-0">
          <DialogTitle>{t('systemLogs.contextTitle')}</DialogTitle>
          <div className="mr-8 flex items-center gap-2 text-xs text-muted-foreground">
            <span>{t('systemLogs.contextLines')}</span>
            <Segmented
              size="mini"
              label={t('systemLogs.contextLines')}
              value={String(lines)}
              onChange={(v) => {
                setLines(Number(v));
                setState((s) => ({ ...s, loading: true }));
              }}
              options={LINES.map((n) => ({ value: String(n), label: String(n) }))}
            />
          </div>
        </DialogHeader>
        <DialogBody className="px-0">
          {state.loading ? (
            <div className="space-y-2 px-3">
              {Array.from({ length: 6 }, (_, i) => (
                <Skeleton key={i} className="h-5 w-full" />
              ))}
            </div>
          ) : state.error || missing ? (
            <p className="px-6 py-8 text-center text-sm text-muted-foreground">{state.error || t('systemLogs.contextMissing')}</p>
          ) : (
            <div className="divide-y divide-border/60 border-y border-border">
              {state.logs.map((entry, i) => (
                <LogRow key={`${entry.timestamp}-${i}`} log={entry} target={i === state.target} />
              ))}
            </div>
          )}
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {t('common.close', { defaultValue: 'Close' })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

LogContextDialog.propTypes = {
  log: PropTypes.shape({ timestamp: PropTypes.string, message: PropTypes.string }).isRequired,
  onClose: PropTypes.func.isRequired
};
