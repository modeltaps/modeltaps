import PropTypes from 'prop-types';
import { useMemo, useState } from 'react';
import { useTable, flexRender } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { ArrowDown, ArrowUp, ChevronsUpDown, Copy, Download, ExternalLink, MoreHorizontal } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { copy, timestamp2string } from 'utils/common';
import { badgeClass } from '../Log/logHelpers';
import { ACTION_TYPE, CODE_TYPE, STATUS_TYPE, deriveRequestTime } from './mjHelpers';

const Badge = ({ color, children }) => <span className={badgeClass(color)}>{children}</span>;
Badge.propTypes = { color: PropTypes.string, children: PropTypes.node };

function renderType(t, types, type) {
  const opt = types[type];
  if (!opt) return <Badge color="error">{t('midjourneyPage.unknown')}</Badge>;
  return <Badge color={opt.color}>{t(opt.labelKey)}</Badge>;
}

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
  const short = text.length > 30 ? `${text.substring(0, 30)}...` : text;
  return (
    <span
      className="cursor-pointer"
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
      {short}
    </span>
  );
}
Truncated.propTypes = { text: PropTypes.string };

function ImageCell({ t, item }) {
  const [open, setOpen] = useState(false);
  if (!item.image_url) return t('common.none');
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

export default function MidjourneyTable({ t, data, userIsAdmin, order, orderBy, onSort, searching }) {
  const columns = useMemo(() => {
    const cols = [
      {
        id: 'mj_id',
        sortable: true,
        header: t('midjourneyPage.taskID'),
        cell: (i) => <span className="whitespace-nowrap">{i.mj_id}</span>
      },
      {
        id: 'submit_time',
        sortable: true,
        header: t('midjourneyPage.submitTime'),
        cell: (i) => <span className="whitespace-nowrap">{timestamp2string(i.submit_time / 1000)}</span>
      }
    ];
    if (userIsAdmin) {
      cols.push({ id: 'channel_id', sortable: true, header: t('midjourneyPage.channel'), cell: (i) => i.channel_id || '' });
      cols.push({ id: 'user_id', sortable: true, header: t('midjourneyPage.user'), cell: (i) => i.user_id || '' });
    }
    cols.push({ id: 'action', sortable: true, header: t('midjourneyPage.type'), cell: (i) => renderType(t, ACTION_TYPE, i.action) });
    if (userIsAdmin) {
      cols.push({
        id: 'code',
        sortable: true,
        header: t('midjourneyPage.submissionResult'),
        cell: (i) => renderType(t, CODE_TYPE, i.code)
      });
      cols.push({
        id: 'status',
        sortable: true,
        header: t('midjourneyPage.taskStatus'),
        cell: (i) => renderType(t, STATUS_TYPE, i.status)
      });
    }
    cols.push({ id: 'progress', header: t('midjourneyPage.progress'), cell: (i) => i.progress });
    cols.push({
      id: 'time',
      header: t('midjourneyPage.timeConsuming'),
      cell: (i) => {
        const { seconds, str } = deriveRequestTime(i, t);
        return str ? <Badge color={seconds > 60 ? 'error' : 'success'}>{str}</Badge> : '';
      }
    });
    cols.push({ id: 'image_url', header: t('midjourneyPage.resultImage'), cell: (i) => <ImageCell t={t} item={i} /> });
    cols.push({ id: 'prompt', header: t('midjourneyPage.prompt'), cell: (i) => <Truncated text={i.prompt} /> });
    cols.push({ id: 'prompt_en', header: t('midjourneyPage.promptEn'), cell: (i) => <Truncated text={i.prompt_en} /> });
    cols.push({ id: 'fail_reason', header: t('midjourneyPage.failureReason'), cell: (i) => <Truncated text={i.fail_reason} /> });
    return cols.map((c) => ({ id: c.id, header: () => c.header, cell: ({ row }) => c.cell(row.original), meta: { sortable: c.sortable } }));
  }, [t, userIsAdmin]);

  const table = useTable({
    features: appTableFeatures,
    data: data || [],
    columns,
    manualSorting: true
  });

  const SortIcon = ({ id }) => {
    if (orderBy !== id) return <ChevronsUpDown className="size-3.5 opacity-50" />;
    return order === 'asc' ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />;
  };
  SortIcon.propTypes = { id: PropTypes.string };

  return (
    <>
      <div className="hidden md:block">
        <Table className="min-w-[800px]">
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id}>
                {hg.headers.map((h) => {
                  const sortable = h.column.columnDef.meta?.sortable;
                  return (
                    <TableHead key={h.id}>
                      {sortable ? (
                        <button
                          type="button"
                          className="inline-flex items-center gap-1 hover:text-foreground"
                          onClick={() => onSort(h.column.id)}
                        >
                          {flexRender(h.column.columnDef.header, h.getContext())}
                          <SortIcon id={h.column.id} />
                        </button>
                      ) : (
                        flexRender(h.column.columnDef.header, h.getContext())
                      )}
                    </TableHead>
                  );
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-24 text-center text-sm text-muted-foreground">
                  {searching ? t('logPage.searching') : t('midjourneyPage.emptyInfo', { defaultValue: 'No records found' })}
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
      <DataCards
        table={table}
        className="md:hidden"
        primaryColumnId="mj_id"
        empty={t('midjourneyPage.emptyInfo', { defaultValue: 'No records found' })}
        searching={searching}
      />
    </>
  );
}

MidjourneyTable.propTypes = {
  t: PropTypes.func.isRequired,
  data: PropTypes.array,
  userIsAdmin: PropTypes.bool,
  order: PropTypes.string,
  orderBy: PropTypes.string,
  onSort: PropTypes.func.isRequired,
  searching: PropTypes.bool
};
