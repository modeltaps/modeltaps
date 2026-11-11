import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import {
  ChevronDown,
  FlaskConical,
  Layers,
  Loader2,
  MoreHorizontal,
  Power,
  PowerOff,
  SlidersHorizontal,
  Tag,
  Trash2,
  X
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

function ToolButton({ icon: Icon, children, className, ...props }) {
  return (
    <Button variant="ghost" size="sm" className={cn('h-[30px] rounded-lg px-2.5', className)} {...props}>
      <Icon />
      {children}
    </Button>
  );
}

ToolButton.propTypes = { icon: PropTypes.elementType.isRequired, children: PropTypes.node, className: PropTypes.string };

// The batch bar on top of the channel table while rows are selected. `floating` pins a compact
// version to the bottom of the screen (phones): enable, disable and a menu with the rest.
export function ChannelBatchBar({ selected, onClear, batch, busy, floating = false }) {
  const { t } = useTranslation();
  const n = selected.length;
  if (floating) {
    return (
      <div
        role="toolbar"
        aria-label={t('channels.batch.label')}
        className="fixed inset-x-0 bottom-0 z-40 flex items-center gap-1.5 border-t border-border bg-popover px-2 pb-[max(0.625rem,env(safe-area-inset-bottom))] pt-2.5 shadow-[0_-4px_16px_rgba(0,0,0,0.06)]"
      >
        <Button variant="ghost" size="icon" className="size-8 shrink-0" aria-label={t('channels.batch.clear')} onClick={onClear}>
          <X />
        </Button>
        <span className="mr-auto whitespace-nowrap text-sm font-medium tabular-nums">{t('channels.batch.selected', { count: n })}</span>
        {busy && <Loader2 className="size-4 animate-spin text-muted-foreground" />}
        <Button variant="outline" size="sm" className="h-8" disabled={busy || !n} onClick={() => batch.enable(selected)}>
          <Power className="size-3.5" />
          {t('channels.batch.enable')}
        </Button>
        <Button variant="outline" size="sm" className="h-8" disabled={busy || !n} onClick={() => batch.disable(selected)}>
          <PowerOff className="size-3.5" />
          {t('channels.batch.disable')}
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="icon" className="size-8" disabled={busy || !n} aria-label={t('channels.more')}>
              <MoreHorizontal className="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" side="top" className="w-44">
            <DropdownMenuItem onClick={() => batch.test(selected)}>
              <FlaskConical className="size-4" />
              {t('channels.batch.test')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => batch.groups(selected)}>
              <Layers className="size-4" />
              {t('channels.batch.groups')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => batch.numbers(selected)}>
              <SlidersHorizontal className="size-4" />
              {t('channels.batch.numbers')}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive" onClick={() => batch.remove(selected)}>
              <Trash2 className="size-4" />
              {t('channels.batch.delete')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    );
  }
  return (
    <div
      className="flex min-h-10 flex-wrap items-center gap-1 rounded-lg border border-primary/30 bg-primary/[0.04] px-3 py-1"
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
      <ToolButton icon={Trash2} disabled={busy} className="text-destructive hover:text-destructive" onClick={() => batch.remove(selected)}>
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

ChannelBatchBar.propTypes = {
  selected: PropTypes.array.isRequired,
  onClear: PropTypes.func.isRequired,
  batch: PropTypes.object.isRequired,
  busy: PropTypes.bool,
  floating: PropTypes.bool
};

// Actions on every channel with the picked tag, shown while a tag condition is active.
export function TagActionsMenu({ tag, tagActions, busy, onTagGone }) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="h-8 gap-1.5" disabled={busy}>
          {busy ? <Loader2 className="animate-spin" /> : <Tag />}
          <span className="max-w-40 truncate">{t('channels.tagActions.label', { tag })}</span>
          <ChevronDown className="text-muted-foreground" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        <DropdownMenuItem onClick={() => tagActions.enable(tag)}>
          <Power className="size-4" />
          {t('channels.tagActions.enable')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => tagActions.disable(tag)}>
          <PowerOff className="size-4" />
          {t('channels.tagActions.disable')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => tagActions.priority(tag)}>
          <SlidersHorizontal className="size-4" />
          {t('channels.tagActions.priority')}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem className="text-destructive" onClick={() => tagActions.remove(tag, onTagGone)}>
          <Trash2 className="size-4" />
          {t('channels.tagActions.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

TagActionsMenu.propTypes = {
  tag: PropTypes.string.isRequired,
  tagActions: PropTypes.object.isRequired,
  busy: PropTypes.bool,
  onTagGone: PropTypes.func
};
