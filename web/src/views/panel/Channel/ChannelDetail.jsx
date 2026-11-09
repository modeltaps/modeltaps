import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import {
  AlertTriangle,
  Copy,
  FlaskConical,
  Gauge,
  Globe,
  ListChecks,
  Loader2,
  MoreHorizontal,
  Power,
  RefreshCw,
  Settings2,
  Trash2
} from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import PageActions from '@/components/chrome/PageActions';
import PageTitleExtra from '@/components/chrome/PageTitleExtra';
import usePageHeader from '@/components/chrome/PageHeaderOverride';
import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import ChannelForm from './ChannelForm';
import ChannelOverview from './ChannelOverview';
import useChannelActions from './useChannelActions';
import { fetchChannel, fetchGroups, statusLabel } from './channelApi';
import { channelUrl } from './channelUrl';

const SECTIONS = ['basic', 'model', 'request', 'advanced', 'plugin'];
const STATUS_DOT = { 1: 'bg-success', 2: 'bg-muted-foreground', 3: 'bg-destructive' };

// Host part of a base URL for the header line; empty when unset.
function hostOf(url) {
  if (!url) return '';
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

// /panel/channel/:id is one channel as a document: identity in the header, a row of key
// figures, then sections that each read first and edit in place. `?view=settings` is the
// full form for everything the sections do not cover. /panel/channel/new is the form in
// create mode; saving a single channel opens its page.
export default function ChannelDetail() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id: idParam } = useParams();
  const isNew = idParam === 'new';
  const id = isNew ? 0 : Number(idParam);
  const [searchParams, setSearchParams] = useSearchParams();
  // `tab=config` is the earlier name of the settings view.
  const settings = isNew || searchParams.get('view') === 'settings' || searchParams.get('tab') === 'config';
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
  const meta = channel ? [`#${channel.id}`, typeName, hostOf(channel.base_url)].filter(Boolean).join(' · ') : '';
  usePageHeader({
    title: isNew ? t('channelDetail.newTitle') : channel?.name || (loadError ? t('channelDetail.notFound') : '…'),
    subtitle: settings && !isNew ? t('channelDetail.allSettings') : meta,
    back:
      settings && !isNew
        ? { label: t('channelDetail.backToChannel'), to: channelUrl(id) }
        : { label: t('channelDetail.back'), to: '/panel/channel' }
  });

  const onSaved = ({ ids }) => {
    setDirty(false);
    if (isNew) navigate(ids?.length === 1 ? channelUrl(ids[0]) : '/panel/channel', { replace: true });
    else load();
  };

  const openSettings = (s) => navigate(channelUrl(id, 'settings', s));

  if (!isNew && loadError) {
    return (
      <div className="py-16 text-center">
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

  if (settings) {
    return (
      <>
        <ChannelForm
          channelId={id || 0}
          initialTab={section}
          groupOptions={groups}
          onDirtyChange={setDirty}
          onTabChange={(s) => {
            const params = new URLSearchParams(searchParams);
            params.delete('tab');
            if (!isNew) params.set('view', 'settings');
            s && s !== 'basic' ? params.set('section', s) : params.delete('section');
            setSearchParams(params, { replace: true });
          }}
          onCancel={() => navigate('/panel/channel')}
          onSaved={onSaved}
        />
        {dialogs}
      </>
    );
  }

  const webUrl = CHANNEL_OPTIONS[channel.type]?.url;
  const testing = testingIds.has(channel.id);
  const enabled = channel.status === 1;

  return (
    <>
      <PageTitleExtra>
        <span
          className={cn(
            'inline-flex h-6 items-center gap-1.5 rounded-md border border-border px-2 text-xs',
            channel.status === 3 ? 'text-destructive' : 'text-muted-foreground'
          )}
        >
          <span className={cn('size-1.5 rounded-full', STATUS_DOT[channel.status] || 'bg-muted-foreground')} />
          {statusLabel(t, channel.status)}
        </span>
      </PageTitleExtra>

      <PageActions>
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
          <DropdownMenuContent align="end" className="w-52">
            <DropdownMenuItem onClick={() => handlers.onToggleStatus(channel)}>
              <Power className="size-4" />
              {enabled ? t('channelDetail.disable') : t('channelDetail.enable')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => openSettings('basic')}>
              <Settings2 className="size-4" />
              {t('channelDetail.allSettings')}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => handlers.onMultiTest(channel)}>
              <Gauge className="size-4" />
              {t('channelTest.action')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => handlers.onCheck(channel)}>
              <ListChecks className="size-4" />
              {t('channel_row.check')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => handlers.onRefreshBalance(channel)}>
              <RefreshCw className="size-4" />
              {t('channelDetail.refreshBalance')}
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

      {channel.status === 3 && (
        <p className="-mt-2 mb-6 flex items-center gap-2 text-sm text-destructive">
          <AlertTriangle className="size-4" />
          {t('channelDetail.autoDisabled')}
        </p>
      )}

      <ChannelOverview
        channel={channel}
        groupOptions={groups}
        actions={handlers}
        busy={{
          testing,
          drift: driftCheckingIds.has(channel.id),
          dismissing: dismissingIds.has(channel.id),
          refreshing: refreshingIds.has(channel.id)
        }}
        onSaved={load}
        onOpenSettings={openSettings}
      />

      {dialogs}
    </>
  );
}
