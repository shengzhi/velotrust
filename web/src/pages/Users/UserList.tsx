import React, { useEffect, useMemo, useState } from 'react';
import { KeyRound, Plus, Power, RefreshCw, Search, UserRoundCog } from 'lucide-react';
import { iamApi, IAMRole, IAMUser } from '../../services/iam';
import { useAuth } from '../../contexts/AuthContext';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Badge } from '../../components/ui/badge';
import { Card } from '../../components/ui/card';
import { Dialog } from '../../components/ui/dialog';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { EmptyState } from '../../components/EmptyState';
import { useToast } from '../../components/ui/toast';
import { AccessChangeStatus, virtualNodeApi } from '../../services/node';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../../components/ui/table';

const emptyForm = { username: '', display_name: '', email: '', password: '' };

export const UserList: React.FC = () => {
  const { user: currentUser, hasPermission } = useAuth();
  const { success, error } = useToast();
  const [users, setUsers] = useState<IAMUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const pageSize = 50;
  const [search, setSearch] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm, setCreateForm] = useState(emptyForm);
  const [saving, setSaving] = useState(false);
  const [statusTarget, setStatusTarget] = useState<IAMUser | null>(null);
  const [statusSaving, setStatusSaving] = useState(false);
  const [roleTarget, setRoleTarget] = useState<IAMUser | null>(null);
  const [roles, setRoles] = useState<IAMRole[]>([]);
  const [userRoleIds, setUserRoleIds] = useState<number[]>([]);
  const [rolesLoading, setRolesLoading] = useState(false);
  const [roleSaving, setRoleSaving] = useState<number | null>(null);
  const [passwordTarget, setPasswordTarget] = useState<IAMUser | null>(null);
  const [newPassword, setNewPassword] = useState('');
  const [passwordSaving, setPasswordSaving] = useState(false);
  const [accessNotice, setAccessNotice] = useState<AccessChangeStatus | null>(null);

  const refreshAccessStatus = async (userID: number) => {
    const started = Date.now();
    let latest: AccessChangeStatus | null = null;
    do {
      try {
        latest = await virtualNodeApi.getAccessChangeStatus('user', userID);
      } catch {
        return latest;
      }
      if (latest) setAccessNotice(latest);
      if (latest?.status === 'completed') return latest;
      if (Date.now() - started >= 5000) break;
      await new Promise((resolve) => window.setTimeout(resolve, 500));
    } while (true);
    return latest;
  };

  const loadUsers = async (pageNumber = page) => {
    setLoading(true);
    try {
      const result = await iamApi.listUsers(pageNumber, pageSize, 'platform');
      setUsers(result.items ?? []);
      setTotal(result.total ?? 0);
      setPage(pageNumber);
    } catch (err) {
      error(err instanceof Error ? err.message : '加载用户失败。');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadUsers();
  }, []);

  const filteredUsers = useMemo(() => {
    const query = search.trim().toLocaleLowerCase();
    if (!query) return users;
    return users.filter((item) =>
      [item.username, item.display_name, item.email ?? ''].some((value) => value.toLocaleLowerCase().includes(query))
    );
  }, [users, search]);

  const openRoleManager = async (target: IAMUser) => {
    setRoleTarget(target);
    setRolesLoading(true);
    try {
      const [allRoles, assignedRoles] = await Promise.all([
        iamApi.listRoles(),
        iamApi.listUserRoles(target.id),
      ]);
      setRoles(allRoles);
      setUserRoleIds(assignedRoles.map((role) => role.id));
    } catch (err) {
      setRoleTarget(null);
      error(err instanceof Error ? err.message : '加载账号角色失败。');
    } finally {
      setRolesLoading(false);
    }
  };

  const toggleRole = async (role: IAMRole) => {
    if (!roleTarget) return;
    setRoleSaving(role.id);
    const assigned = userRoleIds.includes(role.id);
    try {
      if (assigned) await iamApi.revokeRole(roleTarget.id, role.id);
      else await iamApi.grantRole(roleTarget.id, role.id);
      const next = assigned ? userRoleIds.filter((id) => id !== role.id) : [...userRoleIds, role.id];
      setUserRoleIds(next);
      success(assigned ? `已撤销角色「${role.role_name}」` : `已授予角色「${role.role_name}」`);
    } catch (err) {
      error(err instanceof Error ? err.message : '更新角色失败。');
    } finally {
      setRoleSaving(null);
    }
  };

  const createUser = async (event: React.FormEvent) => {
    event.preventDefault();
    if (Array.from(createForm.password).length < 8) {
      error('初始密码至少需要 8 个字符。');
      return;
    }
    setSaving(true);
    try {
      const created = await iamApi.createUser({
        account_type: 'platform',
        username: createForm.username.trim(),
        display_name: createForm.display_name.trim(),
        email: createForm.email.trim(),
        password: createForm.password,
      });
      setCreateOpen(false);
      setCreateForm(emptyForm);
      success('平台账号已创建，首次登录必须修改密码。');
      await loadUsers(1);
      if (hasPermission('iam.role.grant')) await openRoleManager(created);
    } catch (err) {
      error(err instanceof Error ? err.message : '创建账号失败。');
    } finally {
      setSaving(false);
    }
  };

  const changeStatus = async () => {
    if (!statusTarget) return;
    setStatusSaving(true);
    setAccessNotice(null);
    const status = statusTarget.user_status === 'active' ? 'disabled' : 'active';
    try {
      await iamApi.setUserStatus(statusTarget.id, status);
      if (status === 'disabled') {
        const change = await refreshAccessStatus(statusTarget.id);
        success(change?.status === 'completed'
          ? `账号已禁用，${change.acknowledged_count}/${change.target_count} 个 Gateway 已确认更新。`
          : '账号已禁用，Gateway 配置正在同步。');
      } else {
        success('账号已启用，Gateway 将同步恢复授权配置。');
        void refreshAccessStatus(statusTarget.id);
      }
      setStatusTarget(null);
      await loadUsers();
    } catch (err) {
      error(err instanceof Error ? err.message : '修改账号状态失败。');
    } finally {
      setStatusSaving(false);
    }
  };

  const resetPassword = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!passwordTarget) return;
    if (Array.from(newPassword).length < 8) {
      error('临时密码至少需要 8 个字符。');
      return;
    }
    setPasswordSaving(true);
    try {
      await iamApi.resetUserPassword(passwordTarget.id, newPassword);
      success('密码已重置；该账号的会话已撤销，首次登录必须修改密码。');
      setPasswordTarget(null);
      setNewPassword('');
    } catch (err) {
      error(err instanceof Error ? err.message : '重置密码失败。');
    } finally {
      setPasswordSaving(false);
    }
  };

  const canWriteUsers = hasPermission('iam.user.write');
  const canGrantRoles = hasPermission('iam.role.grant') && hasPermission('iam.role.read');

  return (
    <div className="space-y-5">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-slate-900">系统用户管理</h1>
          <p className="mt-1 text-xs text-slate-500">管理可登录 Management 控制台的系统账号及其角色。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void loadUsers()} isLoading={loading} aria-label="刷新用户">
            <RefreshCw className="h-4 w-4" />
          </Button>
          {canWriteUsers && (
            <Button onClick={() => setCreateOpen(true)} className="gap-1.5">
              <Plus className="h-4 w-4" />
              新建系统用户
            </Button>
          )}
        </div>
      </div>

      <Card className="p-4">
        <Input
          placeholder="搜索本页用户名、姓名或邮箱"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          leftIcon={<Search className="h-4 w-4" />}
        />
      </Card>

      {accessNotice?.status === 'pending' && (
        <Card className="flex flex-col gap-3 border-amber-200 bg-amber-50/70 p-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-sm font-semibold text-amber-900">网络访问变更仍在同步</p>
            <p className="mt-1 text-xs text-amber-800">
              已确认 {accessNotice.acknowledged_count}/{accessNotice.target_count} 个 Gateway。离线 Gateway 会在重新连接并应用当前配置后确认。
            </p>
            {accessNotice.last_error && <p className="mt-2 text-xs text-red-700">最近一次配置失败：{accessNotice.last_error}</p>}
          </div>
          <Button variant="outline" size="sm" onClick={() => void refreshAccessStatus(accessNotice.subject_id)}>
            <RefreshCw className="mr-1.5 h-3.5 w-3.5" />再次检查
          </Button>
        </Card>
      )}

      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>用户名 / 身份标识</TableHead>
              <TableHead>姓名</TableHead>
              <TableHead>邮箱</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>密码状态</TableHead>
              <TableHead>最近登录</TableHead>
              <TableHead>创建时间</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow><TableCell colSpan={8} className="py-10 text-center text-slate-500">正在从 Management 加载系统用户…</TableCell></TableRow>
            ) : filteredUsers.length === 0 ? (
              <TableRow><TableCell colSpan={8} className="p-8"><EmptyState icon={UserRoundCog} title="没有匹配的系统用户" description="调整搜索词，或创建一个系统用户。" /></TableCell></TableRow>
            ) : filteredUsers.map((item) => {
              const isSelf = item.id === currentUser?.id;
              return (
                <TableRow key={item.id}>
                  <TableCell className="font-mono text-xs font-semibold">{item.username}</TableCell>
                  <TableCell>{item.display_name || '—'}</TableCell>
                  <TableCell className="text-xs">{item.email || '—'}</TableCell>
                  <TableCell><Badge variant={item.user_status === 'active' ? 'success' : 'outline'}>{item.user_status}</Badge></TableCell>
                  <TableCell>{item.must_change_password ? <Badge variant="warning">须修改</Badge> : <span className="text-xs text-slate-500">正常</span>}</TableCell>
                  <TableCell className="text-xs text-slate-500">{item.last_login_at ? new Date(item.last_login_at).toLocaleString() : '从未登录'}</TableCell>
                  <TableCell className="text-xs text-slate-500">{new Date(item.created_at).toLocaleDateString()}</TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      {canGrantRoles && (
                        <Button variant="ghost" size="sm" title="管理角色" disabled={isSelf} onClick={() => void openRoleManager(item)}>
                          <KeyRound className="h-4 w-4" />
                        </Button>
                      )}
                      {canWriteUsers && !isSelf && (
                        <>
                          <Button variant="ghost" size="sm" title="重置密码" onClick={() => { setPasswordTarget(item); setNewPassword(''); }}>
                            <KeyRound className="h-4 w-4 text-indigo-600" />
                          </Button>
                          <Button variant="ghost" size="sm" title={item.user_status === 'active' ? '禁用账号' : '启用账号'} onClick={() => setStatusTarget(item)}>
                            <Power className="h-4 w-4" />
                          </Button>
                        </>
                      )}
                      {isSelf && <span className="px-2 text-[11px] text-slate-400">当前账号</span>}
                    </div>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
        <div className="flex flex-col justify-between gap-3 border-t border-slate-100 px-4 py-3 text-xs text-slate-500 sm:flex-row sm:items-center">
          <span>共 {total} 个账号 · 第 {page} / {Math.max(1, Math.ceil(total / pageSize))} 页</span>
          <div className="flex gap-2"><Button variant="outline" size="sm" disabled={loading || page <= 1} onClick={() => void loadUsers(page - 1)}>上一页</Button><Button variant="outline" size="sm" disabled={loading || page >= Math.ceil(total / pageSize)} onClick={() => void loadUsers(page + 1)}>下一页</Button></div>
        </div>
      </Card>

      <Dialog isOpen={createOpen} onClose={() => setCreateOpen(false)} title="新建系统用户" description="系统用户可登录 Management 控制台；创建后可分配管理角色。">
        <form className="space-y-4" onSubmit={createUser}>
          <label className="block space-y-1 text-xs font-medium text-slate-700">用户名<Input required maxLength={64} value={createForm.username} onChange={(event) => setCreateForm({ ...createForm, username: event.target.value })} autoComplete="off" /></label>
          <label className="block space-y-1 text-xs font-medium text-slate-700">显示名称<Input maxLength={128} value={createForm.display_name} onChange={(event) => setCreateForm({ ...createForm, display_name: event.target.value })} /></label>
          <label className="block space-y-1 text-xs font-medium text-slate-700">邮箱（可选）<Input type="email" maxLength={128} value={createForm.email} onChange={(event) => setCreateForm({ ...createForm, email: event.target.value })} /></label>
          <label className="block space-y-1 text-xs font-medium text-slate-700">初始密码<Input type="password" minLength={8} maxLength={1024} required value={createForm.password} onChange={(event) => setCreateForm({ ...createForm, password: event.target.value })} autoComplete="new-password" /></label>
          <p className="text-[11px] text-slate-500">密码至少 8 个字符。创建完成后，可将系统用户分配到一个或多个角色。</p>
          <div className="flex justify-end gap-2 border-t border-slate-100 pt-4">
            <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>取消</Button>
            <Button type="submit" isLoading={saving}>创建系统用户</Button>
          </div>
        </form>
      </Dialog>

      <Dialog isOpen={!!roleTarget} onClose={() => setRoleTarget(null)} title={`角色分配 · ${roleTarget?.username ?? ''}`} description="变更立即提交到 Management；当前账号不能给自己授权。">
        {rolesLoading ? <p className="py-8 text-center text-sm text-slate-500">正在加载角色…</p> : roles.length === 0 ? <p className="py-8 text-center text-sm text-slate-500">尚无可用角色，请先创建角色。</p> : (
          <div className="space-y-2">
            {roles.map((role) => {
              const assigned = userRoleIds.includes(role.id);
              return (
                <div key={role.id} className="flex items-center justify-between rounded-lg border border-slate-200 p-3">
                  <div>
                    <div className="text-sm font-medium text-slate-800">{role.role_name}</div>
                    <div className="mt-0.5 font-mono text-[11px] text-slate-500">{role.role_code}</div>
                  </div>
                  <Button size="sm" variant={assigned ? 'outline' : 'primary'} isLoading={roleSaving === role.id} disabled={roleSaving !== null} onClick={() => void toggleRole(role)}>
                    {assigned ? '撤销角色' : '授予角色'}
                  </Button>
                </div>
              );
            })}
          </div>
        )}
      </Dialog>

      <Dialog isOpen={!!passwordTarget} onClose={() => setPasswordTarget(null)} title={`重置密码 · ${passwordTarget?.username ?? ''}`} description="重置会撤销该账号所有活动会话，并要求其下次登录修改密码。">
        <form className="space-y-4" onSubmit={resetPassword}>
          <label className="block space-y-1 text-xs font-medium text-slate-700">新临时密码<Input type="password" minLength={8} maxLength={1024} required value={newPassword} onChange={(event) => setNewPassword(event.target.value)} autoComplete="new-password" /></label>
          <div className="flex justify-end gap-2 border-t border-slate-100 pt-4">
            <Button type="button" variant="outline" onClick={() => setPasswordTarget(null)}>取消</Button>
            <Button type="submit" isLoading={passwordSaving}>重置密码</Button>
          </div>
        </form>
      </Dialog>

      <ConfirmDialog
        isOpen={!!statusTarget}
        onClose={() => setStatusTarget(null)}
        onConfirm={changeStatus}
        title={statusTarget?.user_status === 'active' ? '确认禁用账号？' : '确认启用账号？'}
        description={statusTarget?.user_status === 'active' ? '禁用会撤销此用户的所有活动会话。不能禁用最后一个有效平台管理员。' : '启用后，用户可以重新登录。'}
        confirmText={statusTarget?.user_status === 'active' ? '禁用账号' : '启用账号'}
        isLoading={statusSaving}
      />
    </div>
  );
};
