import React, { useState } from 'react';
import { Node, Site } from '../types';
import {
  Server,
  Network,
  Radio,
  ArrowRight,
  Database,
  Cpu,
  Layers,
  Shield,
  Activity,
  Play,
  Copy,
  Check,
  ExternalLink,
  ChevronRight,
  X,
  CheckCircle2,
  Share2,
} from 'lucide-react';
import { BehindSubnetInfo, getBehindSubnetsForNode } from '../utils/networkTopology';
import { Button } from './ui/button';
import { Badge } from './ui/badge';
import { StatusBadge } from './StatusBadge';

interface GatewayBehindNetworkDiagramProps {
  node: Node;
  sites: Site[];
  allGatewayNodes?: Node[];
  onSelectGateway?: (node: Node) => void;
  onClose?: () => void;
  onNavigateDetail?: (nodeId: string) => void;
}

interface SubnetServiceHost {
  id: string;
  name: string;
  ip: string;
  role: string;
  icon: 'database' | 'server' | 'cpu' | 'layers';
  status: 'online' | 'standby';
  port?: string;
}

// Generate realistic mock services/hosts for each backend subnet
function getMockHostsForSubnet(cidr: string, nodeName: string): SubnetServiceHost[] {
  const prefix = cidr.split('.')[0] + '.' + cidr.split('.')[1] + '.' + cidr.split('.')[2];

  if (cidr.includes('192.168.10.')) {
    return [
      {
        id: 'h1',
        name: '生产数据库主节点',
        ip: `${prefix}.15`,
        port: '5432 (Postgres)',
        role: '核心数据存储',
        icon: 'database',
        status: 'online',
      },
      {
        id: 'h2',
        name: '业务 API 集群',
        ip: `${prefix}.20~25`,
        port: '8080 (REST/gRPC)',
        role: '微服务网关 Pods',
        icon: 'server',
        status: 'online',
      },
      {
        id: 'h3',
        name: '本地应用缓存',
        ip: `${prefix}.30`,
        port: '6379 (Redis)',
        role: '高频读写缓存',
        icon: 'layers',
        status: 'online',
      },
    ];
  }

  if (cidr.includes('172.31.10.')) {
    return [
      {
        id: 'h-nat1',
        name: '跨云 1:1 NAT 转换网关',
        ip: `${prefix}.1`,
        port: '所有端口映射',
        role: 'NAT 虚拟路由器',
        icon: 'cpu',
        status: 'online',
      },
      {
        id: 'h-nat2',
        name: '跨云同步对端 Pod',
        ip: `${prefix}.100`,
        port: '9000 (S3 Sync)',
        role: '跨云无冲突互通',
        icon: 'server',
        status: 'online',
      },
    ];
  }

  if (cidr.includes('192.168.30.')) {
    return [
      {
        id: 'h-sz1',
        name: '物理裸金属计算节点',
        ip: `${prefix}.11`,
        port: 'SSH/22',
        role: '高算力 AI 训练机',
        icon: 'cpu',
        status: 'online',
      },
      {
        id: 'h-sz2',
        name: 'IDC 存储 SAN 存储阵列',
        ip: `${prefix}.50`,
        port: 'iSCSI/NFS',
        role: '本地持久化',
        icon: 'database',
        status: 'online',
      },
    ];
  }

  if (cidr.includes('10.100.')) {
    return [
      {
        id: 'h-fra1',
        name: '灾备只读复制从库',
        ip: `${prefix}.8`,
        port: '5432 (Replica)',
        role: '容灾冷备份',
        icon: 'database',
        status: 'online',
      },
      {
        id: 'h-fra2',
        name: '海外中继代理',
        ip: `${prefix}.10`,
        port: '443 (Proxy)',
        role: '数据同步中继',
        icon: 'server',
        status: 'online',
      },
    ];
  }

  if (cidr.includes('172.16.') || cidr.includes('10.200.')) {
    return [
      {
        id: 'h-us1',
        name: '分支办公研发工作站',
        ip: `${prefix}.15`,
        port: 'RDP/SSH',
        role: '北美研发办公',
        icon: 'server',
        status: 'online',
      },
      {
        id: 'h-us2',
        name: '分支本地 NAS',
        ip: `${prefix}.2`,
        port: 'SMB/445',
        role: '本地研发文件共享',
        icon: 'database',
        status: 'online',
      },
    ];
  }

  // Fallback default host cluster
  return [
    {
      id: 'h-def1',
      name: `${nodeName} 局域网服务群`,
      ip: `${prefix}.10~50`,
      port: '80, 443',
      role: '内部业务 Pods',
      icon: 'server',
      status: 'online',
    },
    {
      id: 'h-def2',
      name: '本地网关代理口',
      ip: `${prefix}.1`,
      port: 'Gateway/DNS',
      role: '子网出口路由',
      icon: 'cpu',
      status: 'online',
    },
  ];
}

