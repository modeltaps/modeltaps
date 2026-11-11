import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';
import { RotateCcw, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { toast } from '@/components/ui/sonner';
import PageActions from '@/components/chrome/PageActions';
import { ListEmptyState, ListFooter, ListSkeleton, useListLayout } from '@/components/list-page';
import { useObjectRoute } from '@/components/objects';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import usePaginatedList from 'hooks/usePaginatedList';
import OrgAdminTable, { ORG_TABLE_ESTIMATE, OrgAdminCards } from './OrgAdminTable';
import OrgPanel from './OrgPanel';
import { ORG_TAB_KEYS } from './orgTabs';

const KEYWORD_DEBOUNCE_MS = 300;

// ==============================|| PANEL — ORG ADMIN (site admin) ||============================== //
// Site-level organization list (spec §3.8 / a8): list + enable/disable + dissolve; a row opens
// the organization's panel (?id=) with pool balance adjustment and the member limit. Global org options moved to the Setting page
// (`/panel/setting/orgpolicy`, root-only) per IA-2/IA-4 — this page is a pure list now.
// Backend is /api/organization/* (RootAuth); route is wrapped in AdminGuard like
// other admin pages and non-admins get the in-place 404.

export default function OrgAdmin() {
  const { t, i18n } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [keywordInput, setKeywordInput] = useState(() => searchParams.get('keyword') || '');
  const [keyword, setKeyword] = useState(() => (searchParams.get('keyword') || '').trim());

  const route = useObjectRoute(ORG_TAB_KEYS);
  const [dissolveTarget, setDissolveTarget] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  // Bumped after a list action so an open organization panel reloads.
  const [reloadKey, setReloadKey] = useState(0);

  // Paginated list (no sorting on this page).
  const { page, setPage, rowsPerPage, listCount, searching, loaded, rows, setRows, refresh, onRowsPerPageChange } = usePaginatedList({
    pageSizeKey: 'org_admin',
    fetcher: async ({ page, rowsPerPage }) => {
      const res = await API.get('/api/organization/', { params: { page: page + 1, size: rowsPerPage, keyword } });
      return res.data;
    },
    deps: [keyword]
  });

  // The search box applies after a short pause in typing.
  useEffect(() => {
    const handle = setTimeout(() => {
      const next = keywordInput.trim();
      if (next === keyword) return;
      setPage(0);
      setKeyword(next);
    }, KEYWORD_DEBOUNCE_MS);
    return () => clearTimeout(handle);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [keywordInput]);

  // The search lives in the URL (replace), so a reload restores it.
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        params.delete('keyword');
        if (keyword) params.set('keyword', keyword);
        return params;
      },
      { replace: true }
    );
  }, [keyword, setSearchParams]);

  // Reload in place after a change (keeps the search), for the balance and dissolve dialogs.
  const handleRefresh = () => refresh();

  // Full table while it fits (the sidebar collapses first), phone cards otherwise.
  const { pageRef, narrow, onNaturalWidth } = useListLayout({ estimate: ORG_TABLE_ESTIMATE, resetKey: 'orgs' });
  const rtf = useMemo(
    () => new Intl.RelativeTimeFormat((i18n.language || 'zh_CN').replace('_', '-'), { numeric: 'auto' }),
    [i18n.language]
  );

  const onToggleStatus = async (org) => {
    const next = org.status === 1 ? 2 : 1;
    try {
      const res = await API.put(`/api/organization/${org.id}/status`, { status: next });
      if (res.data.success) {
        toast.success(t('common.operationSuccess'));
        setRows((prev) => prev.map((x) => (x.id === org.id ? { ...x, status: next } : x)));
        setReloadKey((k) => k + 1);
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const confirmDissolve = async () => {
    if (!dissolveTarget) return;
    setSubmitting(true);
    try {
      const res = await API.delete(`/api/organization/${dissolveTarget.id}`);
      if (res.data.success) {
        toast.success(t('common.operationSuccess'));
        if (dissolveTarget.id === route.openId) route.close();
        setDissolveTarget(null);
        handleRefresh();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  let body;
  if (!loaded) {
    body = narrow ? (
      <ListSkeleton layout="list" rows={3} />
    ) : (
      <ListSkeleton layout="table" rows={3} checkbox={false} cells={[80, 64, 64, 32]} />
    );
  } else if (!searching && rows.length === 0) {
    body = keyword ? (
      <ListEmptyState
        variant="noMatch"
        onClearFilters={() => {
          setKeywordInput('');
          setKeyword('');
        }}
      />
    ) : (
      <ListEmptyState title={t('dashboard_index.no_data', { defaultValue: 'No data' })} />
    );
  } else if (narrow) {
    body = (
      <OrgAdminCards
        rows={rows}
        rtf={rtf}
        openId={route.openId}
        onOpen={route.open}
        onToggleStatus={onToggleStatus}
        onDissolve={setDissolveTarget}
      />
    );
  } else {
    body = (
      <OrgAdminTable
        rows={rows}
        rtf={rtf}
        openId={route.openId}
        onOpen={route.open}
        onToggleStatus={onToggleStatus}
        onDissolve={setDissolveTarget}
        onNaturalWidth={onNaturalWidth}
      />
    );
  }

  const footer = (
    <ListFooter total={listCount} page={page} pageSize={rowsPerPage} onPageChange={setPage} onPageSizeChange={onRowsPerPageChange} />
  );

  return (
    <>
      <div ref={pageRef} className="space-y-3">
        {/* Title row: refresh (icon). */}
        <PageActions>
          <TooltipProvider delayDuration={150}>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="outline" size="sm" onClick={handleRefresh} aria-label={t('logPage.refreshButton')}>
                  <RotateCcw className={searching ? 'size-4 animate-spin' : 'size-4'} />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t('logPage.refreshButton')}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </PageActions>

        <div className="relative max-w-80">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={keywordInput}
            onChange={(e) => setKeywordInput(e.target.value)}
            placeholder={t('orgAdmin.searchPlaceholder')}
            aria-label={t('orgAdmin.searchPlaceholder')}
            className="h-8 pl-8"
          />
        </div>

        {narrow ? (
          <>
            {body}
            <Card>{footer}</Card>
          </>
        ) : (
          <Card>
            {body}
            <div className="border-t border-border">{footer}</div>
          </Card>
        )}

        {route.openId > 0 && (
          <OrgPanel
            key={route.openId}
            orgId={route.openId}
            tab={route.tab}
            onTabChange={route.setTab}
            onClose={route.close}
            onChanged={handleRefresh}
            onToggleStatus={onToggleStatus}
            onDissolve={setDissolveTarget}
            reloadKey={reloadKey}
          />
        )}

        <Dialog open={!!dissolveTarget} onOpenChange={(o) => !o && setDissolveTarget(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('orgAdmin.dissolve')}</DialogTitle>
              <DialogDescription>{t('orgAdmin.dissolveConfirm', { name: dissolveTarget?.name })}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDissolveTarget(null)}>
                {t('token_index.cancel')}
              </Button>
              <Button variant="destructive" onClick={confirmDissolve} disabled={submitting}>
                {t('orgAdmin.dissolve')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </>
  );
}
