import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Activity,
  Check,
  CheckCircle2,
  Copy,
  Info,
  Layers,
  Monitor,
  Network,
  Plus,
  RefreshCw,
  Search,
  Server,
  ShieldAlert,
  ShieldCheck,
  Trash2,
  Zap,
} from 'lucide-react';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { Badge } from '../components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table';
import { Dialog } from '../components/ui/dialog';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { EmptyState } from '../components/EmptyState';
import { useToast } from '../components/ui/toast';
import { formatRelativeTime } from '../utils/format';
import {
  IPAMConflict,
  IPAMLease,
  IPAMOverview,
  IPAMPool,
  IPAMPoolInput,
	PoolGroupOption,
  ipamApi,
} from '../services/ipam';

type Tab = 'pools' | 'leases' | 'heatmap' | 'conflict';

const emptyPool: IPAMPoolInput = {
  pool_name: '', cidr: '', pool_type: 'User-Device', allocation_mode: 'dynamic',
  client_group_id: null, gateway_binding: '', color: '#8b5cf6', description: '',
};
function suggestedPoolCIDR(globalCIDR: string): string {
  const [address, rawBits] = globalCIDR.split('/');
  const bits = Number(rawBits);
  if (!address || !Number.isInteger(bits)) return '';
  return `${address}/${Math.max(24, bits)}`;
}

function modeLabel(mode: string): string {
  if (mode === 'static') return '静态有序分配';
  if (mode === 'dynamic') return '动态租约';
  return mode || '—';
}

