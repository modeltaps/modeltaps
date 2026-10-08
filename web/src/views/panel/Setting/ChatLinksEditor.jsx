import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Plus, Pencil, Trash2, Check, X } from 'lucide-react';

import { showError } from 'utils/common';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table';

// Visual editor for the ChatLinks option (v1 ChatLinksDataGrid equivalent):
// rows of { name, url, show, sort } edited inline and serialized back to the
// same JSON string format, so the save path / backend contract is unchanged.

function randomId() {
  return Math.random().toString(36).substr(2, 9);
}

function parseLinks(value) {
  if (!value || !value.trim()) return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed.filter((it) => it && typeof it === 'object') : null;
  } catch (e) {
    return null;
  }
}

export default function ChatLinksEditor({ value, onChange, disabled }) {
  const { t } = useTranslation();
  const ed = (k) => t(`setting_index.operationSettings.chatLinkSettings.editor.${k}`);
  const [rows, setRows] = useState([]);
  const [drafts, setDrafts] = useState({});
  const [invalid, setInvalid] = useState(false);
  const lastEmitted = useRef(null);

  useEffect(() => {
    if (value === lastEmitted.current) return;
    const parsed = parseLinks(value);
    setInvalid(parsed === null);
    setRows((parsed || []).map((data) => ({ key: randomId(), data, isNew: false })));
    setDrafts({});
  }, [value]);

  const emit = (nextRows) => {
    const json = JSON.stringify(
      nextRows.filter((r) => !r.isNew).map((r) => r.data),
      null,
      2
    );
    lastEmitted.current = json;
    onChange(json);
  };

  const setDraft = (key, patch) => setDrafts((p) => ({ ...p, [key]: { ...p[key], ...patch } }));

  const handleAdd = () => {
    const key = randomId();
    setRows((p) => [{ key, data: { id: key, name: '', url: '', show: true, sort: 0 }, isNew: true }, ...p]);
    setDrafts((p) => ({ ...p, [key]: { name: '', url: '', show: true, sort: '0' } }));
  };

  const handleEdit = (row) =>
    setDraft(row.key, {
      name: row.data.name ?? '',
      url: row.data.url ?? '',
      show: row.data.show !== false,
      sort: row.data.sort === '' || row.data.sort == null ? '' : String(row.data.sort)
    });

  const handleCancel = (row) => {
    setDrafts((p) => {
      const { [row.key]: removed, ...rest } = p; // eslint-disable-line no-unused-vars
      return rest;
    });
    if (row.isNew) setRows((p) => p.filter((r) => r.key !== row.key));
  };

  const handleSave = (row) => {
    const draft = drafts[row.key];
    if (!draft.name) return showError(ed('nameRequired'));
    if (!draft.url) return showError(ed('urlRequired'));
    if (draft.sort !== '' && !/^[0-9]\d*$/.test(String(draft.sort))) return showError(ed('sortInvalid'));
    const data = { ...row.data, name: draft.name, url: draft.url, show: !!draft.show, sort: draft.sort === '' ? '' : Number(draft.sort) };
    const nextRows = rows.map((r) => (r.key === row.key ? { key: r.key, data, isNew: false } : r));
    setRows(nextRows);
    handleCancel({ ...row, isNew: false });
    emit(nextRows);
  };

  const handleDelete = (row) => {
    const nextRows = rows.filter((r) => r.key !== row.key);
    setRows(nextRows);
    emit(nextRows);
  };

  return (
    <div className="space-y-2">
      {invalid && <p className="text-sm text-destructive">{ed('invalidExisting')}</p>}
      <div>
        <Button type="button" variant="outline" size="sm" onClick={handleAdd} disabled={disabled}>
          <Plus className="size-4" />
          {ed('add')}
        </Button>
      </div>
      <div className="hidden md:block rounded-md border border-border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="min-w-[140px]">{ed('name')}</TableHead>
              <TableHead className="min-w-[220px]">{ed('url')}</TableHead>
              <TableHead>{ed('show')}</TableHead>
              <TableHead className="w-24">{ed('sort')}</TableHead>
              <TableHead className="w-24 text-right">{ed('actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="h-16 text-center text-muted-foreground">
                  {ed('empty')}
                </TableCell>
              </TableRow>
            )}
            {rows.map((row) => {
              const draft = drafts[row.key];
              return draft ? (
                <TableRow key={row.key}>
                  <TableCell>
                    <Input
                      value={draft.name}
                      onChange={(e) => setDraft(row.key, { name: e.target.value })}
                      placeholder={ed('name')}
                      disabled={disabled}
                      autoFocus
                    />
                  </TableCell>
                  <TableCell>
                    <Input
                      value={draft.url}
                      onChange={(e) => setDraft(row.key, { url: e.target.value })}
                      placeholder={ed('url')}
                      disabled={disabled}
                    />
                  </TableCell>
                  <TableCell>
                    <Switch checked={draft.show} onCheckedChange={(v) => setDraft(row.key, { show: v })} disabled={disabled} />
                  </TableCell>
                  <TableCell>
                    <Input
                      className="w-20"
                      inputMode="numeric"
                      value={draft.sort}
                      onChange={(e) => setDraft(row.key, { sort: e.target.value })}
                      disabled={disabled}
                    />
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-8"
                      title={ed('saveRow')}
                      aria-label={ed('saveRow')}
                      onClick={() => handleSave(row)}
                      disabled={disabled}
                    >
                      <Check className="size-4 text-foreground" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-8"
                      title={ed('cancel')}
                      aria-label={ed('cancel')}
                      onClick={() => handleCancel(row)}
                      disabled={disabled}
                    >
                      <X className="size-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ) : (
                <TableRow key={row.key}>
                  <TableCell className="max-w-[220px] truncate" title={row.data.name}>
                    {row.data.name}
                  </TableCell>
                  <TableCell className="max-w-[320px] truncate text-muted-foreground" title={row.data.url}>
                    {row.data.url}
                  </TableCell>
                  <TableCell>
                    <Switch checked={row.data.show !== false} disabled />
                  </TableCell>
                  <TableCell>{row.data.sort}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-8"
                      title={ed('edit')}
                      aria-label={ed('edit')}
                      onClick={() => handleEdit(row)}
                      disabled={disabled}
                    >
                      <Pencil className="size-4" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-8"
                      title={ed('delete')}
                      aria-label={ed('delete')}
                      onClick={() => handleDelete(row)}
                      disabled={disabled}
                    >
                      <Trash2 className="size-4 text-destructive" />
                    </Button>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
      {rows.length === 0 ? (
        <div className="md:hidden rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">{ed('empty')}</div>
      ) : (
        <div className="md:hidden flex flex-col gap-3">
          {rows.map((row) => {
            const draft = drafts[row.key];
            return draft ? (
              <div key={row.key} className="rounded-lg border border-border bg-card p-3.5 shadow-sm">
                <dl className="flex flex-col divide-y divide-border/60">
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">{ed('name')}</dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      <Input
                        value={draft.name}
                        onChange={(e) => setDraft(row.key, { name: e.target.value })}
                        placeholder={ed('name')}
                        disabled={disabled}
                        autoFocus
                      />
                    </dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">{ed('url')}</dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      <Input
                        value={draft.url}
                        onChange={(e) => setDraft(row.key, { url: e.target.value })}
                        placeholder={ed('url')}
                        disabled={disabled}
                      />
                    </dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">{ed('show')}</dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      <Switch checked={draft.show} onCheckedChange={(v) => setDraft(row.key, { show: v })} disabled={disabled} />
                    </dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">{ed('sort')}</dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      <Input
                        className="w-20"
                        inputMode="numeric"
                        value={draft.sort}
                        onChange={(e) => setDraft(row.key, { sort: e.target.value })}
                        disabled={disabled}
                      />
                    </dd>
                  </div>
                </dl>
                <div className="mt-3 flex flex-wrap items-center justify-end gap-1 border-t border-border pt-3">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    title={ed('saveRow')}
                    aria-label={ed('saveRow')}
                    onClick={() => handleSave(row)}
                    disabled={disabled}
                  >
                    <Check className="size-4 text-foreground" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    title={ed('cancel')}
                    aria-label={ed('cancel')}
                    onClick={() => handleCancel(row)}
                    disabled={disabled}
                  >
                    <X className="size-4" />
                  </Button>
                </div>
              </div>
            ) : (
              <div key={row.key} className="rounded-lg border border-border bg-card p-3.5 shadow-sm">
                <dl className="flex flex-col divide-y divide-border/60">
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">{ed('name')}</dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm" title={row.data.name}>
                      {row.data.name}
                    </dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">{ed('url')}</dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm text-muted-foreground" title={row.data.url}>
                      {row.data.url}
                    </dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">{ed('show')}</dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">
                      <Switch checked={row.data.show !== false} disabled />
                    </dd>
                  </div>
                  <div className="flex items-start justify-between gap-3 py-1.5 first:pt-0 last:pb-0">
                    <dt className="shrink-0 pt-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">{ed('sort')}</dt>
                    <dd className="flex min-w-0 flex-wrap justify-end gap-1 text-right text-sm">{row.data.sort}</dd>
                  </div>
                </dl>
                <div className="mt-3 flex flex-wrap items-center justify-end gap-1 border-t border-border pt-3">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    title={ed('edit')}
                    aria-label={ed('edit')}
                    onClick={() => handleEdit(row)}
                    disabled={disabled}
                  >
                    <Pencil className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    title={ed('delete')}
                    aria-label={ed('delete')}
                    onClick={() => handleDelete(row)}
                    disabled={disabled}
                  >
                    <Trash2 className="size-4 text-destructive" />
                  </Button>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
