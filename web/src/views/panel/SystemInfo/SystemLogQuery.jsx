import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Code, Search, X } from 'lucide-react';

import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import { formatTimestamp, levelBadge, processLogEntry } from './logHelpers';

// ==============================|| PANEL — SYSTEM LOG QUERY ||============================== //
// Root-only advanced query against `/api/system_info/log/query` (RootAuth). Posts
// the logger.LogQueryParams contract { count, search_term, use_regex, from_file }
// and renders the returned { logs, total_count, has_more } result set.

export default function SystemLogQuery() {
  const { t } = useTranslation();

  const [count, setCount] = useState(50);
  const [searchTerm, setSearchTerm] = useState('');
  const [useRegex, setUseRegex] = useState(false);
  const [fromFile, setFromFile] = useState(false);
  const [logs, setLogs] = useState([]);
  const [totalCount, setTotalCount] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [hasQueried, setHasQueried] = useState(false);

  const runQuery = async () => {
    setLoading(true);
    setError('');
    try {
      const res = await API.post('/api/system_info/log/query', {
        count: Number(count) || 50,
        search_term: searchTerm,
        use_regex: useRegex,
        from_file: fromFile
      });
      if (res.data.success) {
        const result = res.data.data || {};
        setLogs((result.logs || []).map(processLogEntry));
        setTotalCount(result.total_count || 0);
        setHasMore(Boolean(result.has_more));
      } else {
        setError(res.data.message);
        showError(res.data.message);
      }
    } catch (err) {
      setError(err.message);
      showError(err.message);
    } finally {
      setLoading(false);
      setHasQueried(true);
    }
  };

  const handleCount = (e) => {
    const value = parseInt(e.target.value, 10);
    if (!Number.isNaN(value) && value > 0 && value <= 1000) setCount(value);
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('systemLogQuery.title')}</CardTitle>
        <p className="text-sm text-muted-foreground">{t('systemLogQuery.subtitle')}</p>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div className="flex flex-wrap items-end gap-3">
            <label className="flex flex-col gap-1 text-sm">
              {t('systemLogQuery.count')}
              <Input type="number" min={1} max={1000} value={count} onChange={handleCount} className="h-9 w-24" />
            </label>
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={useRegex} onCheckedChange={setUseRegex} />
              <Code className="size-4" /> {t('systemLogQuery.useRegex')}
            </label>
            <label className="flex items-center gap-2 text-sm" title={t('systemLogQuery.fromFileHint')}>
              <Switch checked={fromFile} onCheckedChange={setFromFile} />
              {t('systemLogQuery.fromFile')}
            </label>
          </div>
          <Button size="sm" onClick={runQuery} disabled={loading}>
            <Search className="mr-1 size-4" />
            {loading ? t('systemLogQuery.querying') : t('systemLogQuery.query')}
          </Button>
        </div>

        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && runQuery()}
            placeholder={t('systemLogQuery.searchPlaceholder')}
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

        {error && (
          <Alert variant="error">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {hasQueried && (
          <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
            <span>{t('systemLogQuery.totalCount', { count: totalCount })}</span>
            {hasMore && <span className="text-amber-600 dark:text-amber-400">{t('systemLogQuery.hasMore')}</span>}
          </div>
        )}

        <div className="max-h-[70vh] min-h-[200px] overflow-y-auto rounded-md border border-border">
          {logs.length === 0 ? (
            <div className="py-12 text-center text-sm text-muted-foreground">{t('systemLogQuery.noResults')}</div>
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
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
