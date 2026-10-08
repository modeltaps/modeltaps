import { useCallback, useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { Ban, Link2, Plus, RefreshCw, Search, SlidersHorizontal, UserMinus, UserPlus } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Select, SelectTrigger, SelectContent, SelectItem } from '@/components/ui/select';
import { RowActions, RowActionsSurfaceContext } from '@/components/ui/row-actions';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { EmptyState } from '@/components/ui/empty-state';
import { API } from 'utils/api';
import { copy, renderSpend, showError, showSuccess, timestamp2string } from 'utils/common';
import { PAGE_SIZE_OPTIONS, getPageSize, savePageSize } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
import Pagination from '../components/Pagination';
import MemberLimitsSheet from './MemberLimitsSheet';
import InvitationSheet from './InvitationSheet';
import CreateMemberSheet from './CreateMemberSheet';

// ==============================|| ORGANIZATION — MEMBERS TAB ||============================== //
// Member list (Member+). Admin+: sub-segments "members / pending invitations",
// invite button (InvitationSheet), change role, remove, budget & whitelist,
// copy link / revoke pending invitations (T21a, full history lives in audit).
// Hierarchy mirrors backend: cannot act on self, nor on a higher-weight role.

export const inviteLink = (token) => `${window.location.origin}/panel/organization?invite=${token}`;

const ROLE_WEIGHT = { owner: 3, admin: 2, member: 1 };

