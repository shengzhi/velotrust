import React, { useEffect, useMemo, useState } from 'react';
import { KeyRound, Plus, Power, RefreshCw, Search, UserRoundCog } from 'lucide-react';
import { iamApi, IAMUser } from '../../services/iam';
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../../components/ui/table';

const emptyForm = { username: '', display_name: '', email: '', password: '' };
const pageSize = 50;

export const EmployeeList: React.FC = () => {
  const { hasPermission } = useAuth();
  const { success, error } = useToast();
  const [users, setUsers] = useState<IAMUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [search, setSearch] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm, setCreateForm] = useState(emptyForm);
  const [saving, setSaving] = useState(false);
  const [statusTarget, setStatusTarget] = useState<IAMUser | null>(null);
  const [statusSaving, setStatusSaving] = useState(false);
  const [passwordTarget, setPasswordTarget] = useState<IAMUser | null>(null);
  const [newPassword, setNewPassword] = useState('');
  const [passwordSaving, setPasswordSaving] = useState(false);
  const [accessNotice, setAccessNotice] = useState<AccessChangeStatus | null>(null);

  const loadUsers = async (pageNumber = page) => {
    setLoading(true);
    try {
      const result = await iamApi.listUsers(pageNumber, pageSize, 'employee');
      setUsers(result.items ?? []);
      setTotal(result.total ?? 0);
      setPage(pageNumber);
    } catch (err) {
      error(err instanceof Error ? err.message : '加载员工用户失败。');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadUsers(1); }, []);

  const filteredUsers = useMemo(() => {
    const query = search.trim().toLocaleLowerCase();
    if (!query) return users;
    return users.filter((item) =>
      [item.username, item.display_name, item.email ?? ''].some((value) => value.toLocaleLowerCase().includes(query))
    );
  }, [users, search]);

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
      if (latest?.status === 'completed' || Date.now() - started >= 5000) return latest;
      await new Promise((resolve) => window.setTimeout(resolve, 500));
    } while (true);
  };

  const createEmployee = async (event: React.FormEvent) => {
    event.preventDefault();
    if (Array.from(createForm.password).length < 8) {
      error('Client 登录密码至少需要 8 个字符。');
      return;
    }
    setSaving(true);
    try {
      await iamApi.createUser({
        account_type: 'employee',
        username: createForm.username.trim(),
        display_name: createForm.display_name.trim(),
        email: createForm.email.trim(),
        password: createForm.password,
      });
      setCreateOpen(false);
      setCreateForm(emptyForm);
      success('员工账号已创建。员工可使用该账号密码登录 Client Agent，并在首次登录时注册设备。');
      await loadUsers(1);
    } catch (err) {
      error(err instanceof Error ? err.message : '创建员工用户失败。');
    } finally {
      setSaving(false);
    }
  };

  const resetPassword = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!passwordTarget) return;
    if (Array.from(newPassword).length < 8) {
      error('Client 登录密码至少需要 8 个字符。');
      return;
    }
    setPasswordSaving(true);
    try {
      await iamApi.resetUserPassword(passwordTarget.id, newPassword);
      success('员工 Client 登录密码已重置。');
      setPasswordTarget(null);
      setNewPassword('');
    } catch (err) {
      error(err instanceof Error ? err.message : '重置员工 Client 登录密码失败。');
    } finally {
      setPasswordSaving(false);
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
          ? `员工用户已禁用，${change.acknowledged_count}/${change.target_count} 个 Gateway 已确认更新。`
          : '员工用户已禁用，Gateway 配置正在同步。');
      } else {
        success('员工用户已启用，Gateway 将同步恢复授权配置。');
        void refreshAccessStatus(statusTarget.id);
      }
      setStatusTarget(null);
      await loadUsers();
    } catch (err) {
      error(err instanceof Error ? err.message : '修改员工用户状态失败。');
    } finally {
      setStatusSaving(false);
    }
  };

  const canWriteUsers = hasPermission('iam.user.write');

  return (
    <div className="space-y-5">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-slate-900">员工用户管理</h1>
          <p className="mt-1 text-xs text-slate-500">员工使用账号密码登录 Client Agent 并注册设备；员工账号不能登录 Management 控制台。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void loadUsers()} isLoading={loading} aria-label="刷新员工用户"><RefreshCw className="h-4 w-4" /></Button>
          {canWriteUsers && <Button onClick={() => setCreateOpen(true)} className="gap-1.5"><Plus className="h-4 w-4" />新建员工用户</Button>}
        </div>
      </div>

      <Card className="p-4">
        <Input placeholder="搜索本页用户名、姓名或邮箱" value={search} onChange={(event) => setSearch(event.target.value)} leftIcon={<Search className="h-4 w-4" />} />
      </Card>

      {accessNotice?.status === 'pending' && (
        <Card className="flex flex-col gap-3 border-amber-200 bg-amber-50/70 p-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-sm font-semibold text-amber-900">网络访问变更仍在同步</p>
            <p className="mt-1 text-xs text-amber-800">已确认 {accessNotice.acknowledged_count}/{accessNotice.target_count} 个 Gateway。离线 Gateway 会在重新连接并应用当前配置后确认。</p>
            {accessNotice.last_error && <p className="mt-2 text-xs text-red-700">最近一次配置失败：{accessNotice.last_error}</p>}
          </div>
          <Button variant="outline" size="sm" onClick={() => void refreshAccessStatus(accessNotice.subject_id)}><RefreshCw className="mr-1.5 h-3.5 w-3.5" />再次检查</Button>
        </Card>
      )}

      <Card>
        <Table>
          <TableHeader><TableRow>
            <TableHead>用户名 / 身份标识</TableHead><TableHead>姓名</TableHead><TableHead>邮箱</TableHead>
            <TableHead>状态</TableHead><TableHead>创建时间</TableHead><TableHead className="text-right">操作</TableHead>
          </TableRow></TableHeader>
          <TableBody>
            {loading ? (
              <TableRow><TableCell colSpan={6} className="py-10 text-center text-slate-500">正在加载员工用户…</TableCell></TableRow>
            ) : filteredUsers.length === 0 ? (
              <TableRow><TableCell colSpan={6} className="p-8"><EmptyState icon={UserRoundCog} title="没有匹配的员工用户" description="调整搜索词，或创建一个员工用户。" /></TableCell></TableRow>
            ) : filteredUsers.map((item) => (
              <TableRow key={item.id}>
                <TableCell className="font-mono text-xs font-semibold">{item.username}</TableCell>
                <TableCell>{item.display_name || '—'}</TableCell>
                <TableCell className="text-xs">{item.email || '—'}</TableCell>
                <TableCell><Badge variant={item.user_status === 'active' ? 'success' : 'outline'}>{item.user_status === 'active' ? '启用' : '已禁用'}</Badge></TableCell>
                <TableCell className="text-xs text-slate-500">{new Date(item.created_at).toLocaleDateString()}</TableCell>
                <TableCell className="text-right">
                  {canWriteUsers && <div className="flex justify-end gap-1">
                    <Button variant="ghost" size="sm" title="重置 Client 登录密码" onClick={() => { setPasswordTarget(item); setNewPassword(''); }}><KeyRound className="h-4 w-4" /></Button>
                    <Button variant={item.user_status === 'active' ? 'danger' : 'outline'} size="sm" onClick={() => setStatusTarget(item)}><Power className="h-3.5 w-3.5" />{item.user_status === 'active' ? '禁用' : '启用'}</Button>
                  </div>}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <div className="flex flex-col justify-between gap-3 border-t border-slate-100 px-4 py-3 text-xs text-slate-500 sm:flex-row sm:items-center">
          <span>共 {total} 个员工用户 · 第 {page} / {Math.max(1, Math.ceil(total / pageSize))} 页</span>
          <div className="flex gap-2"><Button variant="outline" size="sm" disabled={loading || page <= 1} onClick={() => void loadUsers(page - 1)}>上一页</Button><Button variant="outline" size="sm" disabled={loading || page >= Math.ceil(total / pageSize)} onClick={() => void loadUsers(page + 1)}>下一页</Button></div>
        </div>
      </Card>

      <Dialog isOpen={createOpen} onClose={() => setCreateOpen(false)} title="新建员工账号" description="员工可使用此账号密码登录 Client Agent；该账号不能登录 Management。">
        <form className="space-y-4" onSubmit={createEmployee}>
          <label className="block space-y-1 text-xs font-medium text-slate-700">用户名<Input required maxLength={64} value={createForm.username} onChange={(event) => setCreateForm({ ...createForm, username: event.target.value })} autoComplete="off" /></label>
          <label className="block space-y-1 text-xs font-medium text-slate-700">姓名<Input maxLength={128} value={createForm.display_name} onChange={(event) => setCreateForm({ ...createForm, display_name: event.target.value })} /></label>
          <label className="block space-y-1 text-xs font-medium text-slate-700">邮箱（可选）<Input type="email" maxLength={128} value={createForm.email} onChange={(event) => setCreateForm({ ...createForm, email: event.target.value })} /></label>
          <label className="block space-y-1 text-xs font-medium text-slate-700">Client 登录密码<Input type="password" minLength={8} maxLength={1024} required value={createForm.password} onChange={(event) => setCreateForm({ ...createForm, password: event.target.value })} autoComplete="new-password" /></label>
          <p className="text-[11px] text-slate-500">密码至少 8 个字符。创建后请通过安全渠道交给员工；员工首次连接 HUB 时会用它登录并登记本机设备。</p>
          <div className="flex justify-end gap-2 border-t border-slate-100 pt-4">
            <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>取消</Button>
            <Button type="submit" isLoading={saving}>创建员工账号</Button>
          </div>
        </form>
      </Dialog>

      <Dialog isOpen={!!passwordTarget} onClose={() => setPasswordTarget(null)} title={`重置 Client 密码 · ${passwordTarget?.username ?? ''}`} description="员工下次登录 Client Agent 时使用新密码。">
        <form className="space-y-4" onSubmit={resetPassword}>
          <label className="block space-y-1 text-xs font-medium text-slate-700">新密码<Input type="password" minLength={8} maxLength={1024} required value={newPassword} onChange={(event) => setNewPassword(event.target.value)} autoComplete="new-password" /></label>
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
        title={statusTarget?.user_status === 'active' ? '确认禁用员工用户？' : '确认启用员工用户？'}
        description={statusTarget?.user_status === 'active' ? '禁用后，该员工关联的 Client 访问授权将从 Gateway 配置中撤销。' : '启用后，Gateway 将根据当前策略恢复该员工关联设备的授权。'}
        confirmText={statusTarget?.user_status === 'active' ? '禁用员工用户' : '启用员工用户'}
        isLoading={statusSaving}
      />
    </div>
  );
};
