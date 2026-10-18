import React, { useState, useEffect, useMemo } from 'react';
import {
  Layers,
  Search,
  Plus,
  RefreshCw,
  Server,
  Monitor,
  Shield,
  ShieldAlert,
  ShieldCheck,
  CheckCircle2,
  Clock,
  ArrowRight,
  Copy,
  Check,
  Trash2,
  ExternalLink,
  Sliders,
  Cpu,
  Key,
  Database,
  Network,
  Activity,
  Zap,
  ArrowLeftRight,
  Globe,
  Info,
  ChevronRight,
  Filter,
} from 'lucide-react';
import { mockApi } from '../data/mock';
import { IpamPool, IpamLease, OverlayNetwork, IpAllocationResult } from '../types';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { Badge } from '../components/ui/badge';
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
import { formatRelativeTime } from '../utils/format';

export const Ipam: React.FC = () => {
  const { success, error, info } = useToast();

  const [overlayData, setOverlayData] = useState<OverlayNetwork | null>(null);
  const [pools, setPools] = useState<IpamPool[]>([]);
  const [leases, setLeases] = useState<IpamLease[]>([]);
  const [loading, setLoading] = useState(true);

  // Filters & Tabs
  const [activeTab, setActiveTab] = useState<'pools' | 'leases' | 'heatmap' | 'conflict' | 'nat'>('pools');
  const [search, setSearch] = useState('');
  const [poolFilter, setPoolFilter] = useState('all');
  const [typeFilter, setTypeFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');

  // Allocation Wizard Modal
  const [isAllocWizardOpen, setIsAllocWizardOpen] = useState(false);
  const [allocating, setAllocating] = useState(false);
  const [allocForm, setAllocForm] = useState({
    target_name: '',
    node_type: 'Peer' as 'Gateway' | 'Peer' | 'Server',
    pool_id: 'pool-dev',
    idp_group: '研发核心组 (飞书)',
    wireguard_pubkey: '',
    endpoint: '',
    notes: '',
  });
  const [allocResult, setAllocResult] = useState<IpAllocationResult | null>(null);
  const [copiedConfig, setCopiedConfig] = useState(false);

  // Create / Edit Subnet Pool Modal
  const [isPoolModalOpen, setIsPoolModalOpen] = useState(false);
  const [editingPool, setEditingPool] = useState<IpamPool | null>(null);
  const [poolForm, setPoolForm] = useState({
    pool_name: '',
    cidr: '',
    type: 'User-Device' as 'Gateway' | 'User-Device' | 'Service-VIP',
    allocation_mode: 'dynamic' as 'static' | 'dynamic',
    idp_group: '',
    gateway_binding: '',
    color: '#10b981',
    description: '',
    total: 4094,
  });
  const [savingPool, setSavingPool] = useState(false);

  // Release Lease Confirmation
  const [leaseToRelease, setLeaseToRelease] = useState<IpamLease | null>(null);
  const [isReleaseConfirmOpen, setIsReleaseConfirmOpen] = useState(false);
  const [releasing, setReleasing] = useState(false);

  // View WG Config Modal
  const [selectedLeaseConfig, setSelectedLeaseConfig] = useState<IpamLease | null>(null);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  // Conflict Checker State
  const [conflictInput, setConflictInput] = useState('10.100.60.0/24');
  const [checkingConflict, setCheckingConflict] = useState(false);
  const [conflictResult, setConflictResult] = useState<{
    isValid: boolean;
    hasConflict: boolean;
    message: string;
    conflictTarget?: string;
  } | null>(null);

  // Heatmap Subnet Selector
  const [heatmapPoolId, setHeatmapPoolId] = useState('pool-dev');

  const fetchData = async () => {
    try {
      setLoading(true);
      const network = await mockApi.getOverlayNetwork();
      setOverlayData(network);
      setPools(network.ipam_pools);
      setLeases(network.active_leases);
    } catch (err: any) {
      error(err.message || '加载网络 IPAM 数据失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  // Filtered Leases
  const filteredLeases = useMemo(() => {
    return leases.filter((l) => {
      const matchSearch =
        search === '' ||
        l.virtual_ip.toLowerCase().includes(search.toLowerCase()) ||
        l.target_name.toLowerCase().includes(search.toLowerCase()) ||
        l.wireguard_pubkey.toLowerCase().includes(search.toLowerCase()) ||
        l.endpoint.toLowerCase().includes(search.toLowerCase());

      const matchPool = poolFilter === 'all' || l.pool_id === poolFilter;
      const matchType = typeFilter === 'all' || l.node_type === typeFilter;
      const matchStatus = statusFilter === 'all' || l.status === statusFilter;

      return matchSearch && matchPool && matchType && matchStatus;
    });
  }, [leases, search, poolFilter, typeFilter, statusFilter]);

  // Handle Allocation
  const handleRunAllocation = async () => {
    if (!allocForm.target_name.trim()) {
      error('请填写接入的目标设备或节点名称');
      return;
    }
    try {
      setAllocating(true);
      const res = await mockApi.allocateIp(allocForm);
      setAllocResult(res);
      success(`成功为 [${allocForm.target_name}] 分配虚拟 IP: ${res.allocated_ip}`);
      await fetchData();
    } catch (err: any) {
      error(err.message || '分配失败');
    } finally {
      setAllocating(false);
    }
  };

  // Reset Allocation Wizard
  const resetAllocWizard = () => {
    setAllocResult(null);
    setAllocForm({
      target_name: '',
      node_type: 'Peer',
      pool_id: 'pool-dev',
      idp_group: '研发核心组 (飞书)',
      wireguard_pubkey: '',
      endpoint: '',
      notes: '',
    });
    setCopiedConfig(false);
  };

  // Handle Save Pool
  const handleSavePool = async () => {
    if (!poolForm.pool_name.trim() || !poolForm.cidr.trim()) {
      error('请完整填写子网池名称与 CIDR 网段');
      return;
    }
    try {
      setSavingPool(true);
      if (editingPool) {
        await mockApi.updateIpamPool(editingPool.id, poolForm);
        success('子网池已更新');
      } else {
        await mockApi.createIpamPool({
          ...poolForm,
          status: 'Active',
        });
        success('新建子网池成功');
      }
      setIsPoolModalOpen(false);
      setEditingPool(null);
      await fetchData();
    } catch (err: any) {
      error(err.message || '保存失败');
    } finally {
      setSavingPool(false);
    }
  };

  // Handle Release Lease
  const handleConfirmRelease = async () => {
    if (!leaseToRelease) return;
    try {
      setReleasing(true);
      await mockApi.releaseIpLease(leaseToRelease.id);
      success(`已回收并释放虚拟 IP: ${leaseToRelease.virtual_ip}`);
      setIsReleaseConfirmOpen(false);
      setLeaseToRelease(null);
      await fetchData();
    } catch (err: any) {
      error(err.message || '释放租约失败');
    } finally {
      setReleasing(false);
    }
  };

  // Run Conflict Check
  const handleRunConflictCheck = async () => {
    try {
      setCheckingConflict(true);
      const res = await mockApi.checkIpConflict(conflictInput);
      setConflictResult(res);
    } catch (err: any) {
      error(err.message || '冲突检测失败');
    } finally {
      setCheckingConflict(false);
    }
  };

  // Copy helper
  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(id);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  // Generate mock heatmap grid items for the selected pool
  const heatmapData = useMemo(() => {
    const selectedPool = pools.find((p) => p.id === heatmapPoolId);
    if (!selectedPool) return [];

    const cidrParts = selectedPool.cidr.split('/')[0].split('.');
    const prefix = `${cidrParts[0]}.${cidrParts[1]}.${cidrParts[2]}`;

    // Map allocated leases in this pool
    const poolLeases = leases.filter((l) => l.pool_id === selectedPool.id);
    const leaseMap = new Map<string, IpamLease>();
    poolLeases.forEach((l) => leaseMap.set(l.virtual_ip, l));

    // Display first 120 IP blocks in this subnet for visual heat matrix
    const items = [];
    const count = selectedPool.total < 254 ? selectedPool.total : 120;

    for (let i = 1; i <= count; i++) {
      const ip = `${prefix}.${i}`;
      const lease = leaseMap.get(ip);
      items.push({
        ip,
        hostNum: i,
        occupied: !!lease,
        lease,
        poolColor: selectedPool.color,
      });
    }
    return items;
  }, [pools, leases, heatmapPoolId]);

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">
            虚拟网段与 IPAM 管理
          </h1>
          <p className="text-xs text-slate-500 mt-0.5">
            企业全局 Overlay 网段层级规划：网段划分、业务子网池管理与动态 IP 租约
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={fetchData}
            disabled={loading}
            className="gap-1.5"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            <span>刷新</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setActiveTab('conflict');
            }}
            className="gap-1.5"
          >
            <ShieldCheck className="h-3.5 w-3.5 text-blue-600" />
            <span>冲突检测</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setEditingPool(null);
              setPoolForm({
                pool_name: '',
                cidr: '10.100.60.0/20',
                type: 'User-Device',
                allocation_mode: 'dynamic',
                idp_group: '',
                gateway_binding: '',
                color: '#8b5cf6',
                description: '',
                total: 4094,
              });
              setIsPoolModalOpen(true);
            }}
            className="gap-1.5"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>新建子网池</span>
          </Button>

          <Button
            size="sm"
            onClick={() => {
              resetAllocWizard();
              setIsAllocWizardOpen(true);
            }}
            className="gap-1.5 bg-blue-600 hover:bg-blue-700 text-white"
          >
            <Zap className="h-3.5 w-3.5" />
            <span>IP 分配向导</span>
          </Button>
        </div>
      </div>

      {/* Metrics Row: Three-tier Architecture & Capacity */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Metric 1: Global Supernet */}
        <Card className="border-slate-200 bg-white shadow-sm hover:border-blue-400 transition-colors">
          <CardContent className="p-4 space-y-2">
            <div className="flex items-center justify-between text-xs text-slate-500">
              <span className="font-semibold text-slate-700">全局 Overlay 超级大网段</span>
              <Badge variant="outline" className="text-[10px] font-mono">
                Class A 私有
              </Badge>
            </div>
            <div className="flex items-baseline justify-between">
              <div className="text-2xl font-bold font-mono text-slate-900">
                {overlayData?.global_cidr || '10.100.0.0/16'}
              </div>
              <span className="text-xs text-slate-500 font-mono">
                总容量: {overlayData?.total_ips.toLocaleString() || '65,534'} IP
              </span>
            </div>
            <div className="space-y-1">
              <div className="flex justify-between text-[11px] text-slate-500">
                <span>
                  已分配: <strong className="text-blue-600">{overlayData?.used_ips || 0}</strong>
                </span>
                <span>
                  空闲: <strong className="text-emerald-600">{overlayData?.free_ips.toLocaleString() || 0}</strong>
                </span>
              </div>
              <div className="h-1.5 w-full bg-slate-100 rounded-full overflow-hidden">
                <div
                  className="h-full bg-blue-600 rounded-full transition-all duration-500"
                  style={{
                    width: `${Math.max(1, Math.min(100, (((overlayData?.used_ips || 0) / (overlayData?.total_ips || 65534)) * 100)))}%`,
                  }}
                />
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Metric 2: Gateway Hubs Pool */}
        <Card className="border-slate-200 bg-white shadow-sm hover:border-blue-400 transition-colors">
          <CardContent className="p-4 space-y-2">
            <div className="flex items-center justify-between text-xs text-slate-500">
              <span className="font-semibold text-blue-700 flex items-center gap-1">
                <Server className="h-3.5 w-3.5" />
                骨干网关专用池 (Hubs)
              </span>
              <span className="text-[10px] bg-blue-50 text-blue-600 px-1.5 py-0.5 rounded font-mono font-medium">
                10.100.0.0/24
              </span>
            </div>
            <div className="flex items-baseline justify-between">
              <div className="text-2xl font-bold font-mono text-blue-700">4 / 254</div>
              <span className="text-xs text-emerald-600 font-medium">有序静态分配</span>
            </div>
            <p className="text-[11px] text-slate-500 leading-relaxed">
              固定保留前254个IP供全球骨干网关及 HA 漂移 VIP 锁定使用。
            </p>
          </CardContent>
        </Card>

        {/* Metric 3: User Spoke Pools */}
        <Card className="border-slate-200 bg-white shadow-sm hover:border-blue-400 transition-colors">
          <CardContent className="p-4 space-y-2">
            <div className="flex items-center justify-between text-xs text-slate-500">
              <span className="font-semibold text-emerald-700 flex items-center gap-1">
                <Monitor className="h-3.5 w-3.5" />
                终端设备动态池 (Spokes)
              </span>
              <span className="text-[10px] bg-emerald-50 text-emerald-600 px-1.5 py-0.5 rounded font-mono font-medium">
                /20 按部门隔离
              </span>
            </div>
            <div className="flex items-baseline justify-between">
              <div className="text-2xl font-bold font-mono text-emerald-700">208 / 12,282</div>
              <span className="text-xs text-emerald-600 font-medium">IdP 动态终身锁定</span>
            </div>
            <p className="text-[11px] text-slate-500 leading-relaxed">
              飞书/Okta认证首次入网时按安全组动态匹配并与公钥指纹绑定。
            </p>
          </CardContent>
        </Card>

        {/* Metric 4: Conflict & Compliance Guard */}
        <Card className="border-slate-200 bg-white shadow-sm hover:border-blue-400 transition-colors">
          <CardContent className="p-4 space-y-2">
            <div className="flex items-center justify-between text-xs text-slate-500">
              <span className="font-semibold text-purple-700 flex items-center gap-1">
                <ShieldCheck className="h-3.5 w-3.5 text-purple-600" />
                VPC & 物理网段防重叠审计
              </span>
              <span className="text-[10px] bg-purple-50 text-purple-600 px-1.5 py-0.5 rounded font-mono font-medium">
                零冲突保障
              </span>
            </div>
            <div className="flex items-baseline justify-between">
              <div className="text-2xl font-bold font-mono text-purple-700">0 冲突重叠</div>
              <span className="text-xs text-emerald-600 font-medium flex items-center gap-1">
                <CheckCircle2 className="h-3 w-3" />
                合规运行中
              </span>
            </div>
            <p className="text-[11px] text-slate-500 leading-relaxed">
              实时比对阿里云 VPC (172.16)、腾讯云 (10.200) 与机房物理网段。
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Navigation Tabs */}
      <div className="flex items-center border-b border-slate-200">
        <button
          onClick={() => setActiveTab('pools')}
          className={`py-2.5 px-4 text-xs font-semibold border-b-2 transition-all flex items-center gap-2 ${
            activeTab === 'pools'
              ? 'border-blue-600 text-blue-600'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          }`}
        >
          <Layers className="h-4 w-4" />
          <span>业务子网池</span>
          <span className="text-[11px] font-normal text-slate-400">({pools.length})</span>
        </button>

        <button
          onClick={() => setActiveTab('leases')}
          className={`py-2.5 px-4 text-xs font-semibold border-b-2 transition-all flex items-center gap-2 ${
            activeTab === 'leases'
              ? 'border-blue-600 text-blue-600'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          }`}
        >
          <Activity className="h-4 w-4" />
          <span>虚拟 IP 租约</span>
          <span className="text-[11px] font-normal text-slate-400">({leases.length})</span>
        </button>

        <button
          onClick={() => setActiveTab('heatmap')}
          className={`py-2.5 px-4 text-xs font-semibold border-b-2 transition-all flex items-center gap-2 ${
            activeTab === 'heatmap'
              ? 'border-blue-600 text-blue-600'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          }`}
        >
          <Network className="h-4 w-4" />
          <span>网段热力图</span>
        </button>

        <button
          onClick={() => setActiveTab('conflict')}
          className={`py-2.5 px-4 text-xs font-semibold border-b-2 transition-all flex items-center gap-2 ${
            activeTab === 'conflict'
              ? 'border-blue-600 text-blue-600'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          }`}
        >
          <ShieldAlert className="h-4 w-4" />
          <span>冲突探测</span>
        </button>

        <button
          onClick={() => setActiveTab('nat')}
          className={`py-2.5 px-4 text-xs font-semibold border-b-2 transition-all flex items-center gap-2 ${
            activeTab === 'nat'
              ? 'border-blue-600 text-blue-600'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          }`}
        >
          <ArrowLeftRight className="h-4 w-4 text-slate-500" />
          <span>NAT 映射规则</span>
        </button>
      </div>

      {/* TAB 1: SUBNETTING POOLS */}
      {activeTab === 'pools' && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {pools.map((pool) => {
              const utilPercent = Math.min(100, Math.round((pool.used / pool.total) * 100));
              return (
                <Card
                  key={pool.id}
                  className="bg-white border-slate-200 shadow-sm hover:shadow-md transition-all flex flex-col justify-between"
                >
                  <CardHeader className="p-5 pb-3">
                    <div className="flex items-start justify-between">
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <span
                            className="h-3 w-3 rounded-full shrink-0"
                            style={{ backgroundColor: pool.color }}
                          />
                          <h3 className="font-bold text-sm text-slate-900 tracking-tight">
                            {pool.pool_name}
                          </h3>
                        </div>
                        <p className="text-xs font-mono text-slate-500 flex items-center gap-1.5">
                          CIDR: <strong className="text-slate-800 font-semibold">{pool.cidr}</strong>
                        </p>
                      </div>
                      <Badge
                        variant={
                          pool.type === 'Gateway'
                            ? 'primary'
                            : pool.type === 'Service-VIP'
                            ? 'slate'
                            : 'success'
                        }
                        className="text-[10px]"
                      >
                        {pool.type}
                      </Badge>
                    </div>
                  </CardHeader>

                  <CardContent className="p-5 pt-0 space-y-4">
                    <p className="text-xs text-slate-600 min-h-[36px] leading-relaxed">
                      {pool.description || '无详细描述'}
                    </p>

                    {/* Meta tags */}
                    <div className="space-y-1.5 text-xs">
                      <div className="flex items-center justify-between text-slate-500">
                        <span>分配策略模式:</span>
                        <span className="font-medium text-slate-700">
                          {pool.allocation_mode === 'static' ? '静态有序锁定 (网关/VIP)' : '动态租约 (IdP触发)'}
                        </span>
                      </div>
                      {pool.idp_group && (
                        <div className="flex items-center justify-between text-slate-500">
                          <span>绑定身份用户组:</span>
                          <span className="font-semibold text-blue-600 bg-blue-50 px-1.5 py-0.5 rounded text-[11px]">
                            {pool.idp_group}
                          </span>
                        </div>
                      )}
                      {pool.gateway_binding && (
                        <div className="flex items-center justify-between text-slate-500">
                          <span>绑定锚点网关:</span>
                          <span className="font-semibold text-slate-700 text-[11px]">
                            {pool.gateway_binding}
                          </span>
                        </div>
                      )}
                    </div>

                    {/* Progress Bar */}
                    <div className="space-y-1.5 pt-2 border-t border-slate-100">
                      <div className="flex items-center justify-between text-xs">
                        <span className="text-slate-500">IP 容量占用:</span>
                        <span className="font-mono text-slate-800 font-semibold">
                          {pool.used} / {pool.total} ({utilPercent}%)
                        </span>
                      </div>
                      <div className="h-2 w-full bg-slate-100 rounded-full overflow-hidden">
                        <div
                          className="h-full rounded-full transition-all duration-300"
                          style={{
                            width: `${Math.max(2, utilPercent)}%`,
                            backgroundColor: pool.color,
                          }}
                        />
                      </div>
                    </div>

                    {/* Actions */}
                    <div className="flex items-center justify-between pt-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setPoolFilter(pool.id);
                          setActiveTab('leases');
                        }}
                        className="text-xs h-7 text-slate-600 hover:text-blue-600 hover:border-blue-400"
                      >
                        查看租约列表 ({pool.used})
                      </Button>

                      <div className="flex items-center gap-1.5">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            setEditingPool(pool);
                            setPoolForm({
                              pool_name: pool.pool_name,
                              cidr: pool.cidr,
                              type: pool.type,
                              allocation_mode: pool.allocation_mode,
                              idp_group: pool.idp_group || '',
                              gateway_binding: pool.gateway_binding || '',
                              color: pool.color,
                              description: pool.description || '',
                              total: pool.total,
                            });
                            setIsPoolModalOpen(true);
                          }}
                          className="h-7 px-2 text-xs text-slate-500 hover:text-slate-900"
                        >
                          编辑
                        </Button>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        </div>
      )}

      {/* TAB 2: ACTIVE LEASES */}
      {activeTab === 'leases' && (
        <div className="space-y-4">
          {/* Filters Bar */}
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-3 bg-white p-4 rounded-xl border border-slate-200 shadow-sm">
            <div className="flex flex-wrap items-center gap-2.5 flex-1">
              <div className="relative w-full md:w-64">
                <Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
                <Input
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="搜索虚拟 IP、设备名、公钥..."
                  className="pl-9 h-9 text-xs"
                />
              </div>

              <div className="w-40">
                <Select
                  value={poolFilter}
                  onChange={(e) => setPoolFilter(e.target.value)}
                  className="h-9 text-xs"
                >
                  <option value="all">全部子网池</option>
                  {pools.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.pool_name}
                    </option>
                  ))}
                </Select>
              </div>

              <div className="w-32">
                <Select
                  value={typeFilter}
                  onChange={(e) => setTypeFilter(e.target.value)}
                  className="h-9 text-xs"
                >
                  <option value="all">全部节点类型</option>
                  <option value="Gateway">网关 (Gateway)</option>
                  <option value="Peer">终端 (Peer)</option>
                  <option value="Server">服务 (Server)</option>
                </Select>
              </div>

              <div className="w-32">
                <Select
                  value={statusFilter}
                  onChange={(e) => setStatusFilter(e.target.value)}
                  className="h-9 text-xs"
                >
                  <option value="all">全部租约状态</option>
                  <option value="Active">活跃 (Active)</option>
                  <option value="Idle">闲置 (Idle)</option>
                  <option value="Reserved">静态保留 (Reserved)</option>
                </Select>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Button
                variant="primary"
                size="sm"
                onClick={() => {
                  resetAllocWizard();
                  setIsAllocWizardOpen(true);
                }}
                className="h-9 text-xs"
              >
                <Plus className="h-3.5 w-3.5 mr-1" />
                新增分配 IP
              </Button>
            </div>
          </div>

          {/* Leases Table */}
          <Card className="border-slate-200 bg-white shadow-sm overflow-hidden">
            <div className="overflow-x-auto">
              <Table>
                <TableHeader className="bg-slate-50">
                  <TableRow>
                    <TableHead className="w-36 text-xs font-semibold text-slate-700">虚拟 IP (Virtual IP)</TableHead>
                    <TableHead className="text-xs font-semibold text-slate-700">目标设备 / 节点名称</TableHead>
                    <TableHead className="w-28 text-xs font-semibold text-slate-700">节点类型</TableHead>
                    <TableHead className="w-40 text-xs font-semibold text-slate-700">自动化分配模式</TableHead>
                    <TableHead className="w-44 text-xs font-semibold text-slate-700">WireGuard 公钥指纹</TableHead>
                    <TableHead className="w-40 text-xs font-semibold text-slate-700">所属子网池</TableHead>
                    <TableHead className="w-24 text-xs font-semibold text-slate-700">租约状态</TableHead>
                    <TableHead className="w-28 text-xs font-semibold text-slate-700 text-right">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredLeases.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={8} className="py-12 text-center">
                        <EmptyState
                          icon={Layers}
                          title="未找到匹配的 IP 租约"
                          description="尝试更改搜索条件，或点击右上角执行自动化 IP 分配"
                        />
                      </TableCell>
                    </TableRow>
                  ) : (
                    filteredLeases.map((lease) => (
                      <TableRow key={lease.id} className="hover:bg-slate-50/80 transition-colors">
                        {/* Virtual IP */}
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <span
                              className={`h-2 w-2 rounded-full ${
                                lease.status === 'Active'
                                  ? 'bg-emerald-500 animate-pulse'
                                  : lease.status === 'Reserved'
                                  ? 'bg-blue-500'
                                  : 'bg-amber-400'
                              }`}
                            />
                            <span className="font-mono font-bold text-xs text-slate-900">
                              {lease.virtual_ip}
                            </span>
                          </div>
                        </TableCell>

                        {/* Name & Endpoint */}
                        <TableCell>
                          <div className="space-y-0.5">
                            <div className="font-semibold text-xs text-slate-800 flex items-center gap-1.5">
                              {lease.target_name}
                            </div>
                            <div className="text-[11px] font-mono text-slate-400">
                              端点: {lease.endpoint}
                            </div>
                          </div>
                        </TableCell>

                        {/* Node Type */}
                        <TableCell>
                          <Badge
                            variant={
                              lease.node_type === 'Gateway'
                                ? 'primary'
                                : lease.node_type === 'Server'
                                ? 'slate'
                                : 'default'
                            }
                            className="text-[10px]"
                          >
                            {lease.node_type}
                          </Badge>
                        </TableCell>

                        {/* Allocation Mode */}
                        <TableCell>
                          <span
                            className={`text-[11px] px-2 py-0.5 rounded font-medium ${
                              lease.allocation_mode.includes('飞书')
                                ? 'bg-sky-50 text-sky-700 border border-sky-200'
                                : lease.allocation_mode.includes('钉钉')
                                ? 'bg-indigo-50 text-indigo-700 border border-indigo-200'
                                : lease.allocation_mode.includes('固定')
                                ? 'bg-blue-50 text-blue-700 border border-blue-200'
                                : 'bg-slate-100 text-slate-700'
                            }`}
                          >
                            {lease.allocation_mode}
                          </span>
                        </TableCell>

                        {/* WireGuard Pubkey */}
                        <TableCell>
                          <div className="flex items-center gap-1.5">
                            <span className="font-mono text-[11px] text-slate-600 truncate max-w-[120px]">
                              {lease.wireguard_pubkey}
                            </span>
                            <button
                              onClick={() => handleCopy(lease.wireguard_pubkey, lease.id)}
                              className="text-slate-400 hover:text-slate-700"
                              title="复制公钥"
                            >
                              {copiedKey === lease.id ? (
                                <Check className="h-3.5 w-3.5 text-emerald-600" />
                              ) : (
                                <Copy className="h-3.5 w-3.5" />
                              )}
                            </button>
                          </div>
                        </TableCell>

                        {/* Pool Name */}
                        <TableCell>
                          <span className="text-xs text-slate-600 font-medium">
                            {lease.pool_name}
                          </span>
                        </TableCell>

                        {/* Status */}
                        <TableCell>
                          <Badge
                            variant={
                              lease.status === 'Active'
                                ? 'success'
                                : lease.status === 'Reserved'
                                ? 'primary'
                                : 'warning'
                            }
                            className="text-[10px]"
                          >
                            {lease.status === 'Active' ? '活跃租约' : lease.status === 'Reserved' ? '永久保留' : '闲置'}
                          </Badge>
                        </TableCell>

                        {/* Action Buttons */}
                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-1">
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => setSelectedLeaseConfig(lease)}
                              className="h-7 px-2 text-xs text-blue-600 hover:bg-blue-50"
                              title="查看 WireGuard 客户端配置文件"
                            >
                              配置
                            </Button>

                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => {
                                setLeaseToRelease(lease);
                                setIsReleaseConfirmOpen(true);
                              }}
                              className="h-7 px-2 text-xs text-red-600 hover:bg-red-50"
                              title="回收并释放该 IP"
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          </Card>
        </div>
      )}

      {/* TAB 3: VISUAL HEATMAP */}
      {activeTab === 'heatmap' && (
        <div className="space-y-4">
          <Card className="border-slate-200 bg-white shadow-sm p-6 space-y-5">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-4 border-b border-slate-100">
              <div>
                <h3 className="text-sm font-bold text-slate-900">
                  子网 IP 地址空间矩阵分布图 (Visual IP Heatmap Matrix)
                </h3>
                <p className="text-xs text-slate-500">
                  以块状网格直观展现每个子网的 IP 占用热度，鼠标悬停可查看 IP 租约绑定的节点与公钥
                </p>
              </div>

              {/* Subnet selector */}
              <div className="flex items-center gap-3">
                <span className="text-xs text-slate-600 font-medium">查看子网池:</span>
                <Select
                  value={heatmapPoolId}
                  onChange={(e) => setHeatmapPoolId(e.target.value)}
                  className="w-56 h-8 text-xs"
                >
                  {pools.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.pool_name} ({p.cidr})
                    </option>
                  ))}
                </Select>
              </div>
            </div>

            {/* Legend */}
            <div className="flex flex-wrap items-center gap-4 text-xs bg-slate-50 p-3 rounded-lg border border-slate-100">
              <span className="font-semibold text-slate-700">图例说明:</span>
              <div className="flex items-center gap-1.5">
                <span className="h-3 w-3 rounded bg-blue-600" />
                <span className="text-slate-600">骨干网关占用 (Gateway)</span>
              </div>
              <div className="flex items-center gap-1.5">
                <span className="h-3 w-3 rounded bg-emerald-500" />
                <span className="text-slate-600">研发终端占用 (Spoke)</span>
              </div>
              <div className="flex items-center gap-1.5">
                <span className="h-3 w-3 rounded bg-purple-600" />
                <span className="text-slate-600">安全运维占用 (SRE)</span>
              </div>
              <div className="flex items-center gap-1.5">
                <span className="h-3 w-3 rounded bg-cyan-500" />
                <span className="text-slate-600">VIP 浮动保留 (HA)</span>
              </div>
              <div className="flex items-center gap-1.5">
                <span className="h-3 w-3 rounded bg-slate-200 border border-slate-300" />
                <span className="text-slate-600">空闲可用 (Free)</span>
              </div>
            </div>

            {/* Heatmap Grid */}
            <div className="grid grid-cols-6 sm:grid-cols-10 md:grid-cols-15 lg:grid-cols-20 gap-1.5">
              {heatmapData.map((item) => (
                <div
                  key={item.ip}
                  className={`group relative h-9 rounded flex items-center justify-center font-mono text-[10px] font-bold cursor-pointer transition-all transform hover:scale-110 hover:z-20 ${
                    item.occupied
                      ? 'text-white shadow-sm'
                      : 'bg-slate-100 text-slate-400 hover:bg-blue-50 hover:text-blue-600 border border-slate-200/60'
                  }`}
                  style={{
                    backgroundColor: item.occupied ? item.poolColor : undefined,
                  }}
                >
                  .{item.hostNum}

                  {/* Tooltip on hover */}
                  <div className="opacity-0 group-hover:opacity-100 pointer-events-none transition-opacity absolute bottom-full mb-2 left-1/2 -translate-x-1/2 w-52 bg-slate-900 text-white p-2.5 rounded-lg shadow-2xl text-[11px] z-50">
                    <div className="font-bold font-mono text-blue-300 border-b border-slate-700 pb-1 mb-1">
                      {item.ip}
                    </div>
                    {item.occupied && item.lease ? (
                      <div className="space-y-1">
                        <div>
                          节点: <strong className="text-white">{item.lease.target_name}</strong>
                        </div>
                        <div>
                          类型: <span className="text-slate-300">{item.lease.node_type}</span>
                        </div>
                        <div>
                          模式: <span className="text-emerald-400">{item.lease.allocation_mode}</span>
                        </div>
                        <div className="truncate text-[10px] text-slate-400 font-mono">
                          端点: {item.lease.endpoint}
                        </div>
                      </div>
                    ) : (
                      <div className="text-emerald-400 font-medium">✓ 空闲可用，可通过向导分配</div>
                    )}
                  </div>
                </div>
              ))}
            </div>

            <p className="text-[11px] text-slate-400 text-center pt-2">
              展示前 {heatmapData.length} 个 IP 地址块。绿色/彩色块表示已绑定 WireGuard 终端，灰色块表示可供分配的空闲池。
            </p>
          </Card>
        </div>
      )}

      {/* TAB 4: VPC CONFLICT ENGINE */}
      {activeTab === 'conflict' && (
        <div className="space-y-4">
          <Card className="border-slate-200 bg-white shadow-sm p-6 space-y-6">
            <div>
              <h3 className="text-sm font-bold text-slate-900 flex items-center gap-2">
                <ShieldCheck className="h-4 w-4 text-purple-600" />
                VPC & 物理机房网段防重叠审计引擎 (VPC Overlap Guard)
              </h3>
              <p className="text-xs text-slate-500 mt-1">
                在将新网段加入 SD-WAN / ZTNA Overlay 之前，进行毫秒级自动碰撞检测，防止与企业云端 VPC (阿里云、腾讯云、AWS) 或物理办公内网产生非对称路由黑洞。
              </p>
            </div>

            {/* Existing Infrastructure Reference */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3 bg-slate-50 p-4 rounded-xl border border-slate-200 text-xs">
              <div className="space-y-1">
                <span className="font-semibold text-slate-700">阿里云华东生产 VPC:</span>
                <div className="font-mono text-slate-600">172.16.0.0/12</div>
                <span className="text-[10px] text-emerald-600">已互通 • 路由隔离保护</span>
              </div>
              <div className="space-y-1">
                <span className="font-semibold text-slate-700">腾讯云华南测试 VPC:</span>
                <div className="font-mono text-slate-600">10.200.0.0/16</div>
                <span className="text-[10px] text-emerald-600">已互通 • 路由隔离保护</span>
              </div>
              <div className="space-y-1">
                <span className="font-semibold text-slate-700">北京总部及研发机房 LAN:</span>
                <div className="font-mono text-slate-600">192.168.0.0/16 (10/20)</div>
                <span className="text-[10px] text-emerald-600">物理锚点 • 严格避免冲突</span>
              </div>
            </div>

            {/* Input form */}
            <div className="space-y-3">
              <label className="text-xs font-semibold text-slate-700">
                输入待规划测试网段 CIDR (例如 10.100.60.0/24 或测试重叠 172.16.10.0/24):
              </label>
              <div className="flex gap-3">
                <Input
                  value={conflictInput}
                  onChange={(e) => setConflictInput(e.target.value)}
                  placeholder="如 10.100.70.0/20"
                  className="font-mono text-sm max-w-md"
                />
                <Button
                  variant="primary"
                  onClick={handleRunConflictCheck}
                  disabled={checkingConflict}
                  className="bg-purple-600 hover:bg-purple-500 text-white"
                >
                  <RefreshCw className={`h-3.5 w-3.5 mr-1.5 ${checkingConflict ? 'animate-spin' : ''}`} />
                  执行合规碰撞检测
                </Button>
              </div>
            </div>

            {/* Result Box */}
            {conflictResult && (
              <div
                className={`p-4 rounded-xl border transition-all ${
                  conflictResult.hasConflict
                    ? 'bg-red-50 border-red-200 text-red-900'
                    : 'bg-emerald-50 border-emerald-200 text-emerald-900'
                }`}
              >
                <div className="flex items-start gap-3">
                  {conflictResult.hasConflict ? (
                    <ShieldAlert className="h-5 w-5 text-red-600 shrink-0 mt-0.5" />
                  ) : (
                    <CheckCircle2 className="h-5 w-5 text-emerald-600 shrink-0 mt-0.5" />
                  )}
                  <div className="space-y-1">
                    <h4 className="font-bold text-sm">
                      {conflictResult.hasConflict ? '检测到路由网段重叠风险 (Conflict Alert)' : '规划合规审计通过 (Passed)'}
                    </h4>
                    <p className="text-xs leading-relaxed">{conflictResult.message}</p>
                  </div>
                </div>
              </div>
            )}
          </Card>
        </div>
      )}

      {/* TAB 5: VPC OVERLAP NAT ENGINE */}
      {activeTab === 'nat' && (
        <div className="space-y-4">
          <Card className="border-indigo-200 bg-white shadow-sm p-6 space-y-6">
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
              <div>
                <h3 className="text-sm font-bold text-slate-900 flex items-center gap-2">
                  <ArrowLeftRight className="h-4 w-4 text-indigo-600" />
                  跨云重叠网段 NAT 映射引擎 (VPC Overlap NAT Engine)
                </h3>
                <p className="text-xs text-slate-500 mt-1">
                  解决多公有云（如阿里云、腾讯云、AWS）真实内网网段冲突问题。为重叠 VPC 映射虚拟影子网段，由 WireGuard 云网关在隧道出口处执行双向 1:1 NAT 动态转换。
                </p>
              </div>

              <Button
                variant="outline"
                size="sm"
                onClick={() => window.location.href = '#/sites'}
                className="text-xs border-indigo-200 text-indigo-700 hover:bg-indigo-50"
              >
                <Globe className="w-3.5 h-3.5 mr-1" />
                前往物理站点与云 VPC 管理
              </Button>
            </div>

            {/* Architecture Explainer */}
            <div className="p-4 rounded-xl bg-gradient-to-r from-indigo-50/70 via-blue-50/70 to-purple-50/70 border border-indigo-200 text-xs space-y-3">
              <div className="font-semibold text-indigo-950 flex items-center gap-1.5">
                <ShieldCheck className="w-4 h-4 text-indigo-600" />
                跨云双向 1:1 NAT 映射数据流图解：
              </div>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3 font-mono text-[11px]">
                <div className="bg-white p-3 rounded-lg border border-indigo-100 shadow-xs space-y-1">
                  <div className="text-slate-400 text-[10px]">步骤 1: 客户端发起请求</div>
                  <div className="font-bold text-slate-800">目标: 172.31.10.5 (影子IP)</div>
                  <p className="text-slate-500 font-sans text-[11px]">员工电脑发起对腾讯云内网测试数据库的访问，填入影子网段 IP</p>
                </div>
                <div className="bg-white p-3 rounded-lg border border-indigo-100 shadow-xs space-y-1">
                  <div className="text-slate-400 text-[10px]">步骤 2: 隧道传输与网关转换</div>
                  <div className="font-bold text-indigo-700">1:1 NAT 动态解包转换</div>
                  <p className="text-slate-500 font-sans text-[11px]">流量到达腾讯云网关后，网关 iptables 自动将目的 IP 转为真实的 192.168.10.5</p>
                </div>
                <div className="bg-white p-3 rounded-lg border border-indigo-100 shadow-xs space-y-1">
                  <div className="text-slate-400 text-[10px]">步骤 3: 回包反向逆转换</div>
                  <div className="font-bold text-emerald-700">逆向 NAT 封包返回</div>
                  <p className="text-slate-500 font-sans text-[11px]">云端回包到达网关时，网关将源 IP 逆向还原为 172.31.10.5，对端完全无感</p>
                </div>
              </div>
            </div>

            {/* Simulated VPC Mapping Matrix */}
            <div className="space-y-3">
              <h4 className="text-xs font-bold text-slate-800">企业现存多云 VPC 网段映射矩阵台账</h4>
              <div className="border border-slate-200 rounded-xl overflow-hidden">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>云服务商 / 站点</TableHead>
                      <TableHead>真实物理/云 VPC 网段</TableHead>
                      <TableHead>WireGuard 映射影子网段</TableHead>
                      <TableHead>NAT 模式</TableHead>
                      <TableHead>冲突规避状态</TableHead>
                      <TableHead className="text-right">网关规则同步</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow>
                      <TableCell className="font-semibold text-slate-900">
                        <div className="flex items-center gap-2">
                          <Badge variant="outline" className="font-mono text-[10px]">Aliyun</Badge>
                          <span>北京总部核心区</span>
                        </div>
                      </TableCell>
                      <TableCell className="font-mono text-xs font-bold text-blue-700">192.168.10.0/24</TableCell>
                      <TableCell className="font-mono text-xs text-slate-500">192.168.10.0/24 (原生)</TableCell>
                      <TableCell className="text-xs text-slate-600">直通透传 (基准网段)</TableCell>
                      <TableCell>
                        <span className="inline-flex items-center gap-1 text-emerald-700 text-xs font-medium">
                          <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" /> 基准原生无冲突
                        </span>
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs text-slate-500">gw-bj-hub-01 (已加载)</TableCell>
                    </TableRow>

                    <TableRow>
                      <TableCell className="font-semibold text-slate-900">
                        <div className="flex items-center gap-2">
                          <Badge variant="outline" className="font-mono text-[10px]">TencentCloud</Badge>
                          <span>上海研发中心 (重叠)</span>
                        </div>
                      </TableCell>
                      <TableCell className="font-mono text-xs font-bold text-amber-700">
                        192.168.10.0/24 <span className="text-[10px] text-rose-500 font-sans font-normal">(与北京同段)</span>
                      </TableCell>
                      <TableCell className="font-mono text-xs font-bold text-indigo-700">
                        172.31.10.0/24 (影子)
                      </TableCell>
                      <TableCell className="text-xs text-indigo-600 font-medium">双向 1:1 NAT</TableCell>
                      <TableCell>
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-50 text-emerald-700 border border-emerald-200">
                          <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" />
                          已规避重叠 (活跃中)
                        </span>
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs text-emerald-600">gw-sh-branch-01 (已生效)</TableCell>
                    </TableRow>

                    <TableRow>
                      <TableCell className="font-semibold text-slate-900">
                        <div className="flex items-center gap-2">
                          <Badge variant="outline" className="font-mono text-[10px]">IDC</Badge>
                          <span>深圳创新基地</span>
                        </div>
                      </TableCell>
                      <TableCell className="font-mono text-xs font-bold text-slate-800">192.168.30.0/24</TableCell>
                      <TableCell className="font-mono text-xs text-slate-500">192.168.30.0/24 (原生)</TableCell>
                      <TableCell className="text-xs text-slate-600">SNAT 出口伪装</TableCell>
                      <TableCell>
                        <span className="inline-flex items-center gap-1 text-emerald-700 text-xs font-medium">
                          <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" /> 独立网段无冲突
                        </span>
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs text-slate-500">gw-sz-branch-01 (已加载)</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>
            </div>
          </Card>
        </div>
      )}

      {/* MODAL 1: IP ALLOCATION WIZARD (Interactive Allocation Simulator) */}
      <Dialog
        isOpen={isAllocWizardOpen}
        onClose={() => setIsAllocWizardOpen(false)}
        title="自动化虚拟 IP 分配向导 (IPAM Engine)"
        description="选择接入类型与归属组织，IPAM 引擎将自动计算空闲 IP、绑定硬件指纹与 WireGuard 密钥并生成隧道配置"
        maxWidth="2xl"
      >
        <div className="space-y-5">
          {!allocResult ? (
            <div className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">
                    接入节点类型 <span className="text-red-500">*</span>
                  </label>
                  <Select
                    value={allocForm.node_type}
                    onChange={(e) => {
                      const val = e.target.value as any;
                      setAllocForm({
                        ...allocForm,
                        node_type: val,
                        pool_id: val === 'Gateway' ? 'pool-gw' : val === 'Server' ? 'pool-vip' : 'pool-dev',
                      });
                    }}
                    className="text-xs"
                  >
                    <option value="Peer">终端员工设备 (Spoke Peer)</option>
                    <option value="Gateway">云端骨干网关 (Hub Gateway)</option>
                    <option value="Server">核心高可用服务 (Service VIP)</option>
                  </Select>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">
                    设备 / 节点名称 <span className="text-red-500">*</span>
                  </label>
                  <Input
                    value={allocForm.target_name}
                    onChange={(e) => setAllocForm({ ...allocForm, target_name: e.target.value })}
                    placeholder="如: 赵亮-MacBook-Pro / gw-hangzhou-01"
                    className="text-xs"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">
                    分配的目标子网池 <span className="text-red-500">*</span>
                  </label>
                  <Select
                    value={allocForm.pool_id}
                    onChange={(e) => setAllocForm({ ...allocForm, pool_id: e.target.value })}
                    className="text-xs"
                  >
                    {pools.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.pool_name} ({p.cidr})
                      </option>
                    ))}
                  </Select>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">
                    关联身份源或部门 (IdP 映射)
                  </label>
                  <Select
                    value={allocForm.idp_group}
                    onChange={(e) => setAllocForm({ ...allocForm, idp_group: e.target.value })}
                    className="text-xs"
                  >
                    <option value="研发核心组 (飞书)">研发核心组 (飞书集成)</option>
                    <option value="安全运维组 (钉钉)">安全运维组 (钉钉集成)</option>
                    <option value="Global-SRE (Okta)">Global-SRE (Okta 认证域)</option>
                    <option value="产品测试组">产品测试组 (自动化CI/CD)</option>
                    <option value="系统骨干网络">系统骨干网络 (网关专网)</option>
                  </Select>
                </div>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700">
                  WireGuard 客户端公钥 (Public Key)
                  <span className="text-slate-400 font-normal ml-2">留空则由系统自动生成标准Base64密钥</span>
                </label>
                <Input
                  value={allocForm.wireguard_pubkey}
                  onChange={(e) => setAllocForm({ ...allocForm, wireguard_pubkey: e.target.value })}
                  placeholder="留空自动生成，如: wA8bC9dE0fG1hI2jK3lM4nO5pQ6rS7tU8vW9xY0z="
                  className="font-mono text-xs"
                />
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700">
                  物理外网端点 (Endpoint，可选)
                </label>
                <Input
                  value={allocForm.endpoint}
                  onChange={(e) => setAllocForm({ ...allocForm, endpoint: e.target.value })}
                  placeholder="自动探测公网IP，如 124.65.178.6:51820"
                  className="font-mono text-xs"
                />
              </div>

              <div className="p-3 bg-blue-50/70 rounded-lg border border-blue-200/60 text-xs text-blue-800 space-y-1">
                <div className="font-semibold flex items-center gap-1.5">
                  <Info className="h-4 w-4 text-blue-600" />
                  自动化分配机制说明:
                </div>
                <p className="text-[11px] leading-relaxed text-blue-700">
                  系统将按三层级联分配算法，检测所选子网池中下一个绝对未被使用的空闲 IP（如 10.100.10.x），将其与该设备的硬件指纹和 WireGuard 公钥进行终身强绑定。
                </p>
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <Button variant="outline" onClick={() => setIsAllocWizardOpen(false)}>
                  取消
                </Button>
                <Button
                  variant="primary"
                  onClick={handleRunAllocation}
                  disabled={allocating}
                  className="bg-blue-600 hover:bg-blue-500"
                >
                  <Zap className={`h-3.5 w-3.5 mr-1.5 ${allocating ? 'animate-spin' : ''}`} />
                  {allocating ? '计算空闲IP并锁定中...' : '立即执行自动分配'}
                </Button>
              </div>
            </div>
          ) : (
            /* Allocation Result Success View */
            <div className="space-y-4">
              <div className="p-4 bg-emerald-50 border border-emerald-200 rounded-xl text-emerald-900 space-y-2">
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-5 w-5 text-emerald-600" />
                  <h4 className="font-bold text-sm">虚拟 IP 自动化分配并绑定成功！</h4>
                </div>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 pt-2 text-xs">
                  <div>
                    <span className="text-slate-500 block">已分配 IP:</span>
                    <span className="font-mono font-bold text-emerald-700 text-sm">
                      {allocResult.allocated_ip}
                    </span>
                  </div>
                  <div>
                    <span className="text-slate-500 block">所属子网池:</span>
                    <span className="font-medium text-slate-800">
                      {allocResult.lease.pool_name}
                    </span>
                  </div>
                  <div>
                    <span className="text-slate-500 block">分配模式:</span>
                    <span className="font-medium text-slate-800">
                      {allocResult.lease.allocation_mode}
                    </span>
                  </div>
                  <div>
                    <span className="text-slate-500 block">隧道 CoreDNS:</span>
                    <span className="font-mono text-slate-800">
                      {allocResult.dns_server}
                    </span>
                  </div>
                </div>
              </div>

              {/* WireGuard Config Preview */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-semibold text-slate-700 flex items-center gap-1.5">
                    <Key className="h-3.5 w-3.5 text-blue-600" />
                    已下发的客户端 WireGuard 接口配置文件 (wg0.conf):
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      navigator.clipboard.writeText(allocResult.wg_interface_config);
                      setCopiedConfig(true);
                      setTimeout(() => setCopiedConfig(false), 2000);
                    }}
                    className="h-7 text-xs"
                  >
                    {copiedConfig ? (
                      <>
                        <Check className="h-3 w-3 mr-1 text-emerald-600" />
                        已复制
                      </>
                    ) : (
                      <>
                        <Copy className="h-3 w-3 mr-1" />
                        复制配置
                      </>
                    )}
                  </Button>
                </div>

                <pre className="p-3 bg-slate-950 text-slate-200 rounded-lg text-xs font-mono overflow-x-auto leading-relaxed border border-slate-800">
                  {allocResult.wg_interface_config}
                </pre>
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <Button
                  variant="outline"
                  onClick={resetAllocWizard}
                >
                  继续为其他设备分配
                </Button>
                <Button
                  variant="primary"
                  onClick={() => setIsAllocWizardOpen(false)}
                >
                  完成并关闭
                </Button>
              </div>
            </div>
          )}
        </div>
      </Dialog>

      {/* MODAL 2: CREATE / EDIT SUBNET POOL */}
      <Dialog
        isOpen={isPoolModalOpen}
        onClose={() => setIsPoolModalOpen(false)}
        title={editingPool ? '编辑虚拟子网池' : '规划新建业务子网池'}
        description="定义 Overlay 网段 CIDR 范围、分配模式及绑定的身份源用户组"
      >
        <div className="space-y-4">
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-slate-700">
              子网池名称 <span className="text-red-500">*</span>
            </label>
            <Input
              value={poolForm.pool_name}
              onChange={(e) => setPoolForm({ ...poolForm, pool_name: e.target.value })}
              placeholder="如: 跨境电商海外研发专网池"
              className="text-xs"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700">
                CIDR 网段 <span className="text-red-500">*</span>
              </label>
              <Input
                value={poolForm.cidr}
                onChange={(e) => setPoolForm({ ...poolForm, cidr: e.target.value })}
                placeholder="如: 10.100.60.0/20"
                className="font-mono text-xs"
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700">
                子网类型
              </label>
              <Select
                value={poolForm.type}
                onChange={(e) => setPoolForm({ ...poolForm, type: e.target.value as any })}
                className="text-xs"
              >
                <option value="User-Device">员工终端设备 (User-Device)</option>
                <option value="Gateway">骨干网关专网 (Gateway)</option>
                <option value="Service-VIP">高可用微服务VIP (Service-VIP)</option>
              </Select>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700">
                分配机制
              </label>
              <Select
                value={poolForm.allocation_mode}
                onChange={(e) => setPoolForm({ ...poolForm, allocation_mode: e.target.value as any })}
                className="text-xs"
              >
                <option value="dynamic">动态租约 (IdP/设备自动注册)</option>
                <option value="static">静态有序固定 (网关/特权节点)</option>
              </Select>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700">
                绑定 IdP 策略用户组
              </label>
              <Input
                value={poolForm.idp_group}
                onChange={(e) => setPoolForm({ ...poolForm, idp_group: e.target.value })}
                placeholder="如: 研发核心组 / 安全运维组"
                className="text-xs"
              />
            </div>
          </div>

          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-slate-700">
              描述说明
            </label>
            <Input
              value={poolForm.description}
              onChange={(e) => setPoolForm({ ...poolForm, description: e.target.value })}
              placeholder="说明该网段的用途和隔离要求..."
              className="text-xs"
            />
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" onClick={() => setIsPoolModalOpen(false)}>
              取消
            </Button>
            <Button
              variant="primary"
              onClick={handleSavePool}
              disabled={savingPool}
            >
              {savingPool ? '保存中...' : '确认保存'}
            </Button>
          </div>
        </div>
      </Dialog>

      {/* MODAL 3: VIEW WG CONFIG MODAL */}
      <Dialog
        isOpen={!!selectedLeaseConfig}
        onClose={() => setSelectedLeaseConfig(null)}
        title="WireGuard 客户端接口配置 (wg0.conf)"
        description={`为目标节点 [${selectedLeaseConfig?.target_name}] 签发的 WireGuard 加密隧道客户端凭证`}
      >
        {selectedLeaseConfig && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-2 p-3 bg-slate-50 rounded-lg text-xs">
              <div>
                <span className="text-slate-500 block">虚拟内网 IP:</span>
                <span className="font-mono font-bold text-blue-700">
                  {selectedLeaseConfig.virtual_ip}/32
                </span>
              </div>
              <div>
                <span className="text-slate-500 block">公网端点 (Endpoint):</span>
                <span className="font-mono text-slate-800">
                  {selectedLeaseConfig.endpoint}
                </span>
              </div>
              <div>
                <span className="text-slate-500 block">分配模式:</span>
                <span className="text-slate-800">{selectedLeaseConfig.allocation_mode}</span>
              </div>
              <div>
                <span className="text-slate-500 block">租约生效时间:</span>
                <span className="text-slate-800">
                  {formatRelativeTime(selectedLeaseConfig.allocated_at)}
                </span>
              </div>
            </div>

            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-slate-700">完整配置文本:</span>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    const text = `[Interface]
# VeloTrust 自动生成的客户端配置
PrivateKey = <CLIENT_PRIVATE_KEY_HERE>
Address = ${selectedLeaseConfig.virtual_ip}/32
DNS = 10.100.0.1, 10.100.250.1

[Peer]
# 骨干 Hub 网关锚点
PublicKey = aJ9uQ0kL8vM7nW2xY1zP4qR3sT6uV5wX8yZ0aB1cD2e=
Endpoint = 202.108.22.5:51820
AllowedIPs = 10.100.0.0/16, 192.168.10.0/24, 192.168.20.0/24
PersistentKeepalive = 25`;
                    navigator.clipboard.writeText(text);
                    success('已复制 WireGuard 客户端配置文件');
                  }}
                  className="h-7 text-xs"
                >
                  <Copy className="h-3 w-3 mr-1" />
                  复制配置
                </Button>
              </div>

              <pre className="p-3 bg-slate-950 text-slate-200 rounded-lg text-xs font-mono overflow-x-auto leading-relaxed border border-slate-800">
{`[Interface]
# VeloTrust 自动分配的客户端配置
# 节点: ${selectedLeaseConfig.target_name}
# 模式: ${selectedLeaseConfig.allocation_mode}
PrivateKey = <CLIENT_PRIVATE_KEY_HERE>
Address = ${selectedLeaseConfig.virtual_ip}/32
DNS = 10.100.0.1, 10.100.250.1

[Peer]
# 北京核心骨干网关 (Primary Hub)
PublicKey = aJ9uQ0kL8vM7nW2xY1zP4qR3sT6uV5wX8yZ0aB1cD2e=
Endpoint = 202.108.22.5:51820
AllowedIPs = 10.100.0.0/16, 192.168.10.0/24, 192.168.20.0/24
PersistentKeepalive = 25`}
              </pre>
            </div>

            <div className="flex justify-end pt-2">
              <Button variant="primary" onClick={() => setSelectedLeaseConfig(null)}>
                关闭
              </Button>
            </div>
          </div>
        )}
      </Dialog>

      {/* CONFIRM RELEASE LEASE DIALOG */}
      <ConfirmDialog
        isOpen={isReleaseConfirmOpen}
        onClose={() => setIsReleaseConfirmOpen(false)}
        onConfirm={handleConfirmRelease}
        title="确认回收并释放 IP 租约？"
        description={`您正在强制回收 [${leaseToRelease?.target_name}] 占用的虚拟 IP: ${leaseToRelease?.virtual_ip}。释放后该 IP 将重返可用池，该终端将立即失去内网隧道访问权限。`}
        confirmText="确认回收释放"
        cancelText="取消"
        danger
        isLoading={releasing}
      />
    </div>
  );
};
