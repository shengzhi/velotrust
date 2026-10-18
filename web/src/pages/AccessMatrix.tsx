import React, { useState, useEffect, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  Shield,
  Users,
  User,
  Monitor,
  Search,
  CheckCircle2,
  XCircle,
  Server,
  Layers,
  Compass,
  ArrowRight,
  Database,
  Terminal,
  Activity,
  AlertTriangle,
  Lock,
  Unlock,
  RefreshCw,
  Sparkles,
  ExternalLink,
} from 'lucide-react';
import { mockApi } from '../data/mock';
import { Group, User as UserType, Device } from '../types';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { Badge } from '../components/ui/badge';
import { StatusBadge } from '../components/StatusBadge';
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card';
import { useToast } from '../components/ui/toast';

export const AccessMatrix: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const { success, error } = useToast();

  const [subjectType, setSubjectType] = useState<'group' | 'user' | 'device'>(
    (searchParams.get('type') as any) || 'group'
  );
  const [selectedId, setSelectedId] = useState<string>(searchParams.get('id') || '');

  const [groups, setGroups] = useState<Group[]>([]);
  const [users, setUsers] = useState<UserType[]>([]);
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(true);

  // Matrix calculation result
  const [matrixData, setMatrixData] = useState<any>(null);
  const [matrixLoading, setMatrixLoading] = useState(false);

  // Probe state
  const [probeInput, setProbeInput] = useState('192.168.10.50:3306');
  const [probing, setProbing] = useState(false);
  const [probeResult, setProbeResult] = useState<any>(null);

  // Active Tab for Resources (All / Subnets / Databases / DNS / Blocked)
  const [resourceTab, setResourceTab] = useState<'all' | 'subnets' | 'services' | 'dns' | 'blocked'>('all');

  // Load subject lists
  useEffect(() => {
    const fetchBase = async () => {
      try {
        const [gList, uList, dList] = await Promise.all([
          mockApi.getGroups(),
          mockApi.getUsers(),
          mockApi.getDevices(),
        ]);
        setGroups(gList);
        setUsers(uList);
        setDevices(dList);

        // Pick initial ID if not specified
        if (!selectedId) {
          if (subjectType === 'group' && gList.length > 0) {
            setSelectedId(gList[0].id);
          } else if (subjectType === 'user' && uList.length > 0) {
            setSelectedId(uList[0].id);
          } else if (subjectType === 'device' && dList.length > 0) {
            setSelectedId(dList[0].id);
          }
        }
      } catch {
        error('加载身份列表失败');
      } finally {
        setLoading(false);
      }
    };
    fetchBase();
  }, []);

  // Fetch Matrix whenever subject changes
  useEffect(() => {
    if (!selectedId) return;

    const fetchMatrix = async () => {
      setMatrixLoading(true);
      setProbeResult(null);
      try {
        const res = await mockApi.getAccessMatrix(subjectType, selectedId);
        setMatrixData(res);
      } catch (err: any) {
        error(err?.message || '计算可达权限矩阵失败');
      } finally {
        setMatrixLoading(false);
      }
    };

    fetchMatrix();
  }, [subjectType, selectedId]);

  const handleSwitchType = (type: 'group' | 'user' | 'device') => {
    setSubjectType(type);
    if (type === 'group' && groups.length > 0) {
      setSelectedId(groups[0].id);
    } else if (type === 'user' && users.length > 0) {
      setSelectedId(users[0].id);
    } else if (type === 'device' && devices.length > 0) {
      setSelectedId(devices[0].id);
    }
  };

  const handleRunProbe = async () => {
    if (!probeInput.trim()) return;
    setProbing(true);
    try {
      const res = await mockApi.probeAccessTarget(subjectType, selectedId, probeInput);
      setProbeResult(res);
    } catch {
      error('探测请求失败');
    } finally {
      setProbing(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Page Title & Controls */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">
            网络访问权限透视
          </h1>
          <p className="text-xs text-slate-500 mt-0.5">
            透视安全组、员工账号或终端在全网可达的物理子网、关键主机与内网 DNS 视界
          </p>
        </div>
      </div>

      {/* Perspective Tabs (Group / User / Device) */}
      <div className="flex flex-wrap items-center justify-between gap-4 p-2 bg-slate-100 rounded-xl border border-slate-200">
        <div className="flex items-center gap-1.5">
          <button
            onClick={() => handleSwitchType('group')}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              subjectType === 'group'
                ? 'bg-white text-blue-700 shadow-2xs'
                : 'text-slate-600 hover:text-slate-900'
            }`}
          >
            <Users className="w-3.5 h-3.5 text-purple-600" />
            <span>按安全组透视</span>
          </button>

          <button
            onClick={() => handleSwitchType('user')}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              subjectType === 'user'
                ? 'bg-white text-blue-700 shadow-2xs'
                : 'text-slate-600 hover:text-slate-900'
            }`}
          >
            <User className="w-3.5 h-3.5 text-blue-600" />
            <span>按员工账号透视</span>
          </button>

          <button
            onClick={() => handleSwitchType('device')}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              subjectType === 'device'
                ? 'bg-white text-blue-700 shadow-2xs'
                : 'text-slate-600 hover:text-slate-900'
            }`}
          >
            <Monitor className="w-3.5 h-3.5 text-amber-600" />
            <span>按终端设备透视</span>
          </button>
        </div>

        {/* Target Subject Dropdown */}
        <div className="flex items-center gap-2 w-full sm:w-72">
          <span className="text-xs text-slate-500 font-medium shrink-0">选择主体:</span>
          <Select
            value={selectedId}
            onChange={(e) => setSelectedId(e.target.value)}
            className="text-xs h-8.5 font-medium"
          >
            {subjectType === 'group' &&
              groups.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.name} ({g.memberCount}人)
                </option>
              ))}
            {subjectType === 'user' &&
              users.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.name} ({u.department})
                </option>
              ))}
            {subjectType === 'device' &&
              devices.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name} ({d.userName} • {d.virtualIp}) {d.status === 'revoked' ? '[已撤销]' : ''}
                </option>
              ))}
          </Select>
        </div>
      </div>

      {/* Subject Identity Banner */}
      {matrixData && (
        <Card className="p-4 bg-white border border-slate-200">
          <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
            <div className="flex items-center gap-3.5">
              <div className="w-11 h-11 rounded-xl bg-slate-100 flex items-center justify-center text-slate-700 border border-slate-200 shrink-0">
                {subjectType === 'group' ? (
                  <Users className="w-5 h-5 text-purple-600" />
                ) : subjectType === 'user' ? (
                  <User className="w-5 h-5 text-blue-600" />
                ) : (
                  <Monitor className="w-5 h-5 text-amber-600" />
                )}
              </div>
              <div>
                <div className="flex items-center gap-2.5">
                  <h2 className="text-sm font-bold text-slate-900 tracking-tight">
                    {matrixData.subject.name}
                  </h2>
                  {matrixData.subject.isBlocked ? (
                    <span className="px-2 py-0.5 rounded text-xs font-semibold bg-rose-50 text-rose-700 border border-rose-200 flex items-center gap-1">
                      <Lock className="w-3 h-3 text-rose-600" />
                      权限已熔断
                    </span>
                  ) : (
                    <span className="px-2 py-0.5 rounded text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200 flex items-center gap-1">
                      <Unlock className="w-3 h-3 text-emerald-600" />
                      正常授权通信
                    </span>
                  )}
                  {matrixData.subject.virtualIp && (
                    <span className="font-mono text-xs text-slate-600 bg-slate-100 px-2 py-0.5 rounded border border-slate-200">
                      IP: {matrixData.subject.virtualIp}
                    </span>
                  )}
                </div>
                <p className="text-xs text-slate-500 mt-0.5">{matrixData.subject.subtitle}</p>
                {matrixData.subject.groups.length > 0 && (
                  <div className="flex items-center gap-1.5 mt-1.5">
                    <span className="text-[11px] text-slate-400">继承组策略:</span>
                    {matrixData.subject.groups.map((g: string) => (
                      <span
                        key={g}
                        className="px-1.5 py-0.2 rounded text-[10px] font-medium bg-slate-100 text-slate-600 border border-slate-200"
                      >
                        {g}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>

            {/* Quick Stats Grid */}
            <div className="grid grid-cols-4 gap-3 bg-slate-50 p-2.5 rounded-xl border border-slate-200/80 text-center w-full md:w-auto">
              <div className="px-2.5">
                <span className="text-[10px] text-slate-500 block uppercase">可达子网</span>
                <span className="text-base font-bold text-slate-900 font-mono">
                  {matrixData.stats.accessibleSubnetsCount}
                </span>
              </div>
              <div className="px-2.5 border-l border-slate-200">
                <span className="text-[10px] text-slate-500 block uppercase">核心资产</span>
                <span className="text-base font-bold text-emerald-600 font-mono">
                  {matrixData.stats.accessiblePortsCount}
                </span>
              </div>
              <div className="px-2.5 border-l border-slate-200">
                <span className="text-[10px] text-slate-500 block uppercase">DNS 视界</span>
                <span className="text-base font-bold text-purple-600 font-mono">
                  {matrixData.stats.accessibleDnsCount}
                </span>
              </div>
              <div className="px-2.5 border-l border-slate-200">
                <span className="text-[10px] text-slate-500 block uppercase">安全评分</span>
                <span className="text-base font-bold text-blue-600 font-mono">
                  {matrixData.stats.securityScore}分
                </span>
              </div>
            </div>
          </div>
        </Card>
      )}

      {/* Permission Penetration Interactive SVG Flow Chart */}
      {matrixData && (
        <Card className="p-5 overflow-hidden">
          <div className="flex items-center justify-between pb-3 border-b border-slate-100">
            <div className="flex items-center gap-2">
              <Layers className="w-4 h-4 text-blue-600" />
              <CardTitle className="text-sm font-semibold">
                零信任访问链路拓扑推导 (Permission Path Topology)
              </CardTitle>
            </div>
            <span className="text-[11px] text-slate-400">
              主体身份 ──[WireGuard 隧道]──→ 出口汇聚网关 ──[微隔离策略]──→ 目标网络资产
            </span>
          </div>

          <div className="mt-3 relative w-full h-48 bg-slate-950 rounded-xl overflow-hidden border border-slate-800 flex items-center justify-around px-6 select-none">
            {/* Background cyber grid */}
            <div className="absolute inset-0 bg-[radial-gradient(#3b82f6_1px,transparent_1px)] [background-size:20px_20px] opacity-15" />

            {/* Left: Subject Node */}
            <div className="relative z-10 flex flex-col items-center text-center">
              <div
                className={`w-14 h-14 rounded-2xl flex items-center justify-center shadow-lg border ${
                  matrixData.subject.isBlocked
                    ? 'bg-rose-950/80 text-rose-400 border-rose-500 shadow-rose-900/50'
                    : 'bg-blue-950/80 text-blue-400 border-blue-500 shadow-blue-900/50'
                }`}
              >
                {subjectType === 'device' ? (
                  <Monitor className="w-6 h-6" />
                ) : subjectType === 'user' ? (
                  <User className="w-6 h-6" />
                ) : (
                  <Users className="w-6 h-6" />
                )}
              </div>
              <span className="text-xs font-bold text-white mt-2 max-w-[120px] truncate">
                {matrixData.subject.name}
              </span>
              <span className="text-[10px] text-slate-400 font-mono">
                {matrixData.subject.virtualIp || '身份主体'}
              </span>
            </div>

            {/* Middle: Gateways Node */}
            <div className="relative z-10 flex flex-col items-center text-center">
              <div
                className={`w-14 h-14 rounded-2xl flex items-center justify-center shadow-lg border ${
                  matrixData.subject.isBlocked
                    ? 'bg-slate-900 text-slate-600 border-slate-800'
                    : 'bg-indigo-950/80 text-indigo-400 border-indigo-500 shadow-indigo-900/50'
                }`}
              >
                <Server className="w-6 h-6" />
              </div>
              <span className="text-xs font-bold text-white mt-2">WireGuard 网关</span>
              <span className="text-[10px] text-slate-400 font-mono">北京/上海汇聚节点</span>
            </div>

            {/* Right: Target Assets Node Cluster */}
            <div className="relative z-10 flex flex-col items-center text-center">
              <div
                className={`w-14 h-14 rounded-2xl flex items-center justify-center shadow-lg border ${
                  matrixData.subject.isBlocked
                    ? 'bg-slate-900 text-slate-600 border-slate-800'
                    : 'bg-emerald-950/80 text-emerald-400 border-emerald-500 shadow-emerald-900/50'
                }`}
              >
                <Database className="w-6 h-6" />
              </div>
              <span className="text-xs font-bold text-white mt-2">目标云端资产群</span>
              <span className="text-[10px] text-emerald-400 font-mono">
                {matrixData.subject.isBlocked
                  ? '不可达 (Blocked)'
                  : `${matrixData.stats.accessibleSubnetsCount}网段 / ${matrixData.stats.accessiblePortsCount}数据库`}
              </span>
            </div>

            {/* Connecting SVG Flow lines */}
            <svg className="absolute inset-0 w-full h-full pointer-events-none">
              {matrixData.subject.isBlocked ? (
                <line
                  x1="22%"
                  y1="50%"
                  x2="48%"
                  y2="50%"
                  stroke="#ef4444"
                  strokeWidth="2.5"
                  strokeDasharray="4 4"
                  opacity="0.8"
                />
              ) : (
                <>
                  <line
                    x1="22%"
                    y1="50%"
                    x2="48%"
                    y2="50%"
                    stroke="#3b82f6"
                    strokeWidth="2.5"
                    opacity="0.8"
                  />
                  <line
                    x1="52%"
                    y1="50%"
                    x2="78%"
                    y2="50%"
                    stroke="#10b981"
                    strokeWidth="2.5"
                    opacity="0.8"
                  />
                  {/* Moving packet dots */}
                  <circle r="3.5" fill="#60a5fa">
                    <animate
                      attributeName="cx"
                      from="22%"
                      to="48%"
                      dur="2.5s"
                      repeatCount="indefinite"
                    />
                    <animate attributeName="cy" values="50%;50%" dur="2.5s" repeatCount="indefinite" />
                  </circle>
                  <circle r="3.5" fill="#34d399">
                    <animate
                      attributeName="cx"
                      from="52%"
                      to="78%"
                      dur="2.5s"
                      repeatCount="indefinite"
                    />
                    <animate attributeName="cy" values="50%;50%" dur="2.5s" repeatCount="indefinite" />
                  </circle>
                </>
              )}
            </svg>
          </div>
        </Card>
      )}

      {/* Two Column Layout: Left Resource Matrix Tabs, Right Live Reachability Prober */}
      {matrixData && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Left 2-Cols: Categorized Resource Inventory */}
          <div className="lg:col-span-2 space-y-4">
            {/* Filter Pills */}
            <div className="flex items-center gap-2">
              <button
                onClick={() => setResourceTab('all')}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
                  resourceTab === 'all'
                    ? 'bg-blue-600 text-white border-blue-600'
                    : 'bg-white text-slate-600 border-slate-200 hover:bg-slate-50'
                }`}
              >
                全部资源清单
              </button>
              <button
                onClick={() => setResourceTab('subnets')}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
                  resourceTab === 'subnets'
                    ? 'bg-blue-600 text-white border-blue-600'
                    : 'bg-white text-slate-600 border-slate-200 hover:bg-slate-50'
                }`}
              >
                物理子网 ({matrixData.accessibleSubnets.length})
              </button>
              <button
                onClick={() => setResourceTab('services')}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
                  resourceTab === 'services'
                    ? 'bg-blue-600 text-white border-blue-600'
                    : 'bg-white text-slate-600 border-slate-200 hover:bg-slate-50'
                }`}
              >
                服务器/数据库 ({matrixData.keyEndpoints.length})
              </button>
              <button
                onClick={() => setResourceTab('dns')}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
                  resourceTab === 'dns'
                    ? 'bg-blue-600 text-white border-blue-600'
                    : 'bg-white text-slate-600 border-slate-200 hover:bg-slate-50'
                }`}
              >
                Split DNS 域名 ({matrixData.dnsZones.length})
              </button>
              <button
                onClick={() => setResourceTab('blocked')}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
                  resourceTab === 'blocked'
                    ? 'bg-rose-600 text-white border-rose-600'
                    : 'bg-white text-rose-700 border-rose-200 hover:bg-rose-50'
                }`}
              >
                零信任阻断区 ({matrixData.deniedAssets.length})
              </button>
            </div>

            {/* 1. Subnets */}
            {(resourceTab === 'all' || resourceTab === 'subnets') && (
              <Card>
                <CardHeader className="py-3 px-4 border-b border-slate-100 flex flex-row items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Server className="w-4 h-4 text-blue-600" />
                    <span className="text-xs font-bold text-slate-900">
                      可访问的站点网络与局域子网 (Subnets & Sites)
                    </span>
                  </div>
                  <Badge variant="primary">{matrixData.accessibleSubnets.length} 个网段</Badge>
                </CardHeader>
                <div className="p-4 space-y-2.5">
                  {matrixData.accessibleSubnets.length === 0 ? (
                    <p className="text-xs text-slate-400 py-3 text-center">无任何可达子网</p>
                  ) : (
                    matrixData.accessibleSubnets.map((sub: any, idx: number) => (
                      <div
                        key={idx}
                        className="flex items-center justify-between p-3 rounded-xl bg-slate-50 border border-slate-200/80 hover:border-blue-300 transition-colors"
                      >
                        <div className="flex items-center gap-3">
                          <div className="w-2 h-2 rounded-full bg-emerald-500" />
                          <div>
                            <span className="text-xs font-bold text-slate-900">{sub.name}</span>
                            <div className="flex items-center gap-2 mt-0.5">
                              <span className="font-mono text-xs text-blue-600 font-bold bg-blue-50 px-1.5 py-0.5 rounded border border-blue-200">
                                {sub.cidr}
                              </span>
                              <span className="text-[11px] text-slate-500">
                                网关: {sub.gatewayName}
                              </span>
                            </div>
                          </div>
                        </div>
                        <div className="text-right text-xs">
                          <span className="text-emerald-700 font-semibold flex items-center gap-1 justify-end">
                            <CheckCircle2 className="w-3.5 h-3.5" />
                            允许通行
                          </span>
                          <span className="text-[10px] text-slate-400">
                            开放端口: {sub.ports}
                          </span>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </Card>
            )}

            {/* 2. Key Services & Databases */}
            {(resourceTab === 'all' || resourceTab === 'services') && (
              <Card>
                <CardHeader className="py-3 px-4 border-b border-slate-100 flex flex-row items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Database className="w-4 h-4 text-emerald-600" />
                    <span className="text-xs font-bold text-slate-900">
                      关键云端服务与数据库集群 (Services & Databases)
                    </span>
                  </div>
                  <Badge variant="success">{matrixData.keyEndpoints.length} 个端点</Badge>
                </CardHeader>
                <div className="p-4 space-y-2.5">
                  {matrixData.keyEndpoints.length === 0 ? (
                    <div className="p-6 text-center text-xs text-slate-400 bg-slate-50 rounded-xl border border-dashed border-slate-200">
                      该主体受到最小特权策略约束，<strong>未被授权访问生产数据库或管理平台</strong>。
                    </div>
                  ) : (
                    matrixData.keyEndpoints.map((svc: any, idx: number) => (
                      <div
                        key={idx}
                        className="flex items-center justify-between p-3 rounded-xl bg-emerald-50/30 border border-emerald-200 hover:border-emerald-300 transition-colors"
                      >
                        <div className="flex items-center gap-3">
                          <div className="w-8 h-8 rounded-lg bg-emerald-100 text-emerald-700 flex items-center justify-center font-bold text-xs">
                            {svc.category === 'database' ? 'DB' : svc.category === 'management' ? 'SSH' : 'API'}
                          </div>
                          <div>
                            <span className="text-xs font-bold text-slate-900">{svc.name}</span>
                            <div className="flex items-center gap-2 mt-0.5">
                              <span className="font-mono text-xs text-emerald-800 font-bold">
                                {svc.ip}:{svc.port}
                              </span>
                              <span className="text-[11px] text-slate-500 font-mono">
                                ({svc.protocol})
                              </span>
                            </div>
                          </div>
                        </div>
                        <Badge variant="success">已授权放行</Badge>
                      </div>
                    ))
                  )}
                </div>
              </Card>
            )}

            {/* 3. Accessible DNS Zones */}
            {(resourceTab === 'all' || resourceTab === 'dns') && (
              <Card>
                <CardHeader className="py-3 px-4 border-b border-slate-100 flex flex-row items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Compass className="w-4 h-4 text-purple-600" />
                    <span className="text-xs font-bold text-slate-900">
                      智能内网 Split DNS 视界路由 (Domain Suffixes)
                    </span>
                  </div>
                  <Badge variant="slate">{matrixData.dnsZones.length} 个域名视界</Badge>
                </CardHeader>
                <div className="p-4 grid grid-cols-1 sm:grid-cols-2 gap-3">
                  {matrixData.dnsZones.length === 0 ? (
                    <div className="col-span-2 p-4 text-center text-xs text-slate-400 bg-slate-50 rounded-xl">
                      暂无分配任何内网专用 DNS 视界
                    </div>
                  ) : (
                    matrixData.dnsZones.map((dns: any, idx: number) => (
                      <div
                        key={idx}
                        className="p-3 rounded-xl bg-purple-50/40 border border-purple-200 flex flex-col justify-between"
                      >
                        <div>
                          <div className="flex items-center justify-between">
                            <span className="font-mono text-xs font-bold text-purple-800">
                              {dns.suffix}
                            </span>
                            <span className="text-[10px] text-purple-600 bg-purple-100 px-1.5 py-0.5 rounded font-mono">
                              {dns.recordsCount} 静态记录
                            </span>
                          </div>
                          <p className="text-[11px] text-slate-600 mt-1">{dns.description}</p>
                        </div>
                        <div className="mt-2 pt-2 border-t border-purple-100 text-[10px] text-slate-400 font-mono">
                          网关: {dns.gateway}
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </Card>
            )}

            {/* 4. Denied & Blocked Assets */}
            {(resourceTab === 'all' || resourceTab === 'blocked') && (
              <Card className="border-rose-200">
                <CardHeader className="py-3 px-4 bg-rose-50/50 border-b border-rose-200 flex flex-row items-center justify-between">
                  <div className="flex items-center gap-2">
                    <AlertTriangle className="w-4 h-4 text-rose-600" />
                    <span className="text-xs font-bold text-rose-900">
                      零信任受限与显式阻断清单 (Denied / Restricted Policies)
                    </span>
                  </div>
                  <Badge variant="danger">{matrixData.deniedAssets.length} 项规则限制</Badge>
                </CardHeader>
                <div className="p-4 space-y-2.5">
                  {matrixData.deniedAssets.map((denied: any, idx: number) => (
                    <div
                      key={idx}
                      className="p-3 rounded-xl bg-white border border-rose-200 flex items-center justify-between"
                    >
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg bg-rose-100 text-rose-600 flex items-center justify-center font-bold">
                          <XCircle className="w-4 h-4" />
                        </div>
                        <div>
                          <span className="text-xs font-bold text-slate-900">
                            {denied.target}
                          </span>
                          <p className="text-[11px] text-rose-600 font-medium mt-0.5">
                            {denied.reason}
                          </p>
                        </div>
                      </div>
                      <div className="text-right">
                        <span className="font-mono text-xs text-rose-700 font-bold bg-rose-50 px-2 py-0.5 rounded border border-rose-200">
                          DENY: {denied.ports}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </Card>
            )}
          </div>

          {/* Right 1-Col: Live Reachability Prober Tool */}
          <div className="space-y-4">
            <Card className="border-slate-200 shadow-sm">
              <CardHeader className="bg-slate-50 border-b border-slate-100">
                <div className="flex items-center gap-2">
                  <Terminal className="w-4 h-4 text-blue-600" />
                  <CardTitle className="text-sm font-bold text-slate-900">
                    端到端实时连通性探测器
                  </CardTitle>
                </div>
                <p className="text-xs text-slate-500 mt-1">
                  输入任意目标 IP / 域名与端口，模拟当前主体发起访问，零信任引擎将实时评估是否放行。
                </p>
              </CardHeader>
              <CardContent className="p-4 space-y-3.5">
                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                    目标资产地址 (IP:端口 或 域名:端口)
                  </label>
                  <Input
                    value={probeInput}
                    onChange={(e) => setProbeInput(e.target.value)}
                    placeholder="例如: 192.168.10.50:3306"
                    className="font-mono text-xs"
                  />
                </div>

                {/* Quick Presets */}
                <div>
                  <span className="text-[11px] text-slate-400 block mb-1.5">快捷测试预设：</span>
                  <div className="flex flex-wrap gap-1.5">
                    <button
                      type="button"
                      onClick={() => setProbeInput('192.168.10.50:3306')}
                      className="px-2 py-1 rounded bg-slate-100 hover:bg-slate-200 text-slate-700 font-mono text-[10px] transition-colors"
                    >
                      MySQL (3306)
                    </button>
                    <button
                      type="button"
                      onClick={() => setProbeInput('192.168.20.100:8080')}
                      className="px-2 py-1 rounded bg-slate-100 hover:bg-slate-200 text-slate-700 font-mono text-[10px] transition-colors"
                    >
                      上海测试 (8080)
                    </button>
                    <button
                      type="button"
                      onClick={() => setProbeInput('10.0.0.1:22')}
                      className="px-2 py-1 rounded bg-slate-100 hover:bg-slate-200 text-slate-700 font-mono text-[10px] transition-colors"
                    >
                      堡垒机 (22)
                    </button>
                    <button
                      type="button"
                      onClick={() => setProbeInput('10.0.0.1:23')}
                      className="px-2 py-1 rounded bg-rose-50 hover:bg-rose-100 text-rose-700 font-mono text-[10px] transition-colors"
                    >
                      Telnet (23-高危)
                    </button>
                  </div>
                </div>

                <Button
                  className="w-full text-xs font-semibold gap-1.5 mt-2"
                  onClick={handleRunProbe}
                  isLoading={probing}
                >
                  <Sparkles className="w-3.5 h-3.5" />
                  <span>执行零信任权限探测</span>
                </Button>

                {/* Probe Output Card */}
                {probeResult && (
                  <div
                    className={`mt-3 p-3.5 rounded-xl border font-mono text-xs space-y-2.5 transition-all ${
                      probeResult.allowed
                        ? 'bg-emerald-950 text-emerald-100 border-emerald-500/40'
                        : 'bg-rose-950 text-rose-100 border-rose-500/40'
                    }`}
                  >
                    <div className="flex items-center justify-between border-b border-white/10 pb-2">
                      <span className="text-slate-400 text-[10px]">探测结果判定</span>
                      <span
                        className={`font-bold flex items-center gap-1 ${
                          probeResult.allowed ? 'text-emerald-400' : 'text-rose-400'
                        }`}
                      >
                        {probeResult.allowed ? (
                          <CheckCircle2 className="w-4 h-4" />
                        ) : (
                          <XCircle className="w-4 h-4" />
                        )}
                        {probeResult.statusLabel}
                      </span>
                    </div>

                    <div className="space-y-1 text-[11px]">
                      <div className="flex justify-between">
                        <span className="text-slate-400">探测目标:</span>
                        <span className="font-bold text-white">{probeResult.target}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-slate-400">出口网关:</span>
                        <span className="text-slate-200">{probeResult.gateway}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-slate-400">延迟测试:</span>
                        <span className="text-blue-300 font-bold">{probeResult.latency} ms</span>
                      </div>
                      <div className="pt-1 text-[10px] text-slate-300 border-t border-white/10 leading-relaxed">
                        <span className="text-slate-400 block mb-0.5">命中策略与判定依据:</span>
                        {probeResult.reason}
                      </div>
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>

            {/* ZTNA Micro-segmentation Principle Explainer */}
            <Card className="p-4 bg-blue-50/60 border-blue-200 text-xs text-blue-900 space-y-2">
              <div className="flex items-center gap-2 font-bold text-blue-950">
                <Shield className="w-4 h-4 text-blue-600" />
                <span>零信任微隔离工作原理</span>
              </div>
              <p className="text-[11px] text-slate-600 leading-relaxed">
                传统 VPN 采用“一旦拨入内网则全局漫游”的大二层结构；VeloTrust ZTNA 则在 WireGuard
                客户端内核配置严格的 <code>AllowedIPs</code> 仅路由白名单网段，并在网关处执行
                nftables/iptables 精细端口鉴权。
              </p>
            </Card>
          </div>
        </div>
      )}
    </div>
  );
};
