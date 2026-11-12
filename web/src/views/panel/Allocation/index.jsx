import { useCallback, useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { Link } from 'react-router';
import { useTranslation } from 'react-i18next';
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  ChevronUp,
  Loader2,
  MoreHorizontal,
  Plus,
  RotateCcw,
  Split,
  Merge,
  Trash2,
  TriangleAlert
} from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { toast } from '@/components/ui/sonner';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import PageActions from '@/components/chrome/PageActions';
import { ProviderTile, useProviderName } from '../Channel/channelParts';
import { fetchAllocation, saveAllocation } from './allocationApi';
import {
  addProvider,
  canMoveEntry,
  entryKey,
  mergeChannels,
  missingEntry,
  moveEntry,
  moveTier,
  providerOfChannel,
  removeEntry,
  sameTiers,
  setWeight,
  splitOut,
  splitProvider,
  tierShares,
  unlistedProviders
} from './allocationModel';

// Allocation policy page: one default order of providers for every model. Tiers are tried first
// to last; providers in one tier share its traffic by weight. Channels of a provider share the
// provider's weight equally unless a channel has an entry of its own.

function useProviderTitle() {
  const providerName = useProviderName();
  const { t } = useTranslation();
  return (provider) => {
    if (!provider) return t('allocation.unknownProvider');
    const name = providerName(provider.type);
    return provider.host ? `${name} · ${provider.host}` : name;
  };
}

