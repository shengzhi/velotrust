import React, { useEffect, useMemo, useState } from 'react';
import { Pencil, Plus, RefreshCw, Search, UsersRound, UserRoundPlus } from 'lucide-react';
import { IAMGroup, IAMGroupMember, iamApi } from '../../services/iam';
import { useAuth } from '../../contexts/AuthContext';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Select } from '../../components/ui/select';
import { Badge } from '../../components/ui/badge';
import { Card } from '../../components/ui/card';
import { Dialog } from '../../components/ui/dialog';
import { EmptyState } from '../../components/EmptyState';
import { useToast } from '../../components/ui/toast';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../../components/ui/table';

type GroupForm = {
  group_name: string;
  group_code: string;
  description: string;
  status: 'active' | 'disabled';
  sort_order: number;
};

const blankForm: GroupForm = { group_name: '', group_code: '', description: '', status: 'active', sort_order: 0 };
const employeeStatusLabels: Record<IAMGroupMember['user_status'], string> = {
  active: '启用',
  disabled: '停用',
  locked: '锁定',
  pending: '待激活',
};

export const GroupList: React.FC = () => {
  const { hasPermission } = useAuth();
  const { success, error } = useToast();
  const [groups, setGroups] = useState<IAMGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<IAMGroup | null>(null);
  const [form, setForm] = useState<GroupForm>(blankForm);
  const [saving, setSaving] = useState(false);
  const [memberGroup, setMemberGroup] = useState<IAMGroup | null>(null);
  const [members, setMembers] = useState<IAMGroupMember[]>([]);
  const [memberSearch, setMemberSearch] = useState('');
  const [membersLoading, setMembersLoading] = useState(false);
  const [membersSaving, setMembersSaving] = useState(false);

  const canWrite = hasPermission('iam.group.write');

  const loadGroups = async () => {
    setLoading(true);
    try {
      setGroups(await iamApi.listGroups());
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '加载用户组失败。');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadGroups(); }, []);

  const filteredGroups = useMemo(() => {
    const query = search.trim().toLocaleLowerCase();
    if (!query) return groups;
    return groups.filter((group) => [group.group_name, group.group_code, group.description ?? '']
      .some((value) => value.toLocaleLowerCase().includes(query)));
  }, [groups, search]);

  const filteredMembers = useMemo(() => {
    const query = memberSearch.trim().toLocaleLowerCase();
    if (!query) return members;
    return members.filter((member) => [member.username, member.display_name, member.email ?? '']
      .some((value) => value.toLocaleLowerCase().includes(query)));
  }, [members, memberSearch]);

  const openCreate = () => {
    setEditing(null);
    setForm(blankForm);
    setEditorOpen(true);
  };

  const openEdit = (group: IAMGroup) => {
    setEditing(group);
    setForm({
      group_name: group.group_name,
      group_code: group.group_code,
      description: group.description ?? '',
      status: group.status,
      sort_order: group.sort_order,
    });
    setEditorOpen(true);
  };

  const saveGroup = async (event: React.FormEvent) => {
    event.preventDefault();
    setSaving(true);
    try {
      await iamApi.saveGroup({
        ...(editing ? { id: editing.id, expected_version: editing.version_no } : {}),
        ...form,
        group_name: form.group_name.trim(),
        group_code: form.group_code.trim().toLowerCase(),
        description: form.description.trim(),
      });
      setEditorOpen(false);
      success(editing ? '用户组已更新。' : '用户组已创建。');
      await loadGroups();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '保存用户组失败。');
    } finally {
      setSaving(false);
    }
  };

  const openMembers = async (group: IAMGroup) => {
    setMemberGroup(group);
    setMembers([]);
    setMemberSearch('');
    setMembersLoading(true);
    try {
      setMembers(await iamApi.listGroupMembers(group.id));
    } catch (cause) {
      setMemberGroup(null);
      error(cause instanceof Error ? cause.message : '加载用户组成员失败。');
    } finally {
      setMembersLoading(false);
    }
  };

  const toggleMember = (userId: number) => {
    setMembers((current) => current.map((member) => member.id === userId ? { ...member, is_member: !member.is_member } : member));
  };

  const saveMembers = async () => {
    if (!memberGroup) return;
    const userIDs = members.filter((member) => member.is_member).map((member) => member.id);
    if (userIDs.some((userID) => !Number.isInteger(userID) || userID <= 0)) {
      error('员工列表包含无效用户 ID，请刷新页面后重试。');
      return;
    }
    setMembersSaving(true);
    try {
      await iamApi.setGroupMembers(memberGroup.id, userIDs);
      success('用户组成员已更新；已登录 Client 会在下一次配置同步时重新核对隧道分配。');
      setMemberGroup(null);
      await loadGroups();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '保存用户组成员失败。');
    } finally {
      setMembersSaving(false);
    }
  };

  return (
    <div className="space-y-5">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-slate-900">安全用户组</h1>
          <p className="mt-1 text-xs text-slate-500">按员工用户组匹配 Client 隧道和动态地址池。用户可加入多个组，分配时按可用映射选择。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void loadGroups()} isLoading={loading} aria-label="刷新用户组"><RefreshCw className="h-4 w-4" /></Button>
          {canWrite && <Button onClick={openCreate}><Plus className="h-4 w-4" />新建用户组</Button>}
        </div>
      </div>

      <Card className="p-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-slate-600">可在 IPAM 的动态 User-Device 地址池中关联员工组，HUB 会据此为员工设备自动分配 Overlay IP。Gateway 访问权限由访问策略控制。</p>
          <div className="w-full sm:w-72"><Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索用户组名称或编码" leftIcon={<Search className="h-4 w-4" />} /></div>
        </div>
      </Card>

      {groups.length === 0 && !loading ? (
        <Card className="p-8"><EmptyState icon={UsersRound} title="还没有安全用户组" description="创建用户组后，将员工加入组，再到 IPAM 配置组、隧道和动态地址池的映射。" /></Card>
      ) : (
        <Card className="overflow-hidden">
          <Table>
            <TableHeader><TableRow>
              <TableHead>用户组</TableHead><TableHead>用户组编码</TableHead><TableHead>成员</TableHead><TableHead>状态</TableHead><TableHead className="text-right">操作</TableHead>
            </TableRow></TableHeader>
            <TableBody>
              {filteredGroups.map((group) => (
                <TableRow key={group.id}>
                  <TableCell><div className="font-medium text-slate-900">{group.group_name}</div>{group.description && <div className="mt-1 max-w-lg truncate text-xs text-slate-500">{group.description}</div>}</TableCell>
                  <TableCell><code className="rounded bg-slate-100 px-1.5 py-1 text-xs">{group.group_code}</code></TableCell>
                  <TableCell>{group.member_count}</TableCell>
                  <TableCell><Badge variant={group.status === 'active' ? 'success' : 'default'}>{group.status === 'active' ? '启用' : '停用'}</Badge></TableCell>
                  <TableCell><div className="flex justify-end gap-2">
                    <Button variant="outline" size="sm" onClick={() => void openMembers(group)}><UserRoundPlus className="h-3.5 w-3.5" />管理成员</Button>
                    {canWrite && <Button variant="ghost" size="sm" onClick={() => openEdit(group)}><Pencil className="h-3.5 w-3.5" />编辑</Button>}
                  </div></TableCell>
                </TableRow>
              ))}
              {filteredGroups.length === 0 && <TableRow><TableCell colSpan={5} className="py-10 text-center text-slate-500">没有匹配的用户组</TableCell></TableRow>}
            </TableBody>
          </Table>
        </Card>
      )}

      <Dialog isOpen={editorOpen} onClose={() => setEditorOpen(false)} title={editing ? '编辑安全用户组' : '新建安全用户组'} description="用户组编码用于展示和租约审计；组与隧道、地址池的关系在 IPAM 中维护。" maxWidth="lg">
        <form className="space-y-4" onSubmit={(event) => void saveGroup(event)}>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="space-y-1.5 text-xs font-medium text-slate-700">用户组名称<Input value={form.group_name} onChange={(event) => setForm((current) => ({ ...current, group_name: event.target.value }))} maxLength={128} required placeholder="例如：研发部门" /></label>
            <label className="space-y-1.5 text-xs font-medium text-slate-700">用户组编码<Input value={form.group_code} onChange={(event) => setForm((current) => ({ ...current, group_code: event.target.value.toLowerCase() }))} maxLength={64} required pattern="[A-Za-z0-9][A-Za-z0-9._-]*" placeholder="例如：engineering" /></label>
          </div>
          <label className="block space-y-1.5 text-xs font-medium text-slate-700">说明<textarea className="min-h-20 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500" value={form.description} onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))} maxLength={512} /></label>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="space-y-1.5 text-xs font-medium text-slate-700">状态<Select value={form.status} onChange={(event) => setForm((current) => ({ ...current, status: event.target.value as GroupForm['status'] }))}><option value="active">启用</option><option value="disabled">停用</option></Select></label>
            <label className="space-y-1.5 text-xs font-medium text-slate-700">排序<Input type="number" min={0} value={form.sort_order} onChange={(event) => setForm((current) => ({ ...current, sort_order: Number(event.target.value) }))} /></label>
          </div>
          <div className="flex justify-end gap-2 border-t border-slate-100 pt-4"><Button type="button" variant="outline" onClick={() => setEditorOpen(false)}>取消</Button><Button type="submit" isLoading={saving}>{editing ? '保存修改' : '创建用户组'}</Button></div>
        </form>
      </Dialog>

      <Dialog isOpen={memberGroup !== null} onClose={() => setMemberGroup(null)} title={memberGroup ? `管理成员：${memberGroup.group_name}` : '管理成员'} description="只列出员工用户。停用的员工账号可保留在组中，但不能登录 Client。" maxWidth="2xl">
        <div className="space-y-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm text-slate-600">已选择 {members.filter((member) => member.is_member).length} 人</p>
            <div className="w-full sm:w-72"><Input value={memberSearch} onChange={(event) => setMemberSearch(event.target.value)} placeholder="搜索员工账号" leftIcon={<Search className="h-4 w-4" />} /></div>
          </div>
          {membersLoading ? <div className="py-10 text-center text-sm text-slate-500">正在加载员工用户…</div> : members.length === 0 ? (
            <Card className="p-6"><EmptyState icon={UsersRound} title="没有员工用户" description="先在员工用户管理中创建员工账号。" /></Card>
          ) : (
            <div className="max-h-[50vh] overflow-auto rounded-lg border border-slate-200">
              {filteredMembers.map((member) => (
                <label key={member.id} className="flex cursor-pointer items-center gap-3 border-b border-slate-100 px-4 py-3 last:border-b-0 hover:bg-slate-50">
                  <input type="checkbox" checked={member.is_member} onChange={() => toggleMember(member.id)} disabled={!canWrite || membersSaving} className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />
                  <span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium text-slate-900">{member.display_name || member.username}</span><span className="block truncate text-xs text-slate-500">{member.username}{member.email ? ` · ${member.email}` : ''}</span></span>
                  <Badge variant={member.user_status === 'active' ? 'success' : 'default'}>{employeeStatusLabels[member.user_status]}</Badge>
                </label>
              ))}
              {filteredMembers.length === 0 && <div className="px-4 py-8 text-center text-sm text-slate-500">没有匹配的员工用户</div>}
            </div>
          )}
          <div className="flex justify-end gap-2 border-t border-slate-100 pt-4"><Button variant="outline" onClick={() => setMemberGroup(null)}>取消</Button>{canWrite && <Button onClick={() => void saveMembers()} isLoading={membersSaving} disabled={membersLoading}>保存成员</Button>}</div>
        </div>
      </Dialog>
    </div>
  );
};
