import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Check, CircleOff, Loader2, Plus, Power, RefreshCw, Shield, Trash2 } from 'lucide-react';
import { policyApi, type AccessPolicy, type AccessPolicyRule, type PolicyGroupOption, type PolicyResourceOption } from '../services/policy';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { Dialog } from '../components/ui/dialog';
import { EmptyState } from '../components/EmptyState';
import { useToast } from '../components/ui/toast';

type PolicySubject = NonNullable<AccessPolicy['subjects']>[number];
interface PolicyDraft {
  id?: number; version_no?: number; policy_name: string; policy_code: string; description: string;
  priority: number; status: AccessPolicy['status']; rules: AccessPolicyRule[];
  subjects: PolicySubject[]; resource_ids: number[];
}

const newRule = (priority = 100, id = 'rule-1'): AccessPolicyRule => ({ id, priority, action: 'allow' });
const policyAction = (policy: AccessPolicy): AccessPolicyRule =>
  policy.rules?.find((rule) => rule.action === 'allow' || rule.action === 'deny') ?? newRule();
const statusLabel: Record<AccessPolicy['status'], string> = { draft: '草稿', active: '已启用', disabled: '已停用' };

function resourceEndpoint(resource: PolicyResourceOption): string {
  const protocol = (resource.protocol || 'TCP').toUpperCase();
  const ports = resource.port_start === 1 && resource.port_end === 65535 ? '任意端口'
    : resource.port_start ? `${resource.port_start}${resource.port_end && resource.port_end !== resource.port_start ? `-${resource.port_end}` : ''}`
      : '未配置端口';
  const address = (resource.address || resource.network_cidr || '未配置地址').replace(/\/32$/, '');
  return `${address} · ${protocol}/${ports}`;
}

