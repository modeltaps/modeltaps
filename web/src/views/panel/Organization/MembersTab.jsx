import { useCallback, useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import {
  Ban,
  Link2,
  MoreHorizontal,
  Plus,
  RefreshCw,
  Search,
  ShieldUser,
  SlidersHorizontal,
  User,
  UserMinus,
  UserPlus,
  Users
} from 'lucide-react';

import { cn } from '@/lib/utils';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Select, SelectTrigger, SelectContent, SelectItem } from '@/components/ui/select';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { ListEmptyState, ListFooter, ListSkeleton } from '@/components/list-page';
import { API } from 'utils/api';
import { copy, renderSpend, showError, showSuccess, timestamp2string } from 'utils/common';
import { relativeTime } from 'utils/relativeTime';
import { getPageSize, savePageSize } from 'constants';
import { createRequestGuard, runGuardedFetch } from 'hooks/paginatedListGuard';
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

// A row's ⋯ menu.
function MoreMenu({ label, children }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="ghost" size="icon" className="size-8" aria-label={label} title={label}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        {children}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

MoreMenu.propTypes = { label: PropTypes.string.isRequired, children: PropTypes.node };

export default function MembersTab({ orgId, role }) {
  const { t, i18n } = useTranslation();
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
  const [loaded, setLoaded] = useState(false);
  const [removeTarget, setRemoveTarget] = useState(null);
  const [removing, setRemoving] = useState(false);
  const [limitsTarget, setLimitsTarget] = useState(null);

  const [invitations, setInvitations] = useState([]);
  const [invCount, setInvCount] = useState(0);
  const [invPage, setInvPage] = useState(0);
  const [invRowsPerPage, setInvRowsPerPage] = useState(() => getPageSize('org_invitations'));
  const [invLoading, setInvLoading] = useState(false);
  const [invLoaded, setInvLoaded] = useState(false);
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
          onFinally: () => {
            setLoading(false);
            setLoaded(true);
          }
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
        onFinally: () => {
          setInvLoading(false);
          setInvLoaded(true);
        }
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

  const rtf = new Intl.RelativeTimeFormat((i18n.language || 'zh_CN').replace('_', '-'), { numeric: 'auto' });
  const when = (ts) => (
    <span title={timestamp2string(ts)} className="whitespace-nowrap">
      {relativeTime(rtf, ts)}
    </span>
  );
  const memberName = (m) => m.display_name || m.username || `#${m.user_id}`;
  const meTag = (m) =>
    m.user_id === myUserId && <span className="shrink-0 text-xs text-muted-foreground">({t('orgPage.members.me')})</span>;
  const roleText = (r) => <span className="whitespace-nowrap text-muted-foreground">{t(`org.roles.${r}`)}</span>;
  const usage = (inv) => `${inv.used_count} / ${inv.max_uses === 0 ? t('orgPage.invitations.unlimited') : inv.max_uses}`;
  const expiry = (inv) => (inv.expired_time > 0 ? when(inv.expired_time) : t('orgPage.myInvitations.never'));
  const invitee = (inv) =>
    inv.invitee_id > 0 ? (
      <span className="min-w-0 truncate font-medium">{inv.email || `#${inv.invitee_id}`}</span>
    ) : (
      <span className="inline-flex min-w-0 items-center gap-1.5 font-medium">
        <Link2 className="size-3.5 shrink-0 text-muted-foreground" />
        <span className="truncate">{t('orgPage.invitations.typeLink')}</span>
      </span>
    );

  const memberMenu = (m, withRole) =>
    canManage(m) && (
      <MoreMenu label={t('orgList.more')}>
        <DropdownMenuItem onClick={() => setLimitsTarget(m)}>
          <SlidersHorizontal className="size-4" />
          {t('orgPage.members.limits')}
        </DropdownMenuItem>
        {withRole && m.role !== 'owner' && (
          <DropdownMenuItem onClick={() => changeRole(m, m.role === 'admin' ? 'member' : 'admin')}>
            {m.role === 'admin' ? <User className="size-4" /> : <ShieldUser className="size-4" />}
            {t(m.role === 'admin' ? 'orgList.makeMember' : 'orgList.makeAdmin')}
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => setRemoveTarget(m)}>
          <UserMinus className="size-4" />
          {t('orgPage.members.remove')}
        </DropdownMenuItem>
      </MoreMenu>
    );

  const invitationMenu = (inv) => (
    <MoreMenu label={t('orgList.more')}>
      {inv.invitee_id === 0 && (
        <DropdownMenuItem onClick={() => copy(inviteLink(inv.token), t('orgPage.invitations.copyLink'))}>
          <Link2 className="size-4" />
          {t('orgPage.invitations.copyLink')}
        </DropdownMenuItem>
      )}
      <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => revoke(inv)}>
        <Ban className="size-4" />
        {t('orgPage.invitations.revoke')}
      </DropdownMenuItem>
    </MoreMenu>
  );

  const skeleton = (
    <>
      <div className="hidden md:block">
        <ListSkeleton cells={[80, 128, 64, 32]} checkbox={false} />
      </div>
      <div className="md:hidden">
        <ListSkeleton layout="list" />
      </div>
    </>
  );

  const footer = (total, current, size, onPage, onSize) => (
    <div className="border-t border-border">
      <ListFooter total={total} page={current} pageSize={size} onPageChange={onPage} onPageSizeChange={onSize} />
    </div>
  );

  return (
    <Card className="overflow-hidden">
      <div className="flex flex-col gap-2 border-b border-border p-3 sm:flex-row sm:items-center">
        {/* 子分段「成员(N) / 邀请(M)」:Member 角色只见成员列表,不渲染分段控件 */}
        {isAdmin && (
          <Tabs value={seg} onValueChange={setSeg}>
            <TabsList>
              <TabsTrigger value="members">
                {t('orgPage.tabs.members')} <span className="ml-1 tabular-nums text-muted-foreground">{count}</span>
              </TabsTrigger>
              <TabsTrigger value="invitations">
                {t('orgPage.members.pendingInvitations')} <span className="ml-1 tabular-nums text-muted-foreground">{invCount}</span>
              </TabsTrigger>
            </TabsList>
          </Tabs>
        )}
        {/* On phones the buttons drop their labels so the row fits beside the search box. */}
        <div className="flex min-w-0 items-center gap-2 sm:ml-auto">
          {!showInvitations && (
            <div className="relative min-w-0 flex-1 sm:w-56 sm:flex-none">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={t('orgPage.members.search')}
                aria-label={t('orgPage.members.search')}
                className="h-8 pl-8"
              />
            </div>
          )}
          <Button
            variant="outline"
            size="icon"
            className={cn('size-8 shrink-0', showInvitations && 'ml-auto sm:ml-0')}
            aria-label={t('orgPage.refresh')}
            title={t('orgPage.refresh')}
            onClick={showInvitations ? fetchInvitations : fetchData}
            disabled={showInvitations ? invLoading : loading}
          >
            <RefreshCw className="size-4" />
          </Button>
          {isAdmin && (
            <Button
              variant="outline"
              size="sm"
              className="h-8 shrink-0 px-2 sm:px-3"
              aria-label={t('orgPage.members.createAccount')}
              title={t('orgPage.members.createAccount')}
              onClick={() => setCreateOpen(true)}
            >
              <UserPlus className="size-4" />
              <span className="hidden sm:inline">{t('orgPage.members.createAccount')}</span>
            </Button>
          )}
          {isAdmin && (
            <Button
              size="sm"
              className="h-8 shrink-0 px-2 sm:px-3"
              aria-label={t('orgPage.members.invite')}
              title={t('orgPage.members.invite')}
              onClick={() => setSheetOpen(true)}
            >
              <Plus className="size-4" />
              <span className="hidden sm:inline">{t('orgPage.members.invite')}</span>
            </Button>
          )}
        </div>
      </div>

      {showInvitations ? (
        <>
          {!invLoaded ? (
            skeleton
          ) : invitations.length === 0 ? (
            <ListEmptyState icon={Link2} title={t('orgPage.members.noInvitations')} />
          ) : (
            <>
              <div className="hidden md:block">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('orgPage.invitations.invitee')}</TableHead>
                      <TableHead>{t('orgPage.members.role')}</TableHead>
                      <TableHead>{t('orgPage.invitations.usage')}</TableHead>
                      <TableHead>{t('orgPage.invitations.expiryTime')}</TableHead>
                      <TableHead className="w-12" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {invitations.map((inv) => (
                      <TableRow key={inv.id}>
                        <TableCell className="max-w-64">
                          <div className="flex min-w-0">{invitee(inv)}</div>
                        </TableCell>
                        <TableCell>{roleText(inv.role)}</TableCell>
                        <TableCell className="tabular-nums text-muted-foreground">{usage(inv)}</TableCell>
                        <TableCell className="text-muted-foreground">{expiry(inv)}</TableCell>
                        <TableCell className="py-0 text-right">{invitationMenu(inv)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <ul className="divide-y divide-border md:hidden">
                {invitations.map((inv) => (
                  <li key={inv.id} className="flex h-14 items-center gap-3 px-4">
                    <div className="min-w-0 flex-1">
                      <div className="flex min-w-0 items-baseline gap-3 text-sm">
                        <span className="flex min-w-0">{invitee(inv)}</span>
                        <span className="ml-auto shrink-0 text-xs">{roleText(inv.role)}</span>
                      </div>
                      <div className="mt-0.5 flex items-baseline gap-3 text-xs text-muted-foreground">
                        <span className="tabular-nums">{usage(inv)}</span>
                        <span className="ml-auto shrink-0">{expiry(inv)}</span>
                      </div>
                    </div>
                    <div className="-mr-2 shrink-0">{invitationMenu(inv)}</div>
                  </li>
                ))}
              </ul>
            </>
          )}
          {footer(invCount, invPage, invRowsPerPage, setInvPage, (n) => {
            setInvRowsPerPage(n);
            setInvPage(0);
            savePageSize('org_invitations', n);
          })}
        </>
      ) : (
        <>
          {!loaded ? (
            skeleton
          ) : filteredMembers.length === 0 ? (
            <ListEmptyState
              variant={q ? 'noMatch' : 'empty'}
              icon={Users}
              title={q ? undefined : t('orgPage.members.noMembers')}
              onClearFilters={q ? () => setSearch('') : undefined}
            />
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
                      {isAdmin && <TableHead className="w-12" />}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredMembers.map((m) => (
                      <TableRow key={m.id}>
                        <TableCell className="max-w-72">
                          <div className="flex min-w-0 items-baseline gap-2">
                            <span className="truncate font-medium">{memberName(m)}</span>
                            {m.username && m.display_name && <span className="truncate text-xs text-muted-foreground">@{m.username}</span>}
                            {meTag(m)}
                          </div>
                        </TableCell>
                        <TableCell className="py-0">
                          {canManage(m) && m.role !== 'owner' ? (
                            <Select value={m.role} onValueChange={(v) => changeRole(m, v)}>
                              <SelectTrigger className="h-8 w-28" aria-label={t('orgPage.members.role')}>
                                <span className="truncate">{t(`org.roles.${m.role}`)}</span>
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="admin">{t('org.roles.admin')}</SelectItem>
                                <SelectItem value="member">{t('org.roles.member')}</SelectItem>
                              </SelectContent>
                            </Select>
                          ) : (
                            // Lines up with the text inside the role selects above and below.
                            <span className="pl-3">{roleText(m.role)}</span>
                          )}
                        </TableCell>
                        {isAdmin && <TableCell className="whitespace-nowrap">{renderBudget(m)}</TableCell>}
                        <TableCell className="text-muted-foreground">{when(m.created_time)}</TableCell>
                        {isAdmin && <TableCell className="py-0 text-right">{memberMenu(m, false)}</TableCell>}
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              {/* Phone rows: name and role on top, @username · joined and the budget below; the
                  role change moves into the ⋯ menu. */}
              <ul className="divide-y divide-border md:hidden">
                {filteredMembers.map((m) => (
                  <li key={m.id} className="flex h-14 items-center gap-3 px-4">
                    <div className="min-w-0 flex-1">
                      <div className="flex min-w-0 items-baseline gap-2 text-sm">
                        <span className="min-w-0 truncate font-medium">{memberName(m)}</span>
                        {meTag(m)}
                        <span className="ml-auto shrink-0 pl-1 text-xs">{roleText(m.role)}</span>
                      </div>
                      <div className="mt-0.5 flex items-baseline gap-3 text-xs text-muted-foreground">
                        <span className="min-w-0 truncate">
                          {m.username && m.display_name && `@${m.username} · `}
                          {when(m.created_time)}
                        </span>
                        {isAdmin && <span className="ml-auto min-w-0 shrink truncate text-right">{renderBudget(m)}</span>}
                      </div>
                    </div>
                    {isAdmin && <div className="-mr-2 flex size-8 shrink-0 items-center justify-center">{memberMenu(m, true)}</div>}
                  </li>
                ))}
              </ul>
            </>
          )}
          {footer(count, page, rowsPerPage, setPage, (n) => {
            setRowsPerPage(n);
            setPage(0);
            savePageSize('org_members', n);
          })}
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
