import { Pencil, PlugZap, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { RowActions, RowActionsSurfaceContext } from '@/components/ui/row-actions';

const tk = (k) => `setting_index.oidcProviders.${k}`;

export default function OidcProviderTable({ items, onEdit, onTest, onDelete, onToggleStatus }) {
  const { t } = useTranslation();

  const renderActions = (item) => (
    <RowActions
      actions={[
        { label: t('common.edit'), icon: Pencil, onClick: () => onEdit(item) },
        { label: t(tk('test')), icon: PlugZap, onClick: () => onTest(item) },
        { label: t('common.delete'), icon: Trash2, destructive: true, onClick: () => onDelete(item) }
      ]}
    />
  );

  const statusCell = (item) => (
    <div className="flex items-center gap-2">
      <Switch checked={!!item.enabled} onCheckedChange={() => onToggleStatus(item)} />
      <Badge variant={item.enabled ? 'default' : 'secondary'}>{item.enabled ? t(tk('enabled')) : t(tk('disabled'))}</Badge>
    </div>
  );

  return (
    <>
      <div className="hidden md:block">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.slug'))}</TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.displayName'))}</TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.issuer'))}</TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.sort'))}</TableHead>
              <TableHead className="whitespace-nowrap">{t(tk('headLabels.status'))}</TableHead>
              <TableHead className="sticky right-0 z-10 w-[1%] whitespace-nowrap bg-card text-right">
                {t(tk('headLabels.action'))}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((item) => (
              <TableRow key={item.id}>
                <TableCell className="whitespace-nowrap">
                  <span className="rounded bg-muted px-2 py-0.5 font-mono text-xs">{item.slug}</span>
                </TableCell>
                <TableCell className="whitespace-nowrap">{item.display_name || '-'}</TableCell>
                <TableCell className="max-w-[22rem] truncate" title={item.issuer}>
                  {item.issuer || '-'}
                </TableCell>
                <TableCell className="whitespace-nowrap tabular-nums">{item.sort}</TableCell>
                <TableCell className="whitespace-nowrap">{statusCell(item)}</TableCell>
                <TableCell className="sticky right-0 z-10 bg-card">{renderActions(item)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <div className="flex flex-col gap-3 md:hidden">
        {items.map((item) => (
          <div key={item.id} className="rounded-lg border border-border bg-card p-3 shadow-sm">
            <div className="mb-1.5 text-sm font-medium">
              <span className="rounded bg-muted px-2 py-0.5 font-mono text-xs">{item.slug}</span>
            </div>
            <dl className="grid grid-cols-2 gap-x-4 gap-y-1.5">
              <div className="flex items-baseline gap-1.5">
                <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t(tk('headLabels.displayName'))}</dt>
                <dd className="min-w-0 flex-1 truncate text-sm">{item.display_name || '-'}</dd>
              </div>
              <div className="flex items-baseline gap-1.5">
                <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t(tk('headLabels.sort'))}</dt>
                <dd className="min-w-0 flex-1 truncate text-sm tabular-nums">{item.sort}</dd>
              </div>
              <div className="col-span-2 flex items-baseline gap-1.5">
                <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t(tk('headLabels.issuer'))}</dt>
                <dd className="min-w-0 flex-1 truncate text-sm" title={item.issuer}>
                  {item.issuer || '-'}
                </dd>
              </div>
              <div className="col-span-2 flex items-center gap-1.5">
                <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t(tk('headLabels.status'))}</dt>
                <dd className="min-w-0 flex-1">{statusCell(item)}</dd>
              </div>
            </dl>
            <div className="mt-2 flex flex-wrap items-center justify-end gap-1 border-t border-border pt-2">
              <RowActionsSurfaceContext.Provider value="card">{renderActions(item)}</RowActionsSurfaceContext.Provider>
            </div>
          </div>
        ))}
      </div>
    </>
  );
}
