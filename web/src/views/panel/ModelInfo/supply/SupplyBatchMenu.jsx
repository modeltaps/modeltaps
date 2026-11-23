import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ChevronDown, Loader2, Minus, Network, Plus } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Combobox } from '@/components/ui/combobox';
import { Dialog, DialogContent, DialogBody, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { toast } from '@/components/ui/sonner';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { fetchAllChannels } from '../../Channel/channelApi';
import { batchSupplies } from './supplyApi';
import { isPublicModelRow } from './supplyHelpers';

// Batch actions for the selected public models: add or remove one channel as a source of all of
// them. Aliases, routing policies and rows without a catalog entry
// are left out, like on the model's own Supplies tab.
export default function SupplyBatchMenu({ rows, onDone }) {
  const { t } = useTranslation();
  const [action, setAction] = useState(null);
  const [channels, setChannels] = useState(null);
  const [channelId, setChannelId] = useState('');
  const [busy, setBusy] = useState(false);

  const models = rows.filter(isPublicModelRow);
  const count = models.length;

  const run = async (call, message) => {
    setBusy(true);
    try {
      const res = await call();
      if (!res.success) return toast.error(res.message);
      toast.success(message(res.data || {}));
      setAction(null);
      onDone();
    } catch {
      /* shown by the API client */
    } finally {
      setBusy(false);
    }
    return undefined;
  };

  const open = (next) => {
    setAction(next);
    setChannelId('');
    if (channels === null) {
      fetchAllChannels()
        .then((list) => setChannels([...list].sort((a, b) => String(a.name).localeCompare(String(b.name)))))
        .catch(() => setChannels([]));
    }
  };

  const submit = () =>
    run(
      () => batchSupplies({ action, channel_id: Number(channelId), models: models.map((row) => row.model) }),
      (data) => t('modelSupply.batch.done', { changed: data.changed ?? 0, skipped: (data.skipped || []).length })
    );

  const options = (channels || []).map((c) => ({
    value: String(c.id),
    label: c.name || `#${c.id}`,
    icon: <ProviderIcon type={c.type} name={c.name} baseUrl={c.base_url} className="size-4" />,
    hint: c.status === 1 ? `#${c.id}` : t('modelSupply.state.channel_off')
  }));

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button size="sm" variant="outline" className="h-7" disabled={count === 0 || busy}>
            {busy ? <Loader2 className="size-3.5 animate-spin" /> : <Network className="size-3.5" />}
            {t('modelSupply.batch.menu')}
            <ChevronDown className="size-3.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          <DropdownMenuItem onClick={() => open('add')}>
            <Plus className="size-4" />
            {t('modelSupply.batch.add')}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => open('remove')}>
            <Minus className="size-4" />
            {t('modelSupply.batch.remove')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={!!action} onOpenChange={(o) => !o && !busy && setAction(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t(action === 'remove' ? 'modelSupply.batch.removeTitle' : 'modelSupply.batch.addTitle', { count })}</DialogTitle>
            <DialogDescription>{t(action === 'remove' ? 'modelSupply.batch.removeHint' : 'modelSupply.batch.addHint')}</DialogDescription>
          </DialogHeader>
          <DialogBody>
            <Combobox
              id="supply-batch-channel"
              value={channelId}
              onValueChange={setChannelId}
              options={options}
              placeholder={channels === null ? t('common.loading', { defaultValue: '…' }) : t('modelSupply.batch.pick')}
              searchPlaceholder={t('modelSupply.add.searchChannel')}
              emptyText={t('modelSupply.add.noChannel')}
              className="rounded-[10px]"
            />
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => setAction(null)} disabled={busy}>
              {t('common.cancel')}
            </Button>
            <Button variant={action === 'remove' ? 'destructive' : 'default'} onClick={submit} disabled={!channelId || busy}>
              {busy && <Loader2 className="size-4 animate-spin" />}
              {t(action === 'remove' ? 'modelSupply.batch.confirmRemove' : 'modelSupply.batch.confirmAdd')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

SupplyBatchMenu.propTypes = { rows: PropTypes.array.isRequired, onDone: PropTypes.func.isRequired };
