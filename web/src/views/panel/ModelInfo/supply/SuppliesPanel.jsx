import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { ArrowDown, ArrowUp, ArrowUpRight, Loader2, MoreHorizontal, Pause, Pencil, Play, Plus, Trash2 } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Combobox } from '@/components/ui/combobox';
import { toast } from '@/components/ui/sonner';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import { createSupply, deleteSupply, listPauses, listSupplies, pauseSupply, resumePause, updateSupply } from './supplyApi';
import {
  PAUSE_DURATIONS,
  SUPPLY_MODES,
  defaultNewPriority,
  groupTiers,
  moveSupply,
  normalizeSupplyMode,
  parseSupplyForm,
  supplyCandidates,
  supplyState,
  supplyUpdateBody,
  tierShares,
  upstreamFor
} from './supplyHelpers';
import { SupplyStateLabel, SupplySwitchNote, durationLabel } from './supplyParts';

const channelTypeName = (type) => CHANNEL_OPTIONS[type]?.text || `#${type}`;

// Two-option switch for the supply mode, saved as soon as it changes.
function ModeSwitch({ value, onChange, disabled }) {
  const { t } = useTranslation();
  return (
    <div role="radiogroup" aria-label={t('modelSupply.mode.label')} className="inline-flex rounded-[10px] bg-muted p-0.5">
      {SUPPLY_MODES.map((mode) => (
        <button
          key={mode}
          type="button"
          role="radio"
          aria-checked={value === mode}
          disabled={disabled}
          onClick={() => value !== mode && onChange(mode)}
          className={cn(
            'h-7 rounded-lg px-3 text-[13px] font-medium transition-colors disabled:opacity-50',
            value === mode ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
          )}
        >
          {t(`modelSupply.mode.${mode}`)}
        </button>
      ))}
    </div>
  );
}

ModeSwitch.propTypes = { value: PropTypes.string, onChange: PropTypes.func.isRequired, disabled: PropTypes.bool };

function NumberCell({ value, onChange, label, className }) {
  return (
    <Input
      type="number"
      inputMode="numeric"
      aria-label={label}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className={cn('h-8 rounded-lg px-2 tabular-nums', className)}
    />
  );
}

NumberCell.propTypes = {
  value: PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  onChange: PropTypes.func,
  label: PropTypes.string,
  className: PropTypes.string
};

