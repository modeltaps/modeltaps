import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ChevronDown, FlaskConical, Layers, Loader2, Power, PowerOff, SlidersHorizontal, Tag, Trash2, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { NO_TAG } from './channelGroups';
import { Segmented } from '@/components/objects';

const STATUS_KEYS = ['all', 'enabled', 'manual', 'auto'];

function ToolButton({ icon: Icon, children, className, ...props }) {
  return (
    <Button variant="ghost" size="sm" className={cn('h-[30px] rounded-lg px-2.5', className)} {...props}>
      <Icon />
      {children}
    </Button>
  );
}

ToolButton.propTypes = { icon: PropTypes.elementType.isRequired, children: PropTypes.node, className: PropTypes.string };

// The one toolbar row on top of the channel table: status and tag filters (with tag-wide
// actions while a tag is picked), or the batch bar while rows are selected.
export default function ChannelToolbar({
  counts,
  tags,
  hasUntagged,
  filters,
  onFiltersChange,
  selected,
  onClear,
  batch,
  tagActions,
  busy
}) {
  const { t } = useTranslation();
  const n = selected.length;

  if (n > 0) {
    return (
      <div
        className="flex min-h-12 flex-wrap items-center gap-1 border-b border-border bg-muted/40 px-3 py-1.5"
        role="toolbar"
        aria-label={t('channels.batch.label')}
      >
        <span className="mr-2 text-[13px] font-medium tabular-nums">{t('channels.batch.selected', { count: n })}</span>
        <ToolButton icon={Power} disabled={busy} onClick={() => batch.enable(selected)}>
          {t('channels.batch.enable')}
        </ToolButton>
        <ToolButton icon={PowerOff} disabled={busy} onClick={() => batch.disable(selected)}>
          {t('channels.batch.disable')}
        </ToolButton>
        <ToolButton icon={FlaskConical} disabled={busy} onClick={() => batch.test(selected)}>
          {t('channels.batch.test')}
        </ToolButton>
        <ToolButton icon={Layers} disabled={busy} onClick={() => batch.groups(selected)}>
          {t('channels.batch.groups')}
        </ToolButton>
        <ToolButton icon={SlidersHorizontal} disabled={busy} onClick={() => batch.numbers(selected)}>
          {t('channels.batch.numbers')}
        </ToolButton>
        <ToolButton
          icon={Trash2}
          disabled={busy}
          className="text-destructive hover:text-destructive"
          onClick={() => batch.remove(selected)}
        >
          {t('channels.batch.delete')}
        </ToolButton>
        <span className="flex-1" />
        {busy && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
        <Button
          variant="ghost"
          size="icon"
          className="size-[30px] rounded-lg"
          aria-label={t('channels.batch.clear')}
          title={t('channels.batch.clear')}
          onClick={onClear}
        >
          <X />
        </Button>
      </div>
    );
  }

  const tagLabel = filters.tag === NO_TAG ? t('channels.filter.noTag') : filters.tag || t('channels.filter.allTags');
  const realTag = filters.tag && filters.tag !== NO_TAG ? filters.tag : '';
  const setTag = (tag) => onFiltersChange({ ...filters, tag });

  return (
    <div
      className="flex min-h-12 flex-wrap items-center gap-2 border-b border-border px-3 py-1.5"
      role="toolbar"
      aria-label={t('channels.filter.label')}
    >
      <Segmented
        label={t('channels.filter.status')}
        value={filters.status || 'all'}
        onChange={(v) => onFiltersChange({ ...filters, status: v === 'all' ? '' : v })}
        options={STATUS_KEYS.map((k) => ({
          value: k,
          label: (
            <>
              {t(`channels.filter.${k}`)}
              <span className="ml-1.5 font-mono text-[11px] text-muted-foreground">{counts[k] ?? 0}</span>
            </>
          )
        }))}
      />
      {(tags.length > 0 || filters.tag) && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" className={cn('h-[30px] rounded-lg px-2.5', filters.tag && 'border-foreground/30')}>
              <Tag />
              <span className="max-w-40 truncate">{tagLabel}</span>
              <ChevronDown className="text-muted-foreground" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-52">
            <DropdownMenuItem onClick={() => setTag('')}>{t('channels.filter.allTags')}</DropdownMenuItem>
            {hasUntagged && <DropdownMenuItem onClick={() => setTag(NO_TAG)}>{t('channels.filter.noTag')}</DropdownMenuItem>}
            {tags.length > 0 && <DropdownMenuSeparator />}
            {tags.map((tag) => (
              <DropdownMenuItem key={tag} onClick={() => setTag(tag)}>
                <span className="truncate font-mono text-xs">{tag}</span>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      <span className="flex-1" />
      {realTag && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="sm" className="h-[30px] rounded-lg px-2.5" disabled={busy}>
              {busy && <Loader2 className="animate-spin" />}
              {t('channels.tagActions.label', { tag: realTag })}
              <ChevronDown className="text-muted-foreground" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            <DropdownMenuItem onClick={() => tagActions.enable(realTag)}>
              <Power className="size-4" />
              {t('channels.tagActions.enable')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => tagActions.disable(realTag)}>
              <PowerOff className="size-4" />
              {t('channels.tagActions.disable')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => tagActions.priority(realTag)}>
              <SlidersHorizontal className="size-4" />
              {t('channels.tagActions.priority')}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => tagActions.remove(realTag, () => setTag(''))}>
              <Trash2 className="size-4" />
              {t('channels.tagActions.delete')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  );
}

ChannelToolbar.propTypes = {
  counts: PropTypes.object.isRequired,
  tags: PropTypes.array.isRequired,
  hasUntagged: PropTypes.bool,
  filters: PropTypes.object.isRequired,
  onFiltersChange: PropTypes.func.isRequired,
  selected: PropTypes.array.isRequired,
  onClear: PropTypes.func.isRequired,
  batch: PropTypes.object.isRequired,
  tagActions: PropTypes.object.isRequired,
  busy: PropTypes.bool
};