export const Policies: React.FC = () => {
  const { success, error } = useToast();
  const errorRef = useRef(error); errorRef.current = error;
  const [policies, setPolicies] = useState<AccessPolicy[]>([]);
  const [groups, setGroups] = useState<PolicyGroupOption[]>([]);
  const [resources, setResources] = useState<PolicyResourceOption[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [changingID, setChangingID] = useState<number | null>(null);
  const [editing, setEditing] = useState<AccessPolicy | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [draft, setDraft] = useState<PolicyDraft | null>(null);

  const loadData = useCallback(async (refresh = false) => {
    if (refresh) setRefreshing(true); else setLoading(true);
    try {
      const [policyItems, groupItems, resourceItems] = await Promise.all([policyApi.list(), policyApi.listGroups(), policyApi.listResources()]);
      setPolicies(policyItems); setGroups(groupItems); setResources(resourceItems);
    } catch (cause) { errorRef.current(cause instanceof Error ? cause.message : '加载访问策略失败'); }
    finally { setLoading(false); setRefreshing(false); }
  }, []);
  useEffect(() => { void loadData(); }, [loadData]);

  const activeCount = policies.filter((item) => item.status === 'active').length;
  const openCreate = () => {
    setEditing(null);
    setDraft({ policy_name: '', policy_code: '', description: '', priority: 100, status: 'draft', rules: [newRule()], subjects: [], resource_ids: [] });
    setDialogOpen(true);
  };
  const openEdit = (policy: AccessPolicy) => {
    setEditing(policy);
    setDraft({ id: policy.id, version_no: policy.version_no, policy_name: policy.policy_name, policy_code: policy.policy_code,
      description: policy.description ?? '', priority: policy.priority, status: policy.status, rules: [policyAction(policy)],
      subjects: policy.subjects ?? [], resource_ids: policy.resource_ids ?? [] });
    setDialogOpen(true);
  };
  const toggleGroup = (groupID: number) => setDraft((current) => {
    if (!current) return current;
    const selected = current.subjects.some((subject) => subject.subject_type === 'group' && subject.subject_id === groupID);
    return { ...current, subjects: selected ? current.subjects.filter((subject) => !(subject.subject_type === 'group' && subject.subject_id === groupID))
      : [...current.subjects, { subject_type: 'group', subject_id: groupID }] };
  });
  const toggleResource = (resourceID: number) => setDraft((current) => {
    if (!current) return current;
    return { ...current, resource_ids: current.resource_ids.includes(resourceID)
      ? current.resource_ids.filter((id) => id !== resourceID) : [...current.resource_ids, resourceID] };
  });
  const updateAction = (action: AccessPolicyRule['action']) => setDraft((current) => current ? ({
    ...current, rules: [{ ...(current.rules[0] ?? newRule()), action }],
  }) : current);
  const handleSave = async (event: React.FormEvent) => {
    event.preventDefault(); if (!draft) return; setSaving(true);
    try {
      await policyApi.save({ ...draft, description: draft.description.trim() || null });
      success(editing ? '访问策略已保存，HUB 正在同步对应 Gateway ACL' : '访问策略已创建');
      setDialogOpen(false); await loadData(true);
    } catch (cause) { error(cause instanceof Error ? cause.message : '保存访问策略失败'); }
    finally { setSaving(false); }
  };
  const handleToggle = async (policy: AccessPolicy) => {
    const status = policy.status === 'active' ? 'disabled' : 'active'; setChangingID(policy.id);
    try { await policyApi.setStatus(policy, status); success(`策略已${status === 'active' ? '启用' : '停用'}，HUB 正在同步 Gateway ACL`); await loadData(true); }
    catch (cause) { error(cause instanceof Error ? cause.message : '更新策略状态失败'); }
    finally { setChangingID(null); }
  };
  const handleDelete = async (policy: AccessPolicy) => {
    if (!window.confirm(`确定删除策略“${policy.policy_name}”吗？关联 Gateway 将移除该策略生成的 ACL。`)) return;
    setChangingID(policy.id);
    try { await policyApi.delete(policy); success('访问策略已删除'); await loadData(true); }
    catch (cause) { error(cause instanceof Error ? cause.message : '删除访问策略失败'); }
    finally { setChangingID(null); }
  };

  return <div className="space-y-6">
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div><div className="flex items-center gap-2 text-slate-900"><Shield className="h-5 w-5 text-blue-600" /><h1 className="text-xl font-bold tracking-tight">访问策略与 ACL</h1></div>
        <p className="mt-1 text-sm text-slate-500">选择安全用户组和企业资源，再设置允许或拒绝。企业资源目标可以是单个 IPv4 地址或 CIDR 网段，HUB 按组成员设备地址和资源目录生成 Gateway ACL。</p></div>
      <div className="flex items-center gap-2"><Button variant="outline" onClick={() => void loadData(true)} disabled={refreshing}><RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} />刷新</Button><Button onClick={openCreate}><Plus className="h-4 w-4" />新建策略</Button></div>
    </div>
    <div className="grid gap-4 sm:grid-cols-3">
      <div className="rounded-xl border border-slate-200 bg-white p-4 shadow-xs"><p className="text-sm text-slate-500">策略总数</p><p className="mt-2 text-2xl font-semibold text-slate-900">{policies.length}</p></div>
      <div className="rounded-xl border border-slate-200 bg-white p-4 shadow-xs"><p className="text-sm text-slate-500">当前启用</p><p className="mt-2 text-2xl font-semibold text-emerald-700">{activeCount}</p></div>
      <div className="rounded-xl border border-slate-200 bg-white p-4 shadow-xs"><p className="text-sm text-slate-500">有效安全用户组 / 企业资源</p><p className="mt-2 text-2xl font-semibold text-slate-900">{groups.filter((item) => item.status === 'active').length} / {resources.length}</p></div>
    </div>
    <div className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-xs">
      {loading ? <div className="flex items-center justify-center gap-2 py-16 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />正在读取 Management 策略</div>
        : policies.length === 0 ? <EmptyState title="还没有访问策略" description="选择安全用户组和企业资源，定义访问结果后由 HUB 编译 Gateway ACL。" actionText="新建策略" onAction={openCreate} />
        : <div className="overflow-x-auto"><table className="w-full min-w-[900px] text-left text-sm"><thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500"><tr><th className="px-5 py-3">策略</th><th className="px-5 py-3">安全用户组</th><th className="px-5 py-3">企业资源</th><th className="px-5 py-3">授权动作</th><th className="px-5 py-3">优先级</th><th className="px-5 py-3">状态</th><th className="px-5 py-3 text-right">操作</th></tr></thead><tbody className="divide-y divide-slate-100">
          {policies.map((policy) => <tr key={policy.id} className="hover:bg-slate-50/70"><td className="px-5 py-4"><div className="font-medium text-slate-900">{policy.policy_name}</div><div className="mt-1 font-mono text-xs text-slate-500">{policy.policy_code}</div></td>
            <td className="px-5 py-4 text-slate-600">{policy.subjects?.length ?? 0} 个组</td><td className="px-5 py-4 text-slate-600">{policy.resource_ids?.length ?? 0} 项</td>
            <td className="px-5 py-4 text-slate-600">{policy.rules?.[0]?.action === 'allow' ? '允许' : policy.rules?.[0]?.action === 'deny' ? '拒绝' : '—'}</td><td className="px-5 py-4 tabular-nums text-slate-600">{policy.priority}</td>
            <td className="px-5 py-4"><span className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium ${policy.status === 'active' ? 'bg-emerald-50 text-emerald-700' : policy.status === 'draft' ? 'bg-amber-50 text-amber-700' : 'bg-slate-100 text-slate-600'}`}>{policy.status === 'active' ? <Check className="h-3 w-3" /> : <CircleOff className="h-3 w-3" />}{statusLabel[policy.status]}</span></td>
            <td className="px-5 py-4"><div className="flex justify-end gap-2"><Button size="sm" variant="outline" onClick={() => openEdit(policy)}>编辑</Button><Button size="sm" variant="outline" disabled={changingID === policy.id} onClick={() => void handleToggle(policy)}>{changingID === policy.id ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Power className="h-3.5 w-3.5" />}{policy.status === 'active' ? '停用' : '启用'}</Button><Button size="sm" variant="ghost" className="text-rose-600 hover:text-rose-700" disabled={changingID === policy.id} onClick={() => void handleDelete(policy)}><Trash2 className="h-3.5 w-3.5" />删除</Button></div></td>
          </tr>)}
        </tbody></table></div>}
    </div>

    <Dialog isOpen={dialogOpen} onClose={() => setDialogOpen(false)} title={editing ? '编辑访问策略' : '新建访问策略'} description="授权主体从安全用户组读取，目标从企业资源管理中心读取；目标可以是 IPv4 主机或 CIDR 网段，HUB 会将目标地址和指定服务端口编译成 Gateway ACL。" maxWidth="4xl">
      {draft && <form className="space-y-6" onSubmit={handleSave}>
        <div className="grid gap-4 sm:grid-cols-2">
          <label className="space-y-1.5 text-xs font-medium text-slate-700">策略名称<Input required maxLength={128} value={draft.policy_name} onChange={(event) => setDraft({ ...draft, policy_name: event.target.value })} placeholder="例如：研发 SSH 访问" /></label>
          <label className="space-y-1.5 text-xs font-medium text-slate-700">策略编码<Input required maxLength={64} value={draft.policy_code} onChange={(event) => setDraft({ ...draft, policy_code: event.target.value })} placeholder="例如：dev-ssh-access" /></label>
          <label className="space-y-1.5 text-xs font-medium text-slate-700">策略优先级<Input type="number" min={0} max={1000000} value={draft.priority} onChange={(event) => setDraft({ ...draft, priority: Number(event.target.value) || 0 })} /><span className="block text-[11px] font-normal text-slate-500">数值较小时先应用，用于解决策略之间的授权冲突。</span></label>
          <label className="space-y-1.5 text-xs font-medium text-slate-700">状态<Select value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value as AccessPolicy['status'] })}><option value="draft">草稿</option><option value="active">启用</option><option value="disabled">停用</option></Select></label>
          <label className="space-y-1.5 text-xs font-medium text-slate-700 sm:col-span-2">说明<Input maxLength={512} value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} placeholder="可选" /></label>
        </div>
        <section className="space-y-3"><div><h2 className="text-sm font-semibold text-slate-900">授权安全用户组</h2><p className="mt-1 text-xs text-slate-500">HUB 按组成员、有效设备和已分配的 Overlay 地址生成来源匹配条件。</p></div><div className="grid gap-2 sm:grid-cols-2">
          {groups.filter((group) => group.status === 'active').map((group) => { const checked = draft.subjects.some((subject) => subject.subject_type === 'group' && subject.subject_id === group.id); return <label key={group.id} className={`flex cursor-pointer items-center gap-3 rounded-lg border p-3 ${checked ? 'border-blue-300 bg-blue-50/60' : 'border-slate-200 bg-white'}`}><input type="checkbox" checked={checked} onChange={() => toggleGroup(group.id)} className="rounded border-slate-300 text-blue-600 focus:ring-blue-500" /><span className="min-w-0 flex-1"><span className="block text-sm font-medium text-slate-800">{group.name}</span><span className="block font-mono text-[11px] text-slate-500">{group.code}</span></span><span className="text-xs text-slate-500">{group.member_count} 名员工</span></label>; })}
          {groups.length === 0 && <p className="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800">还没有安全用户组，请先创建安全用户组并添加员工。</p>}
        </div></section>
        <section className="space-y-3"><div><h2 className="text-sm font-semibold text-slate-900">受保护企业资源</h2><p className="mt-1 text-xs text-slate-500">资源地址可以是单个 IPv4 主机或 CIDR 网段；访问范围仍受资源中配置的协议和端口限制。</p></div><div className="max-h-64 space-y-2 overflow-y-auto">
          {resources.map((resource) => { const checked = draft.resource_ids.includes(resource.id); return <label key={resource.id} className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 ${checked ? 'border-blue-300 bg-blue-50/60' : 'border-slate-200 bg-white'}`}><input type="checkbox" checked={checked} onChange={() => toggleResource(resource.id)} className="mt-1 rounded border-slate-300 text-blue-600 focus:ring-blue-500" /><span className="min-w-0 flex-1"><span className="block text-sm font-medium text-slate-800">{resource.name}<span className="ml-2 font-mono text-[11px] text-slate-500">{resource.code}</span></span><span className="mt-1 block font-mono text-xs text-slate-600">{resourceEndpoint(resource)}</span><span className="mt-1 block text-[11px] text-slate-500">{resource.network_name || '未命名站点'} · {resource.gateway_name || '未指定 Gateway'}</span></span></label>; })}
          {resources.length === 0 && <p className="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800">还没有可用资源，请先创建资源并关联已分配 Gateway 的网络站点。</p>}
        </div></section>
        <section className="space-y-3"><div><h2 className="text-sm font-semibold text-slate-900">授权结果</h2><p className="mt-1 text-xs text-slate-500">此结果应用于上面选择的所有安全用户组和企业资源；资源协议与端口由企业资源目录提供。</p></div>
          <label className="block max-w-sm space-y-1.5 text-xs font-medium text-slate-700">授权动作<Select value={draft.rules[0]?.action ?? 'allow'} onChange={(event) => updateAction(event.target.value as AccessPolicyRule['action'])}><option value="allow">允许访问</option><option value="deny">拒绝访问</option></Select></label>
        </section>
        <div className="flex justify-end gap-2 border-t border-slate-100 pt-4"><Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>取消</Button><Button type="submit" isLoading={saving}>保存策略</Button></div>
      </form>}
    </Dialog>
  </div>;
};