export default function MembersTab({ orgId, role }) {
  const { t } = useTranslation();
  const account = useSelector((state) => state.account);
  const myUserId = account?.user?.id;
  const myWeight = ROLE_WEIGHT[role] || 0;
  const isAdmin = myWeight >= ROLE_WEIGHT.admin;

  const [seg, setSeg] = useState('members');
  const [search, setSearch] = useState('');
  const [members, setMembers] = useState([]);
  const [count, setCount] = useState(0);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(() => getPageSize('org_members'));
  const [loading, setLoading] = useState(false);
  const [removeTarget, setRemoveTarget] = useState(null);
  const [removing, setRemoving] = useState(false);
  const [limitsTarget, setLimitsTarget] = useState(null);

  const [invitations, setInvitations] = useState([]);
  const [invCount, setInvCount] = useState(0);
  const [invPage, setInvPage] = useState(0);
  const [invRowsPerPage, setInvRowsPerPage] = useState(() => getPageSize('org_invitations'));
  const [invLoading, setInvLoading] = useState(false);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  // reqId 守卫(UX-13):快速切页/切组织时丢弃过期响应;成员与邀请两个列表各自独立守卫。
  const membersGuardRef = useRef(null);
  if (!membersGuardRef.current) membersGuardRef.current = createRequestGuard();
  const invGuardRef = useRef(null);
  if (!invGuardRef.current) invGuardRef.current = createRequestGuard();

  const fetchData = useCallback(
    () =>
      runGuardedFetch(
        membersGuardRef.current,
        () => API.get(`/api/org/${orgId}/members`, { params: { page: page + 1, size: rowsPerPage, order: '-id' } }),
        {
          onStart: () => setLoading(true),
          onResult: (res) => {
            const { success, message, data } = res.data;
            if (success) {
              setMembers(data?.data || []);
              setCount(data?.total_count || 0);
            } else {
              showError(message);
            }
          },
          onError: (error) => console.error(error),
          onFinally: () => setLoading(false)
        }
      ),
    [orgId, page, rowsPerPage]
  );

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // 待处理邀请(status=1):仅 Admin+ 拉取;已完成/撤销/过期的历史看审计日志
  const fetchInvitations = useCallback(() => {
    if (!isAdmin) return;
    return runGuardedFetch(
      invGuardRef.current,
      () =>
        API.get(`/api/org/${orgId}/invitations`, {
          params: { page: invPage + 1, size: invRowsPerPage, order: '-id', status: 1 }
        }),
      {
        onStart: () => setInvLoading(true),
        onResult: (res) => {
          const { success, message, data } = res.data;
          if (success) {
            setInvitations(data?.data || []);
            setInvCount(data?.total_count || 0);
          } else {
            showError(message);
          }
        },
        onError: (error) => console.error(error),
        onFinally: () => setInvLoading(false)
      }
    );
  }, [orgId, invPage, invRowsPerPage, isAdmin]);

  useEffect(() => {
    fetchInvitations();
  }, [fetchInvitations]);

  // 角色降级后回到成员分段
  useEffect(() => {
    if (!isAdmin && seg === 'invitations') setSeg('members');
  }, [isAdmin, seg]);

  // 后端层级约束的 UI 镜像:不能操作自己,也不能操作权重高于自己的成员
  const canManage = (m) => isAdmin && m.user_id !== myUserId && (ROLE_WEIGHT[m.role] || 0) <= myWeight;

  // 预算列(T21c):有预算显示「已用 / 上限 · 周期」,无预算显示「不限」;可管理的成员点击打开 MemberLimitsSheet
  const renderBudget = (m) => {
    const over = m.budget && (m.budget_used || 0) >= m.budget.limit;
    const text = m.budget
      ? `${renderSpend(m.budget_used || 0)} / ${renderSpend(m.budget.limit)} · ${t(`orgPage.limits.period_${m.budget.period}`)}`
      : t('orgPage.members.budgetUnlimited');
    if (!canManage(m)) {
      return <span className={`tabular-nums ${over ? 'text-destructive' : 'text-muted-foreground'}`}>{text}</span>;
    }
    return (
      <button
        type="button"
        className={`tabular-nums underline-offset-2 hover:underline ${over ? 'text-destructive' : 'text-muted-foreground hover:text-foreground'}`}
        onClick={() => setLimitsTarget(m)}
      >
        {text}
      </button>
    );
  };

  // 前端按用户名/昵称/邮箱过滤当前页成员
  const q = search.trim().toLowerCase();
  const filteredMembers = q
    ? members.filter((m) => [m.username, m.display_name, m.email].some((v) => v && String(v).toLowerCase().includes(q)))
    : members;

  const changeRole = async (m, newRole) => {
    if (newRole === m.role) return;
    try {
      const res = await API.put(`/api/org/${orgId}/members/${m.user_id}`, { role: newRole });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.members.roleUpdateSuccess'));
        fetchData();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const confirmRemove = async () => {
    if (!removeTarget) return;
    setRemoving(true);
    try {
      const res = await API.delete(`/api/org/${orgId}/members/${removeTarget.user_id}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.members.removeSuccess'));
        fetchData();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setRemoving(false);
      setRemoveTarget(null);
    }
  };

  const revoke = async (inv) => {
    try {
      const res = await API.delete(`/api/org/${orgId}/invitations/${inv.id}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.invitations.revokeSuccess'));
        fetchInvitations();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const showInvitations = isAdmin && seg === 'invitations';

  return (
    <Card>
      <div className="flex flex-col gap-2 border-b border-border p-3 sm:flex-row sm:items-center sm:justify-between">
        {/* 子分段「成员(N) / 邀请(M)」:Member 角色只见成员列表,不渲染分段控件 */}
        {isAdmin ? (
          <Tabs value={seg} onValueChange={setSeg}>
            <TabsList>
              <TabsTrigger value="members">
                {t('orgPage.tabs.members')} ({count})
              </TabsTrigger>
              <TabsTrigger value="invitations">
                {t('orgPage.members.pendingInvitations')} ({invCount})
              </TabsTrigger>
            </TabsList>
          </Tabs>
        ) : (
          <div />
        )}
        <div className="flex items-center gap-2">
          {!showInvitations && (
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={t('orgPage.members.search')}
                className="h-9 w-56 pl-8"
              />
            </div>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={showInvitations ? fetchInvitations : fetchData}
            disabled={showInvitations ? invLoading : loading}
          >
            <RefreshCw className="size-4" />
          </Button>
          {isAdmin && (
            <Button variant="outline" size="sm" onClick={() => setCreateOpen(true)}>
              <UserPlus className="size-4" /> {t('orgPage.members.createAccount')}
            </Button>
          )}
          {isAdmin && (
            <Button size="sm" onClick={() => setSheetOpen(true)}>
              <Plus className="size-4" /> {t('orgPage.members.invite')}
            </Button>
          )}
        </div>
      </div>

      {showInvitations ? (
        <>
          <div className="hidden md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('orgPage.invitations.invitee')}</TableHead>
                  <TableHead>{t('orgPage.members.role')}</TableHead>
                  <TableHead>{t('orgPage.invitations.status')}</TableHead>
                  <TableHead>{t('orgPage.invitations.usage')}</TableHead>
                  <TableHead>{t('orgPage.invitations.expiryTime')}</TableHead>
                  <TableHead className="w-[1%] text-right">{t('orgPage.members.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {invitations.map((inv) => (
                  <TableRow key={inv.id}>
                    <TableCell>
                      {inv.invitee_id > 0 ? (
                        <span className="font-medium">{inv.email || `#${inv.invitee_id}`}</span>
                      ) : (
                        <Badge variant="outline" className="gap-1">
                          <Link2 className="size-3" /> {t('orgPage.invitations.typeLink')}
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell>{t(`org.roles.${inv.role}`)}</TableCell>
                    <TableCell>
                      <Badge variant="default">{t(`orgPage.invitations.status_${inv.status}`)}</Badge>
                    </TableCell>
                    <TableCell className="tabular-nums text-muted-foreground">
                      {inv.used_count} / {inv.max_uses === 0 ? t('orgPage.invitations.unlimited') : inv.max_uses}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {inv.expired_time > 0 ? timestamp2string(inv.expired_time) : t('orgPage.myInvitations.never')}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        {inv.invitee_id === 0 && (
                          <Button variant="ghost" size="sm" onClick={() => copy(inviteLink(inv.token), t('orgPage.invitations.copyLink'))}>
                            <Link2 className="size-4" /> {t('orgPage.invitations.copyLink')}
                          </Button>
                        )}
                        <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive" onClick={() => revoke(inv)}>
                          <Ban className="size-4" /> {t('orgPage.invitations.revoke')}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <div className="md:hidden flex flex-col gap-3 p-3">
            {invitations.map((inv) => (
              <div key={inv.id} className="rounded-lg border border-border bg-card p-3 shadow-sm">
                <div className="mb-1.5 text-sm font-medium">
                  {inv.invitee_id > 0 ? (
                    <span className="font-medium">{inv.email || `#${inv.invitee_id}`}</span>
                  ) : (
                    <Badge variant="outline" className="gap-1">
                      <Link2 className="size-3" /> {t('orgPage.invitations.typeLink')}
                    </Badge>
                  )}
                </div>
                <dl className="grid grid-cols-2 gap-x-4 gap-y-1.5">
                  <div className="flex items-baseline gap-1.5">
                    <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgPage.members.role')}</dt>
                    <dd className="min-w-0 flex-1 truncate text-sm">{t(`org.roles.${inv.role}`)}</dd>
                  </div>
                  <div className="flex items-baseline gap-1.5">
                    <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgPage.invitations.status')}</dt>
                    <dd className="flex min-w-0 flex-1 flex-wrap gap-1 text-sm">
                      <Badge variant="default">{t(`orgPage.invitations.status_${inv.status}`)}</Badge>
                    </dd>
                  </div>
                  <div className="flex items-baseline gap-1.5">
                    <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgPage.invitations.usage')}</dt>
                    <dd className="min-w-0 flex-1 truncate text-sm tabular-nums text-muted-foreground">
                      {inv.used_count} / {inv.max_uses === 0 ? t('orgPage.invitations.unlimited') : inv.max_uses}
                    </dd>
                  </div>
                  <div className="flex items-baseline gap-1.5">
                    <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgPage.invitations.expiryTime')}</dt>
                    <dd className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
                      {inv.expired_time > 0 ? timestamp2string(inv.expired_time) : t('orgPage.myInvitations.never')}
                    </dd>
                  </div>
                </dl>
                <div className="mt-2 flex flex-wrap items-center justify-end gap-1 border-t border-border pt-2">
                  {inv.invitee_id === 0 && (
                    <Button variant="ghost" size="sm" onClick={() => copy(inviteLink(inv.token), t('orgPage.invitations.copyLink'))}>
                      <Link2 className="size-4" /> {t('orgPage.invitations.copyLink')}
                    </Button>
                  )}
                  <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive" onClick={() => revoke(inv)}>
                    <Ban className="size-4" /> {t('orgPage.invitations.revoke')}
                  </Button>
                </div>
              </div>
            ))}
          </div>
          {!invLoading && invitations.length === 0 && <EmptyState icon={Link2} title={t('orgPage.members.noInvitations')} />}
          <Pagination
            page={invPage}
            rowsPerPage={invRowsPerPage}
            count={invCount}
            options={PAGE_SIZE_OPTIONS}
            onPageChange={setInvPage}
            onRowsPerPageChange={(n) => {
              setInvRowsPerPage(n);
              setInvPage(0);
              savePageSize('org_invitations', n);
            }}
          />
        </>
      ) : (
        <>
          <div className="hidden md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('orgPage.members.user')}</TableHead>
                  <TableHead>{t('orgPage.members.role')}</TableHead>
                  {isAdmin && <TableHead>{t('orgPage.members.budget')}</TableHead>}
                  <TableHead>{t('orgPage.members.joinedAt')}</TableHead>
                  {isAdmin && <TableHead className="w-[1%] text-right">{t('orgPage.members.actions')}</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredMembers.map((m) => (
                  <TableRow key={m.id}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{m.display_name || m.username || `#${m.user_id}`}</span>
                        {m.username && m.display_name && <span className="text-xs text-muted-foreground">@{m.username}</span>}
                        {m.user_id === myUserId && (
                          <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
                            {t('orgPage.members.me')}
                          </Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      {canManage(m) && m.role !== 'owner' ? (
                        <Select value={m.role} onValueChange={(v) => changeRole(m, v)}>
                          <SelectTrigger className="h-8 w-32">
                            <span className="truncate">{t(`org.roles.${m.role}`)}</span>
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="admin">{t('org.roles.admin')}</SelectItem>
                            <SelectItem value="member">{t('org.roles.member')}</SelectItem>
                          </SelectContent>
                        </Select>
                      ) : (
                        <Badge variant={m.role === 'owner' ? 'default' : 'secondary'}>{t(`org.roles.${m.role}`)}</Badge>
                      )}
                    </TableCell>
                    {isAdmin && <TableCell>{renderBudget(m)}</TableCell>}
                    <TableCell className="text-muted-foreground">{timestamp2string(m.created_time)}</TableCell>
                    {isAdmin && (
                      <TableCell className="text-right">
                        {canManage(m) && (
                          <RowActions
                            actions={[
                              { label: t('orgPage.members.limits'), icon: SlidersHorizontal, onClick: () => setLimitsTarget(m) },
                              { label: t('orgPage.members.remove'), icon: UserMinus, destructive: true, onClick: () => setRemoveTarget(m) }
                            ]}
                          />
                        )}
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <div className="md:hidden flex flex-col gap-3 p-3">
            {filteredMembers.map((m) => (
              <div key={m.id} className="rounded-lg border border-border bg-card p-3 shadow-sm">
                <div className="mb-1.5 text-sm font-medium">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{m.display_name || m.username || `#${m.user_id}`}</span>
                    {m.username && m.display_name && <span className="text-xs text-muted-foreground">@{m.username}</span>}
                    {m.user_id === myUserId && (
                      <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
                        {t('orgPage.members.me')}
                      </Badge>
                    )}
                  </div>
                </div>
                <dl className="grid grid-cols-2 gap-x-4 gap-y-1.5">
                  <div className="flex items-center gap-1.5">
                    <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgPage.members.role')}</dt>
                    <dd className="flex min-w-0 flex-1 flex-wrap gap-1 text-sm">
                      {canManage(m) && m.role !== 'owner' ? (
                        <Select value={m.role} onValueChange={(v) => changeRole(m, v)}>
                          <SelectTrigger className="h-8 w-32">
                            <span className="truncate">{t(`org.roles.${m.role}`)}</span>
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="admin">{t('org.roles.admin')}</SelectItem>
                            <SelectItem value="member">{t('org.roles.member')}</SelectItem>
                          </SelectContent>
                        </Select>
                      ) : (
                        <Badge variant={m.role === 'owner' ? 'default' : 'secondary'}>{t(`org.roles.${m.role}`)}</Badge>
                      )}
                    </dd>
                  </div>
                  {isAdmin && (
                    <div className="col-span-2 flex items-baseline gap-1.5">
                      <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgPage.members.budget')}</dt>
                      <dd className="flex min-w-0 flex-1 flex-wrap gap-1 text-sm">{renderBudget(m)}</dd>
                    </div>
                  )}
                  <div className="flex items-baseline gap-1.5">
                    <dt className="shrink-0 text-xs font-medium text-muted-foreground">{t('orgPage.members.joinedAt')}</dt>
                    <dd className="min-w-0 flex-1 truncate text-sm text-muted-foreground">{timestamp2string(m.created_time)}</dd>
                  </div>
                </dl>
                {isAdmin && canManage(m) && (
                  <div className="mt-2 flex flex-wrap items-center justify-end gap-1 border-t border-border pt-2">
                    <RowActionsSurfaceContext.Provider value="card">
                      <RowActions
                        actions={[
                          { label: t('orgPage.members.limits'), icon: SlidersHorizontal, onClick: () => setLimitsTarget(m) },
                          { label: t('orgPage.members.remove'), icon: UserMinus, destructive: true, onClick: () => setRemoveTarget(m) }
                        ]}
                      />
                    </RowActionsSurfaceContext.Provider>
                  </div>
                )}
              </div>
            ))}
          </div>
          {!loading && filteredMembers.length === 0 && <EmptyState icon={Search} title={t('orgPage.members.noMembers')} />}
          <Pagination
            page={page}
            rowsPerPage={rowsPerPage}
            count={count}
            options={PAGE_SIZE_OPTIONS}
            onPageChange={setPage}
            onRowsPerPageChange={(n) => {
              setRowsPerPage(n);
              setPage(0);
              savePageSize('org_members', n);
            }}
          />
        </>
      )}

      <InvitationSheet
        open={sheetOpen}
        onOpenChange={setSheetOpen}
        orgId={orgId}
        onSaved={() => {
          setSheetOpen(false);
          setSeg('invitations');
          fetchInvitations();
        }}
      />

      <CreateMemberSheet
        open={createOpen}
        onOpenChange={setCreateOpen}
        orgId={orgId}
        onSaved={() => {
          setCreateOpen(false);
          setSeg('members');
          fetchData();
        }}
      />

      <MemberLimitsSheet
        open={!!limitsTarget}
        onOpenChange={(o) => !o && setLimitsTarget(null)}
        orgId={orgId}
        member={limitsTarget}
        onSaved={() => {
          setLimitsTarget(null);
          fetchData();
        }}
      />

      <Dialog open={!!removeTarget} onOpenChange={(o) => !o && setRemoveTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('orgPage.members.removeTitle')}</DialogTitle>
            <DialogDescription>
              {t('orgPage.members.removeConfirm', { name: removeTarget?.display_name || removeTarget?.username || removeTarget?.user_id })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoveTarget(null)} disabled={removing}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={confirmRemove} disabled={removing}>
              {t('orgPage.members.remove')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

MembersTab.propTypes = {
  orgId: PropTypes.number.isRequired,
  role: PropTypes.string
};
