import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { ArrowDown, ArrowUp, ArrowUpRight, Link2, Loader2, MoreHorizontal, Pause, Pencil, Play, Plus, Trash2 } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Combobox } from '@/components/ui/combobox';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
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
import {
  batchSupplies,
  createSupply,
  deleteSupply,
  listPauses,
  listUserGroups,
  pauseSupply,
  previewSupplies,
  resumePause,
  suggestUpstream,
  updateSupply
} from './supplyApi';
import {
  PAUSE_DURATIONS,
  SUPPLY_MODES,
  channelGroups,
  defaultNewPriority,
  followsChannel,
  moveSupply,
  normalizeSupplyMode,
  parseSupplyForm,
  previewEntryState,
  previewOutcome,
  previewShares,
  previewSupplyList,
  supplyCandidates,
  supplyUpdateBody,
  upstreamFor
} from './supplyHelpers';
import { SupplyStateLabel, SupplySwitchNote, UpstreamName, durationLabel } from './supplyParts';
import { setModelAllocation } from '../../Allocation/allocationApi';

const channelTypeName = (type) => CHANNEL_OPTIONS[type]?.text || `#${type}`;

// Switch for the supply mode (follow the default, balance, failover), saved as soon as it changes.
function ModeSwitch({ value, onChange, disabled }) {
  const { t } = useTranslation();
  return (
    <div role="radiogroup" aria-label={t('modelSupply.mode.label')} className="inline-flex rounded-lg bg-control p-0.5">
      {SUPPLY_MODES.map((mode) => (
        <button
          key={mode}
          type="button"
          role="radio"
          aria-checked={value === mode}
          disabled={disabled}
          onClick={() => value !== mode && onChange(mode)}
          className={cn(
            'h-7 rounded-md px-3 text-[13px] font-medium transition-colors disabled:opacity-50',
            value === mode ? 'bg-control-thumb text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
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
  const [upstreamSource, setUpstreamSource] = useState('');

  // The backend knows more than the channel's mapping (the channel's own list and how its
  // platform names the model), so ask it whenever the channel changes.
  useEffect(() => {
    if (!channelId) return undefined;
    let cancelled = false;
    suggestUpstream({ model, channel_id: Number(channelId) })
      .then(({ success, data }) => {
        if (cancelled || !success || !data?.upstream_model) return;
        setForm((f) => ({ ...f, upstream_model: data.upstream_model }));
        setUpstreamSource(data.source || '');
      })
      .catch(() => {
        /* keep the mapping-based guess */
      });
    return () => {
      cancelled = true;
    };
  }, [channelId, model]);

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
    setUpstreamSource('');
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
            onChange={(e) => {
              setForm({ ...form, upstream_model: e.target.value });
              setUpstreamSource('');
            }}
            className="rounded-[10px] font-mono"
          />
          {upstreamSource === 'library' && <p className="text-xs text-muted-foreground">{t('modelSupply.add.fromLibrary')}</p>}
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

const groupLabel = (g) => (g.name && g.name !== g.symbol ? `${g.name} (${g.symbol})` : g.symbol);

// Which supply a request from a user group would try first, from the preview API. An empty
// group previews a request from any group.
function RequestPreview({ groups, group, onGroupChange, preview, failed = false }) {
  const { t } = useTranslation();
  let summary = null;
  if (preview) {
    const outcome = previewOutcome(preview);
    if (outcome.kind === 'first') {
      const { supply } = outcome;
      summary = t('modelSupply.preview.first', { channel: supply.channel_name || `#${supply.channel_id}` });
    } else if (outcome.kind === 'split') {
      summary = t('modelSupply.preview.split', { count: outcome.count });
    } else {
      summary = t('modelSupply.preview.none');
    }
  }
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-[10px] bg-muted/40 px-3 py-2" data-testid="request-preview">
      <span className="text-[13px] text-muted-foreground">{t('modelSupply.preview.label')}</span>
      {groups.length > 0 && (
        <div className="w-48">
          <Select value={group} onValueChange={onGroupChange}>
            <SelectTrigger className="h-8 rounded-lg text-[13px]" aria-label={t('modelSupply.preview.group')}>
              <SelectValue placeholder={t('modelSupply.preview.anyGroup')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t('modelSupply.preview.anyGroup')}</SelectItem>
              {groups.map((g) => (
                <SelectItem key={g.symbol} value={g.symbol}>
                  {groupLabel(g)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
      <p className="min-w-0 flex-1 text-[13px]" aria-live="polite">
        {preview ? (
          <>
            {summary}
            {group && preview.outside_group > 0 && (
              <span className="text-muted-foreground"> · {t('modelSupply.preview.outside', { count: preview.outside_group, group })}</span>
            )}
            {preview.active === false && <span className="text-muted-foreground"> · {t('modelSupply.preview.notActive')}</span>}
          </>
        ) : failed ? (
          <span className="text-muted-foreground">{t('modelSupply.preview.failed')}</span>
        ) : (
          <Loader2 className="size-3.5 animate-spin text-muted-foreground" />
        )}
      </p>
    </div>
  );
}

RequestPreview.propTypes = {
  groups: PropTypes.array.isRequired,
  group: PropTypes.string.isRequired,
  onGroupChange: PropTypes.func.isRequired,
  preview: PropTypes.object,
  failed: PropTypes.bool
};

function SupplyRow({ supply, state, share, next, outsideGroup, canMoveUp, canMoveDown, busy, onAction }) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(null);
  const offline = state.kind === 'channel_off' || state.kind === 'disabled';
  const groups = channelGroups(supply);
  const groupsText = groups.length > 0 ? t('modelSupply.channelGroups', { groups: groups.join(', ') }) : '';

  const save = async () => {
    const parsed = parseSupplyForm(editing);
    if (parsed.error) return toast.error(t(parsed.error));
    if (await onAction('update', supply, parsed)) setEditing(null);
  };

  // Its own priority or weight: a change to the channel's numbers will not move this supply.
  // Supplies on the allocation policy take the policy's numbers, so this does not apply.
  const own = !supply.follows_policy && !followsChannel(supply);

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
          {next && (
            <span className="shrink-0 rounded-md bg-muted px-1.5 text-xs leading-5 text-muted-foreground">
              {t('modelSupply.preview.next')}
            </span>
          )}
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
        <p className="truncate text-xs text-muted-foreground" title={groupsText || undefined}>
          {own && !outsideGroup && (
            <span className="text-foreground" title={t('modelSupply.ownNumbersTip')}>
              {t('modelSupply.ownNumbers')}
              {' · '}
            </span>
          )}
          {outsideGroup
            ? t('modelSupply.preview.outsideRow', { group: outsideGroup })
            : [channelTypeName(supply.channel_type), supply.channel_tag, groupsText].filter(Boolean).join(' · ')}
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
    <tr
      data-testid="supply-row"
      className={cn(
        '[&>td]:h-12 [&>td]:border-b [&>td]:border-border [&>td]:px-3 last:[&>td]:border-b-0',
        outsideGroup && '[&>td:not(:last-child)]:opacity-50'
      )}
    >
      <td>{channelCell}</td>
      <td className="py-1.5">
        <UpstreamName upstream={supply.upstream_model} model={supply.model} />
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
              {own && (
                <DropdownMenuItem
                  onClick={() => onAction('update', supply, { priority: supply.channel_priority, weight: supply.channel_weight })}
                >
                  <Link2 className="size-4" />
                  {t('modelSupply.followChannel')}
                </DropdownMenuItem>
              )}
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
  next: PropTypes.bool,
  outsideGroup: PropTypes.string,
  canMoveUp: PropTypes.bool,
  canMoveDown: PropTypes.bool,
  busy: PropTypes.bool,
  onAction: PropTypes.func.isRequired
};

// "Supplies" tab of the model panel: which channels serve this public model, in priority tiers.
// Every change is saved right away; the supply mode switch saves the model row.
export default function SuppliesPanel({ model, catalogId, supplyMode, onSupplyModeChange, channels = [], switches = null, onChanged }) {
  const { t } = useTranslation();
  // `preview` is every supply in order with its state; `groupPreview` the same for a request
  // from `group` only. Both come from the preview API.
  const [preview, setPreview] = useState(null);
  const [groupPreview, setGroupPreview] = useState(null);
  const [groupFailed, setGroupFailed] = useState(false);
  const [pauses, setPauses] = useState([]);
  const [groups, setGroups] = useState([]);
  const [group, setGroup] = useState('');
  const [adding, setAdding] = useState(false);
  const [busyId, setBusyId] = useState(0);
  const [modeSaving, setModeSaving] = useState(false);
  const [allocationSaving, setAllocationSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const mode = normalizeSupplyMode(supplyMode);

  const requestId = useRef(0);
  const load = useCallback(async () => {
    const id = ++requestId.current;
    try {
      const [s, p, g] = await Promise.all([
        previewSupplies({ model }),
        listPauses(),
        group ? previewSupplies({ model, group }).catch(() => null) : null
      ]);
      if (id !== requestId.current) return;
      setPreview(s.success ? s.data || { tiers: [] } : { tiers: [] });
      setPauses(p.success ? p.data || [] : []);
      setGroupPreview(g?.success ? g.data : null);
      setGroupFailed(Boolean(group) && !g?.success);
      if (!s.success) toast.error(s.message);
      else if (g && !g.success) toast.error(g.message);
    } catch {
      if (id === requestId.current) setPreview({ tiers: [] });
    }
  }, [model, group]);

  useEffect(() => {
    setPreview(null);
    setGroupPreview(null);
    setGroup('');
    setAdding(false);
  }, [model]);

  useEffect(() => {
    listUserGroups().then(setGroups);
  }, []);

  // The mode changes the shares, so the preview is read again with it.
  useEffect(() => {
    load();
  }, [load, mode]);

  const changeGroup = (next) => {
    setGroupPreview(null);
    setGroupFailed(false);
    setGroup(next);
  };

  const reload = async () => {
    await load();
    onChanged?.();
  };

  const supplies = useMemo(() => (preview ? previewSupplyList(preview) : null), [preview]);
  const tiers = preview?.tiers || [];
  // What a request from the chosen group (or any group) does: its shares, its first choice and
  // the supplies it leaves out.
  const view = group ? groupPreview : preview;
  const serving = useMemo(() => previewShares(view), [view]);
  const showShares = Object.keys(serving.shares).length > 1;
  const inGroup = useMemo(
    () => (group && groupPreview ? new Set(previewSupplyList(groupPreview).map((s) => s.id)) : null),
    [group, groupPreview]
  );

  // Enabled channels that list this model and are not a supply yet: one click adds them with the
  // suggested upstream name and the channel's own priority and weight.
  const available = useMemo(
    () => (supplies ? supplyCandidates(channels, supplies, model).offering.filter((c) => c.status === 1) : []),
    [channels, supplies, model]
  );

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

  // Renumbering a supply of a model on the allocation policy makes the model manual (the server
  // keeps the other supplies where the policy put them).
  const noteManual = (ok) => {
    if (ok && allocation === 'policy') toast.success(t('modelSupply.allocation.becameManual'));
    return ok;
  };

  const onAction = async (kind, supply, arg) => {
    switch (kind) {
      case 'update': {
        const renumber = arg && (arg.priority !== undefined || arg.weight !== undefined);
        const ok = await run(supply.id, () => updateSupply(supplyUpdateBody(supply, arg)));
        return renumber ? noteManual(ok) : ok;
      }
      case 'move': {
        const changes = moveSupply(supplies, supply.id, arg);
        const byId = new Map(supplies.map((s) => [s.id, s]));
        return noteManual(
          await run(supply.id, () =>
            Promise.all(changes.map((c) => updateSupply(supplyUpdateBody(byId.get(c.id), { priority: c.priority }))))
          )
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

  const quickAdd = async (channel) => {
    // Negative ids keep the row spinner apart from supply ids.
    const ok = await run(-channel.id, () => batchSupplies({ action: 'add', channel_id: channel.id, models: [model] }));
    if (ok) toast.success(t('modelSupply.add.done'));
  };

  const confirmDelete = async () => {
    const target = deleteTarget;
    setDeleteTarget(null);
    const ok = await run(target.id, () => deleteSupply(target.id));
    if (ok) toast.success(t('modelSupply.deleted'));
  };

  const allocation = preview?.allocation || '';
  const changeAllocation = async (next) => {
    if (!catalogId) return;
    setAllocationSaving(true);
    try {
      const res = await setModelAllocation({ ids: [catalogId], allocation: next });
      if (!res.success) return toast.error(res.message);
      toast.success(t(next === 'manual' ? 'modelSupply.allocation.nowManual' : 'modelSupply.allocation.nowPolicy'));
      await reload();
    } catch {
      /* shown by the API client */
    } finally {
      setAllocationSaving(false);
    }
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
      {allocation && (
        <div className="flex flex-wrap items-center gap-3">
          <span className="text-[13px] text-muted-foreground">{t('modelSupply.allocation.label')}</span>
          <span className="text-[13px]">
            {t(allocation === 'manual' ? 'modelSupply.allocation.manual' : 'modelSupply.allocation.policy')}
          </span>
          <Link to="/panel/channel" className="text-[13px] text-info underline-offset-2 hover:underline">
            {t('modelSupply.allocation.open')}
          </Link>
          <span className="flex-1" />
          {catalogId > 0 && (
            <Button
              variant="outline"
              size="sm"
              className="rounded-[10px]"
              disabled={allocationSaving}
              onClick={() => changeAllocation(allocation === 'manual' ? 'policy' : 'manual')}
            >
              {allocationSaving && <Loader2 className="size-4 animate-spin" />}
              {t(allocation === 'manual' ? 'modelSupply.allocation.toPolicy' : 'modelSupply.allocation.toManual')}
            </Button>
          )}
        </div>
      )}
      {allocation && (
        <p className="-mt-2 text-xs text-muted-foreground">
          {t(allocation === 'manual' ? 'modelSupply.allocation.manualHint' : 'modelSupply.allocation.policyHint')}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-3">
        {allocation !== 'policy' && (
          <>
            <span className="text-[13px] text-muted-foreground">{t('modelSupply.mode.label')}</span>
            <ModeSwitch value={mode} onChange={changeMode} disabled={modeSaving || !onSupplyModeChange} />
          </>
        )}
        <span className="flex-1" />
        {!adding && (
          <Button variant="outline" size="sm" className="rounded-[10px]" onClick={() => setAdding(true)} disabled={supplies === null}>
            <Plus className="size-4" />
            {t('modelSupply.add.button')}
          </Button>
        )}
      </div>
      {allocation !== 'policy' && (
        <p className="-mt-2 text-xs text-muted-foreground">
          {mode === 'default'
            ? t('modelSupply.mode.defaultHint', {
                mode: t(`modelSupply.mode.${preview?.default_mode === 'failover' ? 'failover' : 'balance'}`)
              })
            : t(`modelSupply.mode.${mode}Hint`)}
        </p>
      )}

      <SupplySwitchNote switches={switches} active={preview?.active} />

      {supplies && supplies.length > 0 && (
        <RequestPreview groups={groups} group={group} onGroupChange={changeGroup} preview={view} failed={Boolean(group) && groupFailed} />
      )}

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
              <col className="w-[28%]" />
              <col />
              <col className="w-[4.5rem]" />
              <col className="w-[18%]" />
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
            {tiers.map((tier, index) => (
              <tbody key={tier.priority}>
                <tr>
                  <td colSpan={5} className="border-b border-border bg-muted/40 px-3 py-1.5 text-xs text-muted-foreground">
                    <span className="text-foreground">{tierLabel(index)}</span>
                    {/* On the allocation policy the numbers are computed from the call order and mean
                        nothing on their own; a manual model shows the priority it was given. */}
                    {allocation !== 'policy' && (
                      <span className="ml-2 tabular-nums">{t('modelSupply.tier.priority', { priority: tier.priority })}</span>
                    )}
                    {view && serving.priority === tier.priority && <span> · {t('modelSupply.preview.servingTier')}</span>}
                  </td>
                </tr>
                {tier.supplies.map((supply) => {
                  const up = moveSupply(supplies, supply.id, 'up').length > 0;
                  const down = moveSupply(supplies, supply.id, 'down').length > 0;
                  const outside = inGroup && !inGroup.has(supply.id);
                  return (
                    <SupplyRow
                      key={supply.id}
                      supply={supply}
                      state={previewEntryState(supply, pauses)}
                      share={showShares && !outside ? serving.shares[supply.id] : undefined}
                      next={Boolean(view?.first_choice) && view.first_choice === supply.id}
                      outsideGroup={outside ? group : undefined}
                      canMoveUp={up}
                      canMoveDown={down}
                      busy={busyId === supply.id}
                      onAction={onAction}
                    />
                  );
                })}
              </tbody>
            ))}
          </table>
        </div>
      )}

      {supplies && supplies.length > 0 && <p className="text-xs text-muted-foreground">{t('modelSupply.orderHint')}</p>}

      {available.length > 0 && (
        <section className="space-y-2" aria-label={t('modelSupply.available.title')}>
          <div className="flex items-baseline justify-between gap-3">
            <h3 className="text-sm font-medium">{t('modelSupply.available.title')}</h3>
            <span className="text-xs text-muted-foreground">{t('modelSupply.available.hint')}</span>
          </div>
          <ul className="divide-y divide-border rounded-[10px] border border-dashed border-border">
            {available.map((c) => (
              <li key={c.id} data-testid="supply-available" className="flex items-center gap-2 px-3 py-2">
                <ProviderIcon type={c.type} name={c.name} baseUrl={c.base_url} className="size-4 shrink-0" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[13px]" title={c.name}>
                    {c.name}
                  </p>
                  <p className="truncate text-xs text-muted-foreground">
                    {[channelTypeName(c.type), c.tag, t('channels.usage.numbers', { priority: c.priority ?? 0, weight: c.weight || 1 })]
                      .filter(Boolean)
                      .join(' · ')}
                  </p>
                </div>
                <Button variant="outline" size="sm" className="h-[30px] rounded-lg" disabled={busyId === -c.id} onClick={() => quickAdd(c)}>
                  {busyId === -c.id ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}
                  {t('modelSupply.add.submit')}
                </Button>
              </li>
            ))}
          </ul>
        </section>
      )}

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
  catalogId: PropTypes.number,
  supplyMode: PropTypes.string,
  onSupplyModeChange: PropTypes.func,
  channels: PropTypes.array,
  switches: PropTypes.object,
  onChanged: PropTypes.func
};
