import { useState, useEffect, useCallback } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { DownloadCloud, Loader2, RefreshCw } from 'lucide-react';

import { API } from 'utils/api';
import { toast } from '@/components/ui/sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Alert } from '@/components/ui/alert';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';

const STORAGE_KEY = 'modeltaps_price_update_url';

const hasRatiosDiff = (oldR, newR) => {
  if (!oldR && !newR) return false;
  if (!oldR || !newR) return true;
  const keys = [...new Set([...Object.keys(oldR), ...Object.keys(newR)])];
  return keys.some((k) => oldR[k] !== newR[k]);
};

// 长上下文分档差异:threshold 为 0(或缺失)视为未启用,无论比率如何都不算差异(上游 89009cc 语义)。
const hasLongContextDiff = (oldLC, newLC) => {
  const oldThreshold = oldLC?.threshold || 0;
  const newThreshold = newLC?.threshold || 0;
  if (!oldThreshold && !newThreshold) return false;
  if (oldThreshold !== newThreshold) return true;
  return oldLC?.input_ratio !== newLC?.input_ratio || oldLC?.output_ratio !== newLC?.output_ratio;
};

export default function CheckUpdatesDialog({ open, onClose, onOk, row = [] }) {
  const { t } = useTranslation();
  const [url, setUrl] = useState('');
  const [loading, setLoading] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [newPricing, setNewPricing] = useState([]);
  const [addModel, setAddModel] = useState([]);
  const [diffModel, setDiffModel] = useState([]);

  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      setUrl(saved);
      return;
    }
    // 后端未配置 update_price_service 时留空,由用户自行填写价格源地址。
    API.get('/api/prices/updateService')
      .then((res) => {
        const u = res.data?.data;
        if (!u) return;
        setUrl(u);
        localStorage.setItem(STORAGE_KEY, u);
      })
      .catch(() => {});
  }, []);

  const onUrlChange = (e) => {
    setUrl(e.target.value);
    localStorage.setItem(STORAGE_KEY, e.target.value);
  };

  const checkUpdates = async () => {
    setLoading(true);
    try {
      const res = await API.get(url);
      const data = Array.isArray(res?.data) ? res.data : res?.data?.data ?? [];
      if (!Array.isArray(data)) toast.error(t('CheckUpdatesTable.dataFormatIncorrect'));
      else setNewPricing(data);
    } catch (err) {
      toast.error(err.message);
    }
    setLoading(false);
  };

  // 从 models.dev 拉取:由后端拉取并换算成价格数组(避免浏览器直连 models.dev 的 CORS 问题),
  // 拿到后与普通拉取一样塞进 newPricing,复用下方的新增/变更预览与同步。
  const fetchModelsDev = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/prices/modelsdev');
      const { success, message, data } = res.data;
      if (success && Array.isArray(data)) setNewPricing(data);
      else toast.error(message || t('CheckUpdatesTable.dataFormatIncorrect'));
    } catch (err) {
      toast.error(err.message);
    }
    setLoading(false);
  };

  const sync = async (updateMode) => {
    if (!newPricing.length) return toast.error(t('CheckUpdatesTable.pleaseFetchData'));
    if (updateMode === 'add' && !addModel.length) return toast.error(t('CheckUpdatesTable.noNewModels'));
    setUpdating(true);
    try {
      const res = await API.post('/api/prices/sync?updateMode=' + updateMode, newPricing);
      const { success, message } = res.data;
      if (success) {
        toast.success(t('CheckUpdatesTable.operationCompleted'));
        // 同步成功后这份拉取数据已是同步前的快照(DB 已改),留着再比对会显示误导性的 diff,
        // 清空预览逼用户重新拉取,拿同步后的最新状态。
        setNewPricing([]);
        setAddModel([]);
        setDiffModel([]);
        onOk();
      } else toast.error(message);
    } catch (err) {
      toast.error(err.message);
    }
    setUpdating(false);
  };

  // 描述长上下文分档的变化(新增/删除/修改),口径同 hasLongContextDiff。
  const describeLongContextChange = useCallback(
    (oldLC, newLC) => {
      const oldThreshold = oldLC?.threshold || 0;
      const newThreshold = newLC?.threshold || 0;
      const format = (lc) =>
        `${t('CheckUpdatesTable.longContextThreshold')}=${lc.threshold} ${t('CheckUpdatesTable.longContextInput')}=${lc.input_ratio} ${t('CheckUpdatesTable.longContextOutput')}=${lc.output_ratio}`;
      if (!oldThreshold && newThreshold) return `${t('CheckUpdatesTable.longContextAdded')} ${format(newLC)}`;
      if (oldThreshold && !newThreshold) return t('CheckUpdatesTable.longContextRemoved');
      return `${t('CheckUpdatesTable.longContextChanged')} ${format(oldLC)} -> ${format(newLC)}`;
    },
    [t]
  );

  const buildDiff = useCallback(() => {
    const added = newPricing.filter((np) => !row.some((r) => r.model === np.model)).map((m) => m.model);
    const changed = row
      .filter((r) =>
        newPricing.some(
          (np) =>
            np.model === r.model &&
            (np.input !== r.input ||
              np.output !== r.output ||
              hasRatiosDiff(r.extra_ratios, np.extra_ratios) ||
              hasLongContextDiff(r.long_context, np.long_context))
        )
      )
      .map((r) => {
        const np = newPricing.find((n) => n.model === r.model);
        let changes = '';
        if (r.input !== np.input) changes += `${t('CheckUpdatesTable.inputMultiplierChanged')} ${r.input} ${t('CheckUpdatesTable.to')} ${np.input}; `;
        if (r.output !== np.output) changes += `${t('CheckUpdatesTable.outputMultiplierChanged')} ${r.output} ${t('CheckUpdatesTable.to')} ${np.output}; `;
        if (hasRatiosDiff(r.extra_ratios, np.extra_ratios)) changes += `${t('CheckUpdatesTable.extraRatiosChanges')}; `;
        if (hasLongContextDiff(r.long_context, np.long_context)) changes += describeLongContextChange(r.long_context, np.long_context);
        return { model: r.model, changes: changes.replace(/;\s*$/, '').trim() };
      });
    setAddModel(added);
    setDiffModel(changed);
  }, [newPricing, row, t, describeLongContextChange]);

  useEffect(() => {
    buildDiff();
  }, [buildDiff]);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t('CheckUpdatesTable.checkUpdates')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="flex gap-2">
            <Input value={url} placeholder={t('CheckUpdatesTable.url')} onChange={onUrlChange} />
            <Button type="button" variant="outline" onClick={checkUpdates} disabled={loading}>
              {loading ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
              {t('CheckUpdatesTable.fetchData')}
            </Button>
            <Button type="button" variant="outline" onClick={fetchModelsDev} disabled={loading}>
              {loading ? <Loader2 className="size-4 animate-spin" /> : <DownloadCloud className="size-4" />}
              {t('CheckUpdatesTable.fetchFromModelsDev')}
            </Button>
          </div>

          {newPricing.length > 0 && (
            <div className="space-y-3">
              <div className="flex flex-wrap gap-2">
                <Badge>{t('CheckUpdatesTable.priceServerTotal')}: {newPricing.length}</Badge>
                <Badge variant="secondary">{t('CheckUpdatesTable.newModels')}: {addModel.length}</Badge>
                <Badge variant="outline">{t('CheckUpdatesTable.priceChangeModels')}: {diffModel.length}</Badge>
              </div>

              {!addModel.length && !diffModel.length && <Alert variant="success">{t('CheckUpdatesTable.noUpdates')}</Alert>}

              {addModel.length > 0 && (
                <div className="max-h-40 overflow-auto rounded-md border border-border p-3">
                  <p className="mb-2 text-sm font-medium">{t('CheckUpdatesTable.newModels')}</p>
                  <div className="flex flex-wrap gap-1.5">
                    {addModel.map((m) => (
                      <Badge key={m} variant="outline">{m}</Badge>
                    ))}
                  </div>
                </div>
              )}

              {diffModel.length > 0 && (
                <div className="max-h-48 space-y-2 overflow-auto rounded-md border border-border p-3">
                  <p className="text-sm font-medium">{t('CheckUpdatesTable.priceChangeModels')}</p>
                  {diffModel.map((item) => (
                    <div key={item.model} className="text-sm">
                      <span className="font-medium">{item.model}</span>
                      {item.changes && <span className="ml-2 text-xs text-muted-foreground">{item.changes}</span>}
                    </div>
                  ))}
                </div>
              )}

              <Alert variant="info">{t('CheckUpdatesTable.note')}: {t('CheckUpdatesTable.overwriteOrAddOnly')}</Alert>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            {t('CheckUpdatesTable.cancel')}
          </Button>
          {newPricing.length > 0 && (
            <>
              <Button type="button" onClick={() => sync('add')} disabled={updating || !addModel.length}>
                {t('CheckUpdatesTable.updateModeAdd')}
              </Button>
              <Button type="button" onClick={() => sync('update')} disabled={updating}>
                {t('CheckUpdatesTable.updateModeUpdate')}
              </Button>
              <Button type="button" variant="destructive" onClick={() => sync('overwrite')} disabled={updating}>
                {t('CheckUpdatesTable.updateModeOverwrite')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

CheckUpdatesDialog.propTypes = {
  open: PropTypes.bool,
  onClose: PropTypes.func,
  onOk: PropTypes.func,
  row: PropTypes.array
};
