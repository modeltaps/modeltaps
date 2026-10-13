import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { CheckCircle2, Loader2, Play, Square, Timer, XCircle } from 'lucide-react';

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

const EMPTY_PROGRESS = { done: 0, total: 0, success: 0, failed: 0 };

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

  const start = useCallback(async () => {
    const targets = models.filter((m) => selectedSet.has(m));
    if (!item || targets.length === 0) return;

    stopRef.current = false;
    const runId = (runIdRef.current += 1);
    const isStopped = () => stopRef.current || runId !== runIdRef.current;
    setRunning(true);
    setResults({});
    setProgress({ ...EMPTY_PROGRESS, total: targets.length });

    let done = 0;
    let success = 0;
    let failed = 0;
    await runWithConcurrency(
      targets,
      TEST_CONCURRENCY,
      async (model) => {
        // Mark a model as testing only once it is picked up, so queued ones stay "not tested".
        setResults((prev) => ({ ...prev, [model]: { status: 'testing' } }));
        const res = await testChannelModel(item.id, model, endpointType === ENDPOINT_AUTO ? '' : endpointType);
        if (runId !== runIdRef.current) return;
        let result;
        if (res?.success) {
          result = { status: 'success', time: Number(res.time) || 0 };
          success += 1;
        } else {
          result = { status: 'error', message: res?.message || t('channelTest.failed') };
          failed += 1;
        }
        done += 1;
        setResults((prev) => ({ ...prev, [model]: result }));
        setProgress({ done, total: targets.length, success, failed });
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
  }, [endpointType, item, models, onFinished, selectedSet, t]);

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
              <SelectTrigger className="h-8 w-64" disabled={running} aria-label={t('channelTest.endpointType')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ENDPOINT_AUTO}>
                  {t('channelTest.endpointType')}: {t('channelTest.endpointAuto')}
                </SelectItem>
                {ENDPOINT_OPTIONS.map((o) => (
                  <SelectItem key={o.value} value={o.value}>
                    {t('channelTest.endpointType')}: {o.label}
                  </SelectItem>
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
                {progress.done}/{progress.total} · {t('channelTest.summary', { success: progress.success, failed: progress.failed })}
              </p>
            </div>
          )}

          {models.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">{t('channelTest.noModels')}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10" />
                  <TableHead>{t('channelTest.colModel')}</TableHead>
                  <TableHead className="w-28">{t('channelTest.colStatus')}</TableHead>
                  <TableHead>{t('channelTest.colResult')}</TableHead>
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
                    <TableCell>
                      <ResultCell t={t} result={results[model]} />
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
            <Button disabled={selected.length === 0} onClick={start}>
              <Play className="size-4" />
              {t('channelTest.start')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
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
  return (
    <span className="line-clamp-2 break-all text-xs text-destructive" title={result.message}>
      {result.message}
    </span>
  );
}

const resultShape = PropTypes.shape({ status: PropTypes.string, time: PropTypes.number, message: PropTypes.string });

StatusCell.propTypes = { t: PropTypes.func.isRequired, result: resultShape };
ResultCell.propTypes = { t: PropTypes.func.isRequired, result: resultShape };

ChannelTestDialog.propTypes = {
  item: PropTypes.object,
  open: PropTypes.bool,
  onClose: PropTypes.func.isRequired,
  onFinished: PropTypes.func
};
