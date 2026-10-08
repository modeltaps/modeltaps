import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2, Download, Upload, CloudDownload, TriangleAlert } from 'lucide-react';

import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { showError, showSuccess } from 'utils/common';
import { API } from 'utils/api';

export default function ImportDialog({ open, onOpenChange, existingModels = [], onImported }) {
  const { t } = useTranslation();
  const [jsonUrl, setJsonUrl] = useState('');
  const [previewData, setPreviewData] = useState([]);
  const [conflictStrategy, setConflictStrategy] = useState('skip');
  const [loading, setLoading] = useState(false);
  const [importing, setImporting] = useState(false);
  const [progress, setProgress] = useState({ current: 0, total: 0 });

  const reset = () => {
    setJsonUrl('');
    setPreviewData([]);
    setProgress({ current: 0, total: 0 });
  };

  const handleClose = () => {
    reset();
    onOpenChange(false);
  };

  const handleFetch = async () => {
    if (!jsonUrl.trim()) return showError(t('modelInfoPage.importUrlRequired'));
    setLoading(true);
    setPreviewData([]);
    try {
      const response = await fetch(jsonUrl);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const jsonData = await response.json();
      if (!jsonData.data || !Array.isArray(jsonData.data)) throw new Error(t('modelInfoPage.importFormatError'));
      const transformed = jsonData.data
        .filter((item) => item.model_info)
        .map((item) => {
          const m = item.model_info;
          const model = m.model || item.model;
          return {
            model,
            name: m.name || model,
            description: m.description || '',
            context_length: m.context_length || 0,
            max_tokens: m.max_tokens || 0,
            input_modalities: JSON.stringify(m.input_modalities || []),
            output_modalities: JSON.stringify(m.output_modalities || []),
            tags: JSON.stringify(m.tags || []),
            isConflict: existingModels.includes(model)
          };
        });
      setPreviewData(transformed);
      if (transformed.length === 0) showError(t('modelInfoPage.importNoData'));
      else showSuccess(t('modelInfoPage.importFetched', { count: transformed.length }));
    } catch (error) {
      showError(`${t('modelInfoPage.importFetchFailed')}: ${error.message}`);
    } finally {
      setLoading(false);
    }
  };

  const handleImport = async () => {
    if (previewData.length === 0) return;
    setImporting(true);
    setProgress({ current: 0, total: previewData.length });
    let success = 0;
    let skip = 0;
    let fail = 0;
    for (let i = 0; i < previewData.length; i++) {
      const item = previewData[i];
      setProgress({ current: i + 1, total: previewData.length });
      if (item.isConflict && conflictStrategy === 'skip') {
        skip++;
        continue;
      }
      try {
        const submit = { ...item };
        delete submit.isConflict;
        await API.post('/api/model_info/', submit);
        success++;
      } catch (error) {
        fail++;
      }
    }
    setImporting(false);
    const summary = t('modelInfoPage.importResult', { success, skip, fail });
    if (fail > 0) showError(summary);
    else showSuccess(summary);
    reset();
    onImported?.();
  };

  const conflictCount = previewData.filter((item) => item.isConflict).length;

  return (
    <Dialog open={open} onOpenChange={(o) => !o && handleClose()}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>{t('modelInfoPage.importTitle')}</DialogTitle>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <div className="flex items-end gap-2">
            <div className="flex-1 space-y-4">
              <Label>{t('modelInfoPage.importUrl')}</Label>
              <Input value={jsonUrl} onChange={(e) => setJsonUrl(e.target.value)} placeholder="https://example.com/models.json" />
            </div>
            <Button onClick={handleFetch} disabled={loading}>
              {loading ? <Loader2 className="size-4 animate-spin" /> : <Download className="size-4" />}
              {t('modelInfoPage.importFetch')}
            </Button>
          </div>

          {conflictCount > 0 && (
            <Alert>
              <TriangleAlert className="size-4" />
              <AlertDescription className="flex flex-wrap items-center gap-2">
                {t('modelInfoPage.importConflict', { count: conflictCount })}
                <Button size="sm" variant={conflictStrategy === 'skip' ? 'default' : 'outline'} onClick={() => setConflictStrategy('skip')}>
                  {t('modelInfoPage.importSkip')}
                </Button>
                <Button
                  size="sm"
                  variant={conflictStrategy === 'overwrite' ? 'default' : 'outline'}
                  onClick={() => setConflictStrategy('overwrite')}
                >
                  {t('modelInfoPage.importOverwrite')}
                </Button>
              </AlertDescription>
            </Alert>
          )}

          {importing && (
            <p className="text-sm text-muted-foreground">
              {t('modelInfoPage.importProgress', { current: progress.current, total: progress.total })}
            </p>
          )}

          {previewData.length > 0 ? (
            <>
              <div className="hidden md:block max-h-80 overflow-auto rounded-md border border-border">
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent">
                      <TableHead>{t('modelInfoPage.model')}</TableHead>
                      <TableHead>{t('modelInfoPage.name')}</TableHead>
                      <TableHead>{t('modelInfoPage.contextLength')}</TableHead>
                      <TableHead>{t('modelInfoPage.maxTokens')}</TableHead>
                      <TableHead>{t('modelInfoPage.status')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {previewData.map((row, i) => (
                      <TableRow key={`${row.model}-${i}`}>
                        <TableCell className="font-mono text-xs">{row.model}</TableCell>
                        <TableCell>{row.name}</TableCell>
                        <TableCell className="tabular-nums">{row.context_length}</TableCell>
                        <TableCell className="tabular-nums">{row.max_tokens}</TableCell>
                        <TableCell>
                          {row.isConflict ? (
                            <Badge variant="secondary">{t('modelInfoPage.importExists')}</Badge>
                          ) : (
                            <Badge
                              variant="outline"
                              className="border-transparent bg-emerald-500/15 text-emerald-600 dark:text-emerald-400"
                            >
                              {t('modelInfoPage.importNew')}
                            </Badge>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <div className="md:hidden flex flex-col gap-3">
                {previewData.map((row, i) => (
                  <div key={`${row.model}-${i}`} className="rounded-lg border border-border bg-card p-3.5 shadow-sm">
                    <div className="mb-2 text-sm font-medium">{row.name}</div>
                    <dl className="flex flex-col divide-y divide-border/60">
                      <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                        <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                          {t('modelInfoPage.model')}
                        </dt>
                        <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right font-mono text-xs">{row.model}</dd>
                      </div>
                      <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                        <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                          {t('modelInfoPage.contextLength')}
                        </dt>
                        <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm tabular-nums">{row.context_length}</dd>
                      </div>
                      <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                        <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                          {t('modelInfoPage.maxTokens')}
                        </dt>
                        <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm tabular-nums">{row.max_tokens}</dd>
                      </div>
                      <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                        <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                          {t('modelInfoPage.status')}
                        </dt>
                        <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                          {row.isConflict ? (
                            <Badge variant="secondary">{t('modelInfoPage.importExists')}</Badge>
                          ) : (
                            <Badge
                              variant="outline"
                              className="border-transparent bg-emerald-500/15 text-emerald-600 dark:text-emerald-400"
                            >
                              {t('modelInfoPage.importNew')}
                            </Badge>
                          )}
                        </dd>
                      </div>
                    </dl>
                  </div>
                ))}
              </div>
            </>
          ) : (
            !loading && (
              <div className="flex flex-col items-center gap-2 py-8 text-center text-muted-foreground">
                <CloudDownload className="size-10 opacity-50" />
                <p className="text-sm">{t('modelInfoPage.importEmpty')}</p>
              </div>
            )
          )}
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={handleClose} disabled={importing}>
            {t('common.cancel')}
          </Button>
          <Button onClick={handleImport} disabled={previewData.length === 0 || importing}>
            {importing ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
            {importing ? t('modelInfoPage.importing') : t('modelInfoPage.importStart')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
