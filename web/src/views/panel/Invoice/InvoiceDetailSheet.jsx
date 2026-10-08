import PropTypes from 'prop-types';
import { useEffect, useState } from 'react';
import { X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { API } from 'utils/api';
import { showError, calculateQuota, thousandsSeparator } from 'utils/common';

// ==============================|| INVOICE — MONTHLY DETAIL SHEET ||============================== //
// Right-side slide-over port of views/Invoice/detail.jsx: per-model breakdown + totals.

function Row({ label, value }) {
  return (
    <div className="flex items-start justify-between gap-4 py-1.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium text-foreground">{value}</span>
    </div>
  );
}
Row.propTypes = { label: PropTypes.string, value: PropTypes.node };

export default function InvoiceDetailSheet({ date, t, onClose }) {
  const [loading, setLoading] = useState(true);
  const [rows, setRows] = useState([]);
  const [user, setUser] = useState(null);

  useEffect(() => {
    let active = true;
    (async () => {
      setLoading(true);
      try {
        const res = await API.get('/api/user/invoice/detail', { params: { date: `${date}-01` } });
        const { success, message, data } = res.data;
        if (!active) return;
        if (success) {
          setRows(data || []);
          const userRes = await API.get('/api/user/self');
          if (active && userRes.data.success) setUser(userRes.data.data);
        } else {
          showError(message);
          onClose();
        }
      } catch (error) {
        console.error(error);
        if (active) onClose();
      } finally {
        if (active) setLoading(false);
      }
    })();
    return () => {
      active = false;
    };
  }, [date, onClose]);

  const totals = rows.reduce(
    (acc, r) => ({
      quota: acc.quota + r.quota,
      prompt: acc.prompt + r.prompt_tokens,
      completion: acc.completion + r.completion_tokens,
      count: acc.count + r.request_count,
      time: acc.time + r.request_time
    }),
    { quota: 0, prompt: 0, completion: 0, count: 0, time: 0 }
  );

  return (
    <div className="fixed inset-0 z-[1200]">
      <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={onClose} aria-hidden="true" />
      <div className="absolute inset-y-0 right-0 flex w-full max-w-2xl flex-col border-l border-border bg-card shadow-xl">
        <div className="flex items-center justify-between border-b border-border px-5 py-4">
          <h2 className="text-base font-semibold">
            {t('invoice_index.invoice')} · {date}
          </h2>
          <Button variant="ghost" size="icon" aria-label="Close" onClick={onClose}>
            <X className="size-5" />
          </Button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {loading ? (
            <p className="py-10 text-center text-sm text-muted-foreground">{t('dashboard_index.loading')}</p>
          ) : (
            <>
              <section className="mb-5">
                <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{t('invoice_index.userinfo')}</h3>
                <Row label={t('invoice_index.username')} value={user?.username} />
                <Row label={t('invoice_index.email')} value={user?.email} />
                <Row label={t('invoice_index.date')} value={date} />
              </section>

              <section className="mb-5">
                <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t('invoice_index.usage_statistics')}
                </h3>
                <Row label={t('invoice_index.promptTokens')} value={thousandsSeparator(totals.prompt)} />
                <Row label={t('invoice_index.completionTokens')} value={thousandsSeparator(totals.completion)} />
                <Row label={t('invoice_index.requestTime')} value={`${(totals.time / 1000).toFixed(3)}s`} />
                <Row label={t('invoice_index.requestCount')} value={thousandsSeparator(totals.count)} />
              </section>

              <section className="mb-5">
                <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t('invoice_index.usage_details')}
                </h3>
                <div className="hidden md:block overflow-x-auto rounded-md border border-border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('invoice_index.modelName')}</TableHead>
                        <TableHead className="text-right">{t('invoice_index.promptTokens')}</TableHead>
                        <TableHead className="text-right">{t('invoice_index.completionTokens')}</TableHead>
                        <TableHead className="text-right">{t('invoice_index.requestCount')}</TableHead>
                        <TableHead className="text-right">{t('invoice_index.requestTime')}</TableHead>
                        <TableHead className="text-right">{t('invoice_index.amount')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {rows.map((item, index) => (
                        <TableRow key={index}>
                          <TableCell className="font-medium">{item.model_name}</TableCell>
                          <TableCell className="text-right">{thousandsSeparator(item.prompt_tokens)}</TableCell>
                          <TableCell className="text-right">{thousandsSeparator(item.completion_tokens)}</TableCell>
                          <TableCell className="text-right">{thousandsSeparator(item.request_count)}</TableCell>
                          <TableCell className="text-right">{(item.request_time / 1000).toFixed(3)}s</TableCell>
                          <TableCell className="text-right">${calculateQuota(item.quota, 6)}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
                <div className="md:hidden flex flex-col gap-3">
                  {rows.map((item, index) => (
                    <div key={index} className="rounded-lg border border-border bg-card p-3.5 shadow-sm">
                      <div className="mb-2 text-sm font-medium">{item.model_name}</div>
                      <dl className="flex flex-col divide-y divide-border/60">
                        <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                          <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            {t('invoice_index.promptTokens')}
                          </dt>
                          <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                            {thousandsSeparator(item.prompt_tokens)}
                          </dd>
                        </div>
                        <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                          <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            {t('invoice_index.completionTokens')}
                          </dt>
                          <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                            {thousandsSeparator(item.completion_tokens)}
                          </dd>
                        </div>
                        <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                          <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            {t('invoice_index.requestCount')}
                          </dt>
                          <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                            {thousandsSeparator(item.request_count)}
                          </dd>
                        </div>
                        <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                          <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            {t('invoice_index.requestTime')}
                          </dt>
                          <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                            {(item.request_time / 1000).toFixed(3)}s
                          </dd>
                        </div>
                        <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                          <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            {t('invoice_index.amount')}
                          </dt>
                          <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">${calculateQuota(item.quota, 6)}</dd>
                        </div>
                      </dl>
                    </div>
                  ))}
                </div>
              </section>

              <div className="flex items-baseline justify-end gap-3">
                <span className="text-sm font-medium text-muted-foreground">{t('invoice_index.quota')}</span>
                <span className="text-xl font-bold text-foreground">${calculateQuota(totals.quota, 6)}</span>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

InvoiceDetailSheet.propTypes = {
  date: PropTypes.string.isRequired,
  t: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired
};
