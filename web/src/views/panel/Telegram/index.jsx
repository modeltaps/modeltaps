import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { flexRender, useTable } from '@tanstack/react-table';
import { appTableFeatures } from 'components/ui/table-features';
import { Info, Plus, RotateCcw, RotateCw, Send } from 'lucide-react';

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import DataCards from '@/components/ui/data-cards';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { FilterBar, filterStateToParams, isEntryActive, paramsToFilterState } from '@/components/filter-bar';
import ResponsiveToolbarButton from '@/components/ResponsiveToolbarButton';
import PageActions from '@/components/chrome/PageActions';
import PageTitleExtra from '@/components/chrome/PageTitleExtra';
import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { PAGE_SIZE_OPTIONS } from 'constants';
import usePaginatedList from 'hooks/usePaginatedList';
import TelegramSheet from './TelegramSheet';
import { ActionsCell } from './TelegramRowCells';
import Pagination from '../components/Pagination';

// FilterBar 字段:单个关键字文本 chip(后端 keyword 单值,匹配命令/描述)。
const TELEGRAM_FILTER_FIELDS = [
  { key: 'keyword', labelKey: 'filterBar.keyword', type: 'text', placeholderKey: 'telegramPage.searchPlaceholder' }
];
const TELEGRAM_FILTER_PARAM_KEYS = ['keyword'];

