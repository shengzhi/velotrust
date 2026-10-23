import React, { useEffect, useMemo, useState } from 'react';
import { Network, Pencil, Plus, RefreshCw, Search, UsersRound, UserRoundPlus } from 'lucide-react';
import { IAMGroup, IAMGroupMember, iamApi } from '../../services/iam';
import { GroupGatewayOption, virtualNodeApi } from '../../services/node';
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
const gatewayStatusLabels: Record<string, string> = {
  pending: '待审批',
  active: '启用',
  disabled: '停用',
  quarantined: '隔离',
  revoked: '已撤销',
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
  const [gatewayAccessGroup, setGatewayAccessGroup] = useState<IAMGroup | null>(null);
  const [gatewayOptions, setGatewayOptions] = useState<GroupGatewayOption[]>([]);
  const [selectedGatewayIDs, setSelectedGatewayIDs] = useState<number[]>([]);
  const [gatewayOptionsLoading, setGatewayOptionsLoading] = useState(false);
  const [gatewayOptionsSaving, setGatewayOptionsSaving] = useState(false);
  const [gatewayOptionsError, setGatewayOptionsError] = useState('');

  const canWrite = hasPermission('iam.group.write');
  const canReadGatewayBindings = hasPermission('ztna.node.read');
  const canWriteGatewayBindings = hasPermission('ztna.node.write');

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

  const openGatewayAccess = async (group: IAMGroup) => {
    setGatewayAccessGroup(group);
    setGatewayOptions([]);
    setSelectedGatewayIDs([]);
    setGatewayOptionsError('');
    setGatewayOptionsLoading(true);
    try {
      const items = await virtualNodeApi.listGroupGatewayOptions(group.id);
      setGatewayOptions(items);
      setSelectedGatewayIDs(items.filter((gateway) => gateway.bound).map((gateway) => gateway.id));
    } catch (cause) {
      setGatewayOptionsError(cause instanceof Error ? cause.message : '加载 Gateway 准入配置失败。');
    } finally {
      setGatewayOptionsLoading(false);
    }
  };

  const saveGatewayAccess = async () => {
    if (!gatewayAccessGroup) return;
    setGatewayOptionsSaving(true);
    try {
      await virtualNodeApi.setGroupGateways(gatewayAccessGroup.id, gatewayAccessGroup.version_no, selectedGatewayIDs);
      success('安全用户组的 Gateway 隧道准入已更新；HUB 正在刷新相关节点配置。');
      setGatewayAccessGroup(null);
      await loadGroups();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '保存 Gateway 准入配置失败。');
    } finally {
      setGatewayOptionsSaving(false);
    }
  };

  const gatewaySelectionChanged = gatewayOptions.some((gateway) => gateway.bound !== selectedGatewayIDs.includes(gateway.id));

  return (
    <div className="space-y-5">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-slate-900">安全用户组</h1>
          <p className="mt-1 text-xs text-slate-500">员工组用于分配 Client Overlay 地址，并配置可接入的 Gateway。用户可加入多个组。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void loadGroups()} isLoading={loading} aria-label="刷新用户组"><RefreshCw className="h-4 w-4" /></Button>
          {canWrite && <Button onClick={openCreate}><Plus className="h-4 w-4" />新建用户组</Button>}
        </div>
      </div>

      <Card className="p-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-slate-600">在此管理用户组允许接入的 Gateway；IPAM 动态 User-Device 地址池负责分配 Overlay IP。Gateway 隧道准入不会授予后方资源访问权限，资源仍由访问策略控制。</p>
          <div className="w-full sm:w-72"><Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索用户组名称或编码" leftIcon={<Search className="h-4 w-4" />} /></div>
        </div>
      </Card>

      {groups.length === 0 && !loading ? (
        <Card className="p-8"><EmptyState icon={UsersRound} title="还没有安全用户组" description="创建用户组并加入员工后，可配置 Gateway 隧道准入和 IPAM Overlay 地址池映射。" /></Card>
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
                    {canReadGatewayBindings && <Button variant="outline" size="sm" onClick={() => void openGatewayAccess(group)}><Network className="h-3.5 w-3.5" />Gateway 准入</Button>}
                    {canWrite && <Button variant="ghost" size="sm" onClick={() => openEdit(group)}><Pencil className="h-3.5 w-3.5" />编辑</Button>}
                  </div></TableCell>
                </TableRow>
              ))}
              {filteredGroups.length === 0 && <TableRow><TableCell colSpan={5} className="py-10 text-center text-slate-500">没有匹配的用户组</TableCell></TableRow>}
            </TableBody>
          </Table>
        </Card>
      )}

      <Dialog isOpen={editorOpen} onClose={() => setEditorOpen(false)} title={editing ? '编辑安全用户组' : '新建安全用户组'} description="用户组编码用于展示和租约审计；员工成员在此维护，Overlay 地址池映射在 IPAM 中配置。" maxWidth="lg">
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

      <Dialog isOpen={gatewayAccessGroup !== null} onClose={() => setGatewayAccessGroup(null)} title={gatewayAccessGroup ? `Gateway 准入：${gatewayAccessGroup.group_name}` : 'Gateway 准入'} description="组内活动员工可以与选中的活动 Gateway 建立隧道；Gateway 后方资源仍由访问策略和 ACL 控制。" maxWidth="2xl">
        <div className="space-y-4">
          {gatewayAccessGroup?.status !== 'active' && <p className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">此用户组已停用，当前绑定已不生效；可以移除旧绑定，但不能添加 Gateway。</p>}
          {gatewayOptionsLoading ? <div className="py-10 text-center text-sm text-slate-500">正在加载 Gateway…</div> : gatewayOptionsError ? (
            <div className="rounded-lg border border-rose-200 bg-rose-50 p-3 text-sm text-rose-800">{gatewayOptionsError}</div>
          ) : gatewayOptions.length === 0 ? (
            <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">没有可绑定的 Gateway。需要先注册并启用 Gateway 节点。</div>
          ) : (
            <div className="max-h-[50vh] divide-y overflow-auto rounded-lg border border-slate-200">
              {gatewayOptions.map((gateway) => {
                const checked = selectedGatewayIDs.includes(gateway.id);
                return (
                  <label key={gateway.id} className="flex cursor-pointer items-center gap-3 p-3 hover:bg-slate-50">
                    <input
                      type="checkbox"
                      checked={checked}
                      disabled={!canWriteGatewayBindings || gatewayOptionsSaving || ((gateway.status !== 'active' || gatewayAccessGroup?.status !== 'active') && !checked)}
                      onChange={(event) => setSelectedGatewayIDs((current) => event.target.checked
                        ? gateway.status === 'active' && gatewayAccessGroup?.status === 'active' ? [...current, gateway.id] : current
                        : current.filter((id) => id !== gateway.id))}
                      className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                    />
                    <span className="min-w-0 flex-1">
                      <span className="block font-medium text-slate-800">{gateway.name}</span>
                      <span className="font-mono text-[10px] text-slate-500">{gateway.node_code}{gateway.status !== 'active' ? ` · ${gatewayStatusLabels[gateway.status]}` : ''}</span>
                    </span>
                    {gateway.bound && <Badge variant="success">已绑定</Badge>}
                  </label>
                );
              })}
            </div>
          )}
          <div className="flex justify-end gap-2 border-t border-slate-100 pt-4">
            <Button variant="outline" onClick={() => setGatewayAccessGroup(null)}>关闭</Button>
            {canWriteGatewayBindings && <Button onClick={() => void saveGatewayAccess()} isLoading={gatewayOptionsSaving} disabled={gatewayOptionsLoading || gatewayOptionsSaving || !!gatewayOptionsError || !gatewaySelectionChanged}>保存 Gateway 准入</Button>}
          </div>
        </div>
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
