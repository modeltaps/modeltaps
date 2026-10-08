import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Search, UserPlus, Plus, Trash2, Send, X, Loader2 } from 'lucide-react';

import { API } from 'utils/api';
import { cn } from '@/lib/utils';
import { toast } from '@/components/ui/sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { fetchChannelData } from './channelApi';

export default function BatchModal({ open, onOpenChange, groupOptions = [], modelOptions = [] }) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t('channel_index.batchProcessing')}</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <Tabs defaultValue="group">
            <TabsList className="grid h-auto w-full grid-cols-4">
              <TabsTrigger value="group" className="whitespace-normal text-xs">
                {t('channel_index.batchAddUserGroup')}
              </TabsTrigger>
              <TabsTrigger value="model" className="whitespace-normal text-xs">
                {t('channel_index.batchAddModel')}
              </TabsTrigger>
              <TabsTrigger value="del" className="whitespace-normal text-xs">
                {t('channel_index.batchDelete')}
              </TabsTrigger>
              <TabsTrigger value="azure" className="whitespace-normal text-xs">
                {t('channel_index.AzureApiVersion')}
              </TabsTrigger>
            </TabsList>
            <TabsContent value="group">
              <BatchAddUserGroup groupOptions={groupOptions} />
            </TabsContent>
            <TabsContent value="model">
              <BatchAddModel modelOptions={modelOptions} />
            </TabsContent>
            <TabsContent value="del">
              <BatchDelModel />
            </TabsContent>
            <TabsContent value="azure">
              <BatchAzureAPI />
            </TabsContent>
          </Tabs>
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

BatchModal.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func,
  groupOptions: PropTypes.array,
  modelOptions: PropTypes.array
};

function SearchBar({ value, onChange, onSearch, placeholder }) {
  return (
    <div className="flex gap-2">
      <Input
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault();
            onSearch();
          }
        }}
      />
      <Button type="button" variant="outline" size="icon" onClick={onSearch}>
        <Search className="size-4" />
      </Button>
    </div>
  );
}

SearchBar.propTypes = {
  value: PropTypes.string,
  onChange: PropTypes.func,
  onSearch: PropTypes.func,
  placeholder: PropTypes.string
};

function EmptyHint({ text }) {
  return <p className="py-6 text-center text-sm text-muted-foreground">{text}</p>;
}

EmptyHint.propTypes = { text: PropTypes.string };

function LoadingHint() {
  const { t } = useTranslation();
  return (
    <div className="flex items-center justify-center gap-2 py-6 text-sm text-muted-foreground">
      <Loader2 className="size-4 animate-spin" />
      {t('channel_index.loadingChannels')}
    </div>
  );
}

function SelectableList({ items, selected, setSelected, primary, secondary, isDisabled }) {
  const { t } = useTranslation();
  const allSelected = items.length > 0 && selected.length === items.length;
  const toggle = (id) => setSelected((prev) => (prev.includes(id) ? prev.filter((i) => i !== id) : [...prev, id]));
  const toggleAll = () => setSelected(allSelected ? [] : items.map((i) => i.id));
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Button type="button" variant="ghost" size="sm" onClick={toggleAll}>
          {allSelected ? t('channel_index.unselectAll') : t('channel_index.selectAll')}
        </Button>
        <span className="text-sm text-muted-foreground">
          {t('channel_index.selectedChannelsCount', { selected: selected.length, total: items.length })}
        </span>
      </div>
      <div className="max-h-72 space-y-1 overflow-y-auto rounded-md border border-border p-2">
        {items.map((item) => {
          const disabled = isDisabled?.(item);
          return (
            <label key={item.id} className={cn('flex items-start gap-2 rounded px-2 py-1.5 hover:bg-accent', disabled && 'opacity-60')}>
              <Checkbox className="mt-0.5" checked={selected.includes(item.id)} disabled={disabled} onCheckedChange={() => !disabled && toggle(item.id)} />
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm">{primary(item)}</div>
                {secondary && <div className="truncate text-xs text-muted-foreground">{secondary(item)}</div>}
              </div>
            </label>
          );
        })}
      </div>
    </div>
  );
}

