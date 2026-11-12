import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Ban, Check, Copy, MoreHorizontal, Pencil, Trash2 } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Switch } from '@/components/ui/switch';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { ROW_OPENS, RowControl, rowOpenProps } from '@/components/objects';

import { timestamp2string } from 'utils/common';
import { relativeTime } from 'utils/relativeTime';

const tk = (k) => `setting_index.inviteCodeSettings.${k}`;
const ENABLED = 1;

// Typical table width before it has been measured.

// The segment a code is in (same order as the backend): disabled, expired, used up, not started,
// usable.
export function inviteState(item, now = Math.floor(Date.now() / 1000)) {
  if (item.status !== ENABLED) return 'disabled';
  if (item.expires_at > 0 && item.expires_at <= now) return 'expired';
  if (item.max_uses > 0 && item.used_count >= item.max_uses) return 'exhausted';
  if (item.starts_at > now) return 'pending';
  return 'usable';
}

// "3 / 50", or "3 / ∞" without a limit.
export const usageText = (item) => `${item.used_count} / ${item.max_uses === 0 ? '∞' : item.max_uses}`;

// Validity in words: when it starts (if later), when it expires, or that it has expired.
function Validity({ item, rtf }) {
  const { t } = useTranslation();
  const state = inviteState(item);
  if (state === 'expired') return <span className="text-warning">{t('inviteList.expired')}</span>;
  if (state === 'pending')
    return <span title={timestamp2string(item.starts_at)}>{t('inviteList.startsIn', { time: relativeTime(rtf, item.starts_at) })}</span>;
  if (!item.expires_at) return <span className="text-muted-foreground">{t('inviteList.never')}</span>;
  return <span title={timestamp2string(item.expires_at)}>{t('inviteList.expiresIn', { time: relativeTime(rtf, item.expires_at) })}</span>;
}

Validity.propTypes = { item: PropTypes.object.isRequired, rtf: PropTypes.object.isRequired };

function Usage({ item, className }) {
  const { t } = useTranslation();
  const full = inviteState(item) === 'exhausted';
  return (
    <span className={cn('font-mono tabular-nums', full && 'text-warning', className)} title={full ? t('inviteList.usedUp') : undefined}>
      {usageText(item)}
    </span>
  );
}

Usage.propTypes = { item: PropTypes.object.isRequired, className: PropTypes.string };

function StatusSwitch({ item, onToggleStatus }) {
  const { t } = useTranslation();
  return (
    <RowControl className="flex">
      <Switch
        size="sm"
        checked={item.status === ENABLED}
        aria-label={t(tk('headLabels.status'))}
        onCheckedChange={() => onToggleStatus(item)}
      />
    </RowControl>
  );
}

StatusSwitch.propTypes = { item: PropTypes.object.isRequired, onToggleStatus: PropTypes.func.isRequired };

