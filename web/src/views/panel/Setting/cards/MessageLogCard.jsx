import { useCallback, useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { CheckCircle2, ChevronLeft, ChevronRight, RotateCw, XCircle } from 'lucide-react';

import { API } from 'utils/api';
import { timestamp2string } from 'utils/common';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { CollectionCard } from '@/components/settings/cards';
import { cardText } from './OptionCard';
import { NotifyChannelIcon } from '../notifyChannels';

// ==============================|| PLATFORM SETTINGS — SEND RECORDS ||============================== //
// What the platform tried to send in the last 30 days, newest first: every email (source "mail")
// or every notification channel delivery (source "notify"), with the error when it failed.

const PAGE_SIZE = 10;

export function MessageLogCard({ source, id }) {
  const { t } = useTranslation();
  const l = (k, o) => t(`settingsUi.platform.messageLog.${k}`, o);
  const [rows, setRows] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [failedOnly, setFailedOnly] = useState(false);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/option/message_log', { params: { source, page, size: PAGE_SIZE, failed: failedOnly } });
      if (res.data?.success) {
        setRows(res.data.data.items || []);
        setTotal(res.data.data.total || 0);
      }
    } catch {
      /* surfaced by the API interceptor */
    } finally {
      setLoading(false);
    }
  }, [source, page, failedOnly]);

  useEffect(() => {
    load();
  }, [load]);

  const kindLabel = (kind) => {
    if (!kind) return l('kinds.general');
    if (source === 'mail') return l(`kinds.${kind}`, { defaultValue: kind });
    if (kind === 'test') return l('kinds.test');
    return t(`setting_index.notifications.kind.${kind}`, { defaultValue: kind });
  };
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const status = (row) =>
    row.success ? (
      <span className="inline-flex items-center gap-1.5 text-[13px] text-emerald-600 dark:text-emerald-400">
        <CheckCircle2 className="size-3.5" />
        {l('sent')}
      </span>
    ) : (
      <span className="inline-flex max-w-[16rem] items-center gap-1.5 text-[13px] text-destructive" title={row.error}>
        <XCircle className="size-3.5 shrink-0" />
        <span className="truncate">{row.error || l('failed')}</span>
      </span>
    );

  return (
    <CollectionCard
      title={cardText(t, id, 'title')}
      description={cardText(t, id, 'description')}
      highlight={id}
      loading={loading}
      empty={failedOnly ? l('emptyFailed') : l('empty')}
      rows={rows}
      aside={
        <>
          <Button
            variant="outline"
            size="sm"
            aria-pressed={failedOnly}
            className={cn(failedOnly && 'border-destructive/50 text-destructive')}
            onClick={() => {
              setPage(1);
              setFailedOnly((v) => !v);
            }}
          >
            <XCircle className="size-4" />
            {l('failedOnly')}
          </Button>
          <Button variant="outline" size="sm" onClick={load} aria-label={l('refresh')}>
            <RotateCw className={cn('size-4', loading && 'animate-spin')} />
          </Button>
        </>
      }
      columns={[
        {
          key: 'time',
          header: l('columns.time'),
          className: 'whitespace-nowrap',
          cell: (row) => <span className="text-xs tabular-nums text-muted-foreground">{timestamp2string(row.created_at)}</span>
        },
        { key: 'kind', header: l('columns.kind'), className: 'whitespace-nowrap', cell: (row) => kindLabel(row.kind) },
        {
          key: 'target',
          header: l('columns.target'),
          className: 'max-w-[14rem]',
          cell: (row) => (
            <span className="inline-flex min-w-0 max-w-full items-center gap-2" title={row.target}>
              {source === 'notify' && <NotifyChannelIcon type={row.channel} />}
              <span className="truncate">{row.target}</span>
            </span>
          )
        },
        {
          key: 'title',
          header: l('columns.title'),
          className: 'max-w-[16rem]',
          cell: (row) => (
            <span className="block truncate text-muted-foreground" title={row.title}>
              {row.title}
            </span>
          )
        },
        { key: 'status', header: t('settingsUi.collection.status'), cell: status }
      ]}
      phoneRow={(row) => ({ primary: `${kindLabel(row.kind)} · ${row.target}`, secondary: status(row) })}
      footer={
        total > PAGE_SIZE && (
          <div className="flex items-center justify-end gap-2 text-xs text-muted-foreground">
            <span>{l('pager', { page, pages, total })}</span>
            <Button variant="ghost" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)} aria-label={l('prev')}>
              <ChevronLeft className="size-4" />
            </Button>
            <Button variant="ghost" size="sm" disabled={page >= pages} onClick={() => setPage((p) => p + 1)} aria-label={l('next')}>
              <ChevronRight className="size-4" />
            </Button>
          </div>
        )
      }
    />
  );
}

MessageLogCard.propTypes = { source: PropTypes.oneOf(['mail', 'notify']).isRequired, id: PropTypes.string.isRequired };

export const MailLogsCard = () => <MessageLogCard source="mail" id="mail-logs" />;
export const NotifyLogsCard = () => <MessageLogCard source="notify" id="notify-logs" />;