SelectableList.propTypes = {
  items: PropTypes.array,
  selected: PropTypes.array,
  setSelected: PropTypes.func,
  primary: PropTypes.func,
  secondary: PropTypes.func,
  isDisabled: PropTypes.func
};

function BatchAddUserGroup({ groupOptions }) {
  const { t } = useTranslation();
  const [keyword, setKeyword] = useState('');
  const [data, setData] = useState([]);
  const [fetching, setFetching] = useState(false);
  const [fetched, setFetched] = useState(false);
  const [selected, setSelected] = useState([]);
  const [group, setGroup] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSearch = async () => {
    setFetching(true);
    try {
      const res = await fetchChannelData(0, 100, keyword.trim() ? { name: keyword } : {}, 'desc', 'id');
      if (res) setData(res.data || []);
      else toast.error(t('channel_index.getChannelsFailed'));
    } finally {
      setFetching(false);
      setFetched(true);
    }
  };

  const hasGroup = (channel) => {
    if (!channel.group || !group) return false;
    return channel.group
      .split(',')
      .map((g) => g.trim())
      .includes(group);
  };

  const handleSubmit = async () => {
    if (selected.length === 0) return toast.error(t('channel_index.pleaseSelectChannelsForUserGroup'));
    if (!group) return toast.error(t('channel_index.pleaseSelectUserGroup'));
    setLoading(true);
    try {
      const res = await API.put('/api/channel/batch/add_user_group', { ids: selected, value: group });
      const { success, message, data: count } = res.data;
      if (success) {
        toast.success(t('channel_index.batchAddUserGroupSuccess', { count, group }));
        setSelected([]);
        setGroup('');
        handleSearch();
      } else toast.error(message);
    } catch (e) {
      toast.error(e.message);
    }
    setLoading(false);
  };

  return (
    <div className="space-y-4">
      <Alert>
        <AlertDescription>{t('channel_index.batchAddUserGroupTip')}</AlertDescription>
      </Alert>
      <SearchBar value={keyword} onChange={setKeyword} onSearch={handleSearch} placeholder={t('channel_index.searchChannelPlaceholder')} />
      {fetching ? (
        <LoadingHint />
      ) : data.length === 0 ? (
        <EmptyHint text={fetched ? t('channel_index.noMatchingChannels') : t('channel_index.clickSearchToGetChannels')} />
      ) : (
        <>
          <SelectableList
            items={data}
            selected={selected}
            setSelected={setSelected}
            isDisabled={hasGroup}
            primary={(item) => (
              <span>
                {item.name}
                {hasGroup(item) && <span className="ml-1 text-xs text-amber-500">{t('channel_index.channelAlreadyHasGroup')}</span>}
              </span>
            )}
            secondary={(item) => `${t('channel_index.currentGroup')}: ${item.group || t('channel_index.noGroup')}`}
          />
          <div className="space-y-4">
            <Label>{t('channel_index.selectUserGroupToAdd')}</Label>
            <Select value={group} onValueChange={setGroup}>
              <SelectTrigger>
                <SelectValue placeholder={t('channel_index.pleaseSelectUserGroup')} />
              </SelectTrigger>
              <SelectContent>
                {groupOptions.filter(Boolean).map((g) => (
                  <SelectItem key={g} value={g}>
                    {g}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Button className="w-full" onClick={handleSubmit} disabled={loading || selected.length === 0 || !group}>
            <UserPlus className="size-4" />
            {loading ? t('channel_index.addingUserGroup') : t('channel_index.addUserGroupToChannels', { count: selected.length })}
          </Button>
        </>
      )}
    </div>
  );
}

BatchAddUserGroup.propTypes = { groupOptions: PropTypes.array };

function BatchAddModel({ modelOptions }) {
  const { t } = useTranslation();
  const [keyword, setKeyword] = useState('');
  const [data, setData] = useState([]);
  const [fetching, setFetching] = useState(false);
  const [fetched, setFetched] = useState(false);
  const [selected, setSelected] = useState([]);
  const [models, setModels] = useState([]);
  const [modelInput, setModelInput] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSearch = async () => {
    setFetching(true);
    try {
      const res = await fetchChannelData(0, 100, keyword.trim() ? { name: keyword } : {}, 'desc', 'id');
      if (res) setData(res.data || []);
      else toast.error(t('channel_index.getChannelsFailed'));
    } finally {
      setFetching(false);
      setFetched(true);
    }
  };

  useEffect(() => {
    handleSearch();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const addModels = (raw) => {
    const parts = raw
      .split(',')
      .map((m) => m.trim())
      .filter(Boolean);
    if (parts.length) setModels((prev) => [...new Set([...prev, ...parts])]);
  };

  const removeModel = (m) => setModels((prev) => prev.filter((x) => x !== m));

  const hasModel = (channel) => {
    if (!channel.models || models.length === 0) return false;
    const list = channel.models.split(',').map((m) => m.trim());
    return models.some((m) => list.includes(m));
  };

  const handleSubmit = async () => {
    if (selected.length === 0) return toast.error(t('channel_index.pleaseSelectChannelsForModel'));
    if (models.length === 0) return toast.error(t('channel_index.pleaseSelectModel'));
    setLoading(true);
    try {
      const value = models.join(',');
      const res = await API.put('/api/channel/batch/add_model', { ids: selected, value });
      const { success, message, data: count } = res.data;
      if (success) {
        toast.success(t('channel_index.batchAddModelSuccess', { count, model: value }));
        setSelected([]);
        setModels([]);
        setModelInput('');
        handleSearch();
      } else toast.error(message);
    } catch (e) {
      toast.error(e.message);
    }
    setLoading(false);
  };

  return (
    <div className="space-y-4">
      <Alert>
        <AlertDescription>{t('channel_index.batchAddModelTip')}</AlertDescription>
      </Alert>
      <SearchBar value={keyword} onChange={setKeyword} onSearch={handleSearch} placeholder={t('channel_index.searchChannelPlaceholder')} />
      {fetching ? (
        <LoadingHint />
      ) : data.length === 0 ? (
        <EmptyHint text={fetched ? t('channel_index.noMatchingChannels') : t('channel_index.clickSearchToGetChannels')} />
      ) : (
        <SelectableList
          items={data}
          selected={selected}
          setSelected={setSelected}
          isDisabled={hasModel}
          primary={(item) => (
            <span>
              {item.name}
              {hasModel(item) && <span className="ml-1 text-xs text-amber-500">{t('channel_index.channelAlreadyHasModel')}</span>}
            </span>
          )}
          secondary={(item) => `${t('channel_index.currentModels')}: ${item.models || t('channel_index.noModels')}`}
        />
      )}
      <div className="space-y-4">
        <Label>{t('channel_index.selectModelToAdd')}</Label>
        {models.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {models.map((m) => (
              <Badge key={m} variant="secondary" className="gap-1">
                {m}
                <button type="button" onClick={() => removeModel(m)} className="rounded-full hover:text-destructive">
                  <X className="size-3" />
                </button>
              </Badge>
            ))}
          </div>
        )}
        <Input
          list="batch-model-options"
          value={modelInput}
          placeholder={t('channel_index.pleaseSelectModel')}
          onChange={(e) => {
            const v = e.target.value;
            if (v.includes(',')) {
              addModels(v);
              setModelInput('');
            } else setModelInput(v);
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ',') {
              e.preventDefault();
              addModels(modelInput);
              setModelInput('');
            }
          }}
        />
        <datalist id="batch-model-options">
          {modelOptions.map((m) => (
            <option key={m.id} value={m.id} />
          ))}
        </datalist>
      </div>
      <Button className="w-full" onClick={handleSubmit} disabled={loading || selected.length === 0 || models.length === 0}>
        <Plus className="size-4" />
        {loading ? t('channel_index.addingModel') : t('channel_index.addModelToChannels', { count: selected.length })}
      </Button>
    </div>
  );
}

BatchAddModel.propTypes = { modelOptions: PropTypes.array };

function BatchDelModel() {
  const { t } = useTranslation();
  const [value, setValue] = useState('');
  const [data, setData] = useState([]);
  const [fetching, setFetching] = useState(false);
  const [fetched, setFetched] = useState(false);
  const [selected, setSelected] = useState([]);
  const [loading, setLoading] = useState(false);

  const handleSearch = async () => {
    setFetching(true);
    try {
      const res = await fetchChannelData(0, 100, { models: value }, 'desc', 'id');
      if (res) setData((res.data || []).filter((item) => item.models && item.models.split(',').length > 1));
      else toast.error(t('channel_index.getChannelsFailed'));
    } finally {
      setFetching(false);
      setFetched(true);
    }
  };

  const handleSubmit = async () => {
    if (value === '' || selected.length === 0) return;
    setLoading(true);
    try {
      const res = await API.put('/api/channel/batch/del_model', { ids: selected, value });
      const { success, message, data: count } = res.data;
      if (success) toast.success(t('channel_index.batchDeleteSuccess', { count }));
      else toast.error(message);
    } catch (e) {
      toast.error(e.message);
    }
    setLoading(false);
  };

  return (
    <div className="space-y-4">
      <Alert>
        <AlertDescription>{t('channel_index.batchDeleteTip')}</AlertDescription>
      </Alert>
      <SearchBar value={value} onChange={setValue} onSearch={handleSearch} placeholder={t('channel_index.batchDeleteModel')} />
      {fetching ? (
        <LoadingHint />
      ) : data.length === 0 ? (
        <EmptyHint text={fetched ? t('common.noData') : t('channel_index.clickSearchToGetChannels')} />
      ) : (
        <>
          <SelectableList items={data} selected={selected} setSelected={setSelected} primary={(item) => item.name} />
          <Button
            className="w-full"
            variant="destructive"
            onClick={handleSubmit}
            disabled={loading || value === '' || selected.length === 0}
          >
            <Trash2 className="size-4" /> {t('common.delete')}
          </Button>
        </>
      )}
    </div>
  );
}

function BatchAzureAPI() {
  const { t } = useTranslation();
  const [value, setValue] = useState('');
  const [data, setData] = useState([]);
  const [fetching, setFetching] = useState(false);
  const [fetched, setFetched] = useState(false);
  const [selected, setSelected] = useState([]);
  const [replaceValue, setReplaceValue] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSearch = async () => {
    setFetching(true);
    try {
      const res = await fetchChannelData(0, 100, { other: value, type: 3 }, 'desc', 'id');
      if (res) setData(res.data || []);
      else toast.error(t('channel_index.getChannelsFailed'));
    } finally {
      setFetching(false);
      setFetched(true);
    }
  };

  const handleSubmit = async () => {
    if (selected.length === 0) return;
    setLoading(true);
    try {
      const res = await API.put('/api/channel/batch/azure_api', { ids: selected, value: replaceValue });
      const { success, message, data: count } = res.data;
      if (success) toast.success(t('channel_index.batchAzureAPISuccess', { count }));
      else toast.error(message);
    } catch (e) {
      toast.error(e.message);
    }
    setLoading(false);
  };

  return (
    <div className="space-y-4">
      <SearchBar value={value} onChange={setValue} onSearch={handleSearch} placeholder={t('channel_index.inputAPIVersion')} />
      {fetching ? (
        <LoadingHint />
      ) : data.length === 0 ? (
        <EmptyHint text={fetched ? t('common.noData') : t('channel_index.clickSearchToGetChannels')} />
      ) : (
        <>
          <SelectableList items={data} selected={selected} setSelected={setSelected} primary={(item) => `${item.name}(${item.other})`} />
          <div className="flex gap-2">
            <Input
              value={replaceValue}
              placeholder={t('channel_index.replaceValue')}
              onChange={(e) => setReplaceValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  handleSubmit();
                }
              }}
            />
            <Button type="button" size="icon" onClick={handleSubmit} disabled={loading || selected.length === 0}>
              <Send className="size-4" />
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
