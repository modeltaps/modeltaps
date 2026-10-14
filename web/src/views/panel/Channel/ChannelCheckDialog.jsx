import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import {
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  Loader2,
  MinusCircle,
  Play,
  RefreshCw,
  RotateCcw,
  Square,
  XCircle
} from 'lucide-react';

import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import CodeBlock from '@/components/ui/code-block';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table';
import { toast } from '@/components/ui/sonner';
import ConfirmDialog from './ConfirmDialog';
import { checkChannelModel, fetchProviderModels } from './channelApi';
import { channelTestModels, defaultTestSelection, runWithConcurrency } from './channelTestUtils';
import {
  CHECK_CONCURRENCY,
  PROBE_KEYS,
  REQUESTS_PER_MODEL,
  STATUS,
  describeResult,
  describeResults,
  isFinished,
  rowOutcome
} from './channelCheckUtils';

const STATUS_META = {
  [STATUS.SUCCESS]: { Icon: CheckCircle2, className: 'text-emerald-600 dark:text-emerald-400', labelKey: 'channelCheck.status.passed' },
  [STATUS.WARNING]: { Icon: AlertTriangle, className: 'text-amber-600 dark:text-amber-400', labelKey: 'channelCheck.status.warning' },
  [STATUS.FAILED]: { Icon: XCircle, className: 'text-destructive', labelKey: 'channelCheck.status.failed' },
  [STATUS.SKIPPED]: { Icon: MinusCircle, className: 'text-muted-foreground', labelKey: 'channelCheck.status.skipped' }
};

const metaOf = (status) => STATUS_META[status] || STATUS_META[STATUS.FAILED];

