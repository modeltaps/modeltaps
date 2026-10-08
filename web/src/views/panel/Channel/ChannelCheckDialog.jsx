import { useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { RefreshCw, Loader2, ChevronDown, ChevronRight, ChevronUp, CheckCircle2, XCircle, CheckSquare, Square } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody } from '@/components/ui/dialog';
import CodeBlock from '@/components/ui/code-block';
import { toast } from '@/components/ui/sonner';
import { fetchProviderModels, checkChannelModels } from './channelApi';

// Per-channel model availability check. Ported from v1
// `views/Channel/component/ChannelCheck.jsx` (MUI) to shadcn primitives.
export default function ChannelCheckDialog({ item, open, onClose }) {
  const { t } = useTranslation();
  const [providerModelsLoad, setProviderModelsLoad] = useState(false);
  const [modelList, setModelList] = useState([]);
  const [checkLoad, setCheckLoad] = useState(false);
  const [selectedModels, setSelectedModels] = useState([]);
  const [checkResults, setCheckResults] = useState([]);
  const [expandedResponses, setExpandedResponses] = useState({});
  const [expandedModels, setExpandedModels] = useState({});
  const abortRef = useRef(null);

  // Reset local state and abort any in-flight SSE stream whenever the dialog
  // opens/closes or the target channel changes, so stale results never leak
  // into a new session.
  useEffect(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    const initialModels = item?.models ? item.models.split(',').filter(Boolean) : [];
    setModelList(initialModels);
    setSelectedModels(initialModels);
    setCheckResults([]);
    setExpandedResponses({});
    setExpandedModels({});
    setProviderModelsLoad(false);
    setCheckLoad(false);
  }, [open, item?.id, item?.models]);

  // Abort the SSE stream on unmount.
  useEffect(() => {
    return () => abortRef.current?.abort();
  }, []);

  const handleModelToggle = (model) => {
    setSelectedModels((prev) => (prev.includes(model) ? prev.filter((m) => m !== model) : [...prev, model]));
  };

  const getProviderModels = async () => {
    setProviderModelsLoad(true);
    try {
      setModelList(await fetchProviderModels(item));
    } catch (error) {
      toast.error(error.message);
    }
    setProviderModelsLoad(false);
  };

  const handleCheck = async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    setCheckLoad(true);
    setCheckResults([]);
    try {
      await checkChannelModels(
        item.id,
        selectedModels.join(','),
        (data) => {
          setCheckResults((prev) => {
            const existingIndex = prev.findIndex((r) => r.model === data.model);
            if (existingIndex !== -1) {
              const next = [...prev];
              next[existingIndex] = data;
              return next;
            }
            return [...prev, data];
          });
        },
        controller.signal
      );
    } catch (error) {
      if (!controller.signal.aborted) {
        toast.error(error.message);
      }
    }
    if (abortRef.current === controller) {
      abortRef.current = null;
    }
    setCheckLoad(false);
  };

  const toggleResponse = (modelIndex, processIndex) => {
    const key = `${modelIndex}-${processIndex}`;
    setExpandedResponses((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  const toggleModel = (modelIndex) => {
    setExpandedModels((prev) => ({ ...prev, [modelIndex]: !prev[modelIndex] }));
  };

  const getModelStatus = (result) => {
    const hasFailure = result.process.some((process) => process.results.some((r) => r.status !== 1));
    return hasFailure
      ? { ok: false, text: t('channel_check.fail') }
      : { ok: true, text: t('channel_check.pass') };
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>{t('channel_check.title')}</DialogTitle>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            <Button size="sm" onClick={getProviderModels} disabled={providerModelsLoad}>
              {providerModelsLoad ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              {t('channel_check.fetchModels')}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setSelectedModels(modelList)}>
              <CheckSquare className="size-4" /> {t('channel_check.selectAll')}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setSelectedModels([])}>
              <Square className="size-4" /> {t('channel_check.unselectAll')}
            </Button>
          </div>

          <div className="flex flex-wrap gap-x-4 gap-y-2 rounded-lg bg-muted/50 p-3">
            {modelList.map((model) => (
              <label key={model} className="flex cursor-pointer items-center gap-2 text-sm">
                <Checkbox checked={selectedModels.includes(model)} onCheckedChange={() => handleModelToggle(model)} />
                {model}
              </label>
            ))}
          </div>

          <Button className="w-full" onClick={handleCheck} disabled={checkLoad || selectedModels.length === 0}>
            {checkLoad && <Loader2 className="size-4 animate-spin" />}
            {t('channel_check.start')}
          </Button>

          <div className="space-y-2">
            {checkResults.map((result, modelIndex) => {
              const status = getModelStatus(result);
              return (
                <div key={result.model} className="rounded-lg bg-muted/50 p-3">
                  <button type="button" className="flex w-full items-center gap-2 text-left" onClick={() => toggleModel(modelIndex)}>
                    {expandedModels[modelIndex] ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
                    <span className="flex-1 text-sm font-medium">{result.model}</span>
                    <span
                      className={`flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium ${
                        status.ok ? 'bg-emerald-500/10 text-emerald-600' : 'bg-destructive/10 text-destructive'
                      }`}
                    >
                      {status.ok ? <CheckCircle2 className="size-3.5" /> : <XCircle className="size-3.5" />}
                      {status.text}
                    </span>
                  </button>

                  {expandedModels[modelIndex] && (
                    <div className="mt-3 space-y-3">
                      {result.process.map((process, processIndex) => (
                        <div key={processIndex} className="space-y-2">
                          <div className="text-sm font-medium text-foreground">{process.name}</div>
                          <div className="space-y-1.5">
                            {process.results.map((r, rIndex) => (
                              <div
                                key={rIndex}
                                className={`flex items-start gap-2 rounded-md border bg-card p-2 ${
                                  r.status === 1 ? 'border-emerald-500/40' : 'border-destructive/40'
                                }`}
                              >
                                {r.status === 1 ? (
                                  <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-600" />
                                ) : (
                                  <XCircle className="mt-0.5 size-4 shrink-0 text-destructive" />
                                )}
                                <div className="min-w-0">
                                  <div className="text-sm font-medium">{r.name}</div>
                                  <div className="whitespace-pre-wrap text-sm text-muted-foreground">{r.remark}</div>
                                </div>
                              </div>
                            ))}
                          </div>
                          <Button variant="ghost" size="sm" onClick={() => toggleResponse(modelIndex, processIndex)}>
                            {t('channel_check.responseDetail')}
                            {expandedResponses[`${modelIndex}-${processIndex}`] ? (
                              <ChevronUp className="size-4" />
                            ) : (
                              <ChevronDown className="size-4" />
                            )}
                          </Button>
                          {expandedResponses[`${modelIndex}-${processIndex}`] && (
                            <CodeBlock language="json" wrap showCopy={false} code={JSON.stringify(process.response, null, 2)} />
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}

ChannelCheckDialog.propTypes = {
  item: PropTypes.object,
  open: PropTypes.bool,
  onClose: PropTypes.func
};