// Row menu: edit, copy the code, enable / disable, then delete (last).
function CodeActions({ item, onEdit, onCopy, onToggleStatus, onDelete }) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="size-[30px] rounded-lg" aria-label={t(tk('headLabels.action'))}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-40">
        <DropdownMenuItem onClick={() => onEdit(item)}>
          <Pencil className="size-4" />
          {t('common.edit')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => onCopy(item)}>
          <Copy className="size-4" />
          {t('inviteList.copyCode')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => onToggleStatus(item)}>
          {item.status === ENABLED ? <Ban className="size-4" /> : <Check className="size-4" />}
          {item.status === ENABLED ? t('inviteList.disable') : t('inviteList.enable')}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => onDelete(item)}>
          <Trash2 className="size-4" />
          {t('common.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

CodeActions.propTypes = {
  item: PropTypes.object.isRequired,
  onEdit: PropTypes.func.isRequired,
  onCopy: PropTypes.func.isRequired,
  onToggleStatus: PropTypes.func.isRequired,
  onDelete: PropTypes.func.isRequired
};

// Invite codes table: one line per code (the code in mono, then its name), usage, validity,
// created, then the switch and the row menu last. A row opens the code for editing.
export default function InviteCodeTable({ items, rtf, selectedIds, onToggleSelect, onToggleSelectAll, actions }) {
  const { t } = useTranslation();
  const allSelected = items.length > 0 && items.every((i) => selectedIds.has(i.id));
  const someSelected = items.some((i) => selectedIds.has(i.id));
  return (
    <Table>
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          <TableHead className="w-[1%] pr-0">
            <Checkbox
              checked={allSelected}
              indeterminate={!allSelected && someSelected}
              onCheckedChange={() => onToggleSelectAll(!allSelected)}
              aria-label={t('listPage.selectAllOnPage')}
            />
          </TableHead>
          <TableHead className="whitespace-nowrap">{t(tk('headLabels.code'))}</TableHead>
          <TableHead className="whitespace-nowrap text-right">{t(tk('headLabels.usage'))}</TableHead>
          <TableHead className="whitespace-nowrap">{t(tk('headLabels.expiresAt'))}</TableHead>
          <TableHead className="whitespace-nowrap">{t(tk('headLabels.createdTime'))}</TableHead>
          <TableHead className="whitespace-nowrap">{t(tk('headLabels.status'))}</TableHead>
          <TableHead className="sticky right-0 z-10 w-[1%] bg-card">
            <span className="sr-only">{t(tk('headLabels.action'))}</span>
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {items.map((item) => {
          const live = inviteState(item) === 'usable';
          return (
            <TableRow
              key={item.id}
              className={ROW_OPENS}
              data-state={selectedIds.has(item.id) ? 'selected' : undefined}
              {...rowOpenProps(() => actions.onEdit(item))}
            >
              <TableCell className="pr-0">
                <RowControl className="flex">
                  <Checkbox
                    checked={selectedIds.has(item.id)}
                    onCheckedChange={() => onToggleSelect(item.id)}
                    aria-label={t('listPage.selectRow', { name: item.code })}
                  />
                </RowControl>
              </TableCell>
              <TableCell className="max-w-80 whitespace-nowrap">
                <span className="flex min-w-0 items-center gap-2">
                  <code className={cn('shrink-0 font-mono text-[13px] font-medium', !live && 'text-muted-foreground')}>{item.code}</code>
                  {item.name && <span className="truncate text-xs text-muted-foreground">{item.name}</span>}
                </span>
              </TableCell>
              <TableCell className="whitespace-nowrap text-right">
                <Usage item={item} />
              </TableCell>
              <TableCell className="whitespace-nowrap">
                <Validity item={item} rtf={rtf} />
              </TableCell>
              <TableCell className="whitespace-nowrap">
                <span title={timestamp2string(item.created_time)}>{relativeTime(rtf, item.created_time)}</span>
              </TableCell>
              <TableCell>
                <StatusSwitch item={item} onToggleStatus={actions.onToggleStatus} />
              </TableCell>
              <TableCell className="sticky right-0 z-10 bg-card text-right">
                <RowControl className="justify-end">
                  <CodeActions item={item} {...actions} />
                </RowControl>
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}

InviteCodeTable.propTypes = {
  items: PropTypes.array.isRequired,
  rtf: PropTypes.object.isRequired,
  selectedIds: PropTypes.instanceOf(Set).isRequired,
  onToggleSelect: PropTypes.func.isRequired,
  onToggleSelectAll: PropTypes.func.isRequired,
  actions: PropTypes.object.isRequired
};

// Phone cards, one per code. Top: the code and its name, the switch on the right. Middle: usage,
// large. Bottom: validity · created, with the row menu. In select mode a checkbox leads.
export function InviteCodeCards({ items, rtf, selectMode, selectedIds, onToggleSelect, actions }) {
  const { t } = useTranslation();
  return (
    <ul className="space-y-2">
      {items.map((item) => {
        const selected = selectedIds.has(item.id);
        const live = inviteState(item) === 'usable';
        return (
          <li
            key={item.id}
            className={cn(
              'rounded-xl border border-border bg-card px-4 pb-2 pt-3 transition-colors',
              ROW_OPENS,
              selected && 'border-primary/40 bg-muted/40'
            )}
            {...rowOpenProps(() => (selectMode ? onToggleSelect(item.id) : actions.onEdit(item)))}
          >
            <div className="flex items-center gap-3">
              {selectMode && (
                <RowControl className="flex">
                  <Checkbox
                    checked={selected}
                    onCheckedChange={() => onToggleSelect(item.id)}
                    aria-label={t('listPage.selectRow', { name: item.code })}
                  />
                </RowControl>
              )}
              <div className="flex min-w-0 flex-1 items-baseline gap-2">
                <code className={cn('shrink-0 font-mono text-sm font-medium', !live && 'text-muted-foreground')}>{item.code}</code>
                {item.name && <span className="truncate text-xs text-muted-foreground">{item.name}</span>}
              </div>
              <StatusSwitch item={item} onToggleStatus={actions.onToggleStatus} />
            </div>
            <div className="mt-1 flex items-baseline gap-2">
              <Usage item={item} className={cn('text-xl font-medium', !live && 'text-muted-foreground')} />
              <span className="text-xs text-muted-foreground">{t('inviteList.usage')}</span>
            </div>
            <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
              <span className="min-w-0 flex-1 truncate">
                <Validity item={item} rtf={rtf} /> · {t('inviteList.createdAgo', { time: relativeTime(rtf, item.created_time) })}
              </span>
              <RowControl className="-mr-2 shrink-0">
                <CodeActions item={item} {...actions} />
              </RowControl>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

InviteCodeCards.propTypes = {
  items: PropTypes.array.isRequired,
  rtf: PropTypes.object.isRequired,
  selectMode: PropTypes.bool,
  selectedIds: PropTypes.instanceOf(Set).isRequired,
  onToggleSelect: PropTypes.func.isRequired,
  actions: PropTypes.object.isRequired
};
