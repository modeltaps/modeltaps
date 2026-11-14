import PropTypes from 'prop-types';
import { useMemo, useState } from 'react';
import { useTable, flexRender } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowDown, ArrowUp, ArrowUpDown, Copy, Download, ExternalLink, MoreHorizontal } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';
import { copy, renderQuota, timestamp2string, timestamp2stringCompact } from 'utils/common';
import { ACTION_TYPE, CODE_TYPE, STATUS_TYPE, deriveRequestTime } from './mjHelpers';

// A type's label as plain text; unknown values say so.
const typeLabel = (t, types, type) => (types[type] ? t(types[type].labelKey) : t('midjourneyPage.unknown'));

// Columns for the shared column picker. By default a task shows when it was sent, what it did,
// the prompt, how it went and the result; ids, channel, user, the submission code and the
// English prompt are one click away. Status shows for everyone (it used to be admin-only).
export const MJ_COLUMNS = [
  { id: 'mj_id', labelKey: 'midjourneyPage.taskID', defaultHidden: true },
  { id: 'submit_time', labelKey: 'midjourneyPage.submitTime', fixed: true },
  { id: 'channel_id', labelKey: 'midjourneyPage.channel', defaultHidden: true, adminOnly: true },
  { id: 'user_id', labelKey: 'midjourneyPage.user', defaultHidden: true, adminOnly: true },
  { id: 'action', labelKey: 'midjourneyPage.type' },
  { id: 'prompt', labelKey: 'midjourneyPage.prompt', fixed: true },
  { id: 'status', labelKey: 'midjourneyPage.taskStatus', fixed: true },
  { id: 'progress', labelKey: 'midjourneyPage.progress' },
  { id: 'time', labelKey: 'midjourneyPage.timeConsuming' },
  { id: 'image_url', labelKey: 'midjourneyPage.resultImage' },
  { id: 'code', labelKey: 'midjourneyPage.submissionResult', defaultHidden: true, adminOnly: true },
  { id: 'prompt_en', labelKey: 'midjourneyPage.promptEn', defaultHidden: true },
  { id: 'quota', labelKey: 'logPage.quotaLabel', defaultHidden: true }
];

// Status as text; a failure is tinted with its reason after it.
function MjStatus({ t, item, withReason = true }) {
  const failed = item.status === 'FAILURE';
  return (
    <span className={cn('inline-flex min-w-0 max-w-64 items-baseline gap-1.5 whitespace-nowrap', failed && 'text-destructive')}>
      <span className={cn(!failed && item.status !== 'SUCCESS' && 'text-muted-foreground')}>{typeLabel(t, STATUS_TYPE, item.status)}</span>
      {withReason && failed && item.fail_reason && (
        <span className="truncate text-xs text-muted-foreground" title={item.fail_reason}>
          {item.fail_reason}
        </span>
      )}
    </span>
  );
}
MjStatus.propTypes = { t: PropTypes.func.isRequired, item: PropTypes.object.isRequired, withReason: PropTypes.bool };

const submitSeconds = (i) => (i.submit_time > 0 ? i.submit_time / 1000 : 0);

async function downloadImage(url, filename) {
  const response = await fetch(url);
  const blob = await response.blob();
  const blobUrl = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = blobUrl;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(blobUrl);
}

function Truncated({ text }) {
  if (!text) return '';
  return (
    <span
      className="block max-w-64 cursor-pointer truncate"
      title={text}
      role="button"
      tabIndex={0}
      onClick={() => copy(text, '')}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          copy(text, '');
        }
      }}
    >
      {text}
    </span>
  );
}
Truncated.propTypes = { text: PropTypes.string };

function ImageCell({ t, item }) {
  const [open, setOpen] = useState(false);
  if (!item.image_url) return <span className="text-muted-foreground">—</span>;
  return (
    <div className="flex items-center gap-1">
      <Button variant="outline" size="sm" onClick={() => setOpen(true)}>
        {t('common.show')}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="icon" aria-label={t('common.show')}>
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={() => copy(item.image_url, t('common.imgUrl'))}>
            <Copy className="size-4" />
            {t('common.copyUrl')}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => downloadImage(item.image_url, `${item.mj_id}.png`)}>
            <Download className="size-4" />
            {t('common.downImg')}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => window.open(item.image_url, '_blank')}>
            <ExternalLink className="size-4" />
            {t('common.newWindos')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-3xl">
          <img src={item.image_url} alt={item.mj_id} className="max-h-[80vh] w-full object-contain" />
        </DialogContent>
      </Dialog>
    </div>
  );
}
ImageCell.propTypes = { t: PropTypes.func, item: PropTypes.object };

