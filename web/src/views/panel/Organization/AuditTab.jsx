import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { RefreshCw, ScrollText, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { ListEmptyState, ListFooter, ListSkeleton } from '@/components/list-page';
import { API } from 'utils/api';
import { showError, timestamp2string, trims } from 'utils/common';
import { relativeTime } from 'utils/relativeTime';
import { getPageSize, savePageSize } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';

// ==============================|| ORGANIZATION — AUDIT LOGS TAB (Admin+, T7) ||============================== //
// Paginated audit trail, newest first. Filters sit in one row: the action (a list of the known
// actions) and the actor ID (applied on Enter). Times are relative with the full time on hover;
// actions read as words, with the raw code kept as the tooltip.

// Every action the backend records, in the order they are offered in the filter.
export const AUDIT_ACTIONS = [
  'org.create',
  'org.update',
  'org.update_budget',
  'org.update_setting',
  'org.transfer_ownership',
  'org.dissolve',
  'org.invitation_create',
  'org.invitation_accept',
  'org.invitation_reject',
  'org.invitation_revoke',
  'org.member_account_create',
  'org.member_role_update',
  'org.member_limits_update',
  'org.member_remove',
  'org.member_leave',
  'quota.transfer',
  'billing.topup',
  'billing.order_create',
  'billing.admin_adjust',
  'token.create',
  'token.update',
  'token.delete',
  'routing_policy.create',
  'routing_policy.update',
  'routing_policy.status',
  'routing_policy.delete',
  'org.admin_enable',
  'org.admin_disable',
  'org.admin_max_members',
  'org.admin_dissolve'
];

const ALL = '__all__';

// An unknown code (a newer backend) falls back to the code itself.
export const auditActionLabel = (t, action) => t(`orgList.actions.${String(action).replace(/\./g, '_')}`, { defaultValue: action });

const actorLabel = (log) => log.actor_name || `#${log.actor_id}`;

export default function AuditTab({ orgId }) {
  const { t, i18n } = useTranslation();
  const rtf = useMemo(
    () => new Intl.RelativeTimeFormat((i18n.language || 'zh_CN').replace('_', '-'), { numeric: 'auto' }),
    [i18n.language]
  );
  const [logs, setLogs] = useState([]);
  const [count, setCount] = useState(0);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('org_audit'));
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
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
      onFinally: () => {
        setLoading(false);
        setLoaded(true);
      }
    });
  }, [orgId, page, rowsPerPage, filters]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const setAction = (value) => {
    setPage(0);
    setFilters((f) => ({ ...f, action: value === ALL ? '' : value }));
  };

  const applyActor = (e) => {
    e?.preventDefault();
    setPage(0);
    setFilters((f) => ({ ...f, actorId: trims(actorInput) }));
  };

  const time = (log) => (
    <span title={timestamp2string(log.created_time)} className="whitespace-nowrap">
      {relativeTime(rtf, log.created_time)}
    </span>
  );
  const action = (log) => <span title={log.action}>{auditActionLabel(t, log.action)}</span>;

  return (
    <Card className="overflow-hidden">
      <form className="flex items-center gap-2 border-b border-border p-3" onSubmit={applyActor}>
        <Select value={filters.action || ALL} onValueChange={setAction}>
          <SelectTrigger className="h-8 min-w-0 flex-1 sm:w-56 sm:flex-none" aria-label={t('orgPage.audit.action')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>{t('orgList.allActions')}</SelectItem>
            {AUDIT_ACTIONS.map((code) => (
              <SelectItem key={code} value={code}>
                {auditActionLabel(t, code)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="relative w-28 shrink-0 sm:w-40">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="h-8 pl-8"
            inputMode="numeric"
            placeholder={t('orgList.actorId')}
            aria-label={t('orgList.actorId')}
            value={actorInput}
            onChange={(e) => setActorInput(e.target.value.replace(/\D/g, ''))}
            onBlur={() => trims(actorInput) !== filters.actorId && applyActor()}
          />
        </div>
        <Button
          type="button"
          variant="outline"
          size="icon"
          className="ml-auto size-8 shrink-0"
          aria-label={t('orgPage.refresh')}
          title={t('orgPage.refresh')}
          onClick={fetchData}
          disabled={loading}
        >
          <RefreshCw className="size-4" />
        </Button>
      </form>

      {!loaded ? (
        <>
          <div className="hidden md:block">
            <ListSkeleton cells={[64, 96, 128, 320]} checkbox={false} />
          </div>
          <div className="md:hidden">
            <ListSkeleton layout="list" />
          </div>
        </>
      ) : logs.length === 0 ? (
        <ListEmptyState icon={ScrollText} title={t('orgList.noAudit')} />
      ) : (
        <>
          <div className="hidden md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-28">{t('orgPage.audit.time')}</TableHead>
                  <TableHead className="w-36">{t('orgPage.audit.actor')}</TableHead>
                  <TableHead className="w-48">{t('orgPage.audit.action')}</TableHead>
                  <TableHead>{t('orgPage.audit.content')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => (
                  <TableRow key={log.id}>
                    <TableCell className="text-muted-foreground">{time(log)}</TableCell>
                    <TableCell className="max-w-36 truncate" title={`#${log.actor_id}`}>
                      {actorLabel(log)}
                    </TableCell>
                    <TableCell className="whitespace-nowrap">{action(log)}</TableCell>
                    <TableCell className="max-w-0 truncate text-muted-foreground" title={log.content}>
                      {log.content}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          {/* Phone rows: the action and time on top, the actor and details below. */}
          <ul className="divide-y divide-border md:hidden">
            {logs.map((log) => (
              <li key={log.id} className="flex h-14 flex-col justify-center px-4" title={log.content}>
                <div className="flex items-baseline gap-3">
                  <span className="min-w-0 truncate text-sm font-medium">{action(log)}</span>
                  <span className="ml-auto shrink-0 text-xs text-muted-foreground">{time(log)}</span>
                </div>
                <div className="mt-0.5 truncate text-xs text-muted-foreground">
                  {actorLabel(log)}
                  {log.content ? ` · ${log.content}` : ''}
                </div>
              </li>
            ))}
          </ul>
        </>
      )}

      <div className="border-t border-border">
        <ListFooter
          total={count}
          page={page}
          pageSize={rowsPerPage}
          onPageChange={setPage}
          onPageSizeChange={(n) => {
            setRowsPerPage(n);
            setPage(0);
            savePageSize('org_audit', n);
          }}
        />
      </div>
    </Card>
  );
}

AuditTab.propTypes = {
  orgId: PropTypes.number.isRequired
};
