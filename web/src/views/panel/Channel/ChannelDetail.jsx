import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Copy, FlaskConical, Gauge, Globe, ListChecks, Loader2, MoreHorizontal, Trash2 } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import PageActions from '@/components/chrome/PageActions';
import usePageHeader from '@/components/chrome/PageHeaderOverride';
import ProviderIcon from '@/components/brand/ProviderIcon';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import ChannelForm from './ChannelForm';
import ChannelOverview from './ChannelOverview';
import useChannelActions from './useChannelActions';
import { fetchChannel, fetchGroups, statusLabel } from './channelApi';
import { ChannelRemark, KeyMissingBadge } from './channelColumns';
import { channelUrl } from './channelUrl';

const TABS = ['overview', 'config'];
const SECTIONS = ['basic', 'model', 'request', 'advanced', 'plugin'];
const STATUS_DOT = { 1: 'bg-success', 2: 'bg-muted-foreground', 3: 'bg-destructive' };

// /panel/channel/:id — one channel on its own page: an overview (spend, balance, traffic,
// health, drift, routing) and the full configuration form. /panel/channel/new is the same
// page in create mode: provider grid, then the form; saving a single channel opens its page.
export default function ChannelDetail() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id: idParam } = useParams();
  const isNew = idParam === 'new';
  const id = isNew ? 0 : Number(idParam);
  const [searchParams, setSearchParams] = useSearchParams();
  const tab = isNew ? 'config' : TABS.includes(searchParams.get('tab')) ? searchParams.get('tab') : 'overview';
  const section = SECTIONS.includes(searchParams.get('section')) ? searchParams.get('section') : 'basic';

  const [channel, setChannel] = useState(null);
  const [loadError, setLoadError] = useState('');
  const [groups, setGroups] = useState([]);
  const [dirty, setDirty] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const { success, message, data } = await fetchChannel(id);
      if (success) {
        setChannel(data);
        setLoadError('');
      } else setLoadError(message || t('channelDetail.notFound'));
    } catch (e) {
      setLoadError(e.message);
    }
  }, [id, t]);

  useEffect(() => {
    setChannel(null);
    setLoadError('');
    load();
  }, [load]);

  useEffect(() => {
    fetchGroups().then(setGroups);
  }, []);

  // Warn before a reload or tab close drops unsaved edits.
  useEffect(() => {
    if (!dirty) return undefined;
    const onBeforeUnload = (e) => {
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [dirty]);

  const { handlers, testingIds, driftCheckingIds, dismissingIds, refreshingIds, dialogs } = useChannelActions({
    onChanged: load,
    onDriftDismissed: (_row, drift) => setChannel((c) => (c ? { ...c, model_drift: drift } : c)),
    onDeleted: () => navigate('/panel/channel', { replace: true })
  });

  const typeName = channel ? CHANNEL_OPTIONS[channel.type]?.text || t('common.unknown') : '';
  usePageHeader({
    title: isNew ? t('channelDetail.newTitle') : channel?.name || (loadError ? t('channelDetail.notFound') : '…'),
    subtitle: channel ? `#${channel.id} · ${typeName}` : '',
    back: { label: t('channelDetail.back'), to: '/panel/channel' }
  });

  // Switching tabs keeps the config section; passing a section moves to it.
  const setTab = (next, nextSection) => {
    const params = new URLSearchParams(searchParams);
    next === 'overview' ? params.delete('tab') : params.set('tab', next);
    if (nextSection !== undefined) {
      nextSection && nextSection !== 'basic' ? params.set('section', nextSection) : params.delete('section');
    }
    setSearchParams(params, { replace: true });
  };

  const onSaved = ({ ids }) => {
    setDirty(false);
    if (isNew) {
      navigate(ids?.length === 1 ? channelUrl(ids[0]) : '/panel/channel', { replace: true });
    } else {
      load();
    }
  };

  if (!isNew && loadError) {
    return (
      <div className="rounded-xl border border-border bg-card px-6 py-12 text-center">
        <p className="text-sm text-muted-foreground">{loadError}</p>
        <Button variant="outline" className="mt-4" onClick={() => navigate('/panel/channel')}>
          {t('channelDetail.back')}
        </Button>
      </div>
    );
  }

  if (!isNew && !channel) {
    return (
      <div className="flex h-64 items-center justify-center text-muted-foreground">
        <Loader2 className="size-5 animate-spin" />
      </div>
    );
  }

  const webUrl = channel ? CHANNEL_OPTIONS[channel.type]?.url : null;
  const testing = channel && testingIds.has(channel.id);

  return (
    <>
      {channel && (
        <PageActions>
          <label className="mr-1 flex items-center gap-2 text-sm">
            <Switch checked={channel.status === 1} onCheckedChange={() => handlers.onToggleStatus(channel)} />
            <span className={cn('inline-flex items-center gap-1.5', channel.status === 3 ? 'text-destructive' : 'text-muted-foreground')}>
              <span className={cn('size-1.5 rounded-full', STATUS_DOT[channel.status] || 'bg-muted-foreground')} />
              {statusLabel(t, channel.status)}
            </span>
          </label>
          <Button variant="outline" size="sm" disabled={testing} onClick={() => handlers.onTest(channel)}>
            {testing ? <Loader2 className="size-4 animate-spin" /> : <FlaskConical className="size-4" />}
            {t('channel_index.test')}
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="icon" className="size-8" aria-label={t('channelDetail.more')}>
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-48">
              <DropdownMenuItem onClick={() => handlers.onMultiTest(channel)}>
                <Gauge className="size-4" />
                {t('channelTest.action')}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => handlers.onCheck(channel)}>
                <ListChecks className="size-4" />
                {t('channel_row.check')}
              </DropdownMenuItem>
              <DropdownMenuItem disabled={!webUrl} onClick={() => webUrl && window.open(webUrl, '_blank', 'noopener,noreferrer')}>
                <Globe className="size-4" />
                {t('channel_row.channelWeb')}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => handlers.onCopy(channel)}>
                <Copy className="size-4" />
                {t('common.copy')}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-destructive" onClick={() => handlers.onDelete(channel)}>
                <Trash2 className="size-4" />
                {t('common.delete')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </PageActions>
      )}

      {channel && (
        <div className="-mt-3 mb-5 flex flex-wrap items-center gap-x-3 gap-y-2">
          <ProviderIcon type={channel.type} name={channel.name} baseUrl={channel.base_url} className="size-5" />
          <ChannelRemark remark={channel.remark} />
          <KeyMissingBadge t={t} keyStatus={channel.key_status} />
        </div>
      )}

      {!isNew && (
        <div role="tablist" aria-label={channel?.name} className="mb-5 flex gap-6 border-b border-border">
          {TABS.map((key) => (
            <button
              key={key}
              type="button"
              role="tab"
              aria-selected={tab === key}
              onClick={() => setTab(key)}
              className={cn(
                '-mb-px border-b-2 pb-2.5 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                tab === key
                  ? 'border-foreground font-medium text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground'
              )}
            >
              {t(`channelDetail.tab_${key}`)}
              {key === 'config' && dirty && <span className="ml-1.5 inline-block size-1.5 rounded-full bg-foreground align-middle" />}
            </button>
          ))}
        </div>
      )}

      {channel && tab === 'overview' && (
        <ChannelOverview
          channel={channel}
          actions={handlers}
          busy={{
            testing,
            drift: driftCheckingIds.has(channel.id),
            dismissing: dismissingIds.has(channel.id),
            refreshing: refreshingIds.has(channel.id)
          }}
          onEditSection={(s) => setTab('config', s)}
        />
      )}

      {/* The form stays mounted while the overview shows, so switching tabs keeps unsaved edits. */}
      <div hidden={tab !== 'config'}>
        <ChannelForm
          channelId={id || 0}
          initialTab={section}
          groupOptions={groups}
          onDirtyChange={setDirty}
          onTabChange={(s) => setTab('config', s)}
          onCancel={() => navigate('/panel/channel')}
          onSaved={onSaved}
        />
      </div>

      {dialogs}
    </>
  );
}