export default function MidjourneyTable({ t, data, userIsAdmin, order, orderBy, onSort, searching, columnVisibility, layout = 'table' }) {
  const columns = useMemo(() => {
    const cells = {
      mj_id: (i) => <span className="font-mono text-xs">{i.mj_id}</span>,
      submit_time: (i) =>
        submitSeconds(i) ? <span title={timestamp2string(submitSeconds(i))}>{timestamp2stringCompact(submitSeconds(i))}</span> : '—',
      channel_id: (i) => i.channel_id || '',
      user_id: (i) => i.user_id || '',
      action: (i) => typeLabel(t, ACTION_TYPE, i.action),
      prompt: (i) => <Truncated text={i.prompt} />,
      status: (i) => <MjStatus t={t} item={i} />,
      progress: (i) => <span className="tabular-nums">{i.progress}</span>,
      time: (i) => {
        const { seconds } = deriveRequestTime(i, t);
        return seconds > 0 ? <span className="tabular-nums">{seconds.toFixed(1)}s</span> : <span className="text-muted-foreground">—</span>;
      },
      image_url: (i) => <ImageCell t={t} item={i} />,
      code: (i) => typeLabel(t, CODE_TYPE, i.code),
      prompt_en: (i) => <Truncated text={i.prompt_en} />,
      quota: (i) => <span className="font-mono tabular-nums">{i.quota ? renderQuota(i.quota) : '—'}</span>
    };
    const sortable = new Set(['mj_id', 'submit_time', 'channel_id', 'user_id', 'action', 'status', 'code']);
    const right = new Set(['progress', 'time', 'quota']);
    return MJ_COLUMNS.filter((c) => !c.adminOnly || userIsAdmin).map((c) => ({
      id: c.id,
      header: () => t(c.labelKey),
      cell: ({ row }) => cells[c.id](row.original),
      meta: { sortable: sortable.has(c.id), className: right.has(c.id) ? 'text-right' : undefined }
    }));
  }, [t, userIsAdmin]);

  const table = useTable({ features: appTableFeatures, data: data || [], columns, manualSorting: true, state: { columnVisibility } });
  const rows = data || [];

  if (rows.length === 0) {
    return (
      <p className="px-4 py-10 text-center text-sm text-muted-foreground">
        {searching ? t('logPage.searching') : t('midjourneyPage.emptyInfo', { defaultValue: 'No records found' })}
      </p>
    );
  }

  if (layout === 'list') {
    // Phone row: the prompt and the status, then the type, time and progress.
    return (
      <ul className="divide-y divide-border">
        {rows.map((i) => (
          <li key={i.id} className="flex h-14 items-center gap-3 px-4">
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline gap-3">
                <span className="min-w-0 truncate text-[13px] font-medium">{i.prompt || '—'}</span>
                <span className="ml-auto shrink-0 text-[13px]">
                  <MjStatus t={t} item={i} withReason={false} />
                </span>
              </div>
              <div className="mt-0.5 truncate text-xs tabular-nums text-muted-foreground">
                {[typeLabel(t, ACTION_TYPE, i.action), submitSeconds(i) ? timestamp2stringCompact(submitSeconds(i)) : null, i.progress]
                  .filter(Boolean)
                  .join(' · ')}
              </div>
            </div>
          </li>
        ))}
      </ul>
    );
  }

  // Sort arrows: the active column shows its direction; others show a faint arrow on hover only.
  const sortIcon = (id) => {
    if (orderBy !== id) return <ArrowUpDown className="size-3 opacity-0 transition-opacity group-hover/sort:opacity-50" />;
    return order === 'asc' ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
  };

  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((hg) => (
          <TableRow key={hg.id} className="hover:bg-transparent">
            {hg.headers.map((h) => (
              <TableHead key={h.id} className={cn('whitespace-nowrap', h.column.columnDef.meta?.className)}>
                {h.column.columnDef.meta?.sortable ? (
                  <button
                    type="button"
                    className={cn(
                      'group/sort inline-flex items-center gap-1 hover:text-foreground',
                      orderBy === h.column.id && 'text-foreground'
                    )}
                    onClick={() => onSort(h.column.id)}
                  >
                    {flexRender(h.column.columnDef.header, h.getContext())}
                    {sortIcon(h.column.id)}
                  </button>
                ) : (
                  flexRender(h.column.columnDef.header, h.getContext())
                )}
              </TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow key={row.id}>
            {row.getVisibleCells().map((cell) => (
              <TableCell key={cell.id} className={cn('whitespace-nowrap', cell.column.columnDef.meta?.className)}>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

MidjourneyTable.propTypes = {
  t: PropTypes.func.isRequired,
  data: PropTypes.array,
  userIsAdmin: PropTypes.bool,
  order: PropTypes.string,
  orderBy: PropTypes.string,
  onSort: PropTypes.func.isRequired,
  searching: PropTypes.bool,
  columnVisibility: PropTypes.object,
  layout: PropTypes.oneOf(['table', 'list'])
};
