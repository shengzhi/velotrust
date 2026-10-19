import React, { useState, useEffect, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  ArrowLeft,
  Trash2,
  Copy,
  Check,
  RefreshCw,
  Server,
  Activity,
  Layers,
  ArrowDownLeft,
  ArrowUpRight,
  Network,
  Plus,
  Play,
  ExternalLink,
  ShieldCheck,
  Radio,
  CheckCircle2,
  Info,
} from 'lucide-react';
import { mockApi } from '../../data/mock';
import { Node, Peer, AclRule, AuditRecord, Site } from '../../types';
import { GatewayBehindNetworkDiagram } from '../../components/GatewayBehindNetworkDiagram';
import { Button } from '../../components/ui/button';
import { Card, CardHeader, CardTitle, CardContent } from '../../components/ui/card';
import { StatusBadge } from '../../components/StatusBadge';
import { Badge } from '../../components/ui/badge';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '../../components/ui/table';
import { Dialog } from '../../components/ui/dialog';
import { Input } from '../../components/ui/input';
import { Select } from '../../components/ui/select';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useToast } from '../../components/ui/toast';
import { formatBytes, formatRelativeTime, formatDateTime } from '../../utils/format';

interface BackendSubnetItem {
  cidr: string;
  name: string;
  type: 'site_primary' | 'nat_shadow' | 'custom_advertised';
  forwardingMode: string;
  status: 'active' | 'suspended';
  metric: number;
  description: string;
}