// `embedded`:嵌入系统设置「集成」主题页时不借用 PageHeader 插槽(那里是设置页
// 自己的标题),状态徽标与操作按钮就地渲染成一行。
export default function Telegram({ embedded = false }) {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [filterState, setFilterState] = useState(() => paramsToFilterState(TELEGRAM_FILTER_FIELDS, searchParams));
  const [status, setStatus] = useState(false);
  const [isWebhook, setIsWebhook] = useState(false);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editId, setEditId] = useState(0);

  // 分页列表状态 + 守卫化 fetch 生命周期(UX-13);本页固定 order=-id,无表头排序。
  const {
    page,
    setPage,
    rowsPerPage,
    listCount,
    searching,
    rows: menus,
    refresh,
    onRowsPerPageChange
  } = usePaginatedList({
    pageSizeKey: 'telegram',
    fetcher: async ({ page, rowsPerPage }) => {
      const res = await API.get('/api/option/telegram/', {
        params: {
          page: page + 1,
          size: rowsPerPage,
          keyword: filterStateToParams(TELEGRAM_FILTER_FIELDS, filterState).keyword || '',
          order: '-id'
        }
      });
      return res.data;
    },
    deps: [filterState]
  });

  const getStatus = async () => {
    try {
      const res = await API.get('/api/option/telegram/status');
      const { success, data } = res.data;
      if (success) {
        setStatus(data.status);
        setIsWebhook(data.is_webhook);
      }
    } catch (e) {
      // surfaced globally
    }
  };

  useEffect(() => {
    getStatus();
  }, []);

  // 已应用筛选同步到 URL(replace,刷新可恢复;先清空筛选键再按当前 state 重写)。
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        TELEGRAM_FILTER_PARAM_KEYS.forEach((k) => params.delete(k));
        const q = filterStateToParams(TELEGRAM_FILTER_FIELDS, filterState);
        for (const [k, v] of Object.entries(q)) params.set(k, String(v));
        return params;
      },
      { replace: true }
    );
  }, [filterState, setSearchParams]);

  const reload = async () => {
    try {
      const res = await API.put('/api/option/telegram/reload');
      const { success, message } = res.data;
      if (success) showSuccess(t('telegramPage.reloadSuccess'));
      else showError(message);
    } catch (e) {
      // surfaced globally
    }
  };

  const manageMenu = async (id, action) => {
    try {
      let res;
      if (action === 'delete') res = await API.delete('/api/option/telegram/' + id);
      else return false;
      if (res.data.success) {
        showSuccess(t('telegramPage.operationSuccess'));
        refresh();
      } else {
        showError(res.data.message);
      }
      return res.data;
    } catch (e) {
      return undefined;
    }
  };

  const openEdit = (id) => {
    setEditId(id);
    setSheetOpen(true);
  };

  const handleFilterChange = (next) => {
    setPage(0);
    setFilterState(next);
  };
  const handleClearFilters = () => {
    setPage(0);
    setFilterState({});
  };

  const columns = useMemo(
    () => [
      { accessorKey: 'id', header: t('telegramPage.id') },
      { accessorKey: 'command', header: t('telegramPage.command') },
      { accessorKey: 'description', header: t('telegramPage.description') },
      { accessorKey: 'parse_mode', header: t('telegramPage.replyType') },
      {
        accessorKey: 'reply_message',
        header: t('telegramPage.replyContent'),
        cell: (c) => <span className="block max-w-xs truncate">{c.getValue()}</span>
      },
      {
        id: 'action',
        header: t('telegramPage.action'),
        meta: { className: 'sticky right-0 bg-card z-10 text-right w-[1%]' },
        cell: (c) => <ActionsCell item={c.row.original} manageMenu={manageMenu} onEdit={openEdit} />
      }
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t]
  );

  const table = useTable({
    features: appTableFeatures,
    data: menus,
    columns
  });

  // 已生效的筛选字段(仅用于判断是否渲染下方 chips 容器)。
  const activeChipFields = useMemo(() => TELEGRAM_FILTER_FIELDS.filter((f) => isEntryActive(f, filterState[f.key])), [filterState]);

  // webhook/polling 状态与说明内联到标题右侧,不再各占独立行。
  const titleExtra = (
    <div className="flex min-w-0 items-center gap-2">
      <Badge variant="outline" className={status ? 'border-border text-foreground' : 'border-destructive text-destructive'}>
        <Send className="size-3" />
        {(status ? t('telegramPage.online') : t('telegramPage.offline')) + (isWebhook ? '(Webhook)' : '(Polling)')}
      </Badge>
      <TooltipProvider delayDuration={150}>
        <Tooltip>
          <TooltipTrigger asChild>
            <button type="button" className="text-muted-foreground hover:text-foreground" aria-label={t('telegramPage.infoMessage')}>
              <Info className="size-4" />
            </button>
          </TooltipTrigger>
          <TooltipContent className="max-w-xs">{t('telegramPage.infoMessage')}</TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </div>
  );

  // 标题行右侧操作:刷新 → 筛选 → 重载菜单 → 新建(primary),对齐 Log 页。
  const actions = (
    <>
      <TooltipProvider delayDuration={150}>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="outline"
              size="sm"
              aria-label={t('telegramPage.refresh')}
              onClick={() => {
                setPage(0);
                refresh();
              }}
            >
              <RotateCcw className={searching ? 'size-4 animate-spin' : 'size-4'} />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t('telegramPage.refresh')}</TooltipContent>
        </Tooltip>
      </TooltipProvider>
      <FilterBar
        fields={TELEGRAM_FILTER_FIELDS}
        state={filterState}
        onChange={handleFilterChange}
        onClearAll={handleClearFilters}
        hideChips
        t={t}
      />
      <ResponsiveToolbarButton variant="outline" size="sm" icon={RotateCw} label={t('telegramPage.reloadMenu')} onClick={reload} />
      <ResponsiveToolbarButton primary icon={Plus} label={t('telegramPage.createMenu')} onClick={() => openEdit(0)} />
    </>
  );

  return (
    <div className="space-y-6">
      {embedded ? (
        <div className="flex flex-wrap items-center justify-between gap-2">
          {titleExtra}
          <div className="flex items-center gap-2">{actions}</div>
        </div>
      ) : (
        <>
          <PageTitleExtra>{titleExtra}</PageTitleExtra>
          <PageActions>{actions}</PageActions>
        </>
      )}

      {/* 标题行下方:OpenRouter 式全宽 chips 容器;仅在有激活筛选时渲染,无筛选不占空间。 */}
      {activeChipFields.length > 0 && (
        <FilterBar
          fields={TELEGRAM_FILTER_FIELDS}
          state={filterState}
          onChange={handleFilterChange}
          onClearAll={handleClearFilters}
          triggerVariant="plus"
          clearAlignEnd
          t={t}
          className="w-full rounded-lg border border-input bg-background px-3 py-2"
        />
      )}

      <Card>
        <div className="hidden md:block">
          <Table>
            <TableHeader>
              {table.getHeaderGroups().map((hg) => (
                <TableRow key={hg.id} className="hover:bg-transparent">
                  {hg.headers.map((h) => (
                    <TableHead key={h.id} className={`whitespace-nowrap ${h.column.columnDef.meta?.className ?? ''}`}>
                      {flexRender(h.column.columnDef.header, h.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {table.getRowModel().rows.length === 0 ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={columns.length} className="py-10 text-center text-sm text-muted-foreground">
                    {t('common.noData', { defaultValue: 'No data' })}
                  </TableCell>
                </TableRow>
              ) : (
                table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id}>
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id} className={cell.column.columnDef.meta?.className}>
                        {flexRender(cell.column.columnDef.cell ?? cell.getValue, cell.getContext())}
                      </TableCell>
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
          primaryColumnId="command"
          actionColumnIds={['action']}
          empty={t('common.noData', { defaultValue: 'No data' })}
          searching={searching}
        />
        <Pagination
          page={page}
          rowsPerPage={rowsPerPage}
          count={listCount}
          options={PAGE_SIZE_OPTIONS}
          onPageChange={setPage}
          onRowsPerPageChange={onRowsPerPageChange}
        />
      </Card>

      <TelegramSheet
        open={sheetOpen}
        menuId={editId}
        onClose={() => setSheetOpen(false)}
        onSaved={() => {
          setSheetOpen(false);
          refresh();
        }}
      />
    </div>
  );
}
