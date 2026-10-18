import React, { useState, useEffect, useMemo } from 'react';
import {
  Compass,
  Plus,
  Search,
  Server,
  Shield,
  Layers,
  Edit,
  Trash2,
  CheckCircle2,
  XCircle,
  HelpCircle,
  ArrowRight,
  RefreshCw,
  Terminal,
  Activity,
  ListPlus,
  X,
  ExternalLink,
} from 'lucide-react';
import { mockApi } from '../data/mock';
import { DnsZone, DnsRecord, Group, Node } from '../types';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { Badge } from '../components/ui/badge';
import { StatusBadge } from '../components/StatusBadge';
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '../components/ui/table';
import { Dialog } from '../components/ui/dialog';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { EmptyState } from '../components/EmptyState';
import { useToast } from '../components/ui/toast';

export const Dns: React.FC = () => {
  const { success, error } = useToast();

  const [zones, setZones] = useState<DnsZone[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  // Zone Modal (Create / Edit Drawer)
  const [isZoneModalOpen, setIsZoneModalOpen] = useState(false);
  const [editingZone, setEditingZone] = useState<DnsZone | null>(null);
  const [zoneForm, setZoneForm] = useState({
    domain_suffix: '',
    description: '',
    dns_servers: ['10.88.0.1'],
    bind_gateway: '',
    assigned_groups: [] as string[],
    records: [] as DnsRecord[],
    status: 'Active' as 'Active' | 'Paused',
  });
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [savingZone, setSavingZone] = useState(false);

  // Static Records Sub-Editor state inside Modal
  const [newRecordHost, setNewRecordHost] = useState('');
  const [newRecordType, setNewRecordType] = useState<'A' | 'CNAME' | 'TXT'>('A');
  const [newRecordValue, setNewRecordValue] = useState('');

  // DNS Simulator Modal
  const [isSimulatorOpen, setIsSimulatorOpen] = useState(false);
  const [simDomain, setSimDomain] = useState('mysql.aliyun.internal');
  const [simClientGroup, setSimClientGroup] = useState('研发核心组');
  const [simLoading, setSimLoading] = useState(false);
  const [simResult, setSimResult] = useState<any>(null);

  // Records Quick-View Modal
  const [viewingRecordsZone, setViewingRecordsZone] = useState<DnsZone | null>(null);

  // Delete Confirmation
  const [zoneToDelete, setZoneToDelete] = useState<DnsZone | null>(null);
  const [deleting, setDeleting] = useState(false);

  const loadData = async () => {
    try {
      const [z, g, n] = await Promise.all([
        mockApi.getDnsZones(),
        mockApi.getGroups(),
        mockApi.getNodes(),
      ]);
      setZones(z);
      setGroups(g);
      setNodes(n.filter((node) => node.type === 'gateway'));
    } catch {
      error('加载内网 DNS 配置失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const filteredZones = useMemo(() => {
    return zones.filter(
      (z) =>
        !search ||
        z.domain_suffix.toLowerCase().includes(search.toLowerCase()) ||
        z.description.toLowerCase().includes(search.toLowerCase()) ||
        (z.bind_gateway && z.bind_gateway.toLowerCase().includes(search.toLowerCase()))
    );
  }, [zones, search]);

  const totalRecordsCount = useMemo(() => {
    return zones.reduce((acc, cur) => acc + (cur.records?.length || 0), 0);
  }, [zones]);

  const totalPeersCount = useMemo(() => {
    return zones.reduce((acc, cur) => acc + (cur.peer_count || 0), 0);
  }, [zones]);

  const openCreateModal = () => {
    setEditingZone(null);
    const defaultGateway = nodes[0]?.name || 'gw-bj-hub-01';
    setZoneForm({
      domain_suffix: 'corp.internal',
      description: '企业内网微服务与测试集群路由',
      dns_servers: ['10.0.0.1'],
      bind_gateway: defaultGateway,
      assigned_groups: groups.slice(0, 2).map((g) => g.name),
      records: [
        { id: `rec-${Date.now()}-1`, type: 'A', host: 'api', value: '10.0.1.101', ttl: 300 },
        { id: `rec-${Date.now()}-2`, type: 'A', host: 'db', value: '10.0.1.102', ttl: 300 },
      ],
      status: 'Active',
    });
    setFormErrors({});
    setIsZoneModalOpen(true);
  };

  const openEditModal = (z: DnsZone) => {
    setEditingZone(z);
    setZoneForm({
      domain_suffix: z.domain_suffix,
      description: z.description,
      dns_servers: [...z.dns_servers],
      bind_gateway: z.bind_gateway || nodes[0]?.name || '',
      assigned_groups: [...z.assigned_groups],
      records: [...z.records],
      status: z.status,
    });
    setFormErrors({});
    setIsZoneModalOpen(true);
  };

  const handleAddRecord = () => {
    if (!newRecordHost.trim() || !newRecordValue.trim()) {
      error('请填写完整的主机名与解析目标值');
      return;
    }
    const newRec: DnsRecord = {
      id: `rec-${Date.now()}`,
      host: newRecordHost.trim().toLowerCase(),
      type: newRecordType,
      value: newRecordValue.trim(),
      ttl: 300,
    };
    setZoneForm((prev) => ({
      ...prev,
      records: [...prev.records, newRec],
    }));
    setNewRecordHost('');
    setNewRecordValue('');
  };

  const handleRemoveRecord = (recId: string) => {
    setZoneForm((prev) => ({
      ...prev,
      records: prev.records.filter((r) => r.id !== recId),
    }));
  };

  const validate = () => {
    const errs: Record<string, string> = {};
    if (!zoneForm.domain_suffix.trim()) {
      errs.domain_suffix = '请输入内网域名后缀，如 aliyun.internal';
    }
    if (zoneForm.dns_servers.length === 0 || !zoneForm.dns_servers[0]?.trim()) {
      errs.dns_servers = '请指定至少一个上游隧道 DNS 解析服务器 IP';
    }
    if (zoneForm.assigned_groups.length === 0) {
      errs.assigned_groups = '请至少选择一个被授权加载该 DNS 视界的安全组';
    }
    setFormErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    setSavingZone(true);
    try {
      if (editingZone) {
        await mockApi.updateDnsZone(editingZone.id, zoneForm);
        success(`DNS 区域 *.${zoneForm.domain_suffix} 已更新`);
      } else {
        await mockApi.createDnsZone({
          ...zoneForm,
          zone_id: `dns_zone_${Date.now().toString(36)}`,
        });
        success(`DNS 区域 *.${zoneForm.domain_suffix} 创建成功并已下发策略`);
      }
      setIsZoneModalOpen(false);
      await loadData();
    } catch {
      error('保存 DNS 区域失败');
    } finally {
      setSavingZone(false);
    }
  };

  const handleToggleZone = async (z: DnsZone) => {
    try {
      const updated = await mockApi.toggleDnsZone(z.id);
      success(`DNS 区域 *.${z.domain_suffix} 已切换为: ${updated.status === 'Active' ? '生效' : '暂停'}`);
      await loadData();
    } catch {
      error('切换 DNS 区域状态失败');
    }
  };

  const handleDeleteConfirm = async () => {
    if (!zoneToDelete) return;
    setDeleting(true);
    try {
      await mockApi.deleteDnsZone(zoneToDelete.id);
      success(`DNS 区域 *.${zoneToDelete.domain_suffix} 已移除`);
      setZoneToDelete(null);
      await loadData();
    } catch {
      error('删除 DNS 区域失败');
    } finally {
      setDeleting(false);
    }
  };

  const runDnsSimulation = async () => {
    if (!simDomain.trim()) return;
    setSimLoading(true);
    try {
      const res = await mockApi.simulateDnsQuery(simDomain, simClientGroup);
      setSimResult(res);
    } catch {
      error('DNS 解析仿真测试失败');
    } finally {
      setSimLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">
            内网 DNS 管理 (Split DNS)
          </h1>
          <p className="text-xs text-slate-500 mt-0.5">
            配置内部域名后缀与 WireGuard 隧道解析路由，智能分流内网与公网请求
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setIsSimulatorOpen(true);
              runDnsSimulation();
            }}
            className="gap-1.5"
          >
            <Terminal className="w-3.5 h-3.5 text-blue-600" />
            <span>视界诊断仿真</span>
          </Button>
          <Button
            size="sm"
            onClick={openCreateModal}
            className="gap-1.5 bg-blue-600 hover:bg-blue-700 text-white"
          >
            <Plus className="w-3.5 h-3.5" />
            <span>新建 DNS 区域</span>
          </Button>
        </div>
      </div>

      {/* 4 Stats Metric Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3.5">
        <Card className="p-3.5">
          <div className="flex items-center justify-between">
            <span className="text-xs text-slate-500 font-medium">活跃 DNS 区域</span>
            <div className="w-7 h-7 rounded-lg bg-blue-50 text-blue-600 flex items-center justify-center">
              <Compass className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="mt-1 text-2xl font-bold text-slate-900">
            {zones.filter((z) => z.status === 'Active').length} <span className="text-xs text-slate-400 font-normal">/ {zones.length}</span>
          </div>
        </Card>

        <Card className="p-3.5">
          <div className="flex items-center justify-between">
            <span className="text-xs text-slate-500 font-medium">静态托管主机记录</span>
            <div className="w-7 h-7 rounded-lg bg-emerald-50 text-emerald-600 flex items-center justify-center">
              <Server className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="mt-1 text-2xl font-bold text-slate-900">
            {totalRecordsCount} <span className="text-xs text-slate-400 font-normal">条</span>
          </div>
        </Card>

        <Card className="p-3.5">
          <div className="flex items-center justify-between">
            <span className="text-xs text-slate-500 font-medium">上游隧道网关</span>
            <div className="w-7 h-7 rounded-lg bg-purple-50 text-purple-600 flex items-center justify-center">
              <Layers className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="mt-1 text-2xl font-bold text-slate-900">
            {nodes.length} <span className="text-xs text-slate-400 font-normal">台</span>
          </div>
        </Card>

        <Card className="p-3.5">
          <div className="flex items-center justify-between">
            <span className="text-xs text-slate-500 font-medium">覆盖终端设备</span>
            <div className="w-7 h-7 rounded-lg bg-amber-50 text-amber-600 flex items-center justify-center">
              <Activity className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="mt-1 text-2xl font-bold text-slate-900">
            {totalPeersCount} <span className="text-xs text-slate-400 font-normal">台</span>
          </div>
        </Card>
      </div>

      {/* Filter and Search Bar */}
      <Card className="p-4">
        <div className="max-w-md">
          <Input
            placeholder="搜索域名后缀 (*.internal)、描述或目标网关..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            leftIcon={<Search className="w-4 h-4 text-slate-400" />}
          />
        </div>
      </Card>

      {/* DNS Zones Table */}
      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>匹配域名后缀 (Domain Suffix)</TableHead>
              <TableHead>说明描述</TableHead>
              <TableHead>上游解析服务器 (Gateway / DNS Server)</TableHead>
              <TableHead>托管记录</TableHead>
              <TableHead>授权用户组 (DNS 视界隔离)</TableHead>
              <TableHead>解析终端数</TableHead>
              <TableHead>状态</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredZones.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="p-8">
                  <EmptyState
                    icon={Compass}
                    title="暂无 DNS 区域配置"
                    description="未检索到符合条件的内网 Split DNS 规则，请创建新区域。"
                  />
                </TableCell>
              </TableRow>
            ) : (
              filteredZones.map((z) => (
                <TableRow key={z.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <div className="w-2 h-2 rounded-full bg-blue-500 animate-pulse" />
                      <span className="font-mono text-xs font-bold text-blue-700 bg-blue-50/80 px-2 py-0.5 rounded border border-blue-200">
                        *.{z.domain_suffix}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className="text-xs text-slate-700 max-w-xs truncate">
                    {z.description}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-col">
                      <span className="font-mono text-xs text-slate-900 font-semibold">
                        {z.dns_servers.join(', ')}
                      </span>
                      {z.bind_gateway && (
                        <span className="text-[10px] text-slate-400 truncate">
                          {z.bind_gateway}
                        </span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <button
                      onClick={() => setViewingRecordsZone(z)}
                      className="inline-flex items-center gap-1.5 text-xs text-blue-600 hover:text-blue-800 font-mono hover:underline"
                    >
                      <span>{z.records.length} 条记录</span>
                      <ExternalLink className="w-3 h-3" />
                    </button>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1 max-w-xs">
                      {z.assigned_groups.map((g) => (
                        <Badge key={g} variant="outline" className="text-[10px]">
                          {g}
                        </Badge>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell>
                    <span className="font-mono text-xs font-semibold px-2 py-0.5 bg-slate-100 text-slate-700 rounded-md">
                      {z.peer_count} 台
                    </span>
                  </TableCell>
                  <TableCell>
                    <StatusBadge
                      status={z.status === 'Active' ? 'active' : 'disabled'}
                      label={z.status === 'Active' ? '生效中' : '已暂停'}
                    />
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        title={z.status === 'Active' ? '暂停该区域解析' : '启用该区域解析'}
                        onClick={() => handleToggleZone(z)}
                        className={`h-8 w-8 p-0 ${
                          z.status === 'Active'
                            ? 'text-slate-400 hover:text-amber-600'
                            : 'text-emerald-600 hover:text-emerald-700'
                        }`}
                      >
                        {z.status === 'Active' ? (
                          <XCircle className="w-3.5 h-3.5" />
                        ) : (
                          <CheckCircle2 className="w-3.5 h-3.5" />
                        )}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        title="编辑配置"
                        onClick={() => openEditModal(z)}
                        className="h-8 w-8 p-0 text-slate-500 hover:text-slate-800"
                      >
                        <Edit className="w-3.5 h-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        title="删除区域"
                        onClick={() => setZoneToDelete(z)}
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

      {/* DNS Zone Create / Edit Drawer Modal */}
      <Dialog
        isOpen={isZoneModalOpen}
        onClose={() => setIsZoneModalOpen(false)}
        title={editingZone ? `编辑内网 DNS 区域 — *.${editingZone.domain_suffix}` : '新建智能内网 DNS 区域 (Split DNS)'}
        description="配置基于域名后缀的按需分流规则、上游网关解析器及客户端视界访问权限"
        maxWidth="2xl"
      >
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                匹配域名后缀 (Domain Suffix) <span className="text-rose-500">*</span>
              </label>
              <div className="relative">
                <span className="absolute left-3 top-2.5 text-slate-400 text-xs font-mono">*.</span>
                <Input
                  value={zoneForm.domain_suffix}
                  onChange={(e) => setZoneForm({ ...zoneForm, domain_suffix: e.target.value })}
                  placeholder="aliyun.internal 或 corp.internal"
                  className="pl-7 font-mono text-xs"
                  error={formErrors.domain_suffix}
                />
              </div>
              <p className="mt-1 text-[11px] text-slate-400">
                终端仅在请求以此后缀结尾的域名时拦截并导入 WireGuard 隧道解析
              </p>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                描述说明
              </label>
              <Input
                value={zoneForm.description}
                onChange={(e) => setZoneForm({ ...zoneForm, description: e.target.value })}
                placeholder="例如: 阿里云 VPC 内网测试集群环境"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                目标解析上游 DNS IP <span className="text-rose-500">*</span>
              </label>
              <Input
                value={zoneForm.dns_servers.join(', ')}
                onChange={(e) =>
                  setZoneForm({
                    ...zoneForm,
                    dns_servers: e.target.value.split(',').map((s) => s.trim()),
                  })
                }
                placeholder="例如: 10.88.0.1"
                className="font-mono text-xs"
                error={formErrors.dns_servers}
              />
              <p className="mt-1 text-[11px] text-slate-400">隧道内部运行 CoreDNS / BIND 的服务器 IP</p>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                绑定关联汇聚网关 (Target Gateway)
              </label>
              <Select
                value={zoneForm.bind_gateway}
                onChange={(e) => setZoneForm({ ...zoneForm, bind_gateway: e.target.value })}
              >
                {nodes.map((node) => (
                  <option key={node.id} value={node.name}>
                    {node.name} ({node.virtualIp}) • {node.siteName}
                  </option>
                ))}
              </Select>
            </div>
          </div>

          {/* Group Isolation Selection */}
          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1.5">
              授权生效安全用户组 (DNS 视界隔离) <span className="text-rose-500">*</span>
            </label>
            <div className="flex flex-wrap gap-2 p-3 bg-slate-50 border border-slate-200 rounded-xl">
              {groups.map((group) => {
                const isSelected = zoneForm.assigned_groups.includes(group.name);
                return (
                  <button
                    key={group.id}
                    type="button"
                    onClick={() => {
                      if (isSelected) {
                        setZoneForm({
                          ...zoneForm,
                          assigned_groups: zoneForm.assigned_groups.filter((g) => g !== group.name),
                        });
                      } else {
                        setZoneForm({
                          ...zoneForm,
                          assigned_groups: [...zoneForm.assigned_groups, group.name],
                        });
                      }
                    }}
                    className={`px-3 py-1.5 rounded-lg text-xs font-medium border flex items-center gap-1.5 transition-all ${
                      isSelected
                        ? 'bg-blue-50 border-blue-300 text-blue-700 shadow-2xs font-semibold'
                        : 'bg-white border-slate-200 text-slate-600 hover:bg-slate-100'
                    }`}
                  >
                    <Shield className="w-3.5 h-3.5 text-blue-600" />
                    <span>{group.name}</span>
                  </button>
                );
              })}
            </div>
            {formErrors.assigned_groups && (
              <p className="mt-1 text-xs text-rose-500">{formErrors.assigned_groups}</p>
            )}
            <p className="mt-1 text-[11px] text-slate-400">
              只有被选中的组名下员工设备才会动态加载该域名的 DNS 路由，未授权组请求将被静默阻断
            </p>
          </div>

          {/* Static Records Section */}
          <div className="border border-slate-200 rounded-xl p-3.5 bg-white space-y-3">
            <div className="flex items-center justify-between border-b border-slate-100 pb-2">
              <div className="flex items-center gap-2">
                <ListPlus className="w-4 h-4 text-emerald-600" />
                <span className="text-xs font-semibold text-slate-800">
                  静态 DNS 记录管理 (可选直管条目: {zoneForm.records.length})
                </span>
              </div>
              <span className="text-[11px] text-slate-400">由控制面板下发至节点解析缓存</span>
            </div>

            {/* Existing Records List */}
            {zoneForm.records.length > 0 && (
              <div className="max-h-36 overflow-y-auto space-y-1.5 pr-1">
                {zoneForm.records.map((rec) => (
                  <div
                    key={rec.id}
                    className="flex items-center justify-between p-2 rounded-lg bg-slate-50 border border-slate-200 text-xs font-mono"
                  >
                    <div className="flex items-center gap-2">
                      <Badge variant="primary" size="sm">
                        {rec.type}
                      </Badge>
                      <span className="font-semibold text-slate-800">
                        {rec.host}.{zoneForm.domain_suffix || 'internal'}
                      </span>
                      <ArrowRight className="w-3 h-3 text-slate-400" />
                      <span className="text-emerald-700 font-semibold">{rec.value}</span>
                    </div>
                    <button
                      type="button"
                      onClick={() => handleRemoveRecord(rec.id)}
                      className="text-slate-400 hover:text-rose-600 p-1"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  </div>
                ))}
              </div>
            )}

            {/* Add Record Row */}
            <div className="flex items-center gap-2 pt-1">
              <div className="w-32">
                <Input
                  placeholder="主机名 如: mysql"
                  value={newRecordHost}
                  onChange={(e) => setNewRecordHost(e.target.value)}
                  className="h-8 text-xs font-mono"
                />
              </div>
              <div className="w-24">
                <Select
                  value={newRecordType}
                  onChange={(e) => setNewRecordType(e.target.value as any)}
                  className="h-8 text-xs font-mono"
                >
                  <option value="A">A 记录</option>
                  <option value="CNAME">CNAME</option>
                  <option value="TXT">TXT</option>
                </Select>
              </div>
              <div className="flex-1">
                <Input
                  placeholder="目标 IP 如: 192.168.10.11"
                  value={newRecordValue}
                  onChange={(e) => setNewRecordValue(e.target.value)}
                  className="h-8 text-xs font-mono"
                />
              </div>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={handleAddRecord}
                className="h-8 text-xs shrink-0"
              >
                + 添加记录
              </Button>
            </div>
          </div>

          <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-100">
            <Button type="button" variant="outline" onClick={() => setIsZoneModalOpen(false)}>
              取消
            </Button>
            <Button type="submit" isLoading={savingZone}>
              保存并发布 DNS 规则
            </Button>
          </div>
        </form>
      </Dialog>

      {/* DNS Resolution Simulator Modal */}
      <Dialog
        isOpen={isSimulatorOpen}
        onClose={() => setIsSimulatorOpen(false)}
        title="DNS 视界与分流解析实时仿真器"
        description="模拟员工终端发起域名解析，校验 Split DNS 规则拦截、安全组授权判定与上游 WireGuard 隧道解析结果"
        maxWidth="lg"
      >
        <div className="space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 bg-slate-50 p-3.5 rounded-xl border border-slate-200">
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                模拟查询的目标域名 (FQDN)
              </label>
              <Input
                value={simDomain}
                onChange={(e) => setSimDomain(e.target.value)}
                placeholder="例如: mysql.aliyun.internal"
                className="font-mono text-xs"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                模拟客户端所属用户组身份
              </label>
              <Select
                value={simClientGroup}
                onChange={(e) => setSimClientGroup(e.target.value)}
              >
                {groups.map((g) => (
                  <option key={g.id} value={g.name}>
                    {g.name}
                  </option>
                ))}
                <option value="非内网匿名访客">非内网匿名访客 (未授权)</option>
              </Select>
            </div>
          </div>

          <div className="flex justify-end">
            <Button size="sm" onClick={runDnsSimulation} isLoading={simLoading} className="gap-1.5">
              <RefreshCw className="w-3.5 h-3.5" />
              <span>执行解析诊断</span>
            </Button>
          </div>

          {/* Simulation Output Card */}
          {simResult && (
            <div className="p-4 rounded-xl border border-slate-200 bg-slate-900 text-slate-200 space-y-3 font-mono text-xs">
              <div className="flex items-center justify-between border-b border-slate-800 pb-2">
                <span className="text-slate-400">解析链路状态:</span>
                {simResult.isIntercepted ? (
                  simResult.clientHasAccess ? (
                    <span className="text-emerald-400 font-bold flex items-center gap-1">
                      <CheckCircle2 className="w-3.5 h-3.5" />
                      隧道拦截成功 (TUNNEL INTERCEPTED)
                    </span>
                  ) : (
                    <span className="text-rose-400 font-bold flex items-center gap-1">
                      <XCircle className="w-3.5 h-3.5" />
                      视界隔离拦截 (NXDOMAIN REFUSED)
                    </span>
                  )
                ) : (
                  <span className="text-amber-400 font-bold">公网直连解析 (PUBLIC BYPASS)</span>
                )}
              </div>

              <div className="space-y-1.5 text-[11px]">
                <div className="flex justify-between">
                  <span className="text-slate-400">查询域名:</span>
                  <span className="text-white font-bold">{simResult.domain}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-400">使用上游解析器:</span>
                  <span className="text-blue-400">{simResult.serverUsed}</span>
                </div>
                {simResult.gateway && (
                  <div className="flex justify-between">
                    <span className="text-slate-400">出口网关节点:</span>
                    <span className="text-purple-400">{simResult.gateway}</span>
                  </div>
                )}
                <div className="flex justify-between">
                  <span className="text-slate-400">模拟解析延迟:</span>
                  <span className="text-emerald-400">{simResult.latencyMs} ms</span>
                </div>
                <div className="flex justify-between items-start pt-1">
                  <span className="text-slate-400 shrink-0">返回解析地址 (A/CNAME):</span>
                  <div className="text-right">
                    {simResult.resolvedIps && simResult.resolvedIps.length > 0 ? (
                      simResult.resolvedIps.map((ip: string) => (
                        <div key={ip} className="text-emerald-400 font-bold">
                          {ip}
                        </div>
                      ))
                    ) : (
                      <span className="text-slate-500">无返回记录 (Refused)</span>
                    )}
                  </div>
                </div>
              </div>

              <div className="pt-2 border-t border-slate-800 text-[10px] text-slate-400 leading-relaxed">
                判定说明: {simResult.reason}
              </div>
            </div>
          )}

          <div className="flex justify-end pt-2 border-t border-slate-100">
            <Button variant="outline" onClick={() => setIsSimulatorOpen(false)}>
              关闭
            </Button>
          </div>
        </div>
      </Dialog>

      {/* Quick View Records Modal */}
      {viewingRecordsZone && (
        <Dialog
          isOpen={!!viewingRecordsZone}
          onClose={() => setViewingRecordsZone(null)}
          title={`静态 DNS 记录清单 — *.${viewingRecordsZone.domain_suffix}`}
          description={`当前区域共配置 ${viewingRecordsZone.records.length} 条静态主机解析映射`}
          maxWidth="md"
        >
          <div className="space-y-3">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>完整域名</TableHead>
                  <TableHead>类型</TableHead>
                  <TableHead>解析目标值</TableHead>
                  <TableHead>TTL</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {viewingRecordsZone.records.map((r) => (
                  <TableRow key={r.id}>
                    <TableCell className="font-mono text-xs font-semibold text-slate-900">
                      {r.host}.{viewingRecordsZone.domain_suffix}
                    </TableCell>
                    <TableCell>
                      <Badge variant="primary" size="sm">
                        {r.type}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-emerald-700 font-medium">
                      {r.value}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-slate-500">
                      {r.ttl || 300}s
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <div className="flex justify-end pt-2">
              <Button onClick={() => setViewingRecordsZone(null)}>关闭</Button>
            </div>
          </div>
        </Dialog>
      )}

      {/* Delete Confirmation */}
      <ConfirmDialog
        isOpen={!!zoneToDelete}
        onClose={() => setZoneToDelete(null)}
        onConfirm={handleDeleteConfirm}
        title="确认删除该 DNS 区域"
        description={`确定要删除区域 "*.${zoneToDelete?.domain_suffix}" 吗？该区域下所有静态主机解析与安全组视界下发规则将被彻底注销，客户端将无法解析该域名后缀。`}
        danger={true}
        isLoading={deleting}
      />
    </div>
  );
};
