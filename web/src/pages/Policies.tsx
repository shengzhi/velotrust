import React, { useState, useEffect, useMemo } from 'react';
import {
  Search,
  Plus,
  Edit,
  Trash2,
  Shield,
  Code2,
  CheckCircle2,
  XCircle,
  Hash,
  Terminal,
  RefreshCw,
  Cpu,
  Smartphone,
  Lock,
  Layers,
  FileCode,
  ShieldCheck,
  AlertTriangle,
} from 'lucide-react';
import { mockApi } from '../data/mock';
import { Policy, Node, AclRule } from '../types';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { Badge } from '../components/ui/badge';
import { StatusBadge } from '../components/StatusBadge';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '../components/ui/table';
import { Card } from '../components/ui/card';
import { Dialog } from '../components/ui/dialog';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { EmptyState } from '../components/EmptyState';
import { useToast } from '../components/ui/toast';
import { formatDateTime } from '../utils/format';

export const Policies: React.FC = () => {
  const { success, error } = useToast();

  const [policies, setPolicies] = useState<Policy[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);

  // Filters
  const [search, setSearch] = useState('');
  const [actionFilter, setActionFilter] = useState<'all' | 'allow' | 'deny'>('all');
  const [statusFilter, setStatusFilter] = useState<'all' | 'enabled' | 'disabled'>('all');

  // Create / Edit Modal
  const [isPolicyModalOpen, setIsPolicyModalOpen] = useState(false);
  const [editingPolicy, setEditingPolicy] = useState<Policy | null>(null);
  const [policyForm, setPolicyForm] = useState({
    name: '',
    subject: 'group:安全运维组',
    resource: 'subnet:prod-db-cluster',
    targetIp: '192.168.10.0/24',
    ports: '80,443',
    action: 'allow' as 'allow' | 'deny',
    priority: 100,
    enabled: true,
    // 进程级应用网络防火墙
    appConstraintEnabled: false,
    processNames: 'DataGrip.exe, Navicat.exe, DBeaver.exe',
    appMode: 'allowlist' as 'allowlist' | 'blocklist',
    binarySignatureCheck: true,
  });
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  // Compile Preview Modal
  const [isCompileModalOpen, setIsCompileModalOpen] = useState(false);
  const [selectedCompileNodeId, setSelectedCompileNodeId] = useState('');
  const [compiledResult, setCompiledResult] = useState<{
    nodeName: string;
    version: string;
    checksum: string;
    compiledAt: string;
    rules: AclRule[];
  } | null>(null);
  const [compiling, setCompiling] = useState(false);

  // Delete State
  const [policyToDelete, setPolicyToDelete] = useState<Policy | null>(null);
  const [deleting, setDeleting] = useState(false);

  const loadData = async () => {
    try {
      const [p, n] = await Promise.all([mockApi.getPolicies(), mockApi.getNodes()]);
      setPolicies(p);
      setNodes(n);
      if (n.length > 0 && !selectedCompileNodeId) {
        setSelectedCompileNodeId(n[0].id);
      }
    } catch {
      error('加载策略数据失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const filteredPolicies = useMemo(() => {
    return policies.filter((pol) => {
      const matchSearch =
        !search ||
        pol.name.toLowerCase().includes(search.toLowerCase()) ||
        pol.subject.toLowerCase().includes(search.toLowerCase()) ||
        pol.resource.toLowerCase().includes(search.toLowerCase()) ||
        pol.targetIp.includes(search);
      const matchAction = actionFilter === 'all' || pol.action === actionFilter;
      const matchStatus =
        statusFilter === 'all' ||
        (statusFilter === 'enabled' ? pol.enabled : !pol.enabled);
      return matchSearch && matchAction && matchStatus;
    });
  }, [policies, search, actionFilter, statusFilter]);

  const openCreateModal = () => {
    setEditingPolicy(null);
    setPolicyForm({
      name: '',
      subject: 'group:安全运维组',
      resource: 'subnet:prod-db-cluster',
      targetIp: '192.168.10.0/24',
      ports: '80,443',
      action: 'allow',
      priority: 100,
      enabled: true,
      appConstraintEnabled: false,
      processNames: 'DataGrip.exe, Navicat.exe, DBeaver.exe',
      appMode: 'allowlist',
      binarySignatureCheck: true,
    });
    setFormErrors({});
    setIsPolicyModalOpen(true);
  };

  const openEditModal = (p: Policy) => {
    setEditingPolicy(p);
    setPolicyForm({
      name: p.name,
      subject: p.subject,
      resource: p.resource,
      targetIp: p.targetIp,
      ports: p.ports,
      action: p.action,
      priority: p.priority,
      enabled: p.enabled,
      appConstraintEnabled: !!p.appConstraint?.enabled,
      processNames: p.appConstraint?.processNames?.join(', ') || 'DataGrip.exe, Navicat.exe',
      appMode: p.appConstraint?.mode || 'allowlist',
      binarySignatureCheck: p.appConstraint?.binarySignatureCheck ?? true,
    });
    setFormErrors({});
    setIsPolicyModalOpen(true);
  };

  const validate = () => {
    const errs: Record<string, string> = {};
    if (!policyForm.name.trim()) errs.name = '请输入策略名称';
    if (!policyForm.targetIp.trim()) errs.targetIp = '请输入目标网段或 IP 地址';
    if (!policyForm.ports.trim()) errs.ports = '请输入端口（例如: 80,443 或 *）';
    if (policyForm.appConstraintEnabled && !policyForm.processNames.trim()) {
      errs.processNames = '请输入至少一个限定的客户端进程可执行文件名';
    }
    setFormErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    setSaving(true);
    try {
      const payload: Partial<Policy> = {
        name: policyForm.name,
        subject: policyForm.subject,
        resource: policyForm.resource,
        targetIp: policyForm.targetIp,
        ports: policyForm.ports,
        action: policyForm.action,
        priority: policyForm.priority,
        enabled: policyForm.enabled,
        appConstraint: {
          enabled: policyForm.appConstraintEnabled,
          processNames: policyForm.processNames
            .split(',')
            .map((s) => s.trim())
            .filter(Boolean),
          mode: policyForm.appMode,
          binarySignatureCheck: policyForm.binarySignatureCheck,
        },
      };

      if (editingPolicy) {
        await mockApi.updatePolicy(editingPolicy.id, payload as any);
        success(`策略 "${policyForm.name}" 已更新`);
      } else {
        await mockApi.createPolicy(payload as any);
        success(`策略 "${policyForm.name}" 创建成功`);
      }
      setIsPolicyModalOpen(false);
      await loadData();
    } catch {
      error('保存策略失败');
    } finally {
      setSaving(false);
    }
  };

  const handleTogglePolicy = async (p: Policy) => {
    try {
      const updated = await mockApi.togglePolicy(p.id);
      success(`策略 "${p.name}" 已${updated.enabled ? '启用' : '停用'}`);
      await loadData();
    } catch {
      error('切换策略状态失败');
    }
  };

  const handleDeleteConfirm = async () => {
    if (!policyToDelete) return;
    setDeleting(true);
    try {
      await mockApi.deletePolicy(policyToDelete.id);
      success(`策略 "${policyToDelete.name}" 已删除`);
      setPolicyToDelete(null);
      await loadData();
    } catch {
      error('删除策略失败');
    } finally {
      setDeleting(false);
    }
  };

  // Compile Preview handler
  const handleOpenCompileModal = async (nodeId?: string) => {
    const targetId = nodeId || selectedCompileNodeId || nodes[0]?.id;
    if (!targetId) return;
    setSelectedCompileNodeId(targetId);
    setCompiling(true);
    setIsCompileModalOpen(true);
    try {
      const res = await mockApi.compileNodeAcl(targetId);
      setCompiledResult(res);
    } catch {
      error('编译 ACL 失败');
    } finally {
      setCompiling(false);
    }
  };

  const handleCompileTargetChange = async (nodeId: string) => {
    setSelectedCompileNodeId(nodeId);
    setCompiling(true);
    try {
      const res = await mockApi.compileNodeAcl(nodeId);
      setCompiledResult(res);
    } catch {
      error('编译 ACL 失败');
    } finally {
      setCompiling(false);
    }
  };

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">访问策略与 ACL</h1>
          <p className="text-xs text-slate-500 mt-0.5">
            基于主体、资源与目标网段的微隔离访问控制，自动编译下发至网络节点
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleOpenCompileModal()}
            className="gap-1.5"
          >
            <Code2 className="w-3.5 h-3.5 text-blue-600" />
            <span>编译预览</span>
          </Button>
          <Button
            size="sm"
            onClick={openCreateModal}
            className="gap-1.5 bg-blue-600 hover:bg-blue-700 text-white"
          >
            <Plus className="w-3.5 h-3.5" />
            <span>新建策略</span>
          </Button>
        </div>
      </div>

      {/* Filter / Search Bar */}
      <Card className="p-4">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <Input
            placeholder="搜索策略名称、主体、资源或目标 IP..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            leftIcon={<Search className="w-4 h-4 text-slate-400" />}
          />

          <Select
            value={actionFilter}
            onChange={(e) => setActionFilter(e.target.value as any)}
          >
            <option value="all">执行动作：全部</option>
            <option value="allow">允许 (Allow)</option>
            <option value="deny">拒绝 (Deny)</option>
          </Select>

          <Select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as any)}
          >
            <option value="all">策略状态：全部</option>
            <option value="enabled">已启用生效</option>
            <option value="disabled">已暂停禁用</option>
          </Select>
        </div>
      </Card>

      {/* Policies Table */}
      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>策略名称</TableHead>
              <TableHead>主体 (Subject)</TableHead>
              <TableHead>资源 (Resource)</TableHead>
              <TableHead>目标 IP / 网段</TableHead>
              <TableHead>端口限制</TableHead>
              <TableHead>限定本地应用/进程</TableHead>
              <TableHead>执行动作</TableHead>
              <TableHead>优先级</TableHead>
              <TableHead>启停状态</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredPolicies.length === 0 ? (
              <TableRow>
                <TableCell colSpan={10} className="p-8">
                  <EmptyState
                    icon={Shield}
                    title="暂无策略规则"
                    description="未检索到符合条件的访问策略，请新建策略或重置搜索。"
                  />
                </TableCell>
              </TableRow>
            ) : (
              filteredPolicies.map((pol) => (
                <TableRow key={pol.id}>
                  <TableCell className="font-semibold text-slate-900">
                    {pol.name}
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className="font-mono text-xs">
                      {pol.subject}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant="slate" className="font-mono text-[11px]">
                      {pol.resource}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-blue-600 font-medium">
                    {pol.targetIp}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-700">
                    {pol.ports}
                  </TableCell>
                  <TableCell>
                    {pol.appConstraint?.enabled && pol.appConstraint.processNames.length > 0 ? (
                      <div className="space-y-1">
                        <div className="flex items-center gap-1">
                          <Badge variant="primary" className="text-[10px] py-0 px-1.5 bg-indigo-50 text-indigo-700 border-indigo-200">
                            <Cpu className="w-2.5 h-2.5 mr-1" />
                            {pol.appConstraint.mode === 'allowlist' ? '仅限授信进程' : '黑名单拦截'}
                          </Badge>
                          {pol.appConstraint.binarySignatureCheck && (
                            <span className="text-[10px] text-emerald-600 font-mono" title="校验 SHA256 签名">
                              ✓哈希
                            </span>
                          )}
                        </div>
                        <div className="text-[11px] font-mono text-slate-600 truncate max-w-[140px]" title={pol.appConstraint.processNames.join(', ')}>
                          {pol.appConstraint.processNames.join(', ')}
                        </div>
                      </div>
                    ) : (
                      <span className="text-xs text-slate-400">无限制 (全流量)</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={pol.action} />
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-500 font-semibold">
                    {pol.priority}
                  </TableCell>
                  <TableCell>
                    {/* Toggle Switch */}
                    <button
                      onClick={() => handleTogglePolicy(pol)}
                      className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${
                        pol.enabled ? 'bg-blue-600' : 'bg-slate-200'
                      }`}
                    >
                      <span
                        className={`pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
                          pol.enabled ? 'translate-x-4' : 'translate-x-0'
                        }`}
                      />
                    </button>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        title="编辑策略"
                        onClick={() => openEditModal(pol)}
                        className="h-8 w-8 p-0 text-slate-500 hover:text-slate-800"
                      >
                        <Edit className="w-3.5 h-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        title="删除策略"
                        onClick={() => setPolicyToDelete(pol)}
                        className="h-8 w-8 p-0 text-slate-400 hover:text-rose-600"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </Card>

      {/* Create / Edit Policy Dialog */}
      <Dialog
        isOpen={isPolicyModalOpen}
        onClose={() => setIsPolicyModalOpen(false)}
        title={editingPolicy ? '修改访问策略' : '新建 Mesh 访问策略'}
        description="定义主体（员工/组/站点）对目标资源网段的放行与阻断动作"
        maxWidth="lg"
      >
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1">
              策略名称 <span className="text-rose-500">*</span>
            </label>
            <Input
              value={policyForm.name}
              onChange={(e) => setPolicyForm({ ...policyForm, name: e.target.value })}
              placeholder="例如: 允许研发组访问预发环境数据库"
              error={formErrors.name}
            />
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                主体 (Subject) <span className="text-rose-500">*</span>
              </label>
              <Input
                value={policyForm.subject}
                onChange={(e) => setPolicyForm({ ...policyForm, subject: e.target.value })}
                placeholder="例如: group:安全运维组 或 site:*"
              />
              <p className="mt-1 text-[11px] text-slate-400">支持 group:xxx, site:xxx, user:xxx 或 *</p>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                目标资源标识 (Resource)
              </label>
              <Input
                value={policyForm.resource}
                onChange={(e) => setPolicyForm({ ...policyForm, resource: e.target.value })}
                placeholder="例如: subnet:prod-db-cluster"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                目标 IP 或 CIDR 网段 <span className="text-rose-500">*</span>
              </label>
              <Input
                value={policyForm.targetIp}
                onChange={(e) => setPolicyForm({ ...policyForm, targetIp: e.target.value })}
                placeholder="例如: 192.168.10.50/32 或 10.0.0.0/16"
                error={formErrors.targetIp}
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                端口列表 (逗号分隔) <span className="text-rose-500">*</span>
              </label>
              <Input
                value={policyForm.ports}
                onChange={(e) => setPolicyForm({ ...policyForm, ports: e.target.value })}
                placeholder="例如: 80,443 或 22 或 *"
                error={formErrors.ports}
              />
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">执行动作</label>
              <Select
                value={policyForm.action}
                onChange={(e) =>
                  setPolicyForm({ ...policyForm, action: e.target.value as 'allow' | 'deny' })
                }
              >
                <option value="allow">允许通行 (Allow)</option>
                <option value="deny">阻断拒绝 (Deny)</option>
              </Select>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                优先级 (Priority，数字越大越优先)
              </label>
              <Input
                type="number"
                value={policyForm.priority}
                onChange={(e) =>
                  setPolicyForm({ ...policyForm, priority: Number(e.target.value) || 100 })
                }
                min={1}
                max={9999}
              />
            </div>
          </div>

          {/* App-Level / Process-Level Split Tunneling & Firewall Section */}
          <div className="p-3.5 bg-indigo-50/60 rounded-xl border border-indigo-200/80 space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <span className="text-xs font-bold text-indigo-950 flex items-center gap-1.5">
                  <Cpu className="w-4 h-4 text-indigo-600" />
                  📱 客户端进程级应用网络防火墙 (Process-Level Split Tunneling)
                </span>
                <p className="text-[11px] text-slate-600 mt-0.5">
                  只有当发起网络请求的本地可执行文件符合白名单时，才允许进入 WireGuard 隧道，防止员工私人恶意软件/未知脚本利用隧道渗透
                </p>
              </div>
              <input
                type="checkbox"
                id="app-constraint-check"
                checked={policyForm.appConstraintEnabled}
                onChange={(e) =>
                  setPolicyForm({ ...policyForm, appConstraintEnabled: e.target.checked })
                }
                className="h-4 w-4 text-indigo-600 rounded border-slate-300 focus:ring-indigo-500"
              />
            </div>

            {policyForm.appConstraintEnabled && (
              <div className="space-y-3 pt-2.5 border-t border-indigo-200/60">
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <div className="sm:col-span-2">
                    <label className="block text-xs font-medium text-indigo-900 mb-1">
                      限定本地应用/进程名称 (逗号分隔) <span className="text-rose-500">*</span>
                    </label>
                    <Input
                      value={policyForm.processNames}
                      onChange={(e) =>
                        setPolicyForm({ ...policyForm, processNames: e.target.value })
                      }
                      placeholder="例如: DataGrip.exe, Navicat.exe, DBeaver.exe, code.exe"
                      error={formErrors.processNames}
                    />
                    <p className="mt-1 text-[11px] text-slate-500">
                      客户端 Agent 自动抓取发送 socket 请求的本地进程名进行强校验
                    </p>
                  </div>

                  <div>
                    <label className="block text-xs font-medium text-indigo-900 mb-1">
                      策略模式
                    </label>
                    <Select
                      value={policyForm.appMode}
                      onChange={(e) =>
                        setPolicyForm({
                          ...policyForm,
                          appMode: e.target.value as 'allowlist' | 'blocklist',
                        })
                      }
                    >
                      <option value="allowlist">白名单放行 (仅允许指定程序)</option>
                      <option value="blocklist">黑名单拦截 (阻断指定程序)</option>
                    </Select>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="binary-hash-check"
                    checked={policyForm.binarySignatureCheck}
                    onChange={(e) =>
                      setPolicyForm({
                        ...policyForm,
                        binarySignatureCheck: e.target.checked,
                      })
                    }
                    className="h-3.5 w-3.5 text-indigo-600 rounded border-slate-300"
                  />
                  <label htmlFor="binary-hash-check" className="text-xs text-indigo-900">
                    开启二进制可执行文件指纹与签名哈希防篡改校验 (SHA256 Binary Integrity)
                  </label>
                </div>
              </div>
            )}
          </div>

          <div className="flex items-center gap-2 pt-2">
            <input
              type="checkbox"
              id="policy-enabled-check"
              checked={policyForm.enabled}
              onChange={(e) => setPolicyForm({ ...policyForm, enabled: e.target.checked })}
              className="rounded border-slate-300 text-blue-600 focus:ring-blue-500 h-4 w-4"
            />
            <label htmlFor="policy-enabled-check" className="text-xs font-medium text-slate-700">
              立即启用该策略并在 Mesh 节点中生效
            </label>
          </div>

          <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-100">
            <Button type="button" variant="outline" onClick={() => setIsPolicyModalOpen(false)}>
              取消
            </Button>
            <Button type="submit" isLoading={saving}>
              保存策略
            </Button>
          </div>
        </form>
      </Dialog>

      {/* Compile Preview Dialog */}
      <Dialog
        isOpen={isCompileModalOpen}
        onClose={() => setIsCompileModalOpen(false)}
        title="策略内核编译预览 (ACL Compiler)"
        description="选择 Mesh 节点，查看控制平面针对该节点内核实时生成的静态路由与包过滤规则"
        maxWidth="2xl"
      >
        <div className="space-y-4">
          <div className="flex items-center justify-between gap-4 bg-slate-50 p-3 rounded-xl border border-slate-200">
            <div className="flex items-center gap-2">
              <Terminal className="w-4 h-4 text-blue-600" />
              <span className="text-xs font-medium text-slate-700">编译目标节点：</span>
            </div>
            <div className="w-64">
              <Select
                value={selectedCompileNodeId}
                onChange={(e) => handleCompileTargetChange(e.target.value)}
              >
                {nodes.map((n) => (
                  <option key={n.id} value={n.id}>
                    {n.name} ({n.virtualIp})
                  </option>
                ))}
              </Select>
            </div>
          </div>

          {compiling ? (
            <div className="py-12 flex flex-col items-center justify-center text-slate-400 gap-2">
              <RefreshCw className="w-6 h-6 animate-spin text-blue-600" />
              <span className="text-xs">正在编译节点 iptables / nftables ACL 规则集...</span>
            </div>
          ) : compiledResult ? (
            <>
              {/* Build Meta Info */}
              <div className="grid grid-cols-3 gap-3 text-xs bg-slate-900 text-slate-200 p-3.5 rounded-xl font-mono">
                <div>
                  <span className="text-slate-400 block text-[10px]">编译版本</span>
                  <span className="text-blue-400 font-bold">{compiledResult.version}</span>
                </div>
                <div>
                  <span className="text-slate-400 block text-[10px]">生成时间</span>
                  <span className="text-slate-300">{formatDateTime(compiledResult.compiledAt)}</span>
                </div>
                <div>
                  <span className="text-slate-400 block text-[10px]">配置指纹 (Checksum)</span>
                  <span className="text-emerald-400 text-[11px] truncate block" title={compiledResult.checksum}>
                    {compiledResult.checksum.slice(0, 18)}...
                  </span>
                </div>
              </div>

              {/* Rules List Table */}
              <div className="border border-slate-200 rounded-xl overflow-hidden">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>方向</TableHead>
                      <TableHead>源地址</TableHead>
                      <TableHead>目标地址</TableHead>
                      <TableHead>端口</TableHead>
                      <TableHead>动作</TableHead>
                      <TableHead>描述</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {compiledResult.rules.map((r) => (
                      <TableRow key={r.id}>
                        <TableCell>
                          <Badge variant={r.direction === 'ingress' ? 'primary' : 'warning'}>
                            {r.direction.toUpperCase()}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono text-xs text-slate-700">
                          {r.source}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-slate-700">
                          {r.destination}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-slate-700">
                          {r.port}
                        </TableCell>
                        <TableCell>
                          <StatusBadge status={r.action} />
                        </TableCell>
                        <TableCell className="text-xs text-slate-500">
                          {r.description || '-'}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </>
          ) : null}

          <div className="flex justify-end pt-2 border-t border-slate-100">
            <Button onClick={() => setIsCompileModalOpen(false)}>关闭预览</Button>
          </div>
        </div>
      </Dialog>

      {/* Delete Confirmation */}
      <ConfirmDialog
        isOpen={!!policyToDelete}
        onClose={() => setPolicyToDelete(null)}
        onConfirm={handleDeleteConfirm}
        title="确认删除策略规则"
        description={`确定要删除策略 "${policyToDelete?.name}" 吗？此规则将从所有 WireGuard 节点的过滤规则链中立即剔除。`}
        danger={true}
        isLoading={deleting}
      />
    </div>
  );
};
