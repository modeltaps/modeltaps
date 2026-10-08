import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2, CloudDownload, Search, ChevronDown, ChevronRight, ListChecks, FlipHorizontal, RotateCcw } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { Switch } from '@/components/ui/switch';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { toast } from '@/components/ui/sonner';
import { fetchUpstreamModelList } from './channelApi';

// Upstream model picker for the channel form. Ported from v1
// `component/ModelSelectorModal.jsx` (MUI) to shadcn primitives. MVP scope:
// fetch + search + grouped checkboxes + overwrite/merge write-back (no price
// column, no model-mapping helpers).
export default function ModelSelectorModal({ open, onClose, onConfirm, channelValues }) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [models, setModels] = useState([]);
  const [selected, setSelected] = useState([]);
  const [searchTerm, setSearchTerm] = useState('');
  const [collapsedGroups, setCollapsedGroups] = useState({});
  const [overwrite, setOverwrite] = useState(false);

  // Re-seed local state from the current form values each time the dialog
  // opens so a previous session never leaks into a new one.
  useEffect(() => {
    if (!open) return;
    const existing = (channelValues?.models || '')
      .split(/[,\n]/)
      .map((m) => m.trim())
      .filter(Boolean);
    setSelected([...new Set(existing)]);
    setModels([]);
    setSearchTerm('');
    setCollapsedGroups({});
    setOverwrite(false);
  }, [open, channelValues]);

  // Same grouping fallback as v1 when no price metadata is available:
  // models namespaced with `/` group by their vendor prefix.
  const groupOf = (model) => (model.includes('/') ? model.split('/')[0] : t('channel_edit.otherModels'));

  const fetchModels = async () => {
    setLoading(true);
    try {
      setModels(await fetchUpstreamModelList(channelValues || {}));
      toast.success(t('channel_edit.modelsFetched'));
    } catch (error) {
      toast.error(error.message || t('channel_edit.modelListError'));
    }
    setLoading(false);
  };

  const filteredModels = useMemo(
    () => models.filter((m) => m.toLowerCase().includes(searchTerm.toLowerCase())),
    [models, searchTerm]
  );

  const filteredGroups = useMemo(() => {
    const groups = {};
    filteredModels.forEach((m) => {
      const g = groupOf(m);
      if (!groups[g]) groups[g] = [];
      groups[g].push(m);
    });
    return groups;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filteredModels]);

  const toggleModel = (model) => {
    setSelected((prev) => (prev.includes(model) ? prev.filter((m) => m !== model) : [...prev, model]));
  };

  const toggleGroup = (group) => {
    const groupModels = filteredGroups[group] || [];
    const allSelected = groupModels.every((m) => selected.includes(m));
    setSelected((prev) =>
      allSelected ? prev.filter((m) => !groupModels.includes(m)) : [...new Set([...prev, ...groupModels])]
    );
  };

  const selectAll = () => {
    const allSelected = filteredModels.every((m) => selected.includes(m));
    setSelected((prev) =>
      allSelected ? prev.filter((m) => !filteredModels.includes(m)) : [...new Set([...prev, ...filteredModels])]
    );
  };

  const invertSelection = () => {
    setSelected((prev) => {
      const next = new Set(prev);
      filteredModels.forEach((m) => (next.has(m) ? next.delete(m) : next.add(m)));
      return [...next];
    });
  };

  const handleConfirm = () => {
    onConfirm(selected, overwrite);
    onClose();
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t('channel_edit.modelSelector')}</DialogTitle>
        </DialogHeader>
        <DialogBody className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <Button size="sm" onClick={fetchModels} disabled={loading}>
              {loading ? <Loader2 className="size-4 animate-spin" /> : <CloudDownload className="size-4" />}
              {t('channel_edit.fetchModels')}
            </Button>
            <Button variant="outline" size="sm" onClick={selectAll} disabled={loading || models.length === 0}>
              <ListChecks className="size-4" /> {t('channel_edit.selectAll')}
            </Button>
            <Button variant="outline" size="sm" onClick={invertSelection} disabled={loading || models.length === 0}>
              <FlipHorizontal className="size-4" /> {t('channel_edit.invertSelection')}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setSelected([])} disabled={selected.length === 0}>
              <RotateCcw className="size-4" /> {t('channel_edit.clearModels')}
            </Button>
          </div>

          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              className="pl-8"
              placeholder={t('channel_edit.searchModels')}
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>

          <div className="max-h-80 overflow-y-auto rounded-md border border-border p-2">
            {models.length === 0 ? (
              <p className="py-8 text-center text-sm text-muted-foreground">{t('channel_edit.noModels')}</p>
            ) : filteredModels.length === 0 ? (
              <p className="py-8 text-center text-sm text-muted-foreground">{t('channel_edit.noMatchingModels')}</p>
            ) : (
              Object.entries(filteredGroups).map(([group, groupModels]) => {
                const selectedCount = groupModels.filter((m) => selected.includes(m)).length;
                const collapsed = collapsedGroups[group];
                return (
                  <div key={group} className="mb-1.5">
                    <div className="flex items-center gap-2 rounded-md bg-muted/50 px-2 py-1.5">
                      <Checkbox
                        checked={
                          selectedCount === groupModels.length ? true : selectedCount > 0 ? 'indeterminate' : false
                        }
                        onCheckedChange={() => toggleGroup(group)}
                      />
                      <button
                        type="button"
                        className="flex flex-1 items-center gap-2 text-left"
                        onClick={() => setCollapsedGroups((prev) => ({ ...prev, [group]: !prev[group] }))}
                      >
                        <span className="flex-1 text-sm font-semibold">{group}</span>
                        <span className="text-xs text-muted-foreground">
                          {selectedCount}/{groupModels.length}
                        </span>
                        {collapsed ? <ChevronRight className="size-4" /> : <ChevronDown className="size-4" />}
                      </button>
                    </div>
                    {!collapsed && (
                      <div className="space-y-0.5 py-1 pl-4">
                        {groupModels.map((model) => (
                          <label
                            key={model}
                            className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 text-sm hover:bg-muted/50"
                          >
                            <Checkbox checked={selected.includes(model)} onCheckedChange={() => toggleModel(model)} />
                            <span className="truncate font-mono text-xs">{model}</span>
                          </label>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>

          <div className="flex items-center justify-between rounded-md border border-border p-3">
            <div className="space-y-0.5 pr-3">
              <p className="text-sm">{t('channel_edit.overwriteModels')}</p>
              <p className="text-xs text-muted-foreground">{t('channel_edit.overwriteModelsTip')}</p>
            </div>
            <Switch checked={overwrite} onCheckedChange={setOverwrite} />
          </div>
        </DialogBody>
        <DialogFooter className="items-center">
          <span className="mr-auto text-xs text-muted-foreground">
            {t('channel_edit.selectedCount', { count: selected.length })}
          </span>
          <Button variant="outline" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={selected.length === 0}>
            {t('common.submit')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

ModelSelectorModal.propTypes = {
  open: PropTypes.bool,
  onClose: PropTypes.func,
  onConfirm: PropTypes.func,
  channelValues: PropTypes.object
};
