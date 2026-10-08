import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ArrowDown, Code, RefreshCw, Search, Trash2, X } from 'lucide-react';

import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import LogContextDialog from './LogContextDialog';
import { filterLogs, formatTimestamp, levelBadge, processLogEntry } from './logHelpers';

// ==============================|| PANEL — SYSTEM LOGS ||============================== //
// shadcn port of v1 SystemInfo/components/SystemLogs. Auto-refresh, regex search,
// and per-entry context lookup against the unchanged `/api/system_info/log` API.

export default function SystemLogs() {
  const { t } = useTranslation();

  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshInterval, setRefreshInterval] = useState(5000);
  const [maxEntries, setMaxEntries] = useState(50);
  const [logs, setLogs] = useState([]);
  const [originalLogs, setOriginalLogs] = useState([]);
  const [searchTerm, setSearchTerm] = useState('');
  const [useRegex, setUseRegex] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [initialized, setInitialized] = useState(false);
  const [contextTarget, setContextTarget] = useState(null);
  const [showScrollDown, setShowScrollDown] = useState(false);

  const scrollRef = useRef(null);
  const userScrolledUpRef = useRef(false);

  const fetchLogs = useCallback(
    async (isAuto = false) => {
      if (!isAuto) setLoading(true);
      setError('');
      try {
        const res = await API.post('/api/system_info/log', { count: maxEntries });
        if (res.data.success) {
          const processed = res.data.data.map(processLogEntry);
          setOriginalLogs(processed);
          setLogs(filterLogs(processed, searchTerm, useRegex));
        } else {
          setError('Failed to fetch logs: ' + res.data.message);
          if (!isAuto) showError(res.data.message);
        }
      } catch (err) {
        setError('Error fetching logs: ' + err.message);
        if (!isAuto) showError(err.message);
      } finally {
        if (!isAuto) setLoading(false);
      }
    },
    [maxEntries, searchTerm, useRegex]
  );

  useEffect(() => {
    if (!initialized) {
      fetchLogs();
      setInitialized(true);
    }
  }, [fetchLogs, initialized]);

  useEffect(() => {
    if (initialized) setLogs(filterLogs(originalLogs, searchTerm, useRegex));
  }, [initialized, originalLogs, searchTerm, useRegex]);

  useEffect(() => {
    if (!autoRefresh || !initialized) return undefined;
    const interval = setInterval(() => fetchLogs(true), refreshInterval);
    return () => clearInterval(interval);
  }, [autoRefresh, initialized, refreshInterval, fetchLogs]);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    if (autoRefresh && !userScrolledUpRef.current) el.scrollTop = el.scrollHeight;
  }, [logs, autoRefresh]);

  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const atBottom = el.scrollTop >= el.scrollHeight - el.clientHeight - 50;
    userScrolledUpRef.current = !atBottom;
    setShowScrollDown(!atBottom && logs.length > 0);
  };

  const scrollToBottom = () => {
    const el = scrollRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
    userScrolledUpRef.current = false;
    setShowScrollDown(false);
  };

  const handleMaxEntries = (e) => {
    const value = parseInt(e.target.value, 10);
    if (!Number.isNaN(value) && value > 0 && value <= 999) setMaxEntries(value);
  };

  const handleInterval = (e) => {
    const seconds = Math.min(Math.max(parseInt(e.target.value, 10) || 1, 1), 60);
    setRefreshInterval(seconds * 1000);
  };

  return (
    <Card>
      <CardContent className="space-y-3 p-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-wrap items-center gap-3">
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={autoRefresh} onCheckedChange={setAutoRefresh} />
              {t('Auto Refresh', { defaultValue: 'Auto Refresh' })}
            </label>
            <Input type="number" min={1} max={60} value={refreshInterval / 1000} onChange={handleInterval} className="h-9 w-20" />
            <Input type="number" min={1} max={999} value={maxEntries} onChange={handleMaxEntries} className="h-9 w-20" />
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={() => fetchLogs()} disabled={loading}>
              <RefreshCw className={loading ? 'size-4 animate-spin' : 'size-4'} />
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setLogs([]);
                setOriginalLogs([]);
              }}
            >
              <Trash2 className="size-4 text-destructive" />
            </Button>
          </div>
        </div>

        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <div className="relative flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              placeholder={
                useRegex
                  ? t('Enter regular expression...', { defaultValue: 'Enter regular expression...' })
                  : t('Search logs...', { defaultValue: 'Search logs...' })
              }
              className="h-9 pl-9 pr-9"
            />
            {searchTerm && (
              <button
                type="button"
                onClick={() => setSearchTerm('')}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              >
                <X className="size-4" />
              </button>
            )}
          </div>
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={useRegex} onCheckedChange={setUseRegex} />
            <Code className="size-4" /> {t('Regex', { defaultValue: 'Regex' })}
          </label>
          <span className="whitespace-nowrap text-sm text-muted-foreground">
            {searchTerm ? `${logs.length} results` : `${logs.length} logs`}
          </span>
        </div>

        {error && (
          <Alert variant="error">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        <div className="relative">
          {showScrollDown && (
            <Button
              variant="outline"
              size="icon"
              onClick={scrollToBottom}
              className="absolute bottom-3 left-1/2 z-10 size-9 -translate-x-1/2 rounded-full shadow-md"
            >
              <ArrowDown className="size-4" />
            </Button>
          )}
          <div
            ref={scrollRef}
            onScroll={handleScroll}
            className="max-h-[70vh] min-h-[300px] overflow-y-auto rounded-md border border-border"
          >
            {logs.length === 0 ? (
              <div className="py-12 text-center text-sm text-muted-foreground">
                {searchTerm
                  ? t('No matching logs found', { defaultValue: 'No matching logs found' })
                  : t('No logs available', { defaultValue: 'No logs available' })}
              </div>
            ) : (
              <table className="w-full text-left text-sm">
                <tbody>
                  {logs.map((log, index) => {
                    const badge = levelBadge(log.type);
                    return (
                      <tr key={index} className="border-b border-border last:border-0 hover:bg-muted/50">
                        <td className="w-44 px-3 py-2 align-top font-mono text-[11px] text-muted-foreground">
                          {formatTimestamp(log.timestamp)}
                        </td>
                        <td className="w-20 px-3 py-2 align-top">
                          <Badge variant={badge.variant} className={badge.className}>
                            {log.type.toUpperCase()}
                          </Badge>
                        </td>
                        <td className="whitespace-pre-wrap break-words px-3 py-2 align-top font-mono text-xs">{log.message}</td>
                        <td className="w-12 px-3 py-2 align-top">
                          <button
                            type="button"
                            onClick={() => setContextTarget({ index, log })}
                            className="text-muted-foreground hover:text-foreground"
                            title={t('View Context', { defaultValue: 'View Context' })}
                          >
                            <Code className="size-4" />
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </CardContent>

      {contextTarget && (
        <LogContextDialog
          target={contextTarget}
          logs={logs}
          originalLogs={originalLogs}
          searchTerm={searchTerm}
          onClose={() => setContextTarget(null)}
        />
      )}
    </Card>
  );
}
