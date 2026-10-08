import { useCallback, useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { RefreshCw, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { API } from 'utils/api';
import { showError, timestamp2string, trims } from 'utils/common';
import { PAGE_SIZE_OPTIONS, getPageSize, savePageSize } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import Pagination from '../components/Pagination';

// ==============================|| ORGANIZATION — AUDIT LOGS TAB (Admin+, T7) ||============================== //
// Paginated audit trail with actor / action filters; newest first.

export default function AuditTab({ orgId }) {
  const { t } = useTranslation();
  const [logs, setLogs] = useState([]);
  const [count, setCount] = useState(0);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('org_audit'));
  const [loading, setLoading] = useState(false);
  const [actionInput, setActionInput] = useState('');
  const [actorInput, setActorInput] = useState('');
  const [filters, setFilters] = useState({ action: '', actorId: '' });

  // reqId 守卫(UX-13):快速切页/切筛选/切组织时丢弃过期响应。
  const guardRef = useRef(null);
  if (!guardRef.current) guardRef.current = createRequestGuard();

  const fetchData = useCallback(() => {
    const params = { page: page + 1, size: rowsPerPage, order: '-created_time' };
    if (filters.action) params.action = filters.action;
    if (filters.actorId) params.actor_id = parseInt(filters.actorId, 10) || 0;
    return runGuardedFetch(guardRef.current, () => API.get(`/api/org/${orgId}/audit_logs`, { params }), {
      onStart: () => setLoading(true),
      onResult: (res) => {
        const { success, message, data } = res.data;
        if (success) {
          setLogs(data?.data || []);
          setCount(data?.total_count || 0);
        } else {
          showError(message);
        }
      },
      onError: (error) => console.error(error),
      onFinally: () => setLoading(false)
    });
  }, [orgId, page, rowsPerPage, filters]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const applyFilters = (e) => {
    e?.preventDefault();
    setPage(0);
    setFilters({ action: trims(actionInput), actorId: trims(actorInput) });
  };

  return (
    <Card>
      <form className="flex flex-col gap-2 border-b border-border p-3 sm:flex-row sm:items-center" onSubmit={applyFilters}>
        <Input
          className="sm:w-56"
          placeholder={t('orgPage.audit.actionFilter')}
          value={actionInput}
          onChange={(e) => setActionInput(e.target.value)}
        />
        <Input
          className="sm:w-40"
          type="number"
          placeholder={t('orgPage.audit.actorFilter')}
          value={actorInput}
          onChange={(e) => setActorInput(e.target.value)}
        />
        <div className="flex gap-2">
          <Button type="submit" variant="outline" size="sm">
            <Search className="size-4" /> {t('orgPage.audit.search')}
          </Button>
          <Button type="button" variant="outline" size="sm" onClick={fetchData} disabled={loading}>
            <RefreshCw className="size-4" />
          </Button>
        </div>
      </form>
      <div className="hidden md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('orgPage.audit.time')}</TableHead>
              <TableHead>{t('orgPage.audit.actor')}</TableHead>
              <TableHead>{t('orgPage.audit.action')}</TableHead>
              <TableHead>{t('orgPage.audit.content')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {logs.map((log) => (
              <TableRow key={log.id}>
                <TableCell className="whitespace-nowrap text-muted-foreground">{timestamp2string(log.created_time)}</TableCell>
                <TableCell className="tabular-nums">#{log.actor_id}</TableCell>
                <TableCell>
                  <Badge variant="outline" className="font-mono text-xs">
                    {log.action}
                  </Badge>
                </TableCell>
                <TableCell className="max-w-md break-words text-sm">{log.content}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <div className="md:hidden flex flex-col gap-3 p-3">
        {logs.map((log) => (
          <div key={log.id} className="rounded-lg border border-border bg-card p-3.5 shadow-sm">
            <div className="mb-2 text-sm font-medium text-muted-foreground">{timestamp2string(log.created_time)}</div>
            <dl className="flex flex-col divide-y divide-border/60">
              <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  {t('orgPage.audit.actor')}
                </dt>
                <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm tabular-nums">#{log.actor_id}</dd>
              </div>
              <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  {t('orgPage.audit.action')}
                </dt>
                <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                  <Badge variant="outline" className="font-mono text-xs">
                    {log.action}
                  </Badge>
                </dd>
              </div>
              <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  {t('orgPage.audit.content')}
                </dt>
                <dd className="flex min-w-0 flex-wrap justify-end gap-1 break-words text-right text-sm">{log.content}</dd>
              </div>
            </dl>
          </div>
        ))}
      </div>
      <Pagination
        page={page}
        rowsPerPage={rowsPerPage}
        count={count}
        options={PAGE_SIZE_OPTIONS}
        onPageChange={setPage}
        onRowsPerPageChange={(n) => {
          setRowsPerPage(n);
          setPage(0);
          savePageSize('org_audit', n);
        }}
      />
    </Card>
  );
}

AuditTab.propTypes = {
  orgId: PropTypes.number.isRequired
};