export const NodeDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { success, error } = useToast();

  const [node, setNode] = useState<Node | null>(null);
  const [site, setSite] = useState<Site | null>(null);
  const [allSites, setAllSites] = useState<Site[]>([]);
  const [subnetViewMode, setSubnetViewMode] = useState<'diagram' | 'table'>('diagram');
  const [peers, setPeers] = useState<Peer[]>([]);
  const [aclRules, setAclRules] = useState<AclRule[]>([]);
  const [nodeAudits, setNodeAudits] = useState<AuditRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [copiedKey, setCopiedKey] = useState(false);
  const [copiedCidr, setCopiedCidr] = useState<string | null>(null);

  // Subnet management
  const [showAddSubnetModal, setShowAddSubnetModal] = useState(false);
  const [newSubnetCidr, setNewSubnetCidr] = useState('');
  const [newSubnetName, setNewSubnetName] = useState('');
  const [newSubnetMode, setNewSubnetMode] = useState('Kernel_Direct');
  const [isSavingSubnet, setIsSavingSubnet] = useState(false);

  // Simulation modal
  const [simulationTarget, setSimulationTarget] = useState<string | null>(null);
  const [isSimulating, setIsSimulating] = useState(false);
  const [simulationStep, setSimulationStep] = useState<number>(0);

  // Delete modal
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // Subnet deletion confirm
  const [subnetToDelete, setSubnetToDelete] = useState<string | null>(null);

  const fetchDetails = async () => {
    if (!id) return;
    setLoading(true);
    try {
      const [nodeData, peersData, aclData, auditsData, sitesData] = await Promise.all([
        mockApi.getNode(id),
        mockApi.getPeers(id),
        mockApi.getAcl(id),
        mockApi.getAudit(),
        mockApi.getSites(),
      ]);
      setNode(nodeData);
      setPeers(peersData);
      setAclRules(aclData);

      // Find matching site
      setAllSites(sitesData);
      const foundSite = sitesData.find(
        (s) => s.id === nodeData.siteId || s.name === nodeData.siteName
      );
      setSite(foundSite || null);

      // Filter audits associated with this node name or id
      const filtered = auditsData.filter(
        (a) => a.node === nodeData.name || a.sourceIp.includes('10.0.1')
      );
      setNodeAudits(filtered.slice(0, 20));
    } catch (err: any) {
      error(err?.message || '加载节点数据失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDetails();
  }, [id]);

  // Compute all backend subnets for this gateway
  const backendSubnets = useMemo<BackendSubnetItem[]>(() => {
    if (!node || node.type !== 'gateway') return [];

    const items: BackendSubnetItem[] = [];
    const seenCidrs = new Set<string>();

    // 1. Site primary local subnet
    if (site?.localSubnet) {
      items.push({
        cidr: site.localSubnet,
        name: `${site.name.split(' ')[0]} 局域网段`,
        type: 'site_primary',
        forwardingMode: '内核直通 (Kernel IP Forwarding)',
        status: 'active',
        metric: 100,
        description: `所属站点 ${site.name} 的原生局域网`,
      });
      seenCidrs.add(site.localSubnet);
    }

    // 2. Site NAT Shadow Subnet (if VPC overlap NAT enabled)
    if (site?.natMapping?.enabled && site.natMapping.virtualCidr) {
      if (!seenCidrs.has(site.natMapping.virtualCidr)) {
        items.push({
          cidr: site.natMapping.virtualCidr,
          name: '跨云 1:1 NAT 影子网段',
          type: 'nat_shadow',
          forwardingMode: '1:1 双向 NAT 映射 (VPC Overlap NAT)',
          status: 'active',
          metric: 100,
          description: site.natMapping.description || '规避跨云同名网段冲突虚拟映射',
        });
        seenCidrs.add(site.natMapping.virtualCidr);
      }
    }

    // 3. Node configured allowedSubnets
    if (node.allowedSubnets && node.allowedSubnets.length > 0) {
      node.allowedSubnets.forEach((cidr, idx) => {
        if (!seenCidrs.has(cidr)) {
          items.push({
            cidr,
            name: idx === 0 ? '生产核心网络段' : `业务扩展子网 0${idx + 1}`,
            type: 'custom_advertised',
            forwardingMode: 'WireGuard AllowedIPs 通告',
            status: 'active',
            metric: 100 + idx * 10,
            description: `${node.networkName || node.siteName} 代理发布的内网路由段`,
          });
          seenCidrs.add(cidr);
        }
      });
    }

    return items;
  }, [node, site]);

  const handleCopyKey = () => {
    if (!node) return;
    navigator.clipboard.writeText(node.publicKey);
    setCopiedKey(true);
    success('公钥已复制到剪贴板');
    setTimeout(() => setCopiedKey(false), 2000);
  };

  const handleCopyCidr = (cidr: string) => {
    navigator.clipboard.writeText(cidr);
    setCopiedCidr(cidr);
    success(`网段 ${cidr} 已复制到剪贴板`);
    setTimeout(() => setCopiedCidr(null), 2000);
  };

  const handleDelete = async () => {
    if (!node) return;
    setDeleting(true);
    try {
      await mockApi.deleteNode(node.id);
      success(`节点 ${node.name} 已删除`);
      navigate('/nodes');
    } catch {
      error('删除节点失败');
    } finally {
      setDeleting(false);
    }
  };

  const handleAddSubnet = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!node) return;
    const trimmed = newSubnetCidr.trim();
    if (!trimmed) {
      error('请输入有效的子网 CIDR (如 10.20.0.0/16)');
      return;
    }
    // simple CIDR check
    if (!trimmed.includes('/')) {
      error('CIDR 格式需包含子网掩码长度，如 192.168.20.0/24');
      return;
    }

    const currentSubnets = node.allowedSubnets || [];
    if (currentSubnets.includes(trimmed)) {
      error('该网络段已存在于网关通告列表中');
      return;
    }

    setIsSavingSubnet(true);
    try {
      const updatedList = [...currentSubnets, trimmed];
      await mockApi.updateNode(node.id, {
        allowedSubnets: updatedList,
      });
      setNode({ ...node, allowedSubnets: updatedList });
      success(`成功向网关通告新增网络段: ${trimmed}`);
      setShowAddSubnetModal(false);
      setNewSubnetCidr('');
      setNewSubnetName('');
    } catch {
      error('保存通告网络段失败');
    } finally {
      setIsSavingSubnet(false);
    }
  };

  const handleConfirmDeleteSubnet = async () => {
    if (!node || !subnetToDelete) return;
    const currentSubnets = node.allowedSubnets || [];
    const updatedList = currentSubnets.filter((c) => c !== subnetToDelete);
    try {
      await mockApi.updateNode(node.id, {
        allowedSubnets: updatedList,
      });
      setNode({ ...node, allowedSubnets: updatedList });
      success(`已从网关通告中移除网段 ${subnetToDelete}`);
      setSubnetToDelete(null);
    } catch {
      error('移除网络段失败');
    }
  };

  const startRouteSimulation = (cidr: string) => {
    setSimulationTarget(cidr);
    setIsSimulating(true);
    setSimulationStep(1);

    setTimeout(() => {
      setSimulationStep(2);
      setTimeout(() => {
        setSimulationStep(3);
        setIsSimulating(false);
      }, 700);
    }, 600);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="flex flex-col items-center gap-3">
          <RefreshCw className="w-8 h-8 text-blue-600 animate-spin" />
          <p className="text-sm text-slate-500">正在获取节点配置与运行指标...</p>
        </div>
      </div>
    );
  }

  if (!node) {
    return (
      <div className="text-center py-20">
        <p className="text-base font-semibold text-slate-800">未找到该节点</p>
        <Button className="mt-4" onClick={() => navigate('/nodes')}>
          返回节点列表
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Top Header & Actions */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate('/nodes')}
            className="h-9 w-9 p-0"
          >
            <ArrowLeft className="w-4 h-4" />
          </Button>
          <div>
            <div className="flex items-center gap-2.5">
              <h1 className="text-xl font-bold text-slate-900 tracking-tight">
                {node.name}
              </h1>
              <StatusBadge status={node.status} />
              <Badge variant={node.type === 'gateway' ? 'primary' : 'success'}>
                {node.type.toUpperCase()}
              </Badge>
              {node.nodeTypeLabel && (
                <span className="text-xs bg-slate-100 text-slate-700 px-2 py-0.5 rounded-md font-medium">
                  {node.nodeTypeLabel}
                </span>
              )}
            </div>
            <p className="text-xs text-slate-500 font-mono mt-0.5">
              节点ID: {node.id} • 所属站点: {node.siteName}
              {node.networkName && ` • 业务网络: ${node.networkName}`}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {node.type === 'gateway' && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                const el = document.getElementById('advertised-subnets-card');
                if (el) el.scrollIntoView({ behavior: 'smooth' });
              }}
              className="gap-1.5 text-blue-600 border-blue-200 hover:bg-blue-50"
            >
              <Network className="w-3.5 h-3.5" />
              <span>查看代理网段 ({backendSubnets.length})</span>
            </Button>
          )}
          <Button
            variant="danger"
            size="sm"
            onClick={() => setShowDeleteModal(true)}
            className="gap-1.5"
          >
            <Trash2 className="w-3.5 h-3.5" />
            <span>删除节点</span>
          </Button>
        </div>
      </div>

      {/* 3 Overview Cards */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {/* Basic Info */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-semibold flex items-center gap-2">
              <Server className="w-4 h-4 text-blue-600" />
              <span>基本信息</span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-xs">
            <div className="flex items-center justify-between py-1 border-b border-slate-100">
              <span className="text-slate-500">虚拟 IP 地址</span>
              <span className="font-mono font-semibold text-slate-800">
                {node.virtualIp}
              </span>
            </div>
            <div className="flex items-center justify-between py-1 border-b border-slate-100">
              <span className="text-slate-500">网络站点</span>
              <span className="font-medium text-slate-800">{node.siteName}</span>
            </div>
            {node.type === 'gateway' && (
              <>
                <div className="flex items-center justify-between py-1 border-b border-slate-100">
                  <span className="text-slate-500">代理后端网络段</span>
                  <div className="flex items-center gap-1.5">
                    <span className="font-mono font-semibold text-blue-600">
                      {backendSubnets.length} 个网段
                    </span>
                    <button
                      onClick={() => {
                        const el = document.getElementById('advertised-subnets-card');
                        if (el) el.scrollIntoView({ behavior: 'smooth' });
                      }}
                      className="text-[11px] text-blue-600 hover:underline"
                    >
                      详情 ↓
                    </button>
                  </div>
                </div>
                {site?.natMapping?.enabled && (
                  <div className="flex items-center justify-between py-1 border-b border-slate-100">
                    <span className="text-slate-500">跨云 1:1 NAT 影子网段</span>
                    <span className="font-mono font-medium text-purple-700 bg-purple-50 px-1.5 py-0.5 rounded text-[11px]">
                      {site.natMapping.virtualCidr}
                    </span>
                  </div>
                )}
              </>
            )}
            <div className="flex items-center justify-between py-1 border-b border-slate-100">
              <span className="text-slate-500">创建接入时间</span>
              <span className="font-mono text-slate-700">
                {formatDateTime(node.createdAt)}
              </span>
            </div>
            <div className="pt-1">
              <span className="text-slate-500 block mb-1">WireGuard 公钥 (Public Key)</span>
              <div className="flex items-center gap-2 bg-slate-50 p-2 rounded-lg border border-slate-200/80 font-mono text-[11px] text-slate-700">
                <span className="truncate flex-1">{node.publicKey}</span>
                <button
                  onClick={handleCopyKey}
                  className="text-slate-400 hover:text-blue-600 transition-colors p-1"
                  title="复制公钥"
                >
                  {copiedKey ? (
                    <Check className="w-3.5 h-3.5 text-emerald-600" />
                  ) : (
                    <Copy className="w-3.5 h-3.5" />
                  )}
                </button>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Connection Status */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-semibold flex items-center gap-2">
              <Activity className="w-4 h-4 text-emerald-600" />
              <span>连接与网络端点</span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-xs">
            <div className="flex items-center justify-between py-1 border-b border-slate-100">
              <span className="text-slate-500">当前运行状态</span>
              <StatusBadge status={node.status} />
            </div>
            <div className="flex items-center justify-between py-1 border-b border-slate-100">
              <span className="text-slate-500">最后握手信号</span>
              <span className="font-mono text-slate-800 font-medium">
                {formatRelativeTime(node.lastHandshake)}
              </span>
            </div>
            <div className="flex items-center justify-between py-1 border-b border-slate-100">
              <span className="text-slate-500">公网 Endpoint</span>
              <span className="font-mono text-blue-600 font-medium">
                {node.endpoint}
              </span>
            </div>
            <div className="flex items-center justify-between py-1 border-b border-slate-100">
              <span className="text-slate-500">心跳保活间隔</span>
              <span className="font-mono text-slate-700">PersistentKeepalive = 25s</span>
            </div>
            <div className="flex items-center justify-between py-1">
              <span className="text-slate-500">有效 Peer 对端数</span>
              <span className="font-mono font-semibold text-slate-800">
                {peers.length} 个
              </span>
            </div>
          </CardContent>
        </Card>

        {/* Traffic Statistics */}
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-semibold flex items-center gap-2">
              <Layers className="w-4 h-4 text-purple-600" />
              <span>流量与吞吐指标</span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-xs">
            <div className="flex items-center justify-between py-1 border-b border-slate-100">
              <span className="text-slate-500 flex items-center gap-1.5">
                <ArrowDownLeft className="w-3.5 h-3.5 text-blue-600" />
                <span>累计接收 (RX)</span>
              </span>
              <span className="font-mono font-semibold text-blue-600">
                {formatBytes(node.rxBytes)}
              </span>
            </div>
            <div className="flex items-center justify-between py-1 border-b border-slate-100">
              <span className="text-slate-500 flex items-center gap-1.5">
                <ArrowUpRight className="w-3.5 h-3.5 text-emerald-600" />
                <span>累计发送 (TX)</span>
              </span>
              <span className="font-mono font-semibold text-emerald-600">
                {formatBytes(node.txBytes)}
              </span>
            </div>
            <div className="flex items-center justify-between py-1 border-b border-slate-100">
              <span className="text-slate-500">今日吞吐总量</span>
              <span className="font-mono font-medium text-slate-800">
                {formatBytes((node.rxBytes + node.txBytes) * 0.18)}
              </span>
            </div>
            <div className="flex items-center justify-between py-1">
              <span className="text-slate-500">加密算法套件</span>
              <span className="font-mono text-slate-700 text-[11px]">
                ChaCha20-Poly1305 / Curve25519
              </span>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* GATEWAY ONLY: Advertised Behind Subnets Card */}
      {node.type === 'gateway' ? (
        <Card id="advertised-subnets-card" className="border-blue-200/80 shadow-xs">
          <CardHeader className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3">
            <div>
              <div className="flex items-center gap-2">
                <Network className="w-4 h-4 text-blue-600" />
                <CardTitle className="text-base font-bold text-slate-900">
                  代理后端网络段与路由通告 (Behind Subnets & Advertised Routes)
                </CardTitle>
                <Badge variant="primary" className="text-xs">
                  {backendSubnets.length} 个通告网段
                </Badge>
              </div>
              <p className="text-xs text-slate-500 mt-1">
                该网关在 WireGuard Mesh 网络中代理转发的局域网/VPC 私有网络段（自动作为 AllowedIPs 下发至全网对端）
              </p>
            </div>
            <div className="flex items-center gap-2.5">
              {/* View Mode Toggle: Diagram (Default) vs Table */}
              <div className="flex items-center bg-slate-100 p-0.5 rounded-lg text-xs border border-slate-200">
                <button
                  type="button"
                  onClick={() => setSubnetViewMode('diagram')}
                  className={`px-3 py-1 rounded-md font-medium transition-colors ${
                    subnetViewMode === 'diagram'
                      ? 'bg-white text-blue-700 shadow-2xs font-semibold'
                      : 'text-slate-600 hover:text-slate-900'
                  }`}
                >
                  拓扑架构图
                </button>
                <button
                  type="button"
                  onClick={() => setSubnetViewMode('table')}
                  className={`px-3 py-1 rounded-md font-medium transition-colors ${
                    subnetViewMode === 'table'
                      ? 'bg-white text-blue-700 shadow-2xs font-semibold'
                      : 'text-slate-600 hover:text-slate-900'
                  }`}
                >
                  规则明细表
                </button>
              </div>

              {subnetViewMode === 'table' && (
                <>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      if (backendSubnets.length > 0) {
                        startRouteSimulation(backendSubnets[0].cidr);
                      }
                    }}
                    className="gap-1.5 text-xs text-slate-700 hover:text-blue-600"
                  >
                    <Play className="w-3.5 h-3.5 text-emerald-600" />
                    <span>全网路由仿真</span>
                  </Button>
                  <Button
                    size="sm"
                    onClick={() => setShowAddSubnetModal(true)}
                    className="gap-1.5 text-xs"
                  >
                    <Plus className="w-3.5 h-3.5" />
                    <span>添加通告网段</span>
                  </Button>
                </>
              )}
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            {subnetViewMode === 'diagram' ? (
              <GatewayBehindNetworkDiagram
                node={node}
                sites={allSites}
              />
            ) : (
              <>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>后端子网 (CIDR)</TableHead>
                  <TableHead>网段名称与类型</TableHead>
                  <TableHead>路由转发方式</TableHead>
                  <TableHead>广播状态</TableHead>
                  <TableHead>优先级 (Metric)</TableHead>
                  <TableHead>说明备注</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {backendSubnets.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7} className="text-center py-8 text-slate-500">
                      <div className="flex flex-col items-center gap-2">
                        <Network className="w-8 h-8 text-slate-300" />
                        <p className="text-sm">该网关暂未配置通告任何后端内网段</p>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setShowAddSubnetModal(true)}
                        >
                          添加首个后端子网
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ) : (
                  backendSubnets.map((sub) => (
                    <TableRow key={sub.cidr} className="group">
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-xs font-semibold text-blue-700 bg-blue-50 px-2 py-0.5 rounded border border-blue-200/60">
                            {sub.cidr}
                          </span>
                          <button
                            onClick={() => handleCopyCidr(sub.cidr)}
                            className="text-slate-400 hover:text-blue-600 transition-colors p-1"
                            title="复制 CIDR"
                          >
                            {copiedCidr === sub.cidr ? (
                              <Check className="w-3 h-3 text-emerald-600" />
                            ) : (
                              <Copy className="w-3 h-3" />
                            )}
                          </button>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="font-medium text-xs text-slate-800">
                          {sub.name}
                        </div>
                        <div className="text-[11px] text-slate-400">
                          {sub.type === 'site_primary'
                            ? '网络站点主局域网'
                            : sub.type === 'nat_shadow'
                            ? '跨云 NAT 影子段'
                            : 'Mesh 路由通告'}
                        </div>
                      </TableCell>
                      <TableCell>
                        <span className="inline-flex items-center gap-1.5 text-xs text-slate-700">
                          <Radio className="w-3 h-3 text-blue-500" />
                          <span>{sub.forwardingMode}</span>
                        </span>
                      </TableCell>
                      <TableCell>
                        <span className="inline-flex items-center gap-1 text-xs text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded-full font-medium border border-emerald-200/60">
                          <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
                          已通告至全网 ({peers.length} 对端)
                        </span>
                      </TableCell>
                      <TableCell className="font-mono text-xs text-slate-600">
                        {sub.metric}
                      </TableCell>
                      <TableCell className="text-xs text-slate-500 max-w-[200px] truncate" title={sub.description}>
                        {sub.description}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex items-center justify-end gap-1.5">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => startRouteSimulation(sub.cidr)}
                            className="h-7 text-xs text-blue-600 hover:bg-blue-50 px-2 gap-1"
                            title="连通性诊断仿真"
                          >
                            <Play className="w-3 h-3" />
                            <span>诊断</span>
                          </Button>
                          {sub.type === 'custom_advertised' && (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => setSubnetToDelete(sub.cidr)}
                              className="h-7 text-xs text-red-600 hover:bg-red-50 px-2"
                              title="移除通告"
                            >
                              <Trash2 className="w-3 h-3" />
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>

            {/* Explanatory callout banner */}
            <div className="bg-slate-50 rounded-xl p-3.5 border border-slate-200/80 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 text-xs text-slate-600">
              <div className="flex items-start gap-2.5">
                <Info className="w-4 h-4 text-blue-600 shrink-0 mt-0.5" />
                <div>
                  <span className="font-semibold text-slate-800">工作机制：</span>
                  网关将以上网段通告后，控制器会自动将各网段写入全网客户端及各分支网关的 WireGuard
                  <code className="text-blue-700 bg-blue-50 px-1 py-0.5 rounded mx-1 font-mono text-[11px]">
                    AllowedIPs
                  </code>
                  路由表中。流量到达本网关后将通过内核
                  <code className="text-slate-700 bg-white px-1 py-0.5 rounded mx-1 font-mono text-[11px] border border-slate-200">
                    net.ipv4.ip_forward=1
                  </code>
                  自动转发至局域网目标主机。
                </div>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => navigate('/sites')}
                className="shrink-0 gap-1 text-xs text-blue-600 border-blue-200 hover:bg-blue-50"
              >
                <span>管理站点 NAT 映射</span>
                <ExternalLink className="w-3 h-3" />
              </Button>
            </div>
            </>
            )}
          </CardContent>
        </Card>
      ) : null}

      {/* Peer List Table */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>已互联 Peer 对端列表</CardTitle>
              <p className="text-xs text-slate-500">
                该节点直接建立 WireGuard 握手通道的远端网关及终端
              </p>
            </div>
            <Badge variant="outline" className="text-xs font-mono">
              {peers.length} 个活跃对端
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>对端名称</TableHead>
                <TableHead>虚拟 IP</TableHead>
                <TableHead>公网 Endpoint</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>网络延迟</TableHead>
                <TableHead>最后握手</TableHead>
                <TableHead>对端流量 (RX / TX)</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {peers.map((peer) => (
                <TableRow key={peer.id}>
                  <TableCell className="font-medium text-slate-800">
                    {peer.peerName}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-700">
                    {peer.virtualIp}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-500">
                    {peer.endpoint}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={peer.status} />
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {peer.latencyMs > 0 ? (
                      <span
                        className={
                          peer.latencyMs > 100
                            ? 'text-amber-600 font-medium'
                            : 'text-emerald-600 font-medium'
                        }
                      >
                        {peer.latencyMs} ms
                      </span>
                    ) : (
                      <span className="text-slate-400">-</span>
                    )}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-500">
                    {formatRelativeTime(peer.lastHandshake)}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-600">
                    <span className="text-blue-600">↓ {formatBytes(peer.rxBytes)}</span>
                    <span className="mx-1 text-slate-300">/</span>
                    <span className="text-emerald-600">↑ {formatBytes(peer.txBytes)}</span>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* ACL Rules List */}
      <Card>
        <CardHeader>
          <CardTitle>该节点下发的 ACL 过滤规则</CardTitle>
          <p className="text-xs text-slate-500">
            经控制平面编译并下发给该节点内核的包过滤策略
          </p>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>方向</TableHead>
                <TableHead>源地址 (Source)</TableHead>
                <TableHead>目标地址 (Destination)</TableHead>
                <TableHead>端口限制</TableHead>
                <TableHead>动作</TableHead>
                <TableHead>优先级</TableHead>
                <TableHead>规则描述</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {aclRules.map((rule) => (
                <TableRow key={rule.id}>
                  <TableCell>
                    <Badge variant={rule.direction === 'ingress' ? 'primary' : 'warning'}>
                      {rule.direction.toUpperCase()}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-700">
                    {rule.source}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-700">
                    {rule.destination}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-700">
                    {rule.port}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={rule.action} />
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-500 font-medium">
                    {rule.priority}
                  </TableCell>
                  <TableCell className="text-xs text-slate-600">
                    {rule.description || '-'}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Node Recent Audit Logs */}
      <Card>
        <CardHeader>
          <CardTitle>该节点关联的安全审计日志</CardTitle>
          <p className="text-xs text-slate-500">最近 20 条流经该节点的数据包审计记录</p>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>时间戳</TableHead>
                <TableHead>发起人</TableHead>
                <TableHead>源 IP</TableHead>
                <TableHead>目的 IP</TableHead>
                <TableHead>目的端口</TableHead>
                <TableHead>动作判定</TableHead>
                <TableHead>数据大小</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {nodeAudits.map((a) => (
                <TableRow key={a.id}>
                  <TableCell className="font-mono text-xs text-slate-500">
                    {formatRelativeTime(a.timestamp)}
                  </TableCell>
                  <TableCell className="font-medium text-slate-800 text-xs">
                    {a.user}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-700">
                    {a.sourceIp}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-700">
                    {a.destIp}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-600">
                    {a.port}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={a.action} />
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-600">
                    {formatBytes(a.bytes)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* MODAL: Add Subnet */}
      <Dialog
        isOpen={showAddSubnetModal}
        onClose={() => setShowAddSubnetModal(false)}
        title="向网关添加通告后端网络段"
        description="配置网关所代理的内网 CIDR，保存后控制平面将自动通知 Mesh 全网节点写入该路由段。"
      >
        <form onSubmit={handleAddSubnet} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-slate-700 mb-1">
              后端子网 CIDR (必填)
            </label>
            <Input
              value={newSubnetCidr}
              onChange={(e) => setNewSubnetCidr(e.target.value)}
              placeholder="例如: 10.20.0.0/16 或 192.168.100.0/24"
              className="font-mono text-sm"
              required
            />
            <p className="text-[11px] text-slate-500 mt-1">
              请确保该网段位于该网关可直连的内网或 VPC 地址段内
            </p>
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-700 mb-1">
              网络段用途说明
            </label>
            <Input
              value={newSubnetName}
              onChange={(e) => setNewSubnetName(e.target.value)}
              placeholder="例如: 生产 Kubernetes Pod 段、财务内部系统局域网"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-700 mb-1">
              路由与转发模式
            </label>
            <Select
              value={newSubnetMode}
              onChange={(e) => setNewSubnetMode(e.target.value)}
            >
              <option value="Kernel_Direct">内核直接转发 (Kernel IP Forwarding - 默认推荐)</option>
              <option value="SNAT_Masquerade">网关源地址伪装 (SNAT Masquerade)</option>
              <option value="NAT_1to1">1:1 双向 NAT 映射 (VPC Overlap NAT)</option>
            </Select>
          </div>

          <div className="bg-blue-50 p-3 rounded-lg border border-blue-100 text-xs text-blue-800 space-y-1">
            <div className="font-semibold flex items-center gap-1.5">
              <ShieldCheck className="w-3.5 h-3.5 text-blue-600" />
              <span>控制平面自动同步</span>
            </div>
            <p className="text-blue-700 text-[11px]">
              提交后，该 CIDR 将即时更新至网关本地 WireGuard 配置及所有已授权对端的 AllowedIPs 列表中。
            </p>
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setShowAddSubnetModal(false)}
            >
              取消
            </Button>
            <Button
              type="submit"
              size="sm"
              isLoading={isSavingSubnet}
            >
              确认添加并广播
            </Button>
          </div>
        </form>
      </Dialog>

      {/* MODAL: Route Simulation */}
      <Dialog
        isOpen={!!simulationTarget}
        onClose={() => {
          setSimulationTarget(null);
          setSimulationStep(0);
        }}
        title="网关路由与子网连通性仿真"
        description={`模拟测试从远程 Mesh 终端向后端网段 [${simulationTarget}] 注入探测流量的路由跳数与连通延迟。`}
      >
        <div className="space-y-4">
          <div className="bg-slate-900 text-white p-4 rounded-xl space-y-3 font-mono text-xs">
            <div className="flex items-center justify-between text-slate-400 pb-2 border-b border-slate-800">
              <span>模拟源: dev-macbook (10.0.1.101)</span>
              <span>目标网段: {simulationTarget}</span>
            </div>

            <div className="space-y-2">
              <div className="flex items-center gap-2 text-slate-300">
                <span className="text-blue-400">HOP 1:</span>
                <span>客户端隧道接口 (wg0) ➔ 查表 AllowedIPs 命中 {simulationTarget}</span>
                {simulationStep >= 1 && (
                  <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400 ml-auto" />
                )}
              </div>

              <div className="flex items-center gap-2 text-slate-300">
                <span className="text-purple-400">HOP 2:</span>
                <span>WireGuard 加密通道 (ChaCha20) ➔ 网关 {node.name} ({node.virtualIp})</span>
                {simulationStep >= 2 ? (
                  <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400 ml-auto" />
                ) : simulationStep === 1 ? (
                  <RefreshCw className="w-3 h-3 text-blue-400 animate-spin ml-auto" />
                ) : null}
              </div>

              <div className="flex items-center gap-2 text-slate-300">
                <span className="text-emerald-400">HOP 3:</span>
                <span>网关内核转发 (ip_forward) ➔ 目标内网主机 (响应 64 bytes)</span>
                {simulationStep >= 3 ? (
                  <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400 ml-auto" />
                ) : simulationStep === 2 ? (
                  <RefreshCw className="w-3 h-3 text-blue-400 animate-spin ml-auto" />
                ) : null}
              </div>
            </div>

            {simulationStep >= 3 && (
              <div className="pt-2 border-t border-slate-800 flex items-center justify-between text-emerald-400 font-semibold">
                <span>连通性状态: 畅通 (0% 丢包)</span>
                <span>RTT 往返时延: 8.4 ms</span>
              </div>
            )}
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button
              variant="outline"
              size="sm"
              disabled={isSimulating}
              onClick={() => {
                if (simulationTarget) startRouteSimulation(simulationTarget);
              }}
            >
              重新测试
            </Button>
            <Button
              size="sm"
              onClick={() => {
                setSimulationTarget(null);
                setSimulationStep(0);
              }}
            >
              完成
            </Button>
          </div>
        </div>
      </Dialog>

      {/* Delete Subnet Confirm */}
      <ConfirmDialog
        isOpen={!!subnetToDelete}
        onClose={() => setSubnetToDelete(null)}
        onConfirm={handleConfirmDeleteSubnet}
        title="确认移除通告网段"
        description={`确定要从网关 "${node.name}" 的路由通告中移除网段 "${subnetToDelete}" 吗？移除后全网节点将不再向该网段转发流量。`}
        danger={true}
      />

      {/* Delete Node Confirmation */}
      <ConfirmDialog
        isOpen={showDeleteModal}
        onClose={() => setShowDeleteModal(false)}
        onConfirm={handleDelete}
        title="确认删除该节点"
        description={`确定要删除节点 "${node.name}" 吗？此操作不可逆，将从整个 WireGuard 拓扑中注销此端点。`}
        danger={true}
        isLoading={deleting}
      />
    </div>
  );
};
