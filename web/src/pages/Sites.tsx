import React, { useState, useEffect, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Search,
  Plus,
  Edit,
  Trash2,
  Globe,
  ExternalLink,
  Server,
  ArrowLeftRight,
  ShieldAlert,
  ShieldCheck,
  CheckCircle2,
  Activity,
  AlertTriangle,
  Radio,
  PlayCircle,
  Settings2,
  Lock,
  RefreshCw,
  HelpCircle,
  Clock,
  ChevronRight,
  Sliders,
} from 'lucide-react';
import { mockApi } from '../data/mock';
import { Site, FailSafeConfig, VpcNatMapping } from '../types';
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
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '../components/ui/card';
import { Dialog } from '../components/ui/dialog';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { EmptyState } from '../components/EmptyState';
import { useToast } from '../components/ui/toast';
import { isValidCidr } from '../utils/format';
import { siteApi, SiteGatewayOption } from '../services/site';

export const Sites: React.FC = () => {
  const navigate = useNavigate();
  const { success, error } = useToast();

  const [sites, setSites] = useState<Site[]>([]);
  const [gatewayNodes, setGatewayNodes] = useState<SiteGatewayOption[]>([]);
  const [failSafeConfig, setFailSafeConfig] = useState<FailSafeConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  // Tab switch between Sites and Fail-Safe
  const [activeTab, setActiveTab] = useState<'sites' | 'fail-safe'>('sites');

  // Create / Edit Site Modal
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingSite, setEditingSite] = useState<Site | null>(null);
  const [siteForm, setSiteForm] = useState({
    name: '',
    localSubnet: '',
    gatewayNodeId: '',
    status: 'active' as 'active' | 'inactive',
    // Optional shadow-prefix DNAT mapping; site SNAT is automatic.
    natEnabled: false,
    virtualCidr: '172.31.10.0/24',
    natDesc: '',
  });
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  // Quick NAT mapping drawer / modal
  const [isNatDrawerOpen, setIsNatDrawerOpen] = useState(false);
  const [targetNatSite, setTargetNatSite] = useState<Site | null>(null);
  const [natForm, setNatForm] = useState<VpcNatMapping>({
    enabled: true,
    virtualCidr: '172.31.10.0/24',
    natType: 'DNAT_VirtualIP',
    description: '',
  });
  const [savingNat, setSavingNat] = useState(false);

  // Fail-Safe Config State
  const [savingFailSafe, setSavingFailSafe] = useState(false);
  const [drilling, setDrilling] = useState(false);
  const [drillLog, setDrillLog] = useState<string | null>(null);

  // Delete modal
  const [siteToDelete, setSiteToDelete] = useState<Site | null>(null);
  const [deleting, setDeleting] = useState(false);

  const loadData = async () => {
    try {
      const [s, gateways, fs] = await Promise.all([
        siteApi.listSites(),
        siteApi.listGateways(),
        mockApi.getFailSafeConfig(),
      ]);
      setSites(s);
      setGatewayNodes(gateways);
      setFailSafeConfig(fs);
    } catch {
      error('加载网络站点与容灾数据失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const filteredSites = useMemo(() => {
    return sites.filter(
      (s) =>
        !search ||
        s.name.toLowerCase().includes(search.toLowerCase()) ||
        s.localSubnet.includes(search) ||
        s.gatewayNodeName.toLowerCase().includes(search.toLowerCase()) ||
        (s.natMapping?.virtualCidr && s.natMapping.virtualCidr.includes(search))
    );
  }, [sites, search]);

  const openCreateModal = () => {
    setEditingSite(null);
    setSiteForm({
      name: '',
      localSubnet: '192.168.100.0/24',
      gatewayNodeId: '',
      status: 'active',
      natEnabled: false,
      virtualCidr: '172.31.100.0/24',
      natDesc: '',
    });
    setFormErrors({});
    setIsModalOpen(true);
  };

  const openEditModal = (s: Site) => {
    setEditingSite(s);
    setSiteForm({
      name: s.name,
      localSubnet: s.localSubnet,
      gatewayNodeId: s.gatewayNodeId || '',
      status: s.status,
      natEnabled: !!s.natMapping?.enabled && s.natMapping.natType !== 'SNAT_Masquerade' && !!s.natMapping.virtualCidr,
      virtualCidr: s.natMapping?.virtualCidr || '172.31.10.0/24',
      natDesc: s.natMapping?.description || '',
    });
    setFormErrors({});
    setIsModalOpen(true);
  };

  const openQuickNatModal = (s: Site) => {
    setTargetNatSite(s);
    const hasShadowMapping = !!s.natMapping?.enabled && s.natMapping.natType !== 'SNAT_Masquerade' && !!s.natMapping.virtualCidr;
    setNatForm({
      enabled: s.natMapping ? hasShadowMapping : true,
      id: s.natMapping?.id,
      version_no: s.natMapping?.version_no,
      network_id: s.networkId,
      virtualCidr: s.natMapping?.virtualCidr || '172.31.10.0/24',
      natType: 'DNAT_VirtualIP',
      description: s.natMapping?.description || `针对 ${s.name} 真实网段 ${s.localSubnet} 的防冲突 NAT 映射`,
    });
    setIsNatDrawerOpen(true);
  };

  const handleSaveNat = async () => {
    if (!targetNatSite) return;
    if (!targetNatSite.networkId) {
      error('站点网络记录缺失，请刷新后重试');
      return;
    }
    if (natForm.enabled && !isValidCidr(natForm.virtualCidr)) {
      error('虚拟影子网段 CIDR 格式不正确，如 172.31.10.0/24');
      return;
    }
    try {
      setSavingNat(true);
      await siteApi.saveNATMapping({
        id: natForm.id ?? 0,
        expected_version: natForm.version_no ?? 0,
        network_id: targetNatSite.networkId,
        nat_type: 'DNAT_VirtualIP',
        virtual_cidr: natForm.virtualCidr || undefined,
        enabled: natForm.enabled,
        description: natForm.description,
      });
      success(`站点 [${targetNatSite.name}] 的影子网段映射已更新`);
      setIsNatDrawerOpen(false);
      setTargetNatSite(null);
      await loadData();
    } catch (err: any) {
      error(err.message || '更新 NAT 映射失败');
    } finally {
      setSavingNat(false);
    }
  };

  const validate = () => {
    const errs: Record<string, string> = {};
    if (!siteForm.name.trim()) errs.name = '请输入站点名称';
    if (!siteForm.localSubnet.trim()) {
      errs.localSubnet = '请输入本地子网 CIDR';
    } else if (!isValidCidr(siteForm.localSubnet)) {
      errs.localSubnet = 'CIDR 格式不正确，例如: 192.168.10.0/24';
    }
    if (siteForm.natEnabled && !isValidCidr(siteForm.virtualCidr)) {
      errs.virtualCidr = '影子虚拟网段 CIDR 格式不正确，例如: 172.31.10.0/24';
    }
    if (siteForm.natEnabled && siteForm.status !== 'active') {
      errs.natEnabled = '只有启用状态的站点才能启用 NAT';
    }
    const gatewayNodeId = siteForm.gatewayNodeId === '' ? null : Number(siteForm.gatewayNodeId);
    if (gatewayNodeId !== null && (!Number.isSafeInteger(gatewayNodeId) || gatewayNodeId <= 0)) {
      errs.gatewayNodeId = '请选择有效的 Gateway 节点';
    }
    setFormErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    setSaving(true);
    try {
      const payload = {
        name: editingSite?.name ?? siteForm.name,
        local_subnet: editingSite?.localSubnet ?? siteForm.localSubnet,
        gateway_node_id: siteForm.gatewayNodeId === '' ? null : Number(siteForm.gatewayNodeId),
        status: siteForm.status === 'active' ? 'active' as const : 'disabled' as const,
        id: editingSite ? Number(editingSite.id) : undefined,
        expected_version: editingSite?.versionNo,
      };

      const savedSite = await siteApi.saveSite(payload);
      setIsModalOpen(false);
      await loadData();
      if (!editingSite && siteForm.status === 'active' && siteForm.natEnabled) {
        if (!savedSite.networkId) throw new Error('站点网络记录缺失');
        try {
          await siteApi.saveNATMapping({
            id: 0,
            expected_version: 0,
            network_id: savedSite.networkId,
            nat_type: 'DNAT_VirtualIP',
            virtual_cidr: siteForm.virtualCidr || undefined,
            enabled: siteForm.natEnabled,
            description: siteForm.natDesc,
          });
        } catch (natError: any) {
          await loadData();
          error(`站点已保存，但影子网段映射未保存：${natError?.message || '请重试'}`);
          return;
        }
      }
      success(editingSite ? `站点 "${siteForm.name}" 已更新` : `站点 "${siteForm.name}" 创建成功`);
      await loadData();
    } catch (err: any) {
      error(err?.message || '保存站点失败');
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!siteToDelete) return;
    setDeleting(true);
    try {
      await siteApi.deleteSite(siteToDelete);
      success(`站点 "${siteToDelete.name}" 已删除`);
      setSiteToDelete(null);
      await loadData();
    } catch (err: any) {
      error(err?.message || '删除站点失败');
    } finally {
      setDeleting(false);
    }
  };

  // Fail-Safe Actions
  const handleSaveFailSafe = async (updates: Partial<FailSafeConfig>) => {
    if (!failSafeConfig) return;
    try {
      setSavingFailSafe(true);
      const updated = await mockApi.updateFailSafeConfig(updates);
      setFailSafeConfig(updated);
      success('分布式网关离线断开逃生策略已更新并同步至全球网关节点');
    } catch {
      error('保存离线逃生策略失败');
    } finally {
      setSavingFailSafe(false);
    }
  };

  const handleTriggerDrill = async () => {
    try {
      setDrilling(true);
      const res = await mockApi.triggerFailSafeDrill();
      setDrillLog(res.log);
      success('全球分布式网关离线断网逃生演练完成！');
      await loadData();
    } catch {
      error('逃生演练触发失败');
    } finally {
      setDrilling(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">
            网络站点与多云互联
          </h1>
          <p className="text-xs text-slate-500 mt-0.5">
            纳管多云 VPC、私有 IDC 与分支站点，支持重叠网段 NAT 转换与离线兜底策略
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setActiveTab(activeTab === 'sites' ? 'fail-safe' : 'sites')}
            className="gap-1.5"
          >
            <ShieldAlert className="h-3.5 w-3.5 text-amber-500" />
            <span>{activeTab === 'sites' ? '断网兜底配置' : '网络站点列表'}</span>
          </Button>

          <Button
            size="sm"
            onClick={openCreateModal}
            className="gap-1.5 bg-blue-600 hover:bg-blue-700 text-white"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>新建站点</span>
          </Button>
        </div>
      </div>

      {/* Tabs Switcher */}
      <div className="flex items-center border-b border-slate-200">
        <button
          onClick={() => setActiveTab('sites')}
          className={`py-2.5 px-4 text-xs font-semibold border-b-2 transition-all flex items-center gap-2 ${
            activeTab === 'sites'
              ? 'border-blue-600 text-blue-600'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          }`}
        >
          <Globe className="h-4 w-4" />
          <span>站点与影子网段映射</span>
          <span className="ml-1 text-[11px] font-normal text-slate-400">
            ({sites.length})
          </span>
        </button>

        <button
          onClick={() => setActiveTab('fail-safe')}
          className={`py-2.5 px-4 text-xs font-semibold border-b-2 transition-all flex items-center gap-2 ${
            activeTab === 'fail-safe'
              ? 'border-blue-600 text-blue-600'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          }`}
        >
          <Radio className="h-4 w-4 text-amber-500" />
          <span>断网逃生兜底策略</span>
        </button>
      </div>

      {/* TAB 1: SITES & NAT MAPPINGS */}
      {activeTab === 'sites' && (
        <div className="space-y-4">

          {/* Search bar */}
          <Card className="p-4">
            <div className="max-w-md">
              <Input
                placeholder="搜索站点名称、真实网段、虚拟影子网段或网关..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                leftIcon={<Search className="w-4 h-4 text-slate-400" />}
              />
            </div>
          </Card>

          {/* Sites Table */}
          <Card>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>站点名称</TableHead>
                  <TableHead>站点内网网段</TableHead>
                  <TableHead>自动 SNAT / 可选影子 DNAT</TableHead>
                  <TableHead>指定汇聚网关</TableHead>
                  <TableHead>关联网关数</TableHead>
                  <TableHead>网络状态</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredSites.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7} className="p-8">
                      <EmptyState
                        icon={Globe}
                        title="暂无网络站点"
                        description="未检索到符合条件的站点，请重置搜索或添加新站点。"
                      />
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredSites.map((site) => {
                    const hasShadowMapping = !!site.natMapping?.enabled && site.natMapping.natType !== 'SNAT_Masquerade' && !!site.natMapping.virtualCidr;
                    const autoSNATActive = site.status === 'active' && !!site.gatewayNodeId;
                    return (
                      <TableRow key={site.id}>
                        <TableCell className="font-semibold text-slate-900">
                          <div className="flex items-center gap-2.5">
                            <div className="h-8 w-8 rounded-lg bg-cyan-50 text-cyan-700 flex items-center justify-center border border-cyan-100 shrink-0">
                              <Globe className="w-4 h-4" />
                            </div>
                            <div>
                              <div className="flex items-center gap-2">
                                <span>{site.name}</span>
                              </div>
                              <div className="flex items-center gap-1.5 mt-0.5">
                                {site.localSubnet === '192.168.10.0/24' && (
                                  <Badge variant="warning" className="text-[10px] py-0 px-1 bg-amber-50 text-amber-700 border border-amber-200">
                                    网段潜在重叠
                                  </Badge>
                                )}
                              </div>
                            </div>
                          </div>
                        </TableCell>

                        <TableCell className="font-mono text-xs font-semibold text-slate-800">
                          {site.localSubnet}
                        </TableCell>

                        {/* NAT Mapping Column */}
                        <TableCell>
                          <div className="space-y-1">
                            <div className="flex items-center gap-1.5">
                              <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium border ${autoSNATActive ? 'bg-emerald-50 text-emerald-700 border-emerald-200' : 'bg-slate-50 text-slate-500 border-slate-200'}`}>
                                <ArrowLeftRight className="w-3 h-3" />
                                {autoSNATActive ? '出站 SNAT 自动下发' : '启用并关联 Gateway 后自动 SNAT'}
                              </span>
                            </div>
                            {hasShadowMapping ? (
                              <div className="text-[11px] text-slate-500 truncate max-w-xs" title={site.natMapping?.description}>
                                影子 DNAT：<span className="font-mono">{site.natMapping?.virtualCidr}</span> → {site.localSubnet}
                              </div>
                            ) : (
                              <span className="text-[11px] text-slate-400">无影子映射（网段不冲突时无需配置）</span>
                            )}
                          </div>
                        </TableCell>

                        <TableCell>
                          {site.gatewayNodeId ? (
                            <button
                              onClick={() => navigate('/enrollment-tokens')}
                              className="text-xs font-medium text-slate-700 hover:text-blue-600 flex items-center gap-1.5 font-mono"
                            >
                              <Server className="w-3.5 h-3.5 text-slate-400" />
                              <span>{site.gatewayNodeName}</span>
                              <ExternalLink className="w-3 h-3 text-slate-400" />
                            </button>
                          ) : (
                            <span className="text-xs text-slate-400">未指定网关</span>
                          )}
                        </TableCell>

                        <TableCell>
                          <span className="font-mono text-xs font-semibold px-2 py-0.5 bg-slate-100 text-slate-700 rounded-md">
                            {site.nodeCount} 个
                          </span>
                        </TableCell>

                        <TableCell>
                          <StatusBadge status={site.status} />
                        </TableCell>

                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-1.5">
                            <Button
                              variant="outline"
                              size="sm"
                              disabled={site.status !== 'active'}
                              title="配置可选影子网段 DNAT"
                              onClick={() => openQuickNatModal(site)}
                              className="h-8 gap-1 text-xs text-indigo-600 border-indigo-200 hover:bg-indigo-50"
                            >
                              <ArrowLeftRight className="w-3.5 h-3.5" />
                              <span>影子映射</span>
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              title="编辑站点"
                              onClick={() => openEditModal(site)}
                              className="h-8 w-8 p-0 text-slate-500 hover:text-slate-800"
                            >
                              <Edit className="w-3.5 h-3.5" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              title="删除站点"
                              onClick={() => setSiteToDelete(site)}
                              className="h-8 w-8 p-0 text-slate-400 hover:text-rose-600"
                            >
                              <Trash2 className="w-3.5 h-3.5" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>
          </Card>
        </div>
      )}

      {/* TAB 2: GATEWAY FAIL-SAFE & RESILIENCE MODES */}
      {activeTab === 'fail-safe' && failSafeConfig && (
        <div className="space-y-6">
          {/* Top resilience overview card */}
          <Card className="border-amber-200 bg-amber-50/40">
            <CardHeader className="pb-3">
              <div className="flex items-start justify-between">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <Radio className="h-5 w-5 text-amber-600 animate-pulse" />
                    <CardTitle className="text-base text-slate-900">
                      网关离线断开逃生与兜底策略 (Fail-Safe Modes)
                    </CardTitle>
                    <Badge variant="outline" className="bg-amber-100 text-amber-800 border-amber-300 text-xs">
                      系统高可用纵深防御
                    </Badge>
                  </div>
                  <CardDescription className="text-xs text-slate-600">
                    当中央控制平台遭遇宕机、网络割接或大规模 DDoS 时，全球分布式云网关 Agent 将自动与大脑断开并进入本地自持兜底逻辑。
                  </CardDescription>
                </div>

                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleTriggerDrill}
                  isLoading={drilling}
                  className="gap-1.5 border-amber-300 text-amber-900 bg-amber-100/60 hover:bg-amber-200"
                >
                  <PlayCircle className="w-4 h-4 text-amber-700" />
                  <span>触发断网逃生演练 (Run Drill)</span>
                </Button>
              </div>
            </CardHeader>
            <CardContent className="pt-0 text-xs text-slate-600">
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4 p-3 bg-white rounded-lg border border-amber-200/60">
                <div>
                  <span className="text-slate-400 block text-[11px]">心跳探针周期 (Heartbeat)</span>
                  <span className="font-mono font-semibold text-slate-800">
                    每 {failSafeConfig.heartbeatIntervalSec} 秒 1 次
                  </span>
                </div>
                <div>
                  <span className="text-slate-400 block text-[11px]">失联判定阈值 (Offline Threshold)</span>
                  <span className="font-mono font-semibold text-slate-800">
                    连续 {failSafeConfig.offlineThresholdSec} 秒无响应即触发
                  </span>
                </div>
                <div>
                  <span className="text-slate-400 block text-[11px]">最近逃生演练记录</span>
                  <span className="font-mono font-semibold text-emerald-700 flex items-center gap-1">
                    <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" />
                    {failSafeConfig.lastDrillRun || '未执行'}
                  </span>
                </div>
              </div>

              {drillLog && (
                <div className="mt-3 p-3 bg-slate-900 text-slate-200 rounded-lg font-mono text-[11px] leading-relaxed border border-slate-700">
                  <div className="text-emerald-400 font-bold mb-1">=== 模拟演练输出日志 ===</div>
                  {drillLog}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Fail-Safe Mode Selection Card */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
            {/* Mode 1: ALLOW_EXISTING_CACHE (Recommended) */}
            <div
              onClick={() => handleSaveFailSafe({ mode: 'ALLOW_EXISTING_CACHE' })}
              className={`p-5 rounded-xl border-2 cursor-pointer transition-all ${
                failSafeConfig.mode === 'ALLOW_EXISTING_CACHE'
                  ? 'border-blue-600 bg-blue-50/30 shadow-md ring-2 ring-blue-500/20'
                  : 'border-slate-200 bg-white hover:border-slate-300'
              }`}
            >
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-2">
                  <ShieldCheck className="w-5 h-5 text-blue-600" />
                  <h3 className="font-semibold text-slate-900 text-sm">自持本地缓存 (推荐)</h3>
                </div>
                {failSafeConfig.mode === 'ALLOW_EXISTING_CACHE' && (
                  <Badge variant="primary" className="text-[10px]">当前激活</Badge>
                )}
              </div>
              <p className="text-xs text-slate-600 leading-relaxed mb-4">
                <strong>保留现有连接通畅：</strong>网关继续沿用最后一次从大脑同步的 WireGuard 本地 Peer 路由表与 ACL 缓存，现有研发人员无感，业务不中断。
              </p>
              <div className="text-[11px] text-slate-500 bg-slate-50 p-2.5 rounded-lg border border-slate-200 space-y-1">
                <div>✓ 优点：最大限度保障企业核心业务连续性</div>
                <div>⚠ 限制：离线期间无法接收新的动态密钥与吊销指令</div>
              </div>
            </div>

            {/* Mode 2: READ_ONLY_FALLBACK */}
            <div
              onClick={() => handleSaveFailSafe({ mode: 'READ_ONLY_FALLBACK' })}
              className={`p-5 rounded-xl border-2 cursor-pointer transition-all ${
                failSafeConfig.mode === 'READ_ONLY_FALLBACK'
                  ? 'border-blue-600 bg-blue-50/30 shadow-md ring-2 ring-blue-500/20'
                  : 'border-slate-200 bg-white hover:border-slate-300'
              }`}
            >
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-2">
                  <Clock className="w-5 h-5 text-indigo-600" />
                  <h3 className="font-semibold text-slate-900 text-sm">只读降级与租约冻结</h3>
                </div>
                {failSafeConfig.mode === 'READ_ONLY_FALLBACK' && (
                  <Badge variant="primary" className="text-[10px]">当前激活</Badge>
                )}
              </div>
              <p className="text-xs text-slate-600 leading-relaxed mb-4">
                <strong>严格时效保护：</strong>只允许已建立握手且在有效 TTL 窗口内的会话通过，禁止任何新设备或过期会话发起握手，直至大脑恢复。
              </p>
              <div className="text-[11px] text-slate-500 bg-slate-50 p-2.5 rounded-lg border border-slate-200 space-y-1">
                <div>✓ 优点：防止已离职人员在断网空窗期利用本地缓存渗透</div>
                <div>⚠ 限制：过期设备无法续期登录</div>
              </div>
            </div>

            {/* Mode 3: BLOCK_ALL_EMERGENCY */}
            <div
              onClick={() => handleSaveFailSafe({ mode: 'BLOCK_ALL_EMERGENCY' })}
              className={`p-5 rounded-xl border-2 cursor-pointer transition-all ${
                failSafeConfig.mode === 'BLOCK_ALL_EMERGENCY'
                  ? 'border-rose-600 bg-rose-50/30 shadow-md ring-2 ring-rose-500/20'
                  : 'border-slate-200 bg-white hover:border-slate-300'
              }`}
            >
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-2">
                  <AlertTriangle className="w-5 h-5 text-rose-600" />
                  <h3 className="font-semibold text-slate-900 text-sm">紧急熔断与完全隔离</h3>
                </div>
                {failSafeConfig.mode === 'BLOCK_ALL_EMERGENCY' && (
                  <Badge variant="danger" className="text-[10px]">当前激活</Badge>
                )}
              </div>
              <p className="text-xs text-slate-600 leading-relaxed mb-4">
                <strong>军工级安全模式：</strong>一旦失去中央控制面授权，网关立即清空本地 WireGuard Peer 路由表并阻断所有跨节点流量，只开放带外管理口。
              </p>
              <div className="text-[11px] text-slate-500 bg-slate-50 p-2.5 rounded-lg border border-slate-200 space-y-1">
                <div>✓ 适用场景：极高安全级别数据中心，杜绝任何失去监控的连通</div>
                <div>✕ 风险：导致全网隧道立即中断</div>
              </div>
            </div>
          </div>

          {/* Granular Fail-Safe Parameters Settings */}
          <Card className="border-slate-200">
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <Sliders className="w-4 h-4 text-blue-600" />
                网关离线断开详细参数调优
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    心跳间隔 (秒)
                  </label>
                  <Input
                    type="number"
                    value={failSafeConfig.heartbeatIntervalSec}
                    onChange={(e) =>
                      setFailSafeConfig({
                        ...failSafeConfig,
                        heartbeatIntervalSec: parseInt(e.target.value) || 15,
                      })
                    }
                  />
                  <p className="text-[11px] text-slate-400 mt-1">推荐 10 ~ 30 秒</p>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    失联超时判定 (秒)
                  </label>
                  <Input
                    type="number"
                    value={failSafeConfig.offlineThresholdSec}
                    onChange={(e) =>
                      setFailSafeConfig({
                        ...failSafeConfig,
                        offlineThresholdSec: parseInt(e.target.value) || 60,
                      })
                    }
                  />
                  <p className="text-[11px] text-slate-400 mt-1">超时后立即平滑触发兜底</p>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    本地缓存最长保留 TTL (小时)
                  </label>
                  <Input
                    type="number"
                    value={failSafeConfig.localCacheTtlHours}
                    onChange={(e) =>
                      setFailSafeConfig({
                        ...failSafeConfig,
                        localCacheTtlHours: parseInt(e.target.value) || 72,
                      })
                    }
                  />
                  <p className="text-[11px] text-slate-400 mt-1">超时未连上大脑则强制丢弃</p>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    应急管理员通道 (Emergency SSH)
                  </label>
                  <Select
                    value={failSafeConfig.emergencyAdminBypass ? 'true' : 'false'}
                    onChange={(e) =>
                      setFailSafeConfig({
                        ...failSafeConfig,
                        emergencyAdminBypass: e.target.value === 'true',
                      })
                    }
                  >
                    <option value="true">允许 (本地指定物理端口维护)</option>
                    <option value="false">禁用 (完全禁止任何绕行)</option>
                  </Select>
                  <p className="text-[11px] text-slate-400 mt-1">用于网关失联时运维本地排障</p>
                </div>
              </div>

              <div className="flex justify-end pt-3 border-t border-slate-100">
                <Button
                  onClick={() => handleSaveFailSafe(failSafeConfig)}
                  isLoading={savingFailSafe}
                  className="bg-blue-600 hover:bg-blue-500 text-white text-xs"
                >
                  保存并同步全球网关容灾参数
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* QUICK NAT MAPPING MODAL */}
      <Dialog
        isOpen={isNatDrawerOpen}
        onClose={() => setIsNatDrawerOpen(false)}
        title={`影子网段 DNAT 配置: ${targetNatSite?.name || ''}`}
        description="解决不同公有云或私有云机房之间相同私网网段 (如 192.168.10.0/24) 路由冲突的关键技术"
        maxWidth="lg"
      >
        <div className="space-y-4">
          <div className="p-3 bg-slate-50 border border-slate-200 rounded-lg text-xs space-y-1">
            <div className="flex justify-between">
              <span className="text-slate-500">目标网络站点:</span>
              <strong className="text-slate-900">{targetNatSite?.name}</strong>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">站点内网网段 (VPC Subnet):</span>
              <strong className="font-mono text-blue-700">{targetNatSite?.localSubnet}</strong>
            </div>
          </div>

          <div className="space-y-3">
            <div className="flex items-center justify-between p-3 border rounded-lg bg-white">
              <div>
                <span className="font-semibold text-xs text-slate-800">
                  启用影子网段映射 (仅网段冲突时需要)
                </span>
                <p className="text-[11px] text-slate-500">
                  Gateway 关联网络的出站 SNAT 会自动配置；开启后客户端访问影子网段时，Gateway 再将目标地址转换为站点真实地址。
                </p>
              </div>
              <input
                type="checkbox"
                checked={natForm.enabled}
                onChange={(e) => setNatForm({ ...natForm, enabled: e.target.checked })}
                className="h-4 w-4 text-blue-600 rounded border-slate-300 focus:ring-blue-500"
              />
            </div>

            {natForm.enabled && (
              <>
                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    虚拟影子网段 (Virtual Shadow CIDR) <span className="text-rose-500">*</span>
                  </label>
                  <Input
                    value={natForm.virtualCidr}
                    onChange={(e) => setNatForm({ ...natForm, virtualCidr: e.target.value })}
                    placeholder="例如: 172.31.10.0/24"
                  />
                  <p className="text-[11px] text-slate-400 mt-1">
                    员工电脑在访问此 VPC 时直接输入此网段内的 IP，网关将无缝转换为真实的 {targetNatSite?.localSubnet}
                  </p>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">备注说明</label>
                  <Input
                    value={natForm.description || ''}
                    onChange={(e) => setNatForm({ ...natForm, description: e.target.value })}
                    placeholder="例如: 腾讯云上海区内网与北京阿里云重叠，映射为 172.31.10.0/24"
                  />
                </div>
              </>
            )}
          </div>

          <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-100">
            <Button variant="outline" onClick={() => setIsNatDrawerOpen(false)}>
              取消
            </Button>
            <Button onClick={handleSaveNat} isLoading={savingNat} className="bg-blue-600 hover:bg-blue-500 text-white">
              保存影子映射
            </Button>
          </div>
        </div>
      </Dialog>

      {/* CREATE / EDIT SITE DIALOG */}
      <Dialog
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        title={editingSite ? '修改网络站点' : '新建网络站点'}
        description={editingSite ? '编辑 Gateway 关联与站点状态；站点名称和网段创建后固定。' : '配置本地网络范围及其 WireGuard 汇聚网关'}
        maxWidth="md"
      >
        <form onSubmit={handleSubmit} className="space-y-4">
          {editingSite ? (
            <div className="grid grid-cols-2 gap-3 rounded-lg border border-slate-200 bg-slate-50 p-3">
              <div>
                <div className="text-xs text-slate-500">站点名称</div>
                <div className="mt-1 text-sm font-medium text-slate-800">{editingSite.name}</div>
              </div>
              <div>
                <div className="text-xs text-slate-500">本地子网段</div>
                <div className="mt-1 font-mono text-sm text-slate-800">{editingSite.localSubnet}</div>
              </div>
            </div>
          ) : (
            <>
              <div>
                <label className="block text-xs font-medium text-slate-700 mb-1">
                  站点名称 <span className="text-rose-500">*</span>
                </label>
                <Input
                  value={siteForm.name}
                  onChange={(e) => setSiteForm({ ...siteForm, name: e.target.value })}
                  placeholder="例如: 杭州研发园区"
                  error={formErrors.name}
                />
              </div>

              <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                本地子网段 (CIDR) <span className="text-rose-500">*</span>
              </label>
              <Input
                value={siteForm.localSubnet}
                onChange={(e) => setSiteForm({ ...siteForm, localSubnet: e.target.value })}
                placeholder="例如: 192.168.40.0/24"
                error={formErrors.localSubnet}
              />
              </div>
            </>
          )}

          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1">
              汇聚网关节点
            </label>
            <Select
              value={siteForm.gatewayNodeId}
              onChange={(e) => setSiteForm({ ...siteForm, gatewayNodeId: e.target.value })}
            >
              <option value="">不指定网关（可稍后配置）</option>
              {gatewayNodes.map((gw) => (
                <option key={gw.id} value={String(gw.id)}>
                  {gw.gateway_name} • {gw.status}
                </option>
              ))}
            </Select>
            {formErrors.gatewayNodeId && <p className="mt-1 text-xs text-rose-500">{formErrors.gatewayNodeId}</p>}
          </div>

          {/* Optional shadow mapping can be created together with a site. */}
          {!editingSite && <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-slate-800 flex items-center gap-1.5">
                <ArrowLeftRight className="w-3.5 h-3.5 text-indigo-600" />
                可选：配置网段冲突的影子映射
              </span>
              <input
                type="checkbox"
                disabled={siteForm.status !== 'active'}
                checked={siteForm.natEnabled}
                onChange={(e) => setSiteForm({ ...siteForm, natEnabled: e.target.checked })}
                className="h-4 w-4 text-blue-600 rounded border-slate-300"
              />
            </div>
            <p className="text-[11px] text-slate-500">
              站点关联 Gateway 后，Overlay 客户端访问该站点的出站 SNAT 会自动配置；只有真实网段冲突时才需要设置影子网段。
            </p>
            {formErrors.natEnabled && <p className="text-[11px] text-rose-600">{formErrors.natEnabled}</p>}

            {siteForm.natEnabled && (
              <div className="pt-2 border-t border-slate-200 space-y-2">
                <div>
                  <label className="block text-[11px] font-medium text-slate-600 mb-0.5">
                    虚拟影子网段 (Shadow CIDR)
                  </label>
                  <Input
                    value={siteForm.virtualCidr}
                    onChange={(e) => setSiteForm({ ...siteForm, virtualCidr: e.target.value })}
                    placeholder="172.31.10.0/24"
                    error={formErrors.virtualCidr}
                  />
                </div>
              </div>
            )}
          </div>}

          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1">站点运行状态</label>
            <Select
              value={siteForm.status}
              onChange={(e) => {
                const status = e.target.value as 'active' | 'inactive';
                setSiteForm({ ...siteForm, status, natEnabled: status === 'active' && siteForm.natEnabled });
              }}
            >
              <option value="active">正常活跃 (active)</option>
              <option value="inactive">维护停用 (inactive)</option>
            </Select>
          </div>

          <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-100">
            <Button type="button" variant="outline" onClick={() => setIsModalOpen(false)}>
              取消
            </Button>
            <Button type="submit" isLoading={saving}>
              保存站点
            </Button>
          </div>
        </form>
      </Dialog>

      {/* DELETE CONFIRMATION */}
      <ConfirmDialog
        isOpen={!!siteToDelete}
        onClose={() => setSiteToDelete(null)}
        onConfirm={handleDeleteConfirm}
        title="确认删除站点"
        description={`确定要删除站点 "${siteToDelete?.name}" (${siteToDelete?.localSubnet}) 吗？该站点将从关联 Gateway 配置中移除，自动 SNAT 和影子映射也会一并撤销。`}
        danger={true}
        isLoading={deleting}
      />
    </div>
  );
};
