import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { Info, Link2, Loader2, RotateCcw, X } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { toast } from '@/components/ui/sonner';
import { attachPendingModel, ignorePendingModel, listCatalogRows, listPendingModels } from './supplyApi';
import { modelSupplyPath, pendingTarget, publicModelNames } from './supplyHelpers';
import { SupplySwitchNote, useSupplySwitches } from './supplyParts';

const SOURCES = ['alias', 'seed', 'bare'];

// Dialog that supplies a public model with a waiting upstream model on some of its channels.
function AttachDialog({ item, rows, onClose, onDone }) {
  const { t } = useTranslation();
  const [model, setModel] = useState(item.suggestion?.model || item.upstream_model);
  const [channelIds, setChannelIds] = useState(() => item.channels.map((c) => c.channel_id));
  const [saving, setSaving] = useState(false);
  const names = useMemo(() => publicModelNames(rows), [rows]);
  const target = pendingTarget(model, rows);

  const toggle = (id, on) => setChannelIds((ids) => (on ? [...ids, id] : ids.filter((x) => x !== id)));

  const submit = async () => {
    setSaving(true);
    try {
      const { success, message, data } = await attachPendingModel({
        upstream_model: item.upstream_model,
        model: target.model,
        channel_ids: channelIds,
        create: target.kind === 'new'
      });
      if (!success) {
        toast.error(message);
        return;
      }
      const attached = data?.supplies?.length || 0;
      const skipped = data?.skipped?.length || 0;
      if (attached > 0) toast.success(t('modelSupply.pendingPage.attached', { model: data.model, count: attached }));
      if (skipped > 0) toast.warning(t('modelSupply.pendingPage.skipped', { model: data.model, count: skipped }));
      onDone();
    } catch {
      /* shown by the API client */
    } finally {
      setSaving(false);
    }
  };

  const blocked = saving || channelIds.length === 0 || target.kind === 'empty' || target.kind === 'policy';

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('modelSupply.pendingPage.attachTitle', { model: item.upstream_model })}</DialogTitle>
          <DialogDescription>{t('modelSupply.pendingPage.attachDescription')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="pending-public-model">{t('modelSupply.pendingPage.publicModel')}</Label>
            <Input
              id="pending-public-model"
              list="pending-public-models"
              className="font-mono"
              value={model}
              onChange={(e) => setModel(e.target.value)}
            />
            <datalist id="pending-public-models">
              {names.map((name) => (
                <option key={name} value={name} />
              ))}
            </datalist>
            {target.kind === 'new' && (
              <p className="text-xs text-muted-foreground">{t('modelSupply.pendingPage.willCreate', { model: target.model })}</p>
            )}
            {target.kind === 'alias' && (
              <p className="text-xs text-muted-foreground">
                {t('modelSupply.pendingPage.isAlias', { model: target.model, canonical: target.canonical })}
              </p>
            )}
            {target.kind === 'policy' && (
              <p className="text-xs text-destructive">{t('modelSupply.pendingPage.isPolicy', { model: target.model })}</p>
            )}
          </div>
          <fieldset className="space-y-2">
            <legend className="mb-1.5 text-sm font-medium">{t('modelSupply.pendingPage.channels')}</legend>
            {item.channels.map((c) => (
              <label key={c.channel_id} className="flex items-center gap-2 text-[13px]">
                <Checkbox checked={channelIds.includes(c.channel_id)} onCheckedChange={(on) => toggle(c.channel_id, on === true)} />
                <span>{c.channel_name || `#${c.channel_id}`}</span>
                {c.channel_status !== 1 && <span className="text-xs text-muted-foreground">{t('modelSupply.state.channel_off')}</span>}
              </label>
            ))}
            {channelIds.length === 0 && <p className="text-xs text-destructive">{t('modelSupply.pendingPage.noChannel')}</p>}
          </fieldset>
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button onClick={submit} disabled={blocked}>
            {saving && <Loader2 className="animate-spin" />}
            {t('modelSupply.pendingPage.attachButton')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

AttachDialog.propTypes = {
  item: PropTypes.object.isRequired,
  rows: PropTypes.array.isRequired,
  onClose: PropTypes.func.isRequired,
  onDone: PropTypes.func.isRequired
};

// "New upstream models" tab of the model page: models a channel's upstream started listing that
// no supply uses yet. They stay private until an admin attaches them to a public model.
export default function PendingModels() {
  const { t } = useTranslation();
  const switches = useSupplySwitches();
  const [items, setItems] = useState(null);
  const [rows, setRows] = useState([]);
  const [attaching, setAttaching] = useState(null);
  const [busy, setBusy] = useState('');

  const load = useCallback(async () => {
    try {
      const [pending, catalog] = await Promise.all([listPendingModels(), listCatalogRows()]);
      setItems(pending.success ? pending.data || [] : []);
      setRows(catalog.success ? catalog.data || [] : []);
      if (!pending.success) toast.error(pending.message);
    } catch {
      setItems([]);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const ignore = async (item) => {
    setBusy(item.upstream_model);
    try {
      const { success, message } = await ignorePendingModel({ upstream_model: item.upstream_model });
      if (success) toast.success(t('modelSupply.pendingPage.ignored', { model: item.upstream_model }));
      else toast.error(message);
    } catch {
      /* shown by the API client */
    } finally {
      setBusy('');
      load();
    }
  };

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        <p className="min-w-0 flex-1 text-[13px] text-muted-foreground">{t('modelSupply.pendingPage.description')}</p>
        <Button variant="outline" size="sm" className="rounded-[10px]" onClick={load}>
          <RotateCcw className="size-4" />
          {t('modelInfoPage.refresh')}
        </Button>
      </div>

      <SupplySwitchNote switches={switches} />

      <div className="overflow-x-auto rounded-2xl border border-border bg-card">
        {items === null ? (
          <div className="flex h-40 items-center justify-center text-muted-foreground">
            <Loader2 className="size-5 animate-spin" />
          </div>
        ) : items.length === 0 ? (
          <div className="px-6 py-14 text-center">
            <p className="text-sm">{t('modelSupply.pendingPage.empty')}</p>
            <p className="mt-1 text-xs text-muted-foreground">{t('modelSupply.pendingPage.emptyHint')}</p>
          </div>
        ) : (
          <table className="w-full min-w-[720px] border-collapse text-[13px]">
            <thead>
              <tr className="text-left text-xs text-muted-foreground [&>th]:h-10 [&>th]:whitespace-nowrap [&>th]:border-b [&>th]:border-border [&>th]:px-4 [&>th]:font-medium">
                <th>{t('modelSupply.pendingPage.upstream')}</th>
                <th>{t('modelSupply.pendingPage.channels')}</th>
                <th>{t('modelSupply.pendingPage.suggestion')}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {items.map((item) => {
                const s = item.suggestion || {};
                return (
                  <tr
                    key={item.upstream_model}
                    data-testid="pending-model"
                    className="align-middle [&>td]:h-12 [&>td]:border-b [&>td]:border-border [&>td]:px-4 last:[&>td]:border-b-0"
                  >
                    <td className="whitespace-nowrap font-mono">{item.upstream_model}</td>
                    <td>
                      <span className="flex flex-wrap gap-x-2 gap-y-0.5">
                        {item.channels.map((c) => (
                          <Link key={c.channel_id} to={`/panel/channel/${c.channel_id}`} className="whitespace-nowrap hover:underline">
                            {c.channel_name || `#${c.channel_id}`}
                          </Link>
                        ))}
                      </span>
                    </td>
                    <td className="whitespace-nowrap">
                      <span className="inline-flex items-center gap-2">
                        {s.exists ? (
                          <Link to={modelSupplyPath(s.model)} className="font-mono hover:underline">
                            {s.model}
                          </Link>
                        ) : (
                          <span className="font-mono">{s.model}</span>
                        )}
                        {!s.exists && <Badge variant="outline">{t('modelSupply.pendingPage.isNew')}</Badge>}
                        {SOURCES.includes(s.source) && (
                          <span className="text-xs text-muted-foreground">{t(`modelSupply.pendingPage.source.${s.source}`)}</span>
                        )}
                      </span>
                    </td>
                    <td className="whitespace-nowrap text-right">
                      <span className="inline-flex gap-2">
                        <Button variant="outline" size="sm" className="rounded-[10px]" onClick={() => setAttaching(item)}>
                          <Link2 className="size-4" />
                          {t('modelSupply.pendingPage.attach')}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="rounded-[10px]"
                          disabled={busy === item.upstream_model}
                          onClick={() => ignore(item)}
                        >
                          {busy === item.upstream_model ? <Loader2 className="animate-spin" /> : <X className="size-4" />}
                          {t('modelSupply.pendingPage.ignore')}
                        </Button>
                      </span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      {items?.length > 0 && (
        <p className="flex items-start gap-2 text-xs text-muted-foreground">
          <Info className="mt-0.5 size-3.5 shrink-0" />
          <span>{t('modelSupply.pendingPage.ignoreHint')}</span>
        </p>
      )}

      {attaching && (
        <AttachDialog
          item={attaching}
          rows={rows}
          onClose={() => setAttaching(null)}
          onDone={() => {
            setAttaching(null);
            load();
          }}
        />
      )}
    </div>
  );
}
