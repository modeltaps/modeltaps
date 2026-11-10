import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2, MoreHorizontal, Plus } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useReportDirty } from './SettingsPage';

// ==============================|| SETTINGS — THE THREE CARD KINDS ||============================== //
// FormCard        a group of related options. Everything in it, switches and the header switch
//                 included, is saved by its footer (left: hint or "unsaved changes"; right: Undo
//                 and Save). One card is one save scope. A header switch that is off greys the
//                 fields out, but they stay editable so they can be filled in beforehand.
// CollectionCard  a list of objects: header "Add" button, a compact table, a click on a row opens
//                 the item's own editor (it saves on its own), a row menu for disable / delete.
//                 No footer.
// ActionRow       a one-off action: button plus a confirm dialog (red when destructive). It can
//                 stand alone in an ActionCard or close a FormCard.

function CardHeading({ title, description, aside }) {
  return (
    <div className="space-y-1">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <h3 className="text-[15px] font-medium leading-6">{title}</h3>
        {aside && <div className="flex flex-wrap items-center gap-2">{aside}</div>}
      </div>
      {description && <div className="max-w-3xl text-sm text-muted-foreground">{description}</div>}
    </div>
  );
}

CardHeading.propTypes = { title: PropTypes.node, description: PropTypes.node, aside: PropTypes.node };

const shellClass = 'overflow-hidden rounded-2xl border border-border bg-card text-card-foreground';

// Header on/off switch of a form card; saved with the footer like every other field.
function HeaderToggle({ id, label, checked, onCheckedChange, disabled }) {
  const { t } = useTranslation();
  return (
    <div className="flex items-center gap-2">
      <Label htmlFor={id} className="font-normal text-muted-foreground">
        {label ?? t('common.enableToggle')}
      </Label>
      <Switch id={id} checked={checked} onCheckedChange={onCheckedChange} disabled={disabled} />
    </div>
  );
}

HeaderToggle.propTypes = {
  id: PropTypes.string,
  label: PropTypes.node,
  checked: PropTypes.bool,
  onCheckedChange: PropTypes.func,
  disabled: PropTypes.bool
};

export function FormCard({
  title,
  description,
  hint,
  highlight,
  enable,
  dirty = false,
  saving = false,
  disabled = false,
  onSave,
  onReset,
  actions,
  children,
  className
}) {
  const { t } = useTranslation();
  useReportDirty(dirty);
  const off = enable && !enable.checked;

  return (
    <section data-highlight={highlight} className={cn(shellClass, className)}>
      <div className="space-y-5 px-6 pb-6 pt-5">
        <CardHeading
          title={title}
          description={description}
          aside={
            enable && (
              <HeaderToggle
                id={enable.id}
                label={enable.label}
                checked={!!enable.checked}
                onCheckedChange={enable.onCheckedChange}
                disabled={disabled}
              />
            )
          }
        />
        {children && <div className={cn('space-y-5 transition-opacity', off && 'opacity-55')}>{children}</div>}
        {actions && <div className="space-y-1 border-t border-border pt-4">{actions}</div>}
      </div>
      <div className="flex min-h-14 items-center justify-between gap-4 border-t border-border bg-muted/30 px-6 py-2.5">
        <p className={cn('min-w-0 text-xs', dirty ? 'font-medium text-orange-600 dark:text-orange-400' : 'text-muted-foreground')}>
          {dirty ? t('settingsUi.unsaved') : hint}
        </p>
        <div className="flex shrink-0 items-center gap-2">
          {dirty && (
            <Button variant="ghost" size="sm" onClick={onReset} disabled={saving}>
              {t('settingsUi.undo')}
            </Button>
          )}
          <Button size="sm" onClick={onSave} disabled={!dirty || saving || disabled}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t('common.save')}
          </Button>
        </div>
      </div>
    </section>
  );
}

FormCard.propTypes = {
  title: PropTypes.node.isRequired,
  description: PropTypes.node,
  hint: PropTypes.node,
  highlight: PropTypes.string,
  enable: PropTypes.shape({ id: PropTypes.string, label: PropTypes.node, checked: PropTypes.bool, onCheckedChange: PropTypes.func }),
  dirty: PropTypes.bool,
  saving: PropTypes.bool,
  disabled: PropTypes.bool,
  onSave: PropTypes.func,
  onReset: PropTypes.func,
  actions: PropTypes.node,
  children: PropTypes.node,
  className: PropTypes.string
};

