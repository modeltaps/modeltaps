import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { formatTimestamp, levelBadge, processLogEntry } from './logHelpers';

// ==============================|| SYSTEM LOGS — CONTEXT DIALOG ||============================== //
// Shows N lines around a target log entry. Resolves from the already-loaded data
// when possible; falls back to the `/api/system_info/log/context` endpoint.

export default function LogContextDialog({ target, logs, originalLogs, searchTerm, onClose }) {
  const { t } = useTranslation();
  const [contextLines, setContextLines] = useState(3);
  const [contextLogs, setContextLogs] = useState([]);
  const [targetPosition, setTargetPosition] = useState(-1);
  const [loading, setLoading] = useState(false);

  const fromBackend = useCallback(async (targetLog, lines) => {
    setLoading(true);
    try {
      const res = await API.post('/api/system_info/log/context', {
        timestamp: targetLog.timestamp,
        message: targetLog.message,
        type: targetLog.type,
        context_lines: lines
      });
      const result = res.data?.success ? res.data.data : null;
      if (result?.logs && Array.isArray(result.logs)) {
        const processed = result.logs.map(processLogEntry);
        setContextLogs(processed);
        setTargetPosition(result.target_position ?? Math.floor(processed.length / 2));
      } else {
        setContextLogs([targetLog]);
        setTargetPosition(0);
      }
    } catch (err) {
      showError('Error getting context: ' + err.message);
      setContextLogs([targetLog]);
      setTargetPosition(0);
    } finally {
      setLoading(false);
    }
  }, []);

  const compute = useCallback(
    (lines) => {
      const targetLog = logs[target.index];
      if (!targetLog) return;
      let source;
      let realIndex;
      if (searchTerm.trim() && originalLogs.length > 0) {
        source = originalLogs;
        realIndex = originalLogs.findIndex(
          (log) => log.timestamp === targetLog.timestamp && log.message === targetLog.message && log.type === targetLog.type
        );
        if (realIndex === -1) realIndex = originalLogs.findIndex((log) => log.message === targetLog.message);
        if (realIndex === -1) {
          fromBackend(targetLog, lines);
          return;
        }
      } else {
        source = logs;
        realIndex = target.index;
      }
      const start = Math.max(0, realIndex - lines);
      const end = Math.min(source.length, realIndex + lines + 1);
      setContextLogs(source.slice(start, end));
      setTargetPosition(realIndex - start);
    },
    [logs, originalLogs, searchTerm, target.index, fromBackend]
  );

  useEffect(() => {
    compute(contextLines);
  }, [compute, contextLines]);

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-4xl">
        <DialogHeader className="flex-row flex-wrap items-center gap-3">
          <DialogTitle>{t('Log Context', { defaultValue: 'Log Context' })}</DialogTitle>
          <Input
            type="number"
            min={1}
            max={20}
            value={contextLines}
            onChange={(e) => {
              const value = parseInt(e.target.value, 10);
              if (!Number.isNaN(value) && value >= 1 && value <= 20) setContextLines(value);
            }}
            className="h-9 w-24"
          />
        </DialogHeader>
        <DialogBody>
          {loading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="size-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <table className="w-full text-left text-sm">
              <tbody>
                {contextLogs.map((log, index) => {
                  const badge = levelBadge(log.type);
                  const isTarget = index === targetPosition;
                  return (
                    <tr key={index} className={isTarget ? 'bg-muted ring-1 ring-inset ring-border' : ''}>
                      <td className="w-44 px-2 py-2 align-top font-mono text-[11px] text-muted-foreground">
                        {formatTimestamp(log.timestamp)}
                      </td>
                      <td className="w-20 px-2 py-2 align-top">
                        <Badge variant={badge.variant} className={badge.className}>
                          {log.type.toUpperCase()}
                        </Badge>
                      </td>
                      <td className="whitespace-pre-wrap break-words px-2 py-2 align-top font-mono text-xs">{log.message}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
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