export const GatewayBehindNetworkDiagram: React.FC<GatewayBehindNetworkDiagramProps> = ({
  node,
  sites,
  allGatewayNodes = [],
  onSelectGateway,
  onClose,
  onNavigateDetail,
}) => {
  const [copiedText, setCopiedText] = useState<string | null>(null);
  const [pingStatus, setPingStatus] = useState<{ [key: string]: { latency: number; testing: boolean } }>({});
  const [selectedSubnetIndex, setSelectedSubnetIndex] = useState<number>(0);

  const subnets = getBehindSubnetsForNode(node, sites);
  const site = sites.find((s) => s.id === node.siteId || s.gatewayNodeId === node.id);

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedText(text);
    setTimeout(() => setCopiedText(null), 2000);
  };

  const handleTestPing = (id: string) => {
    setPingStatus((prev) => ({
      ...prev,
      [id]: { latency: 0, testing: true },
    }));

    setTimeout(() => {
      const mockLatency = Math.floor(Math.random() * 12) + 6; // 6ms - 18ms
      setPingStatus((prev) => ({
        ...prev,
        [id]: { latency: mockLatency, testing: false },
      }));
    }, 500);
  };

  const currentSubnet = subnets[selectedSubnetIndex] || subnets[0];
  const subnetHosts = currentSubnet ? getMockHostsForSubnet(currentSubnet.cidr, node.name) : [];

  return (
    <div className="w-full bg-white rounded-xl border border-slate-200 shadow-xs overflow-hidden select-none transition-all">
      {/* Top Header Bar */}
      <div className="px-5 py-3.5 bg-slate-50/80 border-b border-slate-200 flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-blue-100/80 border border-blue-200 flex items-center justify-center text-blue-700 shadow-2xs">
            <Network className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-sm font-bold text-slate-900">
                {node.name} · 后端局域网络拓扑架构图
              </h2>
              <StatusBadge status={node.status} />
              <Badge variant="primary" className="text-[11px] py-0 px-2 font-normal">
                {node.nodeTypeLabel || '站点接入网关'}
              </Badge>
            </div>
            <p className="text-xs text-slate-500 mt-0.5">
              直观呈现该网关后方代理的 VPC/局域网段、内核转发路径、NAT 映射以及局域网服务分布
            </p>
          </div>
        </div>

        {/* Header Action Controls */}
        <div className="flex items-center gap-2.5">
          {/* Gateway Switcher Dropdown */}
          {allGatewayNodes.length > 1 && onSelectGateway && (
            <div className="flex items-center gap-1.5 text-xs text-slate-500 bg-white border border-slate-200 rounded-lg px-2.5 py-1 shadow-2xs">
              <span>切换网关:</span>
              <select
                value={node.id}
                onChange={(e) => {
                  const target = allGatewayNodes.find((n) => n.id === e.target.value);
                  if (target) onSelectGateway(target);
                }}
                className="bg-transparent text-slate-800 font-medium focus:outline-hidden cursor-pointer"
              >
                {allGatewayNodes.map((gw) => (
                  <option key={gw.id} value={gw.id}>
                    {gw.name} ({gw.siteName})
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Jump to Node Details */}
          {onNavigateDetail && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => onNavigateDetail(node.id)}
              className="gap-1.5 text-xs"
            >
              <span>节点详情</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </Button>
          )}

          {/* Close Diagram Button */}
          {onClose && (
            <button
              onClick={onClose}
              className="p-1.5 text-slate-400 hover:text-slate-700 hover:bg-slate-200/60 rounded-lg transition-colors"
              title="收起局域网络图"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>

      {/* Main Diagram Canvas: Multi-tier Network Flow */}
      <div className="p-6 overflow-x-auto bg-gradient-to-b from-slate-50/40 via-white to-slate-50/30">
        <div className="min-w-[880px] space-y-6">
          {/* --- TIER 1 to TIER 3 Architecture Flow Diagram --- */}
          <div className="grid grid-cols-12 gap-4 items-stretch relative">
            {/* --- Level 1: WireGuard Mesh Backbone Ingress (cols 1-3) --- */}
            <div className="col-span-3 rounded-xl border border-blue-200 bg-blue-50/40 p-4 flex flex-col justify-between shadow-2xs relative">
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-[11px] font-semibold text-blue-900 uppercase tracking-wider flex items-center gap-1.5">
                    <Shield className="w-3.5 h-3.5 text-blue-600" />
                    <span>Layer 1: WireGuard Mesh</span>
                  </span>
                  <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
                </div>

                <div className="p-3 bg-white rounded-lg border border-blue-100 space-y-1.5 shadow-2xs">
                  <div className="flex justify-between items-center text-xs">
                    <span className="text-slate-500">骨干通道</span>
                    <span className="font-mono font-bold text-blue-700">wg0 (UDP:51820)</span>
                  </div>
                  <div className="flex justify-between items-center text-xs">
                    <span className="text-slate-500">虚拟网 IP</span>
                    <span className="font-mono font-semibold text-slate-800">{node.virtualIp}</span>
                  </div>
                  <div className="flex justify-between items-center text-xs">
                    <span className="text-slate-500">公网 Endpoint</span>
                    <span className="font-mono text-slate-600 text-[11px] truncate max-w-[120px]">
                      {node.endpoint}
                    </span>
                  </div>
                </div>

                <div className="text-[11px] text-blue-800/80 bg-blue-100/50 p-2 rounded-md leading-relaxed">
                  通过 ChaCha20-Poly1305 加密隧道与全网其他 Hub 及网关建立直连互联。
                </div>
              </div>

              {/* Ingress packet flow badge */}
              <div className="mt-3 pt-2.5 border-t border-blue-100 flex items-center justify-between text-[11px] text-slate-500">
                <span>实时流量通道</span>
                <span className="font-mono text-blue-700 font-semibold">直连双向打通</span>
              </div>
            </div>

            {/* Connecting Bridge 1 -> 2 */}
            <div className="col-span-1 flex flex-col items-center justify-center">
              <div className="w-full flex items-center justify-center">
                <div className="h-0.5 flex-1 bg-gradient-to-r from-blue-300 to-indigo-400 relative">
                  <span className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-2 h-2 rounded-full bg-blue-600 animate-ping" />
                </div>
                <ArrowRight className="w-4 h-4 text-indigo-500 shrink-0 -ml-1" />
              </div>
              <span className="text-[9px] font-mono text-slate-400 mt-1">内核路由</span>
            </div>

            {/* --- Level 2: Gateway Host & Routing / NAT Engine (cols 5-8) --- */}
            <div className="col-span-4 rounded-xl border border-indigo-200 bg-indigo-50/30 p-4 flex flex-col justify-between shadow-2xs">
              <div className="space-y-2.5">
                <div className="flex items-center justify-between">
                  <span className="text-[11px] font-semibold text-indigo-900 uppercase tracking-wider flex items-center gap-1.5">
                    <Cpu className="w-3.5 h-3.5 text-indigo-600" />
                    <span>Layer 2: 网关内核转发核心</span>
                  </span>
                  <Badge variant="default" className="text-[10px] bg-indigo-100 text-indigo-800 border-indigo-200">
                    ip_forward = 1
                  </Badge>
                </div>

                {/* Gateway Host Specs */}
                <div className="p-3 bg-white rounded-lg border border-indigo-100 space-y-2 shadow-2xs">
                  <div className="flex justify-between items-center text-xs">
                    <span className="text-slate-500">网关主机</span>
                    <span className="font-medium text-slate-800">{node.name}</span>
                  </div>
                  <div className="flex justify-between items-center text-xs">
                    <span className="text-slate-500">网络站点</span>
                    <span className="font-medium text-slate-700">{node.siteName}</span>
                  </div>
                  <div className="flex justify-between items-center text-xs">
                    <span className="text-slate-500">局域网卡</span>
                    <span className="font-mono text-slate-700">eth0 / br0 (物理内网接口)</span>
                  </div>
                </div>

                {/* NAT Mapping Box (If Active) */}
                {site?.natMapping?.enabled ? (
                  <div className="p-2.5 rounded-lg bg-purple-50 border border-purple-200 text-xs space-y-1.5">
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-purple-900 flex items-center gap-1">
                        <Share2 className="w-3 h-3 text-purple-600" />
                        <span>跨云 1:1 NAT 映射引擎</span>
                      </span>
                      <span className="text-[10px] px-1.5 py-0.2 rounded bg-purple-200 text-purple-800 font-medium">
                        双向无冲突
                      </span>
                    </div>
                    <div className="flex items-center justify-between bg-white/80 px-2.5 py-1.5 rounded border border-purple-100 font-mono text-[11px]">
                      <span className="text-slate-700">{site.localSubnet}</span>
                      <span className="text-purple-600 font-bold">⇄</span>
                      <span className="text-purple-700 font-bold">{site.natMapping.virtualCidr}</span>
                    </div>
                  </div>
                ) : (
                  <div className="p-2 rounded-lg bg-emerald-50/70 border border-emerald-100 text-[11px] text-emerald-800 flex items-center justify-between">
                    <span className="flex items-center gap-1.5">
                      <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" />
                      <span>原生 VPC 直通（无地址冲突）</span>
                    </span>
                    <span className="font-mono text-[10px] text-emerald-700">Direct Route</span>
                  </div>
                )}
              </div>

              <div className="mt-2.5 pt-2 border-t border-indigo-100 flex items-center justify-between text-[11px] text-slate-500">
                <span>通告至 Mesh 策略</span>
                <span className="font-mono text-indigo-700 font-semibold">AllowedIPs 自动广播</span>
              </div>
            </div>

            {/* Connecting Bridge 2 -> 3 */}
            <div className="col-span-1 flex flex-col items-center justify-center">
              <div className="w-full flex items-center justify-center">
                <div className="h-0.5 flex-1 bg-gradient-to-r from-indigo-300 to-emerald-400 relative">
                  <span className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-2 h-2 rounded-full bg-indigo-600 animate-ping" />
                </div>
                <ArrowRight className="w-4 h-4 text-emerald-500 shrink-0 -ml-1" />
              </div>
              <span className="text-[9px] font-mono text-slate-400 mt-1">交换机总线</span>
            </div>

            {/* --- Level 3: Behind Subnets Switch Bus & Selector (cols 10-12) --- */}
            <div className="col-span-3 rounded-xl border border-emerald-200 bg-emerald-50/40 p-4 flex flex-col justify-between shadow-2xs">
              <div className="space-y-2.5">
                <div className="flex items-center justify-between">
                  <span className="text-[11px] font-semibold text-emerald-900 uppercase tracking-wider flex items-center gap-1.5">
                    <Network className="w-3.5 h-3.5 text-emerald-600" />
                    <span>Layer 3: 代理后端网络段</span>
                  </span>
                  <Badge variant="success" className="text-[10px]">
                    {subnets.length} 个子网
                  </Badge>
                </div>

                <p className="text-[11px] text-slate-500 leading-relaxed">
                  选择下方具体网段，即可在底栏查看其直连的服务节点架构与连通状态：
                </p>

                {/* Subnet selector cards */}
                <div className="space-y-2">
                  {subnets.map((sub, idx) => {
                    const isSelected = idx === selectedSubnetIndex;
                    const isNat = sub.type === 'nat_mapped';

                    return (
                      <button
                        key={sub.id}
                        onClick={() => setSelectedSubnetIndex(idx)}
                        className={`w-full p-2.5 rounded-lg border text-left transition-all flex items-center justify-between shadow-2xs ${
                          isSelected
                            ? isNat
                              ? 'bg-purple-50 border-purple-400 ring-2 ring-purple-200'
                              : 'bg-emerald-50 border-emerald-400 ring-2 ring-emerald-200'
                            : 'bg-white border-slate-200 hover:border-slate-300'
                        }`}
                      >
                        <div>
                          <div className="flex items-center gap-1.5">
                            <span className="font-mono font-bold text-xs text-slate-900">
                              {sub.cidr}
                            </span>
                            <span
                              className={`text-[9px] px-1 rounded font-medium ${
                                isNat ? 'bg-purple-100 text-purple-700' : 'bg-emerald-100 text-emerald-700'
                              }`}
                            >
                              {isNat ? '1:1 NAT' : '本地原生'}
                            </span>
                          </div>
                          <span className="text-[10px] text-slate-500 block truncate max-w-[150px]">
                            {sub.name}
                          </span>
                        </div>
                        <ChevronRight
                          className={`w-4 h-4 transition-transform ${
                            isSelected ? 'text-emerald-700 translate-x-0.5' : 'text-slate-300'
                          }`}
                        />
                      </button>
                    );
                  })}
                </div>
              </div>

              <div className="mt-2.5 pt-2 border-t border-emerald-100 text-[10px] text-emerald-800 flex justify-between items-center">
                <span>可用 IP 池</span>
                <span className="font-mono font-bold">{currentSubnet?.ipCount || 254} 个地址</span>
              </div>
            </div>
          </div>

          {/* --- TIER 4: Detailed Subnet Internal Host Cluster Diagram --- */}
          {currentSubnet && (
            <div className="p-4 rounded-xl bg-white border border-slate-200 shadow-2xs space-y-4">
              {/* Subnet Cluster Header */}
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-100">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-lg bg-slate-100 text-slate-700">
                    <Database className="w-4 h-4 text-blue-600" />
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-bold text-slate-900 font-mono">
                        {currentSubnet.cidr}
                      </span>
                      <span className="text-xs text-slate-600">({currentSubnet.name})</span>
                      <button
                        onClick={() => handleCopy(currentSubnet.cidr)}
                        className="text-slate-400 hover:text-slate-600 transition-colors"
                        title="复制 CIDR"
                      >
                        {copiedText === currentSubnet.cidr ? (
                          <Check className="w-3.5 h-3.5 text-emerald-600" />
                        ) : (
                          <Copy className="w-3.5 h-3.5" />
                        )}
                      </button>
                    </div>
                    <p className="text-[11px] text-slate-500 mt-0.5">
                      路由模式：<strong className="text-slate-700">{currentSubnet.routingMode}</strong> · 广播状态：正常
                    </p>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <span className="text-[11px] text-slate-400">
                    该网段后方承载的实际业务 Pod / 物理主机拓扑：
                  </span>
                </div>
              </div>

              {/* Connected Host / Service Nodes Grid */}
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3.5">
                {subnetHosts.map((host) => {
                  const ping = pingStatus[host.id];

                  return (
                    <div
                      key={host.id}
                      className="p-3.5 rounded-lg border border-slate-200 bg-slate-50/50 hover:bg-slate-50 hover:border-blue-200 transition-all space-y-2.5 shadow-2xs"
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex items-center gap-2">
                          <div className="w-7 h-7 rounded-md bg-white border border-slate-200 flex items-center justify-center text-blue-600 shadow-2xs">
                            {host.icon === 'database' ? (
                              <Database className="w-3.5 h-3.5" />
                            ) : host.icon === 'cpu' ? (
                              <Cpu className="w-3.5 h-3.5" />
                            ) : (
                              <Server className="w-3.5 h-3.5" />
                            )}
                          </div>
                          <div>
                            <span className="font-semibold text-slate-900 text-xs block">
                              {host.name}
                            </span>
                            <span className="text-[10px] text-slate-500">{host.role}</span>
                          </div>
                        </div>

                        <span className="flex items-center gap-1 text-[10px] text-emerald-600 font-medium bg-emerald-50 px-1.5 py-0.5 rounded border border-emerald-100">
                          <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
                          在线
                        </span>
                      </div>

                      {/* IP & Port Info */}
                      <div className="p-2 bg-white rounded border border-slate-200/80 space-y-1 text-xs">
                        <div className="flex justify-between items-center">
                          <span className="text-slate-400 text-[11px]">局域网 IP</span>
                          <span className="font-mono font-bold text-slate-800">{host.ip}</span>
                        </div>
                        {host.port && (
                          <div className="flex justify-between items-center text-[11px]">
                            <span className="text-slate-400">服务端口</span>
                            <span className="font-mono text-blue-600">{host.port}</span>
                          </div>
                        )}
                      </div>

                      {/* Diagnostic Ping Button & Latency */}
                      <div className="flex items-center justify-between pt-1">
                        <button
                          onClick={() => handleTestPing(host.id)}
                          disabled={ping?.testing}
                          className="flex items-center gap-1 px-2 py-1 rounded bg-white hover:bg-blue-50 text-slate-700 hover:text-blue-700 border border-slate-200 text-[10px] font-medium transition-colors shadow-2xs"
                        >
                          <Play className={`w-2.5 h-2.5 ${ping?.testing ? 'animate-spin text-blue-600' : 'text-emerald-600'}`} />
                          <span>{ping?.testing ? '探测中...' : '连通性测试'}</span>
                        </button>

                        {ping && !ping.testing ? (
                          <span className="font-mono text-[11px] text-emerald-700 font-semibold">
                            {ping.latency} ms (0% 丢包)
                          </span>
                        ) : (
                          <span className="text-[10px] text-slate-400">路由健康</span>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