export const Ipam: React.FC = () => {
  const { success, error } = useToast();
  const errorRef = useRef(error);
  errorRef.current = error;

  const [tab, setTab] = useState<Tab>('pools');
  const [overview, setOverview] = useState<IPAMOverview | null>(null);
  const [pools, setPools] = useState<IPAMPool[]>([]);
  const [leases, setLeases] = useState<IPAMLease[]>([]);
  const [employeeGroups, setEmployeeGroups] = useState<PoolGroupOption[]>([]);
  const [loading, setLoading] = useState(true);

  const [search, setSearch] = useState('');
  const [poolFilter, setPoolFilter] = useState('all');
  const [typeFilter, setTypeFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const [heatmapPoolId, setHeatmapPoolId] = useState(0);

  const [overlayNetworkOpen, setOverlayNetworkOpen] = useState(false);
  const [overlayNetworkSaving, setOverlayNetworkSaving] = useState(false);
  const [overlayCIDR, setOverlayCIDR] = useState('10.100.0.0/16');
  const [poolOpen, setPoolOpen] = useState(false);
  const [poolSaving, setPoolSaving] = useState(false);
  const [poolForm, setPoolForm] = useState<IPAMPoolInput>(emptyPool);
  const [editingPool, setEditingPool] = useState<IPAMPool | null>(null);

  const [allocationOpen, setAllocationOpen] = useState(false);
  const [allocationSaving, setAllocationSaving] = useState(false);
  const [allocationResult, setAllocationResult] = useState<IPAMLease | null>(null);
  const [allocationForm, setAllocationForm] = useState({
    pool_id: 0,
    target_name: '',
    node_type: 'Peer' as IPAMLease['node_type'],
    requested_ip: '',
    idp_group: '',
    wireguard_pubkey: '',
    endpoint: '',
  });
  const [selectedLease, setSelectedLease] = useState<IPAMLease | null>(null);
  const [copiedKey, setCopiedKey] = useState<number | null>(null);
  const [releaseTarget, setReleaseTarget] = useState<IPAMLease | null>(null);
  const [releaseSaving, setReleaseSaving] = useState(false);

  const [checkCIDR, setCheckCIDR] = useState('');
  const [checking, setChecking] = useState(false);
  const [conflictResult, setConflictResult] = useState<IPAMConflict | null>(null);
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [summary, leaseRows, groupRows] = await Promise.all([
        ipamApi.overview(), ipamApi.listLeases(), ipamApi.listEmployeeGroups(),
      ]);
      const poolRows = Array.isArray(summary?.ipam_pools) ? summary.ipam_pools : [];
      setOverview({ ...summary, ipam_pools: poolRows, active_leases: Array.isArray(summary?.active_leases) ? summary.active_leases : [] });
      setPools(poolRows);
      setLeases(Array.isArray(leaseRows) ? leaseRows : []);
      setEmployeeGroups(Array.isArray(groupRows) ? groupRows : []);
      setAllocationForm((form) => ({
        ...form,
        pool_id: poolRows.some((pool) => pool.id === form.pool_id && pool.status === 'active')
          ? form.pool_id
          : (poolRows.find((pool) => pool.status === 'active')?.id ?? 0),
      }));
      setHeatmapPoolId((id) => poolRows.some((pool) => pool.id === id) ? id : (poolRows[0]?.id ?? 0));
    } catch (cause) {
      errorRef.current(cause instanceof Error ? cause.message : '加载 IPAM 数据失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void fetchData(); }, [fetchData]);

  const activePools = useMemo(() => pools.filter((pool) => pool.status === 'active'), [pools]);
  const gatewayPools = useMemo(() => pools.filter((pool) => pool.type === 'Gateway' && pool.status === 'active'), [pools]);
  const userPools = useMemo(() => pools.filter((pool) => pool.type === 'User-Device' && pool.status === 'active'), [pools]);
  const selectedPoolGroup = employeeGroups.find((group) => group.id === poolForm.client_group_id);
  const selectedAllocationPool = activePools.find((pool) => pool.id === allocationForm.pool_id);
  const heatmapPool = pools.find((pool) => pool.id === heatmapPoolId);

  const filteredLeases = useMemo(() => {
    const query = search.trim().toLowerCase();
    return leases.filter((lease) => {
      const matchesSearch = !query
        || lease.virtual_ip.toLowerCase().includes(query)
        || lease.target_name.toLowerCase().includes(query)
        || (lease.wireguard_pubkey ?? '').toLowerCase().includes(query)
        || (lease.endpoint ?? '').toLowerCase().includes(query);
      return matchesSearch
        && (poolFilter === 'all' || lease.pool_id === Number(poolFilter))
        && (typeFilter === 'all' || lease.node_type === typeFilter)
        && (statusFilter === 'all' || lease.status === statusFilter);
    });
  }, [leases, poolFilter, search, statusFilter, typeFilter]);

  const heatmapCells = useMemo(() => {
    if (!heatmapPool) return [];
    const [address, rawBits] = heatmapPool.cidr.split('/');
    const octets = address.split('.').map(Number);
    const bits = Number(rawBits);
    if (octets.length !== 4 || octets.some((value) => !Number.isInteger(value) || value < 0 || value > 255) || !Number.isInteger(bits) || bits < 16 || bits > 30) return [];
    const base = octets.reduce((value, part) => value * 256 + part, 0);
    const count = Math.min(120, heatmapPool.total);
    const leaseByIP = new Map(leases.filter((lease) => lease.pool_id === heatmapPool.id && lease.status !== 'Released').map((lease) => [lease.virtual_ip, lease]));
    return Array.from({ length: count }, (_, index) => {
      const value = base + index + 1;
      const ip = [Math.floor(value / 16777216) % 256, Math.floor(value / 65536) % 256, Math.floor(value / 256) % 256, value % 256].join('.');
      return { ip, hostNum: index + 1, lease: leaseByIP.get(ip) };
    });
  }, [heatmapPool, leases]);

  const saveOverlayNetwork = async () => {
    if (overview?.global_cidr) { error('全局 Overlay 网段初始化后不可修改'); return; }
    if (!overlayCIDR.trim()) { error('请填写全局 Overlay 网段'); return; }
    setOverlayNetworkSaving(true);
    try {
      await ipamApi.saveOverlayNetwork(overlayCIDR.trim(), overview?.global_network_version ?? 0);
      success('全局 Overlay 网段已初始化并锁定');
      setOverlayNetworkOpen(false);
      await fetchData();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '保存全局 Overlay 网段失败');
    } finally {
      setOverlayNetworkSaving(false);
    }
  };

  const openPool = (pool?: IPAMPool) => {
    setEditingPool(pool ?? null);
    setPoolForm(pool ? {
      pool_name: pool.pool_name,
      cidr: pool.cidr,
      pool_type: pool.type,
      allocation_mode: pool.allocation_mode,
      client_group_id: pool.client_group_id ?? null,
      gateway_binding: pool.gateway_binding ?? '',
      color: pool.color,
      description: pool.description ?? '',
    } : { ...emptyPool, cidr: suggestedPoolCIDR(overview?.global_cidr ?? '') });
    setPoolOpen(true);
  };

  const savePool = async () => {
    if (!poolForm.pool_name.trim() || !poolForm.cidr.trim()) { error('请填写子网池名称和 CIDR'); return; }
    if (poolForm.client_group_id && (poolForm.pool_type !== 'User-Device' || poolForm.allocation_mode !== 'dynamic')) {
      error('员工组自动分配只能用于动态 User-Device 地址池');
      return;
    }
    setPoolSaving(true);
    const input = {
      ...poolForm,
      pool_name: poolForm.pool_name.trim(),
      cidr: poolForm.cidr.trim(),
      client_group_id: poolForm.pool_type === 'User-Device' && poolForm.allocation_mode === 'dynamic'
        ? poolForm.client_group_id
        : null,
      gateway_binding: poolForm.gateway_binding?.trim(),
      description: poolForm.description?.trim(),
    };
    try {
      if (editingPool) await ipamApi.updatePool(editingPool, input);
      else await ipamApi.createPool(input);
      success(editingPool ? '子网池已更新' : '新建子网池成功');
      setPoolOpen(false);
      await fetchData();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '保存子网池失败');
    } finally {
      setPoolSaving(false);
    }
  };

  const togglePool = async (pool: IPAMPool) => {
    try {
      await ipamApi.setPoolStatus(pool, pool.status === 'active' ? 'disabled' : 'active');
      success('子网池状态已更新');
      await fetchData();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '更新子网池状态失败');
    }
  };

  const openAllocation = () => {
    const initialPool = activePools.find((pool) => pool.type === 'User-Device') ?? activePools[0];
    setAllocationResult(null);
    setAllocationForm({
      pool_id: initialPool?.id ?? 0,
      target_name: '',
      node_type: 'Peer',
      requested_ip: '',
      idp_group: '',
      wireguard_pubkey: '',
      endpoint: '',
    });
    setAllocationOpen(true);
  };

  const allocate = async () => {
    if (!allocationForm.pool_id || !allocationForm.target_name.trim()) {
      error('请选择地址池并填写接入节点名称');
      return;
    }
    if (selectedAllocationPool?.allocation_mode === 'static' && !allocationForm.requested_ip.trim()) {
      error('静态地址池必须填写指定 IPv4 地址');
      return;
    }
    setAllocationSaving(true);
    try {
      const lease = await ipamApi.allocateAddress({
        ...allocationForm,
        target_name: allocationForm.target_name.trim(),
        requested_ip: allocationForm.requested_ip.trim(),
        idp_group: allocationForm.idp_group.trim(),
        wireguard_pubkey: allocationForm.wireguard_pubkey.trim(),
        endpoint: allocationForm.endpoint.trim(),
      });
      setAllocationResult(lease);
      success(`已为 ${lease.target_name} 分配 ${lease.virtual_ip}`);
      await fetchData();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '分配 IP 失败');
    } finally {
      setAllocationSaving(false);
    }
  };

  const releaseLease = async () => {
    if (!releaseTarget) return;
    setReleaseSaving(true);
    try {
      await ipamApi.releaseLease(releaseTarget.id);
      success(`已释放虚拟 IP：${releaseTarget.virtual_ip}`);
      setReleaseTarget(null);
      await fetchData();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '释放租约失败');
    } finally {
      setReleaseSaving(false);
    }
  };

  const checkCIDRConflict = async () => {
    if (!checkCIDR.trim()) { error('请输入待检查的 CIDR'); return; }
    setChecking(true);
    try {
      setConflictResult(await ipamApi.checkPrefix(checkCIDR.trim()));
    } catch (cause) {
      error(cause instanceof Error ? cause.message : 'CIDR 冲突检查失败');
    } finally {
      setChecking(false);
    }
  };

  const copyPublicKey = async (lease: IPAMLease) => {
    if (!lease.wireguard_pubkey) { error('此租约尚未登记 WireGuard 公钥'); return; }
    try {
      await navigator.clipboard.writeText(lease.wireguard_pubkey);
      setCopiedKey(lease.id);
      window.setTimeout(() => setCopiedKey((id) => id === lease.id ? null : id), 1800);
      success('公钥已复制');
    } catch {
      error('复制失败，请检查浏览器剪贴板权限');
    }
  };

  const gatewayUsed = gatewayPools.reduce((sum, pool) => sum + pool.used, 0);
  const gatewayTotal = gatewayPools.reduce((sum, pool) => sum + pool.total, 0);
  const userUsed = userPools.reduce((sum, pool) => sum + pool.used, 0);
  const userTotal = userPools.reduce((sum, pool) => sum + pool.total, 0);
  const overlayPercent = overview?.total_ips ? Math.min(100, (overview.used_ips / overview.total_ips) * 100) : 0;
  const auditState = conflictResult
    ? (!conflictResult.isValid ? 'CIDR 无效' : conflictResult.hasConflict ? '发现冲突' : '检查通过')
    : '待检测';

  const tabs = [
    { key: 'pools' as const, label: 'Overlay 地址池', icon: Layers, count: pools.length },
    { key: 'leases' as const, label: '虚拟 IP 租约', icon: Activity, count: leases.length },
    { key: 'heatmap' as const, label: '网段热力图', icon: Network },
    { key: 'conflict' as const, label: '冲突探测', icon: ShieldAlert },
  ];

  return (
    <div className="space-y-6">
      <div className="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-slate-900">虚拟网段与 IPAM 管理</h1>
          <p className="mt-0.5 text-xs text-slate-500">Overlay 地址规划、地址池管理、动态 IP 租约与冲突检测</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => void fetchData()} disabled={loading} className="gap-1.5">
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />刷新
          </Button>
          {overview?.global_cidr ? (
            <Badge variant="success" className="h-8 items-center px-3">全局范围已锁定</Badge>
          ) : (
            <Button variant="outline" size="sm" onClick={() => { setOverlayCIDR('10.100.0.0/16'); setOverlayNetworkOpen(true); }} className="gap-1.5">
              <ShieldCheck className="h-3.5 w-3.5 text-blue-600" />初始化 Overlay
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={() => setTab('conflict')} className="gap-1.5">
            <ShieldCheck className="h-3.5 w-3.5 text-blue-600" />冲突检测
          </Button>
          {tab === 'pools' && (
            <div className="flex gap-2">
              <Button size="sm" onClick={() => openPool()} disabled={!overview?.global_cidr} className="gap-1.5">
                <Plus className="h-3.5 w-3.5" />新建子网池
              </Button>
            </div>
          )}
          {tab === 'leases' && (
            <Button size="sm" onClick={openAllocation} disabled={!activePools.length} className="gap-1.5 bg-blue-600 text-white hover:bg-blue-700">
              <Zap className="h-3.5 w-3.5" />IP 分配向导
            </Button>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card className="border-slate-200 bg-white shadow-sm transition-colors hover:border-blue-400">
          <CardContent className="space-y-2 p-4">
            <div className="flex items-center justify-between text-xs text-slate-500">
              <span className="font-semibold text-slate-700">全局 Overlay 超级网段</span>
              <Badge variant="outline" className="font-mono text-[10px]">10/8 私有地址</Badge>
            </div>
            <div className="flex items-baseline justify-between gap-2">
              <span className="truncate font-mono text-xl font-bold text-slate-900">{overview?.global_cidr || '未初始化'}</span>
              <span className="shrink-0 font-mono text-[10px] text-slate-500">池容量 {(overview?.total_ips ?? 0).toLocaleString()} IP</span>
            </div>
            <div className="space-y-1">
              <div className="flex justify-between text-[11px] text-slate-500">
                <span>已分配：<strong className="text-blue-600">{(overview?.used_ips ?? 0).toLocaleString()}</strong></span>
                <span>空闲：<strong className="text-emerald-600">{(overview?.free_ips ?? 0).toLocaleString()}</strong></span>
              </div>
              <div className="h-1.5 w-full overflow-hidden rounded-full bg-slate-100">
                <div className="h-full rounded-full bg-blue-600 transition-all" style={{ width: `${overlayPercent}%` }} />
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="border-slate-200 bg-white shadow-sm transition-colors hover:border-blue-400">
          <CardContent className="space-y-2 p-4">
            <div className="flex items-center justify-between text-xs text-slate-500">
              <span className="flex items-center gap-1 font-semibold text-blue-700"><Server className="h-3.5 w-3.5" />骨干网关专用池 (Hubs)</span>
              <span className="rounded bg-blue-50 px-1.5 py-0.5 font-mono text-[10px] font-medium text-blue-600">{gatewayPools.length} 个池</span>
            </div>
            <div className="flex items-baseline justify-between">
              <span className="font-mono text-2xl font-bold text-blue-700">{gatewayUsed.toLocaleString()} / {gatewayTotal.toLocaleString()}</span>
              <span className="text-xs font-medium text-blue-600">静态或动态分配</span>
            </div>
            <p className="text-[11px] leading-relaxed text-slate-500">为 Gateway 节点和高可用虚拟地址预留 Overlay 地址空间。</p>
          </CardContent>
        </Card>

        <Card className="border-slate-200 bg-white shadow-sm transition-colors hover:border-emerald-400">
          <CardContent className="space-y-2 p-4">
            <div className="flex items-center justify-between text-xs text-slate-500">
              <span className="flex items-center gap-1 font-semibold text-emerald-700"><Monitor className="h-3.5 w-3.5" />终端设备池 (Spokes)</span>
              <span className="rounded bg-emerald-50 px-1.5 py-0.5 font-mono text-[10px] font-medium text-emerald-600">{userPools.length} 个池</span>
            </div>
            <div className="flex items-baseline justify-between">
              <span className="font-mono text-2xl font-bold text-emerald-700">{userUsed.toLocaleString()} / {userTotal.toLocaleString()}</span>
              <span className="text-xs font-medium text-emerald-600">租约状态实时统计</span>
            </div>
            <p className="text-[11px] leading-relaxed text-slate-500">终端地址与设备名称、公钥和可选身份组关联。</p>
          </CardContent>
        </Card>

        <Card className="border-slate-200 bg-white shadow-sm transition-colors hover:border-purple-400">
          <CardContent className="space-y-2 p-4">
            <div className="flex items-center justify-between text-xs text-slate-500">
              <span className="flex items-center gap-1 font-semibold text-purple-700"><ShieldCheck className="h-3.5 w-3.5 text-purple-600" />CIDR 冲突检测</span>
              <span className="rounded bg-purple-50 px-1.5 py-0.5 font-mono text-[10px] font-medium text-purple-600">{pools.length} 个地址池</span>
            </div>
            <div className="flex items-baseline justify-between">
              <span className={`font-mono text-xl font-bold ${conflictResult?.hasConflict ? 'text-rose-700' : 'text-purple-700'}`}>{auditState}</span>
              <span className={`flex items-center gap-1 text-xs font-medium ${conflictResult?.hasConflict ? 'text-rose-600' : 'text-emerald-600'}`}>
                {conflictResult?.hasConflict ? <ShieldAlert className="h-3 w-3" /> : <CheckCircle2 className="h-3 w-3" />}
                {conflictResult ? (conflictResult.hasConflict ? '需要处理' : '实时检查') : '等待检测'}
              </span>
            </div>
            <p className="text-[11px] leading-relaxed text-slate-500">检查 Overlay 网段规划是否存在冲突。</p>
          </CardContent>
        </Card>
      </div>

      <div className="flex items-center overflow-x-auto border-b border-slate-200">
        {tabs.map(({ key, label, icon: Icon, count }) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`flex shrink-0 items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-all ${tab === key ? 'border-blue-600 text-blue-600' : 'border-transparent text-slate-500 hover:text-slate-800'}`}
          >
            <Icon className="h-4 w-4" />
            <span>{label}</span>
            {count !== undefined && <span className="text-[11px] font-normal text-slate-400">({count})</span>}
          </button>
        ))}
      </div>

      {tab === 'pools' && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
            {pools.map((pool) => {
              const percent = pool.total ? Math.min(100, Math.round((pool.used / pool.total) * 100)) : 0;
              return (
                <Card key={pool.id} className="flex flex-col justify-between border-slate-200 bg-white shadow-sm transition-all hover:shadow-md">
                  <CardHeader className="p-5 pb-3">
                    <div className="flex items-start justify-between gap-3">
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <span className="h-3 w-3 shrink-0 rounded-full" style={{ backgroundColor: pool.color }} />
                          <CardTitle className="text-sm tracking-tight">{pool.pool_name}</CardTitle>
                        </div>
                        <p className="flex items-center gap-1.5 font-mono text-xs text-slate-500">CIDR: <strong className="font-semibold text-slate-800">{pool.cidr}</strong></p>
                      </div>
                      <div className="flex flex-col items-end gap-1">
                        <Badge variant={pool.type === 'Gateway' ? 'primary' : pool.type === 'Service-VIP' ? 'slate' : 'success'} className="text-[10px]">{pool.type}</Badge>
                        <StatusBadge status={pool.status} />
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-4 p-5 pt-0">
                    <p className="min-h-9 text-xs leading-relaxed text-slate-600">{pool.description || '无详细描述'}</p>
                    <div className="space-y-1.5 text-xs">
                      <div className="flex items-center justify-between gap-3 text-slate-500"><span>分配策略模式:</span><span className="font-medium text-slate-700">{modeLabel(pool.allocation_mode)}</span></div>
                      {pool.client_group_id && <div className="flex items-center justify-between gap-3 text-slate-500"><span>自动分配员工组:</span><span className="rounded bg-blue-50 px-1.5 py-0.5 text-[11px] font-semibold text-blue-600">{pool.client_group_name ?? employeeGroups.find((group) => group.id === pool.client_group_id)?.name ?? `组 #${pool.client_group_id}`}</span></div>}
                      {pool.gateway_binding && <div className="flex items-center justify-between gap-3 text-slate-500"><span>绑定锚点网关:</span><span className="text-[11px] font-semibold text-slate-700">{pool.gateway_binding}</span></div>}
                    </div>
                    <div className="space-y-1.5 border-t border-slate-100 pt-2">
                      <div className="flex items-center justify-between text-xs"><span className="text-slate-500">IP 容量占用:</span><span className="font-mono font-semibold text-slate-800">{pool.used.toLocaleString()} / {pool.total.toLocaleString()} ({percent}%)</span></div>
                      <div className="h-2 w-full overflow-hidden rounded-full bg-slate-100"><div className="h-full rounded-full transition-all" style={{ width: `${percent}%`, backgroundColor: pool.color }} /></div>
                    </div>
                    <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
                      <Button variant="outline" size="sm" onClick={() => { setPoolFilter(String(pool.id)); setTab('leases'); }} className="h-7 text-xs">查看租约列表 ({pool.used})</Button>
                      <div className="flex items-center gap-1">
                        <Button variant="ghost" size="sm" onClick={() => void togglePool(pool)} className="h-7 px-2 text-xs">{pool.status === 'active' ? '停用' : '启用'}</Button>
                        <Button variant="ghost" size="sm" onClick={() => openPool(pool)} className="h-7 px-2 text-xs text-slate-500">编辑</Button>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              );
            })}
            {!pools.length && <div className="md:col-span-2 lg:col-span-3"><EmptyState icon={Layers} title={loading ? '正在读取地址池…' : '尚未创建 Overlay 地址池'} description="先初始化全局 Overlay 范围，再规划 Gateway、终端或服务 VIP 地址池。" actionText={!overview?.global_cidr ? '初始化全局 Overlay' : undefined} onAction={!overview?.global_cidr ? () => setOverlayNetworkOpen(true) : undefined} /></div>}
          </div>
        </div>
      )}

      {tab === 'leases' && (
        <div className="space-y-4">
          <div className="flex flex-col justify-between gap-3 rounded-xl border border-slate-200 bg-white p-4 shadow-sm md:flex-row md:items-center">
            <div className="flex flex-1 flex-wrap items-center gap-2.5">
              <div className="relative w-full md:w-64"><Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-400" /><Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索虚拟 IP、设备名、公钥…" className="h-9 pl-9 text-xs" /></div>
              <Select value={poolFilter} onChange={(event) => setPoolFilter(event.target.value)} className="h-9 w-40 text-xs"><option value="all">全部子网池</option>{pools.map((pool) => <option key={pool.id} value={pool.id}>{pool.pool_name}</option>)}</Select>
              <Select value={typeFilter} onChange={(event) => setTypeFilter(event.target.value)} className="h-9 w-36 text-xs"><option value="all">全部节点类型</option><option value="Gateway">网关 Gateway</option><option value="Peer">终端 Peer</option><option value="Server">服务 Server</option></Select>
              <Select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)} className="h-9 w-36 text-xs"><option value="all">全部租约状态</option><option value="Active">活动 Active</option><option value="Reserved">静态保留</option><option value="Released">已释放</option></Select>
            </div>
            <Button onClick={openAllocation} disabled={!activePools.length} className="h-9 shrink-0 text-xs"><Plus className="mr-1 h-3.5 w-3.5" />新增分配 IP</Button>
          </div>

          <Card className="overflow-hidden border-slate-200 bg-white shadow-sm">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-36">虚拟 IP (Virtual IP)</TableHead>
                  <TableHead>目标设备 / 节点名称</TableHead>
                  <TableHead className="w-28">节点类型</TableHead>
                  <TableHead className="w-36">分配模式</TableHead>
                  <TableHead className="w-44">WireGuard 公钥指纹</TableHead>
                  <TableHead className="w-40">所属子网池</TableHead>
                  <TableHead className="w-24">租约状态</TableHead>
                  <TableHead className="w-32 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredLeases.map((lease) => (
                  <TableRow key={lease.id}>
                    <TableCell><div className="flex items-center gap-2"><span className={`h-2 w-2 rounded-full ${lease.status === 'Active' ? 'animate-pulse bg-emerald-500' : lease.status === 'Reserved' ? 'bg-blue-500' : 'bg-slate-300'}`} /><span className="font-mono text-xs font-bold text-slate-900">{lease.virtual_ip}</span></div></TableCell>
                    <TableCell><div className="space-y-0.5"><div className="flex items-center gap-1.5 text-xs font-semibold text-slate-800">{lease.target_name}</div><div className="font-mono text-[11px] text-slate-400">端点: {lease.endpoint || '未登记'}</div></div></TableCell>
                    <TableCell><Badge variant={lease.node_type === 'Gateway' ? 'primary' : lease.node_type === 'Server' ? 'slate' : 'default'} className="text-[10px]">{lease.node_type}</Badge></TableCell>
                    <TableCell><span className="rounded border border-slate-200 bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-700">{modeLabel(lease.allocation_mode)}</span></TableCell>
                    <TableCell><div className="flex max-w-44 items-center gap-1.5"><span className="truncate font-mono text-[11px] text-slate-600">{lease.wireguard_pubkey || '未登记'}</span>{lease.wireguard_pubkey && <button onClick={() => void copyPublicKey(lease)} className="shrink-0 text-slate-400 hover:text-slate-700" title="复制公钥">{copiedKey === lease.id ? <Check className="h-3.5 w-3.5 text-emerald-600" /> : <Copy className="h-3.5 w-3.5" />}</button>}</div></TableCell>
                    <TableCell><span className="text-xs font-medium text-slate-600">{lease.pool_name}</span></TableCell>
                    <TableCell><LeaseStatus status={lease.status} /></TableCell>
                    <TableCell className="text-right"><div className="flex items-center justify-end gap-1"><Button variant="ghost" size="sm" onClick={() => setSelectedLease(lease)} className="h-7 px-2 text-xs text-blue-600">详情</Button>{lease.status !== 'Released' && <Button variant="ghost" size="sm" onClick={() => setReleaseTarget(lease)} className="h-7 px-2 text-xs text-rose-600" title="回收并释放 IP"><Trash2 className="h-3.5 w-3.5" /></Button>}</div></TableCell>
                  </TableRow>
                ))}
                {!filteredLeases.length && <TableRow><TableCell colSpan={8} className="py-12 text-center"><EmptyState icon={Layers} title={loading ? '正在读取租约…' : '未找到匹配的 IP 租约'} description="尝试更改筛选条件，或使用 IP 分配向导新增租约。" /></TableCell></TableRow>}
              </TableBody>
            </Table>
          </Card>
        </div>
      )}

      {tab === 'heatmap' && (
        <Card className="space-y-5 border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col justify-between gap-4 border-b border-slate-100 pb-4 md:flex-row md:items-center">
            <div><h2 className="text-sm font-bold text-slate-900">子网 IP 地址空间矩阵分布图</h2><p className="text-xs text-slate-500">以网格展现所选地址池的真实租约占用，悬停可查看节点信息。</p></div>
            <div className="flex items-center gap-3"><span className="text-xs font-medium text-slate-600">查看子网池:</span><Select value={heatmapPoolId || ''} onChange={(event) => setHeatmapPoolId(Number(event.target.value))} className="h-8 w-60 text-xs"><option value="">选择地址池</option>{pools.map((pool) => <option key={pool.id} value={pool.id}>{pool.pool_name} ({pool.cidr})</option>)}</Select></div>
          </div>
          <div className="flex flex-wrap items-center gap-4 rounded-lg border border-slate-100 bg-slate-50 p-3 text-xs">
            <span className="font-semibold text-slate-700">图例说明:</span>
            <Legend color="bg-blue-600" label="Gateway 地址租约" />
            <Legend color="bg-emerald-500" label="Peer 终端租约" />
            <Legend color="bg-purple-600" label="Server / VIP 租约" />
            <Legend color="bg-slate-200 border border-slate-300" label="空闲可用" />
          </div>
          {heatmapPool ? (
            <>
              <div className="grid grid-cols-6 gap-1.5 sm:grid-cols-10 lg:grid-cols-12 xl:grid-cols-15">
                {heatmapCells.map(({ ip, hostNum, lease }) => {
                  const colorClass = !lease ? 'bg-slate-100 text-slate-400 hover:bg-blue-50 hover:text-blue-600 border border-slate-200/60' : lease.node_type === 'Gateway' ? 'bg-blue-600 text-white' : lease.node_type === 'Server' ? 'bg-purple-600 text-white' : 'bg-emerald-500 text-white';
                  return (
                    <div key={ip} title={lease ? `${ip} · ${lease.target_name} · ${lease.node_type} · ${lease.wireguard_pubkey || '未登记公钥'}` : `${ip} · 空闲可用`} className={`group relative flex h-9 cursor-default items-center justify-center rounded font-mono text-[10px] font-bold transition-all hover:z-20 hover:scale-110 ${colorClass}`}>
                      .{hostNum}
                      {lease && <div className="pointer-events-none absolute bottom-full left-1/2 z-50 mb-2 w-52 -translate-x-1/2 rounded-lg bg-slate-900 p-2.5 text-[11px] text-white opacity-0 shadow-2xl transition-opacity group-hover:opacity-100"><div className="mb-1 border-b border-slate-700 pb-1 font-mono font-bold text-blue-300">{ip}</div><div>节点: <strong>{lease.target_name}</strong></div><div>类型: {lease.node_type}</div><div>模式: {modeLabel(lease.allocation_mode)}</div><div className="truncate font-mono text-[10px] text-slate-400">端点: {lease.endpoint || '未登记'}</div></div>}
                    </div>
                  );
                })}
              </div>
              <div className="flex flex-wrap items-center justify-between gap-2 border-t border-slate-100 pt-3 text-[11px] text-slate-500"><span>展示前 {heatmapCells.length} 个 IPv4 地址块。颜色块表示已登记租约，灰色块表示可分配。</span><span>{heatmapPool.used.toLocaleString()} / {heatmapPool.total.toLocaleString()} 已使用</span></div>
            </>
          ) : <EmptyState icon={Network} title="没有可视化的地址池" description="请先创建 IPv4 Overlay 地址池。" />}
        </Card>
      )}

      {tab === 'conflict' && (
        <div className="space-y-4">
          <Card className="space-y-6 border-slate-200 bg-white p-6 shadow-sm">
            <div><h2 className="flex items-center gap-2 text-sm font-bold text-slate-900"><ShieldCheck className="h-4 w-4 text-purple-600" />Overlay CIDR 冲突检测</h2><p className="mt-1 text-xs text-slate-500">规划 Overlay 地址池前检查 CIDR，避免地址重叠造成路由异常。</p></div>
            <div className="grid grid-cols-1 gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 text-xs md:grid-cols-2">
              <div className="space-y-1"><span className="font-semibold text-blue-700">全局 Overlay 网段</span><div className="font-mono text-slate-600">{overview?.global_cidr || '尚未初始化'}</div></div>
              <div className="space-y-1"><span className="font-semibold text-emerald-700">启用的 Overlay 地址池</span>{activePools.length ? activePools.slice(0, 4).map((pool) => <div key={pool.id} className="flex justify-between gap-2 text-slate-600"><span className="truncate">{pool.pool_name}</span><span className="shrink-0 font-mono">{pool.cidr}</span></div>) : <div className="text-slate-400">尚未启用地址池</div>}</div>
            </div>
            <div className="space-y-3">
              <label className="text-xs font-semibold text-slate-700">输入待规划 CIDR 进行冲突检查</label>
              <div className="flex flex-col gap-3 sm:flex-row"><Input value={checkCIDR} onChange={(event) => setCheckCIDR(event.target.value)} placeholder="例如 10.100.60.0/24" className="max-w-md font-mono text-sm" /><Button onClick={() => void checkCIDRConflict()} disabled={checking || !checkCIDR.trim()} className="bg-purple-600 text-white hover:bg-purple-700"><RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${checking ? 'animate-spin' : ''}`} />执行 CIDR 冲突检测</Button></div>
            </div>
            {conflictResult && <div className={`rounded-xl border p-4 ${conflictResult.hasConflict || !conflictResult.isValid ? 'border-rose-200 bg-rose-50 text-rose-900' : 'border-emerald-200 bg-emerald-50 text-emerald-900'}`}><div className="flex items-start gap-3">{conflictResult.hasConflict || !conflictResult.isValid ? <ShieldAlert className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" /> : <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-emerald-600" />}<div><h3 className="text-sm font-bold">{conflictResult.hasConflict ? `检测到与 ${conflictResult.conflictTarget ?? '现有目录'} 重叠` : conflictResult.isValid ? '规划合规审计通过' : 'CIDR 格式无效'}</h3><p className="mt-1 text-xs leading-relaxed">{conflictResult.message}</p></div></div></div>}
          </Card>
        </div>
      )}

      <Dialog isOpen={overlayNetworkOpen && !overview?.global_cidr} onClose={() => !overlayNetworkSaving && setOverlayNetworkOpen(false)} title="初始化全局 Overlay 网段" description="使用 RFC1918 的 10.0.0.0/8 私有地址空间，范围支持 /8 至 /16。初始化后不可修改。" maxWidth="lg">
        <div className="space-y-5"><Field label="全局 Overlay CIDR"><Input value={overlayCIDR} onChange={(event) => setOverlayCIDR(event.target.value)} placeholder="10.100.0.0/16" className="font-mono text-sm" /></Field><div className="rounded-lg border border-blue-200 bg-blue-50/70 p-3 text-xs leading-relaxed text-blue-800"><div className="mb-1 flex items-center gap-1.5 font-semibold"><Info className="h-4 w-4" />初始化前请确认规划</div>地址池必须完全位于此范围内；保存后该全局范围将锁定。</div><DialogActions close={() => setOverlayNetworkOpen(false)} save={() => void saveOverlayNetwork()} saving={overlayNetworkSaving} label="初始化并锁定" /></div>
      </Dialog>

      <Dialog isOpen={poolOpen} onClose={() => !poolSaving && setPoolOpen(false)} title={editingPool ? '编辑 Overlay 地址池' : '规划新建 Overlay 地址池'} description={`定义 Overlay 网段和终端自动分配配置。全局范围：${overview?.global_cidr ?? '未初始化'}`}>
        <div className="space-y-4">
          <Field label="子网池名称 *">
            <Input value={poolForm.pool_name} onChange={(event) => setPoolForm({ ...poolForm, pool_name: event.target.value })} placeholder="例如：研发终端动态池" className="text-xs" />
          </Field>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field label="CIDR 网段 *">
              <Input value={poolForm.cidr} onChange={(event) => setPoolForm({ ...poolForm, cidr: event.target.value })} placeholder="例如：10.100.0.0/24" className="font-mono text-xs" />
            </Field>
            <Field label="子网类型">
              <Select value={poolForm.pool_type} onChange={(event) => setPoolForm({ ...poolForm, pool_type: event.target.value as IPAMPool['type'], client_group_id: null })} className="text-xs">
                <option value="User-Device">员工终端设备 (User-Device)</option>
                <option value="Gateway">骨干网关专网 (Gateway)</option>
                <option value="Service-VIP">高可用服务 VIP (Service-VIP)</option>
              </Select>
            </Field>
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field label="分配机制">
              <Select value={poolForm.allocation_mode} onChange={(event) => setPoolForm({ ...poolForm, allocation_mode: event.target.value as IPAMPool['allocation_mode'], client_group_id: null })} className="text-xs">
                <option value="dynamic">动态租约</option>
                <option value="static">静态固定分配</option>
              </Select>
            </Field>
            {poolForm.pool_type === 'User-Device' && poolForm.allocation_mode === 'dynamic' && (
              <Field label="员工用户组">
                <Select value={poolForm.client_group_id ?? ''} onChange={(event) => setPoolForm({ ...poolForm, client_group_id: event.target.value ? Number(event.target.value) : null })} className="text-xs">
                  <option value="">不关联员工组</option>
                  {employeeGroups.map((group) => <option key={group.id} value={group.id}>{group.name}（{group.user_count} 名员工）</option>)}
                </Select>
              </Field>
            )}
          </div>
          {poolForm.pool_type === 'User-Device' && poolForm.allocation_mode === 'dynamic' && selectedPoolGroup && (
            <p className="rounded-lg border border-blue-200 bg-blue-50/70 p-3 text-[11px] leading-relaxed text-blue-800">
              HUB 登录时按设备所有者的员工组匹配此地址池并自动分配 Overlay IP。该关联只负责 IP 分配；客户端可连接的 Gateway 由访问策略决定。
            </p>
          )}
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field label="绑定锚点网关">
              <Input value={poolForm.gateway_binding ?? ''} onChange={(event) => setPoolForm({ ...poolForm, gateway_binding: event.target.value })} placeholder="可选" className="text-xs" />
            </Field>
            <Field label="标记颜色">
              <input type="color" value={poolForm.color} onChange={(event) => setPoolForm({ ...poolForm, color: event.target.value })} className="h-10 w-full rounded-lg border border-slate-300 bg-white p-1" />
            </Field>
          </div>
          <Field label="描述说明">
            <Input value={poolForm.description ?? ''} onChange={(event) => setPoolForm({ ...poolForm, description: event.target.value })} placeholder="说明地址池用途和隔离要求" className="text-xs" />
          </Field>
          {editingPool && editingPool.cidr !== poolForm.cidr && <p className="rounded border border-amber-200 bg-amber-50 p-2 text-xs text-amber-800">已有地址租约的地址池不能更改 CIDR；保存前服务端会再次校验。</p>}
          <DialogActions close={() => setPoolOpen(false)} save={() => void savePool()} saving={poolSaving} label="确认保存" />
        </div>
      </Dialog>

      <Dialog isOpen={allocationOpen} onClose={() => !allocationSaving && setAllocationOpen(false)} title="自动化虚拟 IP 分配向导" description="选择接入类型和归属子网池，IPAM 将分配空闲地址并登记节点信息。" maxWidth="2xl">
        <div className="space-y-5">
          {!allocationResult ? (
            <div className="space-y-4"><div className="grid grid-cols-1 gap-4 md:grid-cols-2"><Field label="接入节点类型 *"><Select value={allocationForm.node_type} onChange={(event) => { const nodeType = event.target.value as IPAMLease['node_type']; const preferredType = nodeType === 'Gateway' ? 'Gateway' : nodeType === 'Server' ? 'Service-VIP' : 'User-Device'; const preferredPool = activePools.find((pool) => pool.type === preferredType); setAllocationForm({ ...allocationForm, node_type: nodeType, pool_id: preferredPool?.id ?? activePools[0]?.id ?? 0, requested_ip: '' }); }} className="text-xs"><option value="Peer">终端员工设备 (Peer)</option><option value="Gateway">云端骨干网关 (Gateway)</option><option value="Server">服务地址 (Server / VIP)</option></Select></Field><Field label="设备 / 节点名称 *"><Input value={allocationForm.target_name} onChange={(event) => setAllocationForm({ ...allocationForm, target_name: event.target.value })} placeholder="例如：研发工作站 / gw-east-01" className="text-xs" /></Field></div><div className="grid grid-cols-1 gap-4 md:grid-cols-2"><Field label="分配的目标子网池 *"><Select value={allocationForm.pool_id || ''} onChange={(event) => setAllocationForm({ ...allocationForm, pool_id: Number(event.target.value), requested_ip: '' })} className="text-xs"><option value="">选择启用的地址池</option>{activePools.map((pool) => <option key={pool.id} value={pool.id}>{pool.pool_name} ({pool.cidr})</option>)}</Select></Field>{selectedAllocationPool?.allocation_mode === 'static' ? <Field label="指定 IPv4 地址 *"><Input value={allocationForm.requested_ip} onChange={(event) => setAllocationForm({ ...allocationForm, requested_ip: event.target.value })} placeholder="例如：10.100.0.10" className="font-mono text-xs" /></Field> : <Field label="关联身份源或部门 (IdP 映射)"><Input value={allocationForm.idp_group} onChange={(event) => setAllocationForm({ ...allocationForm, idp_group: event.target.value })} placeholder="可选，例如：研发核心组" className="text-xs" /></Field>}</div><div className="grid grid-cols-1 gap-4 md:grid-cols-2"><Field label="WireGuard 客户端公钥（可选）"><Input value={allocationForm.wireguard_pubkey} onChange={(event) => setAllocationForm({ ...allocationForm, wireguard_pubkey: event.target.value })} placeholder="节点本地生成的 Base64 公钥" className="font-mono text-xs" /></Field><Field label="公网端点 (Endpoint，可选)"><Input value={allocationForm.endpoint} onChange={(event) => setAllocationForm({ ...allocationForm, endpoint: event.target.value })} placeholder="例如：203.0.113.10:51820" className="font-mono text-xs" /></Field></div><div className="rounded-lg border border-blue-200 bg-blue-50/70 p-3 text-xs text-blue-800"><div className="mb-1 flex items-center gap-1.5 font-semibold"><Info className="h-4 w-4 text-blue-600" />地址分配机制说明</div><p className="text-[11px] leading-relaxed text-blue-700">动态池自动选择空闲地址；静态池要求指定池内可用地址。该流程只登记公钥，不生成或保存 WireGuard 私钥、Peer 配置或隧道凭证。</p></div><div className="flex justify-end gap-2 pt-2"><Button variant="outline" onClick={() => setAllocationOpen(false)}>取消</Button><Button onClick={() => void allocate()} disabled={allocationSaving || !activePools.length} className="bg-blue-600 hover:bg-blue-700"><Zap className={`mr-1.5 h-3.5 w-3.5 ${allocationSaving ? 'animate-spin' : ''}`} />{allocationSaving ? '计算空闲 IP 并锁定中…' : '立即执行自动分配'}</Button></div></div>
          ) : (
            <div className="space-y-4"><div className="space-y-3 rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-emerald-900"><div className="flex items-center gap-2"><CheckCircle2 className="h-5 w-5 text-emerald-600" /><h3 className="text-sm font-bold">虚拟 IP 自动分配成功</h3></div><div className="grid grid-cols-2 gap-3 pt-2 text-xs sm:grid-cols-4"><InfoValue label="已分配 IP" value={`${allocationResult.virtual_ip}/32`} mono /><InfoValue label="所属子网池" value={allocationResult.pool_name} /><InfoValue label="分配模式" value={modeLabel(allocationResult.allocation_mode)} /><InfoValue label="节点类型" value={allocationResult.node_type} /></div></div><p className="rounded-lg border border-blue-200 bg-blue-50 p-3 text-xs text-blue-800">请在节点本地生成并保管 WireGuard 私钥。此分配结果只记录地址与节点公钥信息。</p><div className="flex justify-end gap-2"><Button variant="outline" onClick={openAllocation}>继续为其他设备分配</Button><Button onClick={() => setAllocationOpen(false)}>完成并关闭</Button></div></div>
          )}
        </div>
      </Dialog>

      <Dialog isOpen={!!selectedLease} onClose={() => setSelectedLease(null)} title="Overlay 地址租约详情" description={`节点：${selectedLease?.target_name ?? ''}`}>
        {selectedLease && <div className="space-y-4"><div className="grid grid-cols-2 gap-3 rounded-lg bg-slate-50 p-4 text-xs"><InfoValue label="虚拟内网 IP" value={`${selectedLease.virtual_ip}/32`} mono /><InfoValue label="节点类型" value={selectedLease.node_type} /><InfoValue label="所属子网池" value={selectedLease.pool_name} /><InfoValue label="分配模式" value={modeLabel(selectedLease.allocation_mode)} /><InfoValue label="公网端点" value={selectedLease.endpoint || '未登记'} mono /><InfoValue label="租约生效时间" value={formatRelativeTime(selectedLease.allocated_at)} /><InfoValue label="租约状态" value={selectedLease.status} /><InfoValue label="身份组" value={selectedLease.idp_group || '未关联'} /></div><div className="space-y-1.5"><div className="flex items-center justify-between"><span className="text-xs font-semibold text-slate-700">WireGuard 公钥</span>{selectedLease.wireguard_pubkey && <Button variant="outline" size="sm" onClick={() => void copyPublicKey(selectedLease)} className="h-7 text-xs"><Copy className="mr-1 h-3 w-3" />复制</Button>}</div><pre className="overflow-x-auto rounded-lg border border-slate-800 bg-slate-950 p-3 font-mono text-xs leading-relaxed text-slate-200">{selectedLease.wireguard_pubkey || '尚未登记公钥'}</pre></div><div className="flex justify-end"><Button variant="outline" onClick={() => setSelectedLease(null)}>关闭</Button></div></div>}
      </Dialog>

      <ConfirmDialog isOpen={!!releaseTarget} onClose={() => setReleaseTarget(null)} onConfirm={releaseLease} title="确认回收并释放 IP 租约？" description={`将释放 ${releaseTarget?.target_name} 占用的 ${releaseTarget?.virtual_ip}。历史租约记录会保留。`} confirmText="确认释放" cancelText="取消" danger isLoading={releaseSaving} />
    </div>
  );
};

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="block space-y-1.5 text-xs font-semibold text-slate-700">{label}{children}</label>;
}

function DialogActions({ close, save, saving, label }: { close: () => void; save: () => void; saving: boolean; label: string }) {
  return <div className="flex justify-end gap-2 border-t border-slate-100 pt-3"><Button variant="outline" onClick={close} disabled={saving}>取消</Button><Button onClick={save} isLoading={saving}>{label}</Button></div>;
}

function StatusBadge({ status }: { status: 'active' | 'disabled' }) {
  return <Badge variant={status === 'active' ? 'success' : 'outline'} className="text-[10px]">{status === 'active' ? '启用' : '停用'}</Badge>;
}

function LeaseStatus({ status }: { status: IPAMLease['status'] }) {
  const label = status === 'Active' ? '活跃租约' : status === 'Reserved' ? '静态保留' : '已释放';
  const variant = status === 'Active' ? 'success' : status === 'Reserved' ? 'primary' : 'outline';
  return <Badge variant={variant} className="text-[10px]">{label}</Badge>;
}

function Legend({ color, label }: { color: string; label: string }) {
  return <div className="flex items-center gap-1.5"><span className={`h-3 w-3 rounded ${color}`} /><span className="text-slate-600">{label}</span></div>;
}

function InfoValue({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return <div className="min-w-0"><span className="mb-1 block text-slate-500">{label}</span><span className={`block truncate text-slate-800 ${mono ? 'font-mono' : 'font-medium'}`}>{value}</span></div>;
}