// A card without a save scope: action rows only (sign-in methods, danger zone).
export function ActionCard({ title, description, highlight, aside, children, className }) {
  return (
    <section data-highlight={highlight} className={cn(shellClass, className)}>
      <div className="space-y-3 px-6 pb-4 pt-5">
        <CardHeading title={title} description={description} aside={aside} />
        <div>{children}</div>
      </div>
    </section>
  );
}

ActionCard.propTypes = {
  title: PropTypes.node.isRequired,
  description: PropTypes.node,
  highlight: PropTypes.string,
  aside: PropTypes.node,
  children: PropTypes.node,
  className: PropTypes.string
};

// One-off action. With `confirm` the button opens a dialog first; `confirm.body` can hold inputs the
// action needs (a cutoff date, a target), and `confirm.disabled` keeps the confirm button off until
// they are valid. Without `confirm`, the button runs `onAction` directly (opening a dialog of the
// action's own, for instance).
export function ActionRow({
  icon: Icon,
  title,
  description,
  actionLabel,
  onAction,
  confirm,
  destructive = false,
  disabled = false,
  children
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  const run = async () => {
    setBusy(true);
    try {
      const ok = await onAction?.();
      if (ok !== false) setOpen(false);
    } finally {
      setBusy(false);
    }
  };

  const control =
    children !== undefined ? (
      children
    ) : (
      <Button
        variant={destructive ? 'destructive' : 'outline'}
        size="sm"
        disabled={disabled}
        onClick={() => {
          if (!confirm) return onAction?.();
          confirm.onOpen?.();
          return setOpen(true);
        }}
      >
        {actionLabel}
      </Button>
    );

  return (
    <div className="flex items-center justify-between gap-4 py-2.5">
      <div className="flex min-w-0 items-center gap-3">
        {Icon && (
          <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-foreground">
            <Icon className="size-[18px]" aria-hidden="true" />
          </span>
        )}
        <div className="min-w-0">
          <p className="text-sm font-medium">{title}</p>
          {description && <p className="text-xs text-muted-foreground">{description}</p>}
        </div>
      </div>
      {control !== null && <div className="shrink-0">{control}</div>}
      {confirm && (
        <Dialog open={open} onOpenChange={(o) => !busy && setOpen(o)}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{confirm.title ?? title}</DialogTitle>
              {confirm.description && <DialogDescription>{confirm.description}</DialogDescription>}
            </DialogHeader>
            {confirm.body && <DialogBody className="space-y-4">{confirm.body}</DialogBody>}
            <DialogFooter>
              <Button variant="outline" disabled={busy} onClick={() => setOpen(false)}>
                {t('common.cancel')}
              </Button>
              <Button variant={destructive ? 'destructive' : 'default'} disabled={busy || confirm.disabled} onClick={run}>
                {busy && <Loader2 className="size-4 animate-spin" />}
                {confirm.label ?? actionLabel}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

ActionRow.propTypes = {
  icon: PropTypes.elementType,
  title: PropTypes.node.isRequired,
  description: PropTypes.node,
  actionLabel: PropTypes.node,
  onAction: PropTypes.func,
  confirm: PropTypes.shape({
    title: PropTypes.node,
    description: PropTypes.node,
    body: PropTypes.node,
    label: PropTypes.node,
    disabled: PropTypes.bool,
    onOpen: PropTypes.func
  }),
  destructive: PropTypes.bool,
  disabled: PropTypes.bool,
  children: PropTypes.node
};

// Status cell for collection rows: a dot and a word, no badge.
export function StatusDot({ on, onLabel, offLabel }) {
  const { t } = useTranslation();
  return (
    <span className="inline-flex items-center gap-2 text-[13px] text-muted-foreground">
      <span className={cn('size-1.5 rounded-full', on ? 'bg-emerald-500' : 'bg-muted-foreground/40')} />
      {on ? (onLabel ?? t('settingsUi.collection.enabled')) : (offLabel ?? t('settingsUi.collection.disabled'))}
    </span>
  );
}

StatusDot.propTypes = { on: PropTypes.bool, onLabel: PropTypes.node, offLabel: PropTypes.node };

// columns: [{ key, header, cell(row), className }]; menu(row) → [{ label, icon, onClick, destructive, separator }]
export function CollectionCard({
  title,
  description,
  highlight,
  addLabel,
  onAdd,
  aside,
  columns,
  rows,
  rowKey = (row) => row.id,
  onRowClick,
  menu,
  loading = false,
  empty,
  footer,
  className
}) {
  const { t } = useTranslation();
  return (
    <section data-highlight={highlight} className={cn(shellClass, className)}>
      <div className="space-y-4 px-6 pb-6 pt-5">
        <CardHeading
          title={title}
          description={description}
          aside={
            (aside || onAdd) && (
              <>
                {aside}
                {onAdd && (
                  <Button size="sm" variant="outline" onClick={onAdd}>
                    <Plus className="size-4" />
                    {addLabel ?? t('settingsUi.collection.add')}
                  </Button>
                )}
              </>
            )
          }
        />
        <div className="overflow-x-auto rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                {columns.map((col) => (
                  <TableHead key={col.key} className={cn('h-9 whitespace-nowrap text-xs', col.className)}>
                    {col.header}
                  </TableHead>
                ))}
                {menu && <TableHead className="h-9 w-10" />}
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.length === 0 ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={columns.length + (menu ? 1 : 0)} className="py-8 text-center text-sm text-muted-foreground">
                    {loading ? <Loader2 className="mx-auto size-4 animate-spin" /> : (empty ?? t('settingsUi.collection.empty'))}
                  </TableCell>
                </TableRow>
              ) : (
                rows.map((row) => {
                  const items = menu ? menu(row).filter(Boolean) : [];
                  return (
                    <TableRow
                      key={rowKey(row)}
                      className={cn(onRowClick && 'cursor-pointer')}
                      onClick={onRowClick ? () => onRowClick(row) : undefined}
                    >
                      {columns.map((col) => (
                        <TableCell key={col.key} className={cn('py-2.5', col.className)}>
                          {col.cell ? col.cell(row) : row[col.key]}
                        </TableCell>
                      ))}
                      {menu && (
                        <TableCell className="w-10 py-1.5 text-right" onClick={(e) => e.stopPropagation()}>
                          {items.length > 0 && (
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button variant="ghost" size="icon" className="size-8" aria-label={t('settingsUi.collection.more')}>
                                  <MoreHorizontal className="size-4" />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                {items.map((item, i) => [
                                  item.separator && i > 0 ? <DropdownMenuSeparator key={`sep-${item.label}`} /> : null,
                                  <DropdownMenuItem
                                    key={item.label}
                                    className={item.destructive ? 'text-destructive' : undefined}
                                    disabled={item.disabled}
                                    onClick={item.onClick}
                                  >
                                    {item.icon && <item.icon />}
                                    {item.label}
                                  </DropdownMenuItem>
                                ])}
                              </DropdownMenuContent>
                            </DropdownMenu>
                          )}
                        </TableCell>
                      )}
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </div>
        {footer}
      </div>
    </section>
  );
}

CollectionCard.propTypes = {
  title: PropTypes.node.isRequired,
  description: PropTypes.node,
  highlight: PropTypes.string,
  addLabel: PropTypes.node,
  onAdd: PropTypes.func,
  aside: PropTypes.node,
  columns: PropTypes.arrayOf(
    PropTypes.shape({ key: PropTypes.string.isRequired, header: PropTypes.node, cell: PropTypes.func, className: PropTypes.string })
  ).isRequired,
  rows: PropTypes.array.isRequired,
  rowKey: PropTypes.func,
  onRowClick: PropTypes.func,
  menu: PropTypes.func,
  loading: PropTypes.bool,
  empty: PropTypes.node,
  footer: PropTypes.node,
  className: PropTypes.string
};

// Several switches as one bordered list, one option per row.
export function OptionList({ children, className }) {
  return <div className={cn('divide-y divide-border overflow-hidden rounded-lg border border-border', className)}>{children}</div>;
}

OptionList.propTypes = { children: PropTypes.node, className: PropTypes.string };

// Sub-heading inside a form card; rows are grouped by headings, not lines.
export function FieldSection({ title, description, children }) {
  return (
    <div className="space-y-3">
      <div className="space-y-0.5">
        <h4 className="text-sm font-medium">{title}</h4>
        {description && <p className="text-xs text-muted-foreground">{description}</p>}
      </div>
      {children}
    </div>
  );
}

FieldSection.propTypes = { title: PropTypes.node, description: PropTypes.node, children: PropTypes.node };