// Checks the models of one channel for real availability: a random arithmetic challenge, image
// input, JSON schema output, tool calling and a relay hint. One request per model, two models at
// a time; every check sends real requests to the upstream and spends its quota.
export default function ChannelCheckDialog({ item, open, onClose }) {
  const { t } = useTranslation();
  const stopRef = useRef(false);
  // Bumped on close / restart so late results of an earlier run are dropped.
  const runIdRef = useRef(0);
  const [models, setModels] = useState([]);
  const [selected, setSelected] = useState([]);
  const [results, setResults] = useState({});
  const [expanded, setExpanded] = useState({});
  const [running, setRunning] = useState(false);
  const [fetching, setFetching] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [progress, setProgress] = useState({ done: 0, total: 0 });
  const selectedSet = useMemo(() => new Set(selected), [selected]);

  useEffect(() => {
    if (!open) return undefined;
    const initial = channelTestModels(item);
    setModels(initial);
    // Deep research and media models stay unselected: each check sends several real requests.
    setSelected(defaultTestSelection(initial));
    setResults({});
    setExpanded({});
    setRunning(false);
    setFetching(false);
    setConfirming(false);
    setProgress({ done: 0, total: 0 });
    return () => {
      stopRef.current = true;
      runIdRef.current += 1;
    };
  }, [open, item]);

  const fetchUpstream = useCallback(async () => {
    const runId = runIdRef.current;
    setFetching(true);
    try {
      const list = await fetchProviderModels(item);
      if (runId !== runIdRef.current) return;
      setModels(list);
      setSelected(defaultTestSelection(list));
      setResults({});
      setExpanded({});
      setProgress({ done: 0, total: 0 });
    } catch (error) {
      if (runId === runIdRef.current) toast.error(error.message || t('channelCheck.fetchFailed'));
    } finally {
      if (runId === runIdRef.current) setFetching(false);
    }
  }, [item, t]);

  const toggleModel = useCallback((model) => {
    setSelected((prev) => (prev.includes(model) ? prev.filter((m) => m !== model) : [...prev, model]));
  }, []);

  // checkOne checks one model and writes its row; returns null when its run was dropped.
  const checkOne = useCallback(
    async (model, runId) => {
      setResults((prev) => ({ ...prev, [model]: { status: 'checking' } }));
      let rowResult;
      try {
        rowResult = { status: 'done', data: await checkChannelModel(item.id, model) };
      } catch (error) {
        rowResult = { status: 'error', message: error.message || t('channelCheck.status.failed') };
      }
      if (runId !== runIdRef.current) return null;
      setResults((prev) => ({ ...prev, [model]: rowResult }));
      return rowResult;
    },
    [item, t]
  );

  // runBatch checks targets; with reset false the other rows keep their results (retry failed).
  const runBatch = useCallback(
    async (targets, reset) => {
      if (!item || targets.length === 0) return;
      stopRef.current = false;
      const runId = (runIdRef.current += 1);
      const isStopped = () => stopRef.current || runId !== runIdRef.current;
      setRunning(true);
      if (reset) {
        setResults({});
        setExpanded({});
      }
      setProgress({ done: 0, total: targets.length });

      let done = 0;
      const counts = { passed: 0, warning: 0, failed: 0 };
      await runWithConcurrency(
        targets,
        CHECK_CONCURRENCY,
        async (model) => {
          const rowResult = await checkOne(model, runId);
          if (!rowResult) return;
          counts[rowOutcome(rowResult)] += 1;
          done += 1;
          setProgress({ done, total: targets.length });
        },
        isStopped
      );
      if (runId !== runIdRef.current) return;

      setRunning(false);
      const summary = t('channelCheck.summary', counts);
      if (stopRef.current && done < targets.length) toast.info(t('channelCheck.stopped', { summary }));
      else if (counts.failed > 0) toast.error(summary);
      else toast.success(summary);
    },
    [checkOne, item, t]
  );

  const targets = useMemo(() => models.filter((m) => selectedSet.has(m)), [models, selectedSet]);
  const failedModels = useMemo(
    () => models.filter((m) => isFinished(results[m]) && rowOutcome(results[m]) === 'failed'),
    [models, results]
  );
  // A single-row check joins the current run instead of starting one, so closing the dialog drops it too.
  const checkRow = useCallback((model) => checkOne(model, runIdRef.current), [checkOne]);

  // While single-row checks are in flight no batch may start, and single-row checks are capped at
  // the batch concurrency, so the two never stack up against the upstream.
  const checkingCount = useMemo(() => Object.values(results).filter((r) => r.status === 'checking').length, [results]);
  const busy = running || checkingCount > 0;
  const percent = progress.total === 0 ? 0 : Math.round((progress.done / progress.total) * 100);
  const columnCount = PROBE_KEYS.length + 5;

  return (
    // Escape while the confirmation is open only closes the confirmation.
    <Dialog open={open} onOpenChange={(v) => !v && !confirming && onClose()}>
      <DialogContent className="max-w-5xl">
        <DialogHeader>
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <DialogTitle>{t('channelCheck.title')}</DialogTitle>
              <DialogDescription>
                {item?.name} · #{item?.id}
              </DialogDescription>
            </div>
            <Button variant="outline" size="sm" disabled={busy || fetching} onClick={fetchUpstream}>
              {fetching ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              {t('channelCheck.fetchUpstream')}
            </Button>
          </div>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <Alert variant="warning">
            <AlertTriangle />
            <AlertDescription>{t('channelCheck.hint', { requests: REQUESTS_PER_MODEL })}</AlertDescription>
          </Alert>

          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" size="sm" disabled={running} onClick={() => setSelected(models)}>
              {t('channelCheck.selectAll')}
            </Button>
            <Button variant="outline" size="sm" disabled={running} onClick={() => setSelected([])}>
              {t('channelCheck.clear')}
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={running}
              onClick={() => setSelected((prev) => models.filter((m) => !prev.includes(m)))}
            >
              {t('channelCheck.invert')}
            </Button>
            <span className="ml-auto text-xs text-muted-foreground">{t('channelCheck.selected', { count: selected.length })}</span>
          </div>

          {progress.total > 0 && (
            <div className="space-y-1">
              <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                <div className="h-full bg-primary transition-all" style={{ width: `${percent}%` }} />
              </div>
              <p className="text-xs text-muted-foreground tabular-nums">
                {progress.done}/{progress.total}
              </p>
            </div>
          )}

          {models.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">{t('channelCheck.noModels')}</p>
          ) : (
            <Table className="table-fixed">
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10" />
                  <TableHead>{t('channelCheck.colModel')}</TableHead>
                  <TableHead className="w-24">{t('channelCheck.colStatus')}</TableHead>
                  {PROBE_KEYS.map((key) => (
                    <TableHead key={key} className="w-16 text-center">
                      {t(`channelCheck.probe.${key}`)}
                    </TableHead>
                  ))}
                  <TableHead className="w-12 text-center">
                    <span className="sr-only">{t('channelCheck.colAction')}</span>
                  </TableHead>
                  <TableHead className="w-10" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {models.map((model) => {
                  const result = results[model];
                  const probes = result?.status === 'done' ? Object.fromEntries((result.data.items || []).map((p) => [p.key, p])) : {};
                  const finished = isFinished(result);
                  const failed = finished && rowOutcome(result) === 'failed';
                  return (
                    <Fragment key={model}>
                      <TableRow data-model={model}>
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
                          <OverallCell t={t} result={result} />
                        </TableCell>
                        {PROBE_KEYS.map((key) => (
                          <TableCell key={key} className="text-center">
                            <ProbeCell t={t} probeKey={key} probe={probes[key]} />
                          </TableCell>
                        ))}
                        <TableCell className="text-center">
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-7"
                            aria-label={t(failed ? 'channelCheck.retry' : 'channelCheck.checkOne', { model })}
                            title={t(failed ? 'channelCheck.retry' : 'channelCheck.checkOne', { model })}
                            disabled={running || fetching || checkingCount >= CHECK_CONCURRENCY || result?.status === 'checking'}
                            onClick={() => checkRow(model)}
                          >
                            {failed ? <RotateCcw className="size-4" /> : <Play className="size-4" />}
                          </Button>
                        </TableCell>
                        <TableCell>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-7"
                            disabled={!finished}
                            aria-expanded={!!expanded[model]}
                            aria-label={t('channelCheck.details', { model })}
                            onClick={() => setExpanded((prev) => ({ ...prev, [model]: !prev[model] }))}
                          >
                            {expanded[model] ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
                          </Button>
                        </TableCell>
                      </TableRow>
                      {finished && expanded[model] && (
                        <TableRow className="hover:bg-transparent">
                          <TableCell colSpan={columnCount} className="whitespace-normal bg-muted/30">
                            <DetailPanel t={t} result={result} />
                          </TableCell>
                        </TableRow>
                      )}
                    </Fragment>
                  );
                })}
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
              {t('channelCheck.stop')}
            </Button>
          ) : (
            <>
              {failedModels.length > 0 && (
                <Button variant="outline" disabled={busy || fetching} onClick={() => runBatch(failedModels, false)}>
                  <RotateCcw className="size-4" />
                  {t('channelCheck.retryFailed', { count: failedModels.length })}
                </Button>
              )}
              <Button disabled={targets.length === 0 || busy || fetching} onClick={() => setConfirming(true)}>
                <Play className="size-4" />
                {t('channelCheck.start')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
      <ConfirmDialog
        open={confirming}
        title={t('channelCheck.confirmTitle')}
        content={t('channelCheck.confirmBody', { models: targets.length, requests: targets.length * REQUESTS_PER_MODEL })}
        onCancel={() => setConfirming(false)}
        onConfirm={() => {
          setConfirming(false);
          runBatch(targets, true);
        }}
      />
    </Dialog>
  );
}

function OverallCell({ t, result }) {
  if (!result) return <span className="text-xs text-muted-foreground">{t('channelCheck.notChecked')}</span>;
  if (result.status === 'checking') {
    return (
      <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
        <Loader2 className="size-3 animate-spin" />
        {t('channelCheck.checking')}
      </span>
    );
  }
  const meta = result.status === 'error' ? metaOf(STATUS.FAILED) : metaOf(result.data.status);
  const { Icon } = meta;
  return (
    <span
      className={`inline-flex items-center gap-1 text-xs ${meta.className}`}
      title={result.status === 'error' ? result.message : undefined}
    >
      <Icon className="size-3.5" />
      {t(meta.labelKey)}
    </span>
  );
}

function ProbeCell({ t, probeKey, probe }) {
  if (!probe) return <span className="text-xs text-muted-foreground">-</span>;
  const meta = metaOf(probe.status);
  const { Icon } = meta;
  const label = `${t(`channelCheck.probe.${probeKey}`)}: ${t(meta.labelKey)}`;
  return (
    <span className={`inline-flex ${meta.className}`} role="img" aria-label={label} title={describeResults(t, probe.results)}>
      <Icon className="size-4" />
    </span>
  );
}

function DetailPanel({ t, result }) {
  const [openResponse, setOpenResponse] = useState({});

  if (result.status === 'error') {
    return <p className="break-words text-xs text-destructive">{result.message}</p>;
  }

  return (
    <div className="space-y-3">
      {(result.data.items || []).map((probe) => {
        const meta = metaOf(probe.status);
        const { Icon } = meta;
        return (
          <div key={probe.key} className="space-y-1.5">
            <div className="flex flex-wrap items-center gap-2">
              <Icon className={`size-4 ${meta.className}`} />
              <span className="text-sm font-medium">{t(`channelCheck.probe.${probe.key}`, { defaultValue: probe.key })}</span>
              {probe.category && (
                <Badge variant="outline" className="border-destructive/40 text-destructive">
                  {t(`channelCheck.category.${probe.category}`, { defaultValue: probe.category })}
                </Badge>
              )}
              {probe.latency_ms > 0 && (
                <span className="text-xs tabular-nums text-muted-foreground">
                  {t('channelCheck.seconds', { time: (probe.latency_ms / 1000).toFixed(2) })}
                </span>
              )}
              {probe.response && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="ml-auto h-7"
                  aria-expanded={!!openResponse[probe.key]}
                  onClick={() => setOpenResponse((prev) => ({ ...prev, [probe.key]: !prev[probe.key] }))}
                >
                  {t('channelCheck.rawResponse')}
                  {openResponse[probe.key] ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
                </Button>
              )}
            </div>
            <ul className="space-y-1 pl-6">
              {(probe.results || []).map((r, index) => {
                const { label, message } = describeResult(t, r);
                return (
                  <li key={`${r.name}-${index}`} className="flex gap-2 text-xs">
                    <span className={`w-28 shrink-0 font-medium ${metaOf(r.status).className}`}>{label}</span>
                    <span className="min-w-0 whitespace-pre-wrap break-words text-muted-foreground">{message}</span>
                  </li>
                );
              })}
            </ul>
            {probe.response && openResponse[probe.key] && (
              <div className="pl-6">
                <CodeBlock language="json" wrap showCopy={false} code={JSON.stringify(probe.response, null, 2)} />
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

const resultShape = PropTypes.shape({
  status: PropTypes.string,
  message: PropTypes.string,
  data: PropTypes.shape({ status: PropTypes.number, items: PropTypes.array })
});

OverallCell.propTypes = { t: PropTypes.func.isRequired, result: resultShape };
ProbeCell.propTypes = { t: PropTypes.func.isRequired, probeKey: PropTypes.string.isRequired, probe: PropTypes.object };
DetailPanel.propTypes = { t: PropTypes.func.isRequired, result: resultShape.isRequired };

ChannelCheckDialog.propTypes = {
  item: PropTypes.object,
  open: PropTypes.bool,
  onClose: PropTypes.func.isRequired
};
