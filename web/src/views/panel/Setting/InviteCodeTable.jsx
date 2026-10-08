import { Infinity as InfinityIcon, Copy, Pencil, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Checkbox } from '@/components/ui/checkbox';
import { RowActions, RowActionsSurfaceContext } from '@/components/ui/row-actions';
import { timestamp2string } from 'utils/common';

const tk = (k) => `setting_index.inviteCodeSettings.${k}`;

export default function InviteCodeTable({ items, selected, onSelectAll, onSelectOne, onCopy, onEdit, onDelete, onToggleStatus }) {
  const { t } = useTranslation();
  const allChecked = items.length > 0 && selected.length === items.length;
  const someChecked = selected.length > 0 && selected.length < items.length;

  const fmt = (ts, fallback) => (ts ? timestamp2string(ts) : fallback);

  const renderActions = (item) => (
    <RowActions
      actions={[
        { label: t('common.edit'), icon: Pencil, onClick: () => onEdit(item) },
        { label: t(tk('copyTip')), icon: Copy, onClick: () => onCopy(item) },
        { label: t('common.delete'), icon: Trash2, destructive: true, onClick: () => onDelete(item) }
      ]}
    />
  );

  return (
    <>
      <div className="hidden md:block">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="w-10">
                <Checkbox checked={allChecked} indeterminate={someChecked} onCheckedChange={onSelectAll} aria-label="select all" />
              </TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.id'))}</TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.code'))}</TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.name'))}</TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.usage'))}</TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.startsAt'))}</TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.expiresAt'))}</TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.createdTime'))}</TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.status'))}</TableHead>
              <TableHead className="sticky right-0 z-10 w-[1%] whitespace-nowrap bg-card text-right">{t(tk('headLabels.action'))}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((item) => (
              <TableRow key={item.id}>
                <TableCell>
                  <Checkbox
                    checked={selected.includes(item.id)}
                    onCheckedChange={() => onSelectOne(item.id)}
                    aria-label={`select ${item.id}`}
                  />
                </TableCell>
                <TableCell className="whitespace-nowrap tabular-nums">{item.id}</TableCell>
                <TableCell className="whitespace-nowrap">
                  <button
                    type="button"
                    onClick={() => onCopy(item)}
                    className="rounded bg-muted px-2 py-0.5 font-mono text-xs hover:bg-muted/70 hover:text-foreground"
                  >
                    {item.code}
                  </button>
                </TableCell>
                <TableCell className="whitespace-nowrap">{item.name || '-'}</TableCell>
                <TableCell className="whitespace-nowrap tabular-nums">
                  <span className="inline-flex items-center gap-1">
                    {item.used_count} / {item.max_uses === 0 ? <InfinityIcon className="size-4 text-muted-foreground" /> : item.max_uses}
                  </span>
                </TableCell>
                <TableCell className="whitespace-nowrap">{fmt(item.starts_at, t(tk('immediate')))}</TableCell>
                <TableCell className="whitespace-nowrap">{fmt(item.expires_at, t(tk('never')))}</TableCell>
                <TableCell className="whitespace-nowrap">{fmt(item.created_time, '-')}</TableCell>
                <TableCell className="whitespace-nowrap">
                  <div className="flex items-center gap-2">
                    <Switch checked={item.status === 1} onCheckedChange={() => onToggleStatus(item)} />
                    <Badge variant={item.status === 1 ? 'default' : 'secondary'}>
                      {item.status === 1 ? t(tk('statusEnabled')) : t(tk('statusDisabled'))}
                    </Badge>
                  </div>
                </TableCell>
                <TableCell className="sticky right-0 z-10 bg-card">{renderActions(item)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <div className="md:hidden flex flex-col gap-3">
        {items.map((item) => (
          <div key={item.id} className="rounded-lg border border-border bg-card p-3 shadow-sm">
            <div className="mb-1.5 text-sm font-medium">
              <button
                type="button"
                onClick={() => onCopy(item)}
                className="rounded bg-muted px-2 py-0.5 font-mono text-xs hover:bg-muted/70 hover:text-foreground"
              >
                {item.code}
              </button>
            </div>
            <dl className="grid grid-cols-2 gap-x-4 gap-y-1.5">
              <div className="flex items-baseline gap-1.5">
                <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t(tk('headLabels.id'))}</dt>
                <dd className="min-w-0 flex-1 truncate text-sm tabular-nums">{item.id}</dd>
              </div>
              <div className="flex items-baseline gap-1.5">
                <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t(tk('headLabels.name'))}</dt>
                <dd className="min-w-0 flex-1 truncate text-sm" title={item.name || undefined}>
                  {item.name || '-'}
                </dd>
              </div>
              <div className="flex items-baseline gap-1.5">
                <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t(tk('headLabels.usage'))}</dt>
                <dd className="flex min-w-0 flex-1 items-center gap-1 text-sm tabular-nums">
                  {item.used_count} / {item.max_uses === 0 ? <InfinityIcon className="size-4 text-muted-foreground" /> : item.max_uses}
                </dd>
              </div>
              <div className="flex items-baseline gap-1.5">
                <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t(tk('headLabels.startsAt'))}</dt>
                <dd className="min-w-0 flex-1 truncate text-sm">{fmt(item.starts_at, t(tk('immediate')))}</dd>
              </div>
              <div className="flex items-baseline gap-1.5">
                <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t(tk('headLabels.expiresAt'))}</dt>
                <dd className="min-w-0 flex-1 truncate text-sm">{fmt(item.expires_at, t(tk('never')))}</dd>
              </div>
              <div className="flex items-baseline gap-1.5">
                <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t(tk('headLabels.createdTime'))}</dt>
                <dd className="min-w-0 flex-1 truncate text-sm">{fmt(item.created_time, '-')}</dd>
              </div>
              <div className="flex items-center gap-1.5">
                <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t(tk('headLabels.status'))}</dt>
                <dd className="flex min-w-0 flex-1 items-center gap-2 text-sm">
                  <Switch checked={item.status === 1} onCheckedChange={() => onToggleStatus(item)} />
                  <Badge variant={item.status === 1 ? 'default' : 'secondary'}>
                    {item.status === 1 ? t(tk('statusEnabled')) : t(tk('statusDisabled'))}
                  </Badge>
                </dd>
              </div>
            </dl>
            <div className="mt-2 flex flex-wrap items-center justify-end gap-1 border-t border-border pt-2">
              <Checkbox
                checked={selected.includes(item.id)}
                onCheckedChange={() => onSelectOne(item.id)}
                aria-label={`select ${item.id}`}
              />
              <RowActionsSurfaceContext.Provider value="card">{renderActions(item)}</RowActionsSurfaceContext.Provider>
            </div>
          </div>
        ))}
      </div>
    </>
  );
}
