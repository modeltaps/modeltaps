import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { CheckCircle2, Loader2, Play, RotateCcw, Square, Timer, XCircle } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table';
import { toast } from '@/components/ui/sonner';
import { testChannelModel } from './channelApi';
import {
  ENDPOINT_AUTO,
  ENDPOINT_OPTIONS,
  TEST_CONCURRENCY,
  channelTestModels,
  defaultTestSelection,
  runWithConcurrency
} from './channelTestUtils';

const EMPTY_PROGRESS = { done: 0, total: 0 };

// Tests several models of one channel through the regular channel test endpoint, a few at a
// time, and lists the latency or error of each. A manual test never disables the channel.
export default function ChannelTestDialog({ item, open, onClose, onFinished }) {
  const { t } = useTranslation();
  const stopRef = useRef(false);
  // Bumped on close / restart so late results of an earlier run are dropped.
  const runIdRef = useRef(0);
  const [selected, setSelected] = useState([]);
  const [results, setResults] = useState({});
  const [running, setRunning] = useState(false);
  const [progress, setProgress] = useState(EMPTY_PROGRESS);
  const [endpointType, setEndpointType] = useState(ENDPOINT_AUTO);

  const models = useMemo(() => channelTestModels(item), [item]);
  const selectedSet = useMemo(() => new Set(selected), [selected]);

  useEffect(() => {
    if (!open) return undefined;
    setSelected(defaultTestSelection(models));
    setResults({});
    setRunning(false);
    setProgress(EMPTY_PROGRESS);
    setEndpointType(ENDPOINT_AUTO);
    return () => {
      stopRef.current = true;
      runIdRef.current += 1;
    };
  }, [open, models]);

  const toggleModel = useCallback((model) => {
    setSelected((prev) => (prev.includes(model) ? prev.filter((m) => m !== model) : [...prev, model]));
  }, []);

  // testOne tests one model and writes its row; returns null when its run was dropped.
  const testOne = useCallback(
    async (model, runId) => {
      // Mark a model as testing only once it is picked up, so queued ones keep their state.
      setResults((prev) => ({ ...prev, [model]: { status: 'testing' } }));
      const res = await testChannelModel(item.id, model, endpointType === ENDPOINT_AUTO ? '' : endpointType);
      if (runId !== runIdRef.current) return null;
      const result = res?.success
        ? { status: 'success', time: Number(res.time) || 0 }
        : { status: 'error', message: res?.message || t('channelTest.failed') };
      setResults((prev) => ({ ...prev, [model]: result }));
      return result;
    },
    [endpointType, item, t]
  );

  // runBatch tests targets; with reset false the other rows keep their results (retry failed).
  const runBatch = useCallback(
    async (targets, reset) => {
      if (!item || targets.length === 0) return;

      stopRef.current = false;
      const runId = (runIdRef.current += 1);
      const isStopped = () => stopRef.current || runId !== runIdRef.current;
      setRunning(true);
      if (reset) setResults({});
      setProgress({ done: 0, total: targets.length });

      let done = 0;
      let success = 0;
      let failed = 0;
      await runWithConcurrency(
        targets,
        TEST_CONCURRENCY,
        async (model) => {
          const result = await testOne(model, runId);
          if (!result) return;
          done += 1;
          if (result.status === 'success') success += 1;
          else failed += 1;
          setProgress({ done, total: targets.length });
        },
        isStopped
      );
      if (runId !== runIdRef.current) return;

      setRunning(false);
      onFinished?.();
      if (stopRef.current && done < targets.length) {
        toast.info(t('channelTest.stopped', { success, failed }));
      } else if (failed > 0) {
        toast.error(t('channelTest.summary', { success, failed }));
      } else {
        toast.success(t('channelTest.summary', { success, failed }));
      }
    },
    [item, onFinished, t, testOne]
  );

  const start = useCallback(
    () =>
      runBatch(
        models.filter((m) => selectedSet.has(m)),
        true
      ),
    [models, runBatch, selectedSet]
  );
  const failedModels = useMemo(() => models.filter((m) => results[m]?.status === 'error'), [models, results]);
  // A single-row test joins the current run instead of starting one, so closing the dialog drops it too.
  const testRow = useCallback((model) => testOne(model, runIdRef.current), [testOne]);

  // Counted from the rows, so single-row tests and retries keep the summary right.
  const counts = useMemo(() => {
    const values = Object.values(results);
    return {
      success: values.filter((r) => r.status === 'success').length,
      failed: values.filter((r) => r.status === 'error').length,
      testing: values.filter((r) => r.status === 'testing').length
    };
  }, [results]);
  // While single-row tests are in flight no batch may start, and single-row tests are capped at
  // the batch concurrency, so the two never stack up against the upstream.
  const busy = running || counts.testing > 0;

  const percent = progress.total === 0 ? 0 : Math.round((progress.done / progress.total) * 100);

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>{t('channelTest.title')}</DialogTitle>
          <DialogDescription>
            {item?.name} · #{item?.id}
          </DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <p className="text-xs text-muted-foreground">{t('channelTest.hint')}</p>

          <div className="flex flex-wrap items-center gap-2">
            <Select value={endpointType} onValueChange={setEndpointType}>
              <SelectTrigger className="h-8 w-80" disabled={busy} aria-label={t('channelTest.endpointType')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {/* SelectValue only shows an item's label when its children is a single string. */}
                <SelectItem value={ENDPOINT_AUTO}>{`${t('channelTest.endpointType')}: ${t('channelTest.endpointAuto')}`}</SelectItem>
                {ENDPOINT_OPTIONS.map((o) => (
                  <SelectItem key={o.value} value={o.value}>{`${t('channelTest.endpointType')}: ${o.label}`}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button variant="outline" size="sm" disabled={running} onClick={() => setSelected(models)}>
              {t('channelTest.selectAll')}
            </Button>
            <Button variant="outline" size="sm" disabled={running} onClick={() => setSelected([])}>
              {t('channelTest.clear')}
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={running}
              onClick={() => setSelected((prev) => models.filter((m) => !prev.includes(m)))}
            >
              {t('channelTest.invert')}
            </Button>
            <span className="ml-auto text-xs text-muted-foreground">{t('channelTest.selected', { count: selected.length })}</span>
          </div>

          {progress.total > 0 && (
            <div className="space-y-1">
              <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                <div className="h-full bg-primary transition-all" style={{ width: `${percent}%` }} />
              </div>
              <p className="text-xs text-muted-foreground tabular-nums">
                {progress.done}/{progress.total} · {t('channelTest.summary', { success: counts.success, failed: counts.failed })}
              </p>
            </div>
          )}

          {models.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">{t('channelTest.noModels')}</p>
          ) : (
            <Table className="table-fixed">
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10" />
                  <TableHead className="w-[38%]">{t('channelTest.colModel')}</TableHead>
                  <TableHead className="w-24">{t('channelTest.colStatus')}</TableHead>
                  <TableHead>{t('channelTest.colResult')}</TableHead>
                  <TableHead className="w-12 text-center">
                    <span className="sr-only">{t('channelTest.colAction')}</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {models.map((model) => (
                  <TableRow key={model}>
                    <TableCell>
                      <Checkbox
                        checked={selectedSet.has(model)}
                        disabled={running}
                        aria-label={model}
                        onCheckedChange={() => toggleModel(model)}
                      />
                    </TableCell>
                    <TableCell className="break-all font-mono text-xs">{model}</TableCell>
                    <TableCell>
                      <StatusCell t={t} result={results[model]} />
                    </TableCell>
                    <TableCell className="whitespace-normal">
                      <ResultCell t={t} result={results[model]} />
                    </TableCell>
                    <TableCell className="text-center">
                      <RowTestButton
                        t={t}
                        model={model}
                        retry={results[model]?.status === 'error'}
                        disabled={running || counts.testing >= TEST_CONCURRENCY || results[model]?.status === 'testing'}
                        onClick={() => testRow(model)}
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {t('common.close', { defaultValue: 'Close' })}
          </Button>
          {running ? (
            <Button variant="destructive" onClick={() => (stopRef.current = true)}>
              <Square className="size-4" />
              {t('channelTest.stop')}
            </Button>
          ) : (
            <>
              {failedModels.length > 0 && (
                <Button variant="outline" disabled={busy} onClick={() => runBatch(failedModels, false)}>
                  <RotateCcw className="size-4" />
                  {t('channelTest.retryFailed', { count: failedModels.length })}
                </Button>
              )}
              <Button disabled={selected.length === 0 || busy} onClick={start}>
                <Play className="size-4" />
                {t('channelTest.start')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function RowTestButton({ t, model, retry, disabled, onClick }) {
  const label = t(retry ? 'channelTest.retry' : 'channelTest.testOne', { model });
  return (
    <Button variant="ghost" size="icon" className="size-7" aria-label={label} title={label} disabled={disabled} onClick={onClick}>
      {retry ? <RotateCcw className="size-4" /> : <Play className="size-4" />}
    </Button>
  );
}

function StatusCell({ t, result }) {
  if (!result) return <span className="text-xs text-muted-foreground">{t('channelTest.notTested')}</span>;
  if (result.status === 'testing') {
    return (
      <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
        <Loader2 className="size-3 animate-spin" />
        {t('channelTest.testing')}
      </span>
    );
  }
  if (result.status === 'success') {
    return (
      <span className="inline-flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
        <CheckCircle2 className="size-3.5" />
        {t('channelTest.passed')}
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 text-xs text-destructive">
      <XCircle className="size-3.5" />
      {t('channelTest.failed')}
    </span>
  );
}

function ResultCell({ t, result }) {
  if (!result || result.status === 'testing') return <span className="text-xs text-muted-foreground">-</span>;
  if (result.status === 'success') {
    return (
      <span className="inline-flex items-center gap-1 text-xs tabular-nums">
        <Timer className="size-3.5 text-muted-foreground" />
        {t('channelTest.seconds', { time: result.time.toFixed(2) })}
      </span>
    );
  }
  // The status column already says the test failed; drop the backend's English lead-in.
  const message = (result.message || '').replace(/^speed test failed, reason:\s*/i, '');
  return (
    <span className="line-clamp-3 block break-words text-xs text-destructive" title={message}>
      {message}
    </span>
  );
}

const resultShape = PropTypes.shape({ status: PropTypes.string, time: PropTypes.number, message: PropTypes.string });

RowTestButton.propTypes = {
  t: PropTypes.func.isRequired,
  model: PropTypes.string.isRequired,
  retry: PropTypes.bool,
  disabled: PropTypes.bool,
  onClick: PropTypes.func.isRequired
};
StatusCell.propTypes = { t: PropTypes.func.isRequired, result: resultShape };
ResultCell.propTypes = { t: PropTypes.func.isRequired, result: resultShape };

ChannelTestDialog.propTypes = {
  item: PropTypes.object,
  open: PropTypes.bool,
  onClose: PropTypes.func.isRequired,
  onFinished: PropTypes.func
};