function EntryRow({ entry, share, shared, providers, actions }) {
  const { t } = useTranslation();
  const title = useProviderTitle();
  const provider = entry.channel_id ? providerOfChannel(providers, entry.channel_id) : providers.find((p) => p.key === entry.provider);
  const channel = entry.channel_id ? provider?.channels.find((c) => c.id === entry.channel_id) : null;
  const missing = missingEntry(entry, providers);
  const enabled = provider ? provider.channels.filter((c) => c.status === 1).length : 0;

  let name;
  let detail;
  if (missing) {
    name = entry.channel_id ? t('allocation.channelGone', { id: entry.channel_id }) : entry.provider;
    detail = t('allocation.gone');
  } else if (channel) {
    name = channel.name;
    detail = [title(provider), channel.status === 1 ? '' : t('allocation.channelOff')].filter(Boolean).join(' · ');
  } else {
    name = title(provider);
    detail = t('allocation.channels', { total: provider.channels.length, enabled });
  }

  return (
    <div data-testid="allocation-entry" className="flex min-h-12 items-center gap-3 px-3 py-2 [&+&]:border-t [&+&]:border-border">
      <span className={cn(missing && 'opacity-40 grayscale')}>
        <ProviderTile type={provider?.type} />
      </span>
      <div className="min-w-0 flex-1">
        <p className={cn('truncate text-sm', missing && 'text-muted-foreground')}>{name}</p>
        <p className="truncate text-xs text-muted-foreground">{detail}</p>
      </div>
      {shared && (
        <div className="flex items-center gap-2">
          <Input
            aria-label={t('allocation.weightOf', { name })}
            inputMode="numeric"
            className="h-8 w-16 rounded-lg px-2 text-right tabular-nums"
            value={String(entry.weight || 1)}
            onChange={(e) => actions.weight(e.target.value.replace(/\D/g, ''))}
          />
          <span className="w-10 text-right text-xs tabular-nums text-muted-foreground">{share}%</span>
        </div>
      )}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" className="size-8 rounded-lg" aria-label={t('allocation.entryActions', { name })}>
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          <DropdownMenuItem disabled={!actions.canUp} onClick={actions.up}>
            <ArrowUp className="size-4" />
            {t('allocation.moveUp')}
          </DropdownMenuItem>
          <DropdownMenuItem disabled={!actions.canDown} onClick={actions.down}>
            <ArrowDown className="size-4" />
            {t('allocation.moveDown')}
          </DropdownMenuItem>
          {shared && (
            <DropdownMenuItem onClick={actions.splitOut}>
              <Split className="size-4" />
              {t('allocation.ownTier')}
            </DropdownMenuItem>
          )}
          {!entry.channel_id && provider && provider.channels.length > 1 && (
            <DropdownMenuItem onClick={actions.splitProvider}>
              <Split className="size-4" />
              {t('allocation.byChannel')}
            </DropdownMenuItem>
          )}
          {entry.channel_id && actions.canMerge && (
            <DropdownMenuItem onClick={actions.merge}>
              <Merge className="size-4" />
              {t('allocation.mergeChannels')}
            </DropdownMenuItem>
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={actions.remove}>
            <Trash2 className="size-4" />
            {t('allocation.remove')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

EntryRow.propTypes = {
  entry: PropTypes.object.isRequired,
  share: PropTypes.number,
  shared: PropTypes.bool,
  providers: PropTypes.array.isRequired,
  actions: PropTypes.object.isRequired
};

export default function Allocation() {
  const { t } = useTranslation();
  const title = useProviderTitle();
  const [data, setData] = useState(null);
  const [tiers, setTiers] = useState([]);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await fetchAllocation();
      if (!res.success) return toast.error(res.message);
      setData(res.data);
      setTiers(res.data.tiers || []);
    } catch {
      /* shown by the API client */
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const providers = useMemo(() => data?.providers || [], [data]);
  const unlisted = useMemo(() => unlistedProviders(providers, tiers), [providers, tiers]);
  const dirty = data && !sameTiers(tiers, data.tiers || []);

  const save = async () => {
    setSaving(true);
    try {
      const res = await saveAllocation(tiers);
      if (!res.success) return toast.error(res.message);
      setData(res.data);
      setTiers(res.data.tiers || []);
      toast.success(t('allocation.saved'));
    } catch {
      /* shown by the API client */
    } finally {
      setSaving(false);
    }
  };

  if (!data) {
    return (
      <div className="flex h-40 items-center justify-center text-muted-foreground">
        <Loader2 className="size-5 animate-spin" />
      </div>
    );
  }

  const tierHint = (index) => {
    if (index === 0) return t('allocation.tierFirst');
    return t('allocation.tierNext');
  };

  return (
    <div className="mx-auto max-w-3xl space-y-5">
      <PageActions>
        {dirty && (
          <Button variant="outline" size="sm" onClick={() => setTiers(data.tiers || [])} disabled={saving}>
            <RotateCcw className="size-4" />
            {t('allocation.revert')}
          </Button>
        )}
        <Button size="sm" onClick={save} disabled={!dirty || saving}>
          {saving && <Loader2 className="size-4 animate-spin" />}
          {t('allocation.save')}
        </Button>
      </PageActions>

      {!data.supply_routing && (
        <div className="flex gap-2 rounded-[10px] border border-warning/40 bg-warning/10 p-3 text-sm">
          <TriangleAlert className="mt-0.5 size-4 shrink-0 text-warning" />
          <p>{t('allocation.notActive')}</p>
        </div>
      )}

      <p className="text-sm text-muted-foreground">
        {t('allocation.summary', { policy: data.policy_models, manual: data.manual_models })}{' '}
        <Link to="/panel/pricing" className="text-info underline-offset-2 hover:underline">
          {t('allocation.openModels')}
        </Link>
      </p>

      {tiers.length === 0 && unlisted.length === 0 && (
        <div className="rounded-[10px] border border-dashed border-border px-4 py-8 text-center text-sm text-muted-foreground">
          {t('allocation.empty')}
        </div>
      )}

      {tiers.map((tier, ti) => {
        const shares = tierShares(tier);
        const shared = tier.length > 1;
        return (
          <section key={tier.map(entryKey).join('|')} data-testid="allocation-tier" className="rounded-[10px] border border-border">
            <header className="flex items-center gap-2 border-b border-border bg-muted/40 px-3 py-1.5">
              <span className="text-xs font-medium">{t('allocation.tier', { n: ti + 1 })}</span>
              <span className="text-xs text-muted-foreground">
                {tierHint(ti)}
                {shared && ` · ${t('allocation.tierShared')}`}
              </span>
              <span className="flex-1" />
              <Button
                variant="ghost"
                size="icon"
                className="size-7 rounded-md"
                aria-label={t('allocation.tierUp', { n: ti + 1 })}
                disabled={ti === 0}
                onClick={() => setTiers(moveTier(tiers, ti, 'up'))}
              >
                <ChevronUp className="size-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="size-7 rounded-md"
                aria-label={t('allocation.tierDown', { n: ti + 1 })}
                disabled={ti === tiers.length - 1}
                onClick={() => setTiers(moveTier(tiers, ti, 'down'))}
              >
                <ChevronDown className="size-4" />
              </Button>
            </header>
            {tier.map((entry, ei) => {
              const owner = entry.channel_id ? providerOfChannel(providers, entry.channel_id) : null;
              return (
                <EntryRow
                  key={entryKey(entry)}
                  entry={entry}
                  share={shares[ei]}
                  shared={shared}
                  providers={providers}
                  actions={{
                    canUp: canMoveEntry(tiers, ti, 'up'),
                    canDown: canMoveEntry(tiers, ti, 'down'),
                    canMerge: Boolean(owner) && !tiers.some((x) => x.some((e) => e.provider === owner.key)),
                    up: () => setTiers(moveEntry(tiers, ti, ei, 'up')),
                    down: () => setTiers(moveEntry(tiers, ti, ei, 'down')),
                    splitOut: () => setTiers(splitOut(tiers, ti, ei)),
                    splitProvider: () =>
                      setTiers(
                        splitProvider(
                          tiers,
                          ti,
                          ei,
                          providers.find((p) => p.key === entry.provider)
                        )
                      ),
                    merge: () => setTiers(mergeChannels(tiers, ti, owner)),
                    remove: () => setTiers(removeEntry(tiers, ti, ei)),
                    weight: (w) => setTiers(setWeight(tiers, ti, ei, w))
                  }}
                />
              );
            })}
          </section>
        );
      })}

      {unlisted.length > 0 && (
        <section className="rounded-[10px] border border-dashed border-border">
          <header className="border-b border-dashed border-border px-3 py-1.5">
            <span className="text-xs font-medium">{t('allocation.unlisted')}</span>
            <span className="ml-2 text-xs text-muted-foreground">{t('allocation.unlistedHint')}</span>
          </header>
          {unlisted.map((provider) => (
            <div key={provider.key} className="flex min-h-12 items-center gap-3 px-3 py-2 [&+&]:border-t [&+&]:border-border">
              <ProviderTile type={provider.type} />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm">{title(provider)}</p>
                <p className="truncate text-xs text-muted-foreground">
                  {t('allocation.channels', {
                    total: provider.channels.length,
                    enabled: provider.channels.filter((c) => c.status === 1).length
                  })}
                </p>
              </div>
              <Button variant="outline" size="sm" className="rounded-lg" onClick={() => setTiers(addProvider(tiers, provider.key))}>
                <Plus className="size-4" />
                {t('allocation.place')}
              </Button>
            </div>
          ))}
        </section>
      )}

      <p className="text-xs text-muted-foreground">{t('allocation.footnote')}</p>
    </div>
  );
}