// Form to add a supply: pick a channel (the ones that already list the model first), the
// upstream name is filled in from the channel's model mapping; priority defaults to a backup tier.
function AddSupplyForm({ model, channels, supplies, onCancel, onAdded }) {
  const { t } = useTranslation();
  const { offering, others } = useMemo(() => supplyCandidates(channels, supplies, model), [channels, supplies, model]);
  const [channelId, setChannelId] = useState(() => (offering[0] ? String(offering[0].id) : ''));
  const [form, setForm] = useState(() => ({
    upstream_model: offering[0] ? upstreamFor(offering[0], model) : model,
    priority: String(defaultNewPriority(supplies)),
    weight: '1'
  }));
  const [saving, setSaving] = useState(false);

  const toOption = (c) => ({
    value: String(c.id),
    label: c.name || `#${c.id}`,
    icon: <ProviderIcon type={c.type} name={c.name} baseUrl={c.base_url} className="size-4" />,
    hint: c.status === 1 ? `#${c.id}` : t('modelSupply.state.channel_off')
  });
  const groups = [
    { key: 'offering', heading: t('modelSupply.add.offering'), options: offering.map(toOption) },
    { key: 'others', heading: t('modelSupply.add.others'), options: others.map(toOption) }
  ].filter((g) => g.options.length > 0);

  const pickChannel = (value) => {
    setChannelId(value);
    const channel = channels.find((c) => String(c.id) === value);
    if (channel) setForm((f) => ({ ...f, upstream_model: upstreamFor(channel, model) }));
  };

  const submit = async () => {
    if (!channelId) return toast.error(t('modelSupply.add.channelRequired'));
    const parsed = parseSupplyForm(form);
    if (parsed.error) return toast.error(t(parsed.error));
    setSaving(true);
    try {
      const { success, message } = await createSupply({ model, channel_id: Number(channelId), ...parsed, note: '' });
      if (!success) return toast.error(message);
      toast.success(t('modelSupply.add.done'));
      onAdded();
    } catch {
      /* shown by the API client */
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-3 rounded-[10px] border border-border p-4">
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <label htmlFor="supply-channel" className="text-[13px] font-medium">
            {t('modelSupply.channel')}
          </label>
          <Combobox
            id="supply-channel"
            value={channelId}
            onValueChange={pickChannel}
            groups={groups}
            placeholder={t('modelSupply.add.pickChannel')}
            searchPlaceholder={t('modelSupply.add.searchChannel')}
            emptyText={t('modelSupply.add.noChannel')}
            className="rounded-[10px]"
          />
        </div>
        <div className="space-y-1.5">
          <label htmlFor="supply-upstream" className="text-[13px] font-medium">
            {t('modelSupply.upstream')}
          </label>
          <Input
            id="supply-upstream"
            value={form.upstream_model}
            onChange={(e) => setForm({ ...form, upstream_model: e.target.value })}
            className="rounded-[10px] font-mono"
          />
        </div>
      </div>
      <div className="flex flex-wrap items-end gap-3">
        <div className="w-28 space-y-1.5">
          <label htmlFor="supply-priority" className="text-[13px] font-medium">
            {t('modelSupply.priority')}
          </label>
          <Input
            id="supply-priority"
            type="number"
            value={form.priority}
            onChange={(e) => setForm({ ...form, priority: e.target.value })}
            className="rounded-[10px] tabular-nums"
          />
        </div>
        <div className="w-28 space-y-1.5">
          <label htmlFor="supply-weight" className="text-[13px] font-medium">
            {t('modelSupply.weight')}
          </label>
          <Input
            id="supply-weight"
            type="number"
            min={0}
            value={form.weight}
            onChange={(e) => setForm({ ...form, weight: e.target.value })}
            className="rounded-[10px] tabular-nums"
          />
        </div>
        <p className="min-w-0 flex-1 pb-2 text-xs text-muted-foreground">{t('modelSupply.add.hint')}</p>
      </div>
      <div className="flex justify-end gap-2">
        <Button variant="outline" size="sm" className="rounded-[10px]" onClick={onCancel}>
          {t('common.cancel')}
        </Button>
        <Button size="sm" className="rounded-[10px]" disabled={saving} onClick={submit}>
          {saving && <Loader2 className="animate-spin" />}
          {t('modelSupply.add.submit')}
        </Button>
      </div>
    </div>
  );
}

AddSupplyForm.propTypes = {
  model: PropTypes.string.isRequired,
  channels: PropTypes.array.isRequired,
  supplies: PropTypes.array.isRequired,
  onCancel: PropTypes.func.isRequired,
  onAdded: PropTypes.func.isRequired
};

function SupplyRow({ supply, state, share, canMoveUp, canMoveDown, busy, onAction }) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(null);
  const offline = state.kind === 'channel_off' || state.kind === 'disabled';

  const save = async () => {
    const parsed = parseSupplyForm(editing);
    if (parsed.error) return toast.error(t(parsed.error));
    if (await onAction('update', supply, parsed)) setEditing(null);
  };

  const channelCell = (
    <div className="flex min-w-0 items-center gap-2">
      <span className={cn('inline-flex shrink-0', state.kind === 'channel_off' && 'opacity-40 grayscale')}>
        <ProviderIcon type={supply.channel_type} name={supply.channel_name} className="size-4" />
      </span>
      <div className="min-w-0">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className={cn('truncate', offline && 'text-muted-foreground')} title={supply.channel_name}>
            {supply.channel_name || t('modelSupply.channelDeleted', { id: supply.channel_id })}
          </span>
          {supply.channel_name && (
            <Link
              to={`/panel/channel/${supply.channel_id}`}
              aria-label={t('modelSupply.openChannel', { name: supply.channel_name })}
              className="shrink-0 text-muted-foreground hover:text-foreground"
            >
              <ArrowUpRight className="size-3.5" />
            </Link>
          )}
        </div>
        <p className="truncate text-xs text-muted-foreground">
          {[channelTypeName(supply.channel_type), supply.channel_tag].filter(Boolean).join(' · ')}
        </p>
      </div>
    </div>
  );

  if (editing) {
    return (
      <tr data-testid="supply-row" className="[&>td]:border-b [&>td]:border-border [&>td]:px-3 [&>td]:py-2 last:[&>td]:border-b-0">
        <td>{channelCell}</td>
        <td>
          <Input
            aria-label={t('modelSupply.upstream')}
            value={editing.upstream_model}
            onChange={(e) => setEditing({ ...editing, upstream_model: e.target.value })}
            className="h-8 rounded-lg px-2 font-mono"
          />
        </td>
        <td colSpan={3}>
          <div className="flex items-center gap-2">
            <NumberCell
              label={t('modelSupply.priority')}
              value={editing.priority}
              onChange={(v) => setEditing({ ...editing, priority: v })}
              className="w-20"
            />
            <NumberCell
              label={t('modelSupply.weight')}
              value={editing.weight}
              onChange={(v) => setEditing({ ...editing, weight: v })}
              className="w-20"
            />
            <span className="flex-1" />
            <Button variant="ghost" size="sm" className="rounded-lg" onClick={() => setEditing(null)}>
              {t('common.cancel')}
            </Button>
            <Button size="sm" className="rounded-lg" disabled={busy} onClick={save}>
              {busy && <Loader2 className="animate-spin" />}
              {t('modelSupply.save')}
            </Button>
          </div>
        </td>
      </tr>
    );
  }

  const pauses = state.pauses || [];
  return (
    <tr data-testid="supply-row" className="[&>td]:h-12 [&>td]:border-b [&>td]:border-border [&>td]:px-3 last:[&>td]:border-b-0">
      <td>{channelCell}</td>
      <td className="max-w-0 truncate font-mono text-[13px] text-muted-foreground" title={supply.upstream_model}>
        {supply.upstream_model}
      </td>
      <td className="whitespace-nowrap text-right tabular-nums">
        {supply.weight || 1}
        {share !== undefined && <span className="ml-1.5 text-xs text-muted-foreground">{share}%</span>}
      </td>
      <td className="overflow-hidden">
        <SupplyStateLabel state={state} />
      </td>
      <td className="!px-1">
        <div className="flex items-center justify-end gap-1">
          <Switch
            checked={supply.enabled !== false}
            disabled={busy}
            aria-label={t('modelSupply.enabled')}
            onCheckedChange={(enabled) => onAction('update', supply, { enabled })}
          />
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="size-8 rounded-lg" aria-label={t('modelSupply.actions')}>
                {busy ? <Loader2 className="size-4 animate-spin" /> : <MoreHorizontal className="size-4" />}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-52">
              <DropdownMenuItem
                onClick={() =>
                  setEditing({
                    upstream_model: supply.upstream_model,
                    priority: String(supply.priority ?? 0),
                    weight: String(supply.weight ?? 1)
                  })
                }
              >
                <Pencil className="size-4" />
                {t('modelSupply.edit')}
              </DropdownMenuItem>
              <DropdownMenuItem disabled={!canMoveUp} onClick={() => onAction('move', supply, 'up')}>
                <ArrowUp className="size-4" />
                {t('modelSupply.moveUp')}
              </DropdownMenuItem>
              <DropdownMenuItem disabled={!canMoveDown} onClick={() => onAction('move', supply, 'down')}>
                <ArrowDown className="size-4" />
                {t('modelSupply.moveDown')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              {pauses.length > 0 ? (
                pauses.map((p) => (
                  <DropdownMenuItem key={p.id} onClick={() => onAction('resume', supply, p)}>
                    <Play className="size-4" />
                    {p.model ? t('modelSupply.resume') : t('modelSupply.resumeChannel')}
                  </DropdownMenuItem>
                ))
              ) : (
                <>
                  <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">{t('modelSupply.pauseFor')}</DropdownMenuLabel>
                  {PAUSE_DURATIONS.map((seconds) => (
                    <DropdownMenuItem key={seconds} disabled={offline} onClick={() => onAction('pause', supply, seconds)}>
                      <Pause className="size-4" />
                      {durationLabel(t, seconds)}
                    </DropdownMenuItem>
                  ))}
                </>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => onAction('delete', supply)}>
                <Trash2 className="size-4" />
                {t('modelSupply.delete')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </td>
    </tr>
  );
}

SupplyRow.propTypes = {
  supply: PropTypes.object.isRequired,
  state: PropTypes.object.isRequired,
  share: PropTypes.number,
  canMoveUp: PropTypes.bool,
  canMoveDown: PropTypes.bool,
  busy: PropTypes.bool,
  onAction: PropTypes.func.isRequired
};

// "Supplies" tab of the model panel: which channels serve this public model, in priority tiers.
// Every change is saved right away; the supply mode switch saves the model row.
export default function SuppliesPanel({ model, supplyMode, onSupplyModeChange, channels = [], switches = null, onChanged }) {
  const { t } = useTranslation();
  const [supplies, setSupplies] = useState(null);
  const [pauses, setPauses] = useState([]);
  const [now, setNow] = useState(0);
  const [adding, setAdding] = useState(false);
  const [busyId, setBusyId] = useState(0);
  const [modeSaving, setModeSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const mode = normalizeSupplyMode(supplyMode);

  const load = useCallback(async () => {
    try {
      const [s, p] = await Promise.all([listSupplies({ model }), listPauses()]);
      setSupplies(s.success ? s.data || [] : []);
      setPauses(p.success ? p.data || [] : []);
      setNow(Math.floor(Date.now() / 1000));
      if (!s.success) toast.error(s.message);
    } catch {
      setSupplies([]);
    }
  }, [model]);

  useEffect(() => {
    setSupplies(null);
    setAdding(false);
    load();
  }, [load]);

  const reload = async () => {
    await load();
    onChanged?.();
  };

  const tiers = useMemo(() => groupTiers(supplies || []), [supplies]);

  const run = async (id, fn) => {
    setBusyId(id);
    try {
      const results = await fn();
      const failed = [].concat(results).find((r) => !r?.success);
      if (failed) {
        toast.error(failed.message);
        return false;
      }
      return true;
    } catch {
      return false;
    } finally {
      setBusyId(0);
      await reload();
    }
  };

  const onAction = async (kind, supply, arg) => {
    switch (kind) {
      case 'update':
        return run(supply.id, () => updateSupply(supplyUpdateBody(supply, arg)));
      case 'move': {
        const changes = moveSupply(supplies, supply.id, arg);
        const byId = new Map(supplies.map((s) => [s.id, s]));
        return run(supply.id, () =>
          Promise.all(changes.map((c) => updateSupply(supplyUpdateBody(byId.get(c.id), { priority: c.priority }))))
        );
      }
      case 'pause': {
        const ok = await run(supply.id, () => pauseSupply({ channel_id: supply.channel_id, model: supply.model, seconds: arg }));
        if (ok) toast.success(t('modelSupply.paused', { time: durationLabel(t, arg) }));
        return ok;
      }
      case 'resume': {
        const ok = await run(supply.id, () => resumePause(arg.id));
        if (ok) toast.success(t('modelSupply.resumed'));
        return ok;
      }
      case 'delete':
        setDeleteTarget(supply);
        return false;
      default:
        return false;
    }
  };

  const confirmDelete = async () => {
    const target = deleteTarget;
    setDeleteTarget(null);
    const ok = await run(target.id, () => deleteSupply(target.id));
    if (ok) toast.success(t('modelSupply.deleted'));
  };

  const changeMode = async (next) => {
    setModeSaving(true);
    try {
      await onSupplyModeChange(next);
    } finally {
      setModeSaving(false);
    }
  };

  const tierLabel = (index) => {
    if (tiers.length === 1) return t('modelSupply.tier.only');
    if (index === 0) return t('modelSupply.tier.first');
    return t('modelSupply.tier.backup', { count: index });
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <span className="text-[13px] text-muted-foreground">{t('modelSupply.mode.label')}</span>
        <ModeSwitch value={mode} onChange={changeMode} disabled={modeSaving || !onSupplyModeChange} />
        <span className="flex-1" />
        {!adding && (
          <Button variant="outline" size="sm" className="rounded-[10px]" onClick={() => setAdding(true)} disabled={supplies === null}>
            <Plus className="size-4" />
            {t('modelSupply.add.button')}
          </Button>
        )}
      </div>
      <p className="-mt-2 text-xs text-muted-foreground">{t(`modelSupply.mode.${mode}Hint`)}</p>

      <SupplySwitchNote switches={switches} />

      {adding && supplies && (
        <AddSupplyForm
          model={model}
          channels={channels}
          supplies={supplies}
          onCancel={() => setAdding(false)}
          onAdded={async () => {
            setAdding(false);
            await reload();
          }}
        />
      )}

      {supplies === null ? (
        <div className="flex h-32 items-center justify-center text-muted-foreground">
          <Loader2 className="size-5 animate-spin" />
        </div>
      ) : supplies.length === 0 ? (
        !adding && (
          <div className="rounded-[10px] border border-dashed border-border px-4 py-8 text-center">
            <p className="text-sm">{t('modelSupply.empty')}</p>
            <p className="mt-1 text-xs text-muted-foreground">{t('modelSupply.emptyHint')}</p>
          </div>
        )
      ) : (
        <div className="overflow-x-auto rounded-[10px] border border-border">
          <table className="w-full min-w-[480px] table-fixed border-collapse text-[13px]">
            <colgroup>
              <col className="w-[30%]" />
              <col />
              <col className="w-[4.5rem]" />
              <col className="w-[26%]" />
              <col className="w-[5.5rem]" />
            </colgroup>
            <thead>
              <tr className="text-left text-xs text-muted-foreground [&>th]:h-9 [&>th]:whitespace-nowrap [&>th]:border-b [&>th]:border-border [&>th]:px-3 [&>th]:font-medium">
                <th>{t('modelSupply.channel')}</th>
                <th>{t('modelSupply.upstream')}</th>
                <th className="text-right">{t('modelSupply.weight')}</th>
                <th>{t('modelSupply.stateLabel')}</th>
                <th>
                  <span className="sr-only">{t('modelSupply.actions')}</span>
                </th>
              </tr>
            </thead>
            {tiers.map((tier, index) => {
              const shares = tierShares(tier, mode);
              return (
                <tbody key={tier.priority}>
                  <tr>
                    <td colSpan={5} className="border-b border-border bg-muted/40 px-3 py-1.5 text-xs text-muted-foreground">
                      <span className="text-foreground">{tierLabel(index)}</span>
                      <span className="ml-2 tabular-nums">{t('modelSupply.tier.priority', { priority: tier.priority })}</span>
                    </td>
                  </tr>
                  {tier.supplies.map((supply) => {
                    const up = moveSupply(supplies, supply.id, 'up').length > 0;
                    const down = moveSupply(supplies, supply.id, 'down').length > 0;
                    return (
                      <SupplyRow
                        key={supply.id}
                        supply={supply}
                        state={supplyState(supply, pauses, now)}
                        share={tier.supplies.length > 1 ? (shares[supply.id] ?? 0) : undefined}
                        canMoveUp={up}
                        canMoveDown={down}
                        busy={busyId === supply.id}
                        onAction={onAction}
                      />
                    );
                  })}
                </tbody>
              );
            })}
          </table>
        </div>
      )}

      {supplies && supplies.length > 0 && <p className="text-xs text-muted-foreground">{t('modelSupply.orderHint')}</p>}

      <Dialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('modelSupply.deleteTitle')}</DialogTitle>
            <DialogDescription>
              {t('modelSupply.deleteConfirm', { channel: deleteTarget?.channel_name || `#${deleteTarget?.channel_id}`, model })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={confirmDelete}>
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

SuppliesPanel.propTypes = {
  model: PropTypes.string.isRequired,
  supplyMode: PropTypes.string,
  onSupplyModeChange: PropTypes.func,
  channels: PropTypes.array,
  switches: PropTypes.object,
  onChanged: PropTypes.func
};
