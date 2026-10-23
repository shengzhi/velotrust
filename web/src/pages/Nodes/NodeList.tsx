import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Activity, ExternalLink, Key, RefreshCw, Search, Server } from 'lucide-react';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Select } from '../../components/ui/select';
import { Badge } from '../../components/ui/badge';
import { StatusBadge } from '../../components/StatusBadge';
import { Card } from '../../components/ui/card';
import { EmptyState } from '../../components/EmptyState';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../../components/ui/table';
import { useToast } from '../../components/ui/toast';
import { formatRelativeTime } from '../../utils/format';
import { VirtualNode, VirtualNodeStatus, virtualNodeApi } from '../../services/node';

const typeLabels: Record<VirtualNode['node_type'], string> = {
  client_agent: '客户端 Agent',
  gateway: 'Gateway',
  relay: 'Relay',
  connector: 'Connector',
};

const statusLabels: Record<VirtualNodeStatus, string> = {
  pending: '待注册',
  active: '已启用',
  disabled: '已禁用',
  quarantined: '已隔离',
  revoked: '已撤销',
};

const healthLabels: Record<VirtualNode['health_status'], string> = {
  unknown: '未知',
  healthy: '健康',
  degraded: '降级',
  unhealthy: '异常',
};

const connectionLabels: Record<VirtualNode['connection_status'], string> = {
  online: '在线',
  offline: '离线',
  unknown: '未连接',
};

function isGatewayApprovalPending(node: VirtualNode): boolean {
  return node.node_type === 'gateway'
    && node.node_status === 'pending'
    && node.require_approval
    && Boolean(node.public_key && node.registered_at);
}

export const NodeList: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const { error } = useToast();
  const [nodes, setNodes] = useState<VirtualNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [refreshFailed, setRefreshFailed] = useState(false);
  const [search, setSearch] = useState(searchParams.get('q') ?? '');
  const [typeFilter, setTypeFilter] = useState(searchParams.get('type') ?? 'all');
  const [statusFilter, setStatusFilter] = useState(searchParams.get('status') ?? 'all');
  const approvalOnly = searchParams.get('approval') === 'pending';

  const loadNodes = async (refresh = false, notifyError = refresh) => {
    if (refresh) setRefreshing(true);
    try {
      setNodes(await virtualNodeApi.list());
      setRefreshFailed(false);
    } catch (err) {
      setRefreshFailed(true);
      if (notifyError) error(err instanceof Error ? err.message : '获取虚拟节点失败');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    void loadNodes(false, true);
  }, []);

  useEffect(() => {
    const timer = window.setInterval(() => void loadNodes(), 15000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    const params: Record<string, string> = {};
    if (search) params.q = search;
    if (typeFilter !== 'all') params.type = typeFilter;
    if (statusFilter !== 'all') params.status = statusFilter;
    if (approvalOnly) params.approval = 'pending';
    setSearchParams(params, { replace: true });
  }, [search, typeFilter, statusFilter, approvalOnly, setSearchParams]);

  const filteredNodes = useMemo(() => {
    const normalizedSearch = search.trim().toLowerCase();
    return nodes.filter((node) => {
      const matchesSearch = !normalizedSearch || [
        node.node_name,
        node.node_code,
        node.public_endpoint ?? '',
        node.private_endpoint ?? '',
        node.ipam_address ?? '',
        node.networks.map((network) => `${network.network_name} ${network.cidr}`).join(' '),
      ].some((value) => value.toLowerCase().includes(normalizedSearch));
      return matchesSearch
        && (typeFilter === 'all' || node.node_type === typeFilter)
        && (statusFilter === 'all' || node.node_status === statusFilter)
        && (!approvalOnly || isGatewayApprovalPending(node));
    });
  }, [nodes, search, typeFilter, statusFilter, approvalOnly]);

  const approvalCount = useMemo(() => nodes.filter(isGatewayApprovalPending).length, [nodes]);
  const countByStatus = useMemo(() => nodes.reduce<Record<string, number>>((counts, node) => {
    if (node.node_status === 'pending' && isGatewayApprovalPending(node)) return counts;
    counts[node.node_status] = (counts[node.node_status] ?? 0) + 1;
    return counts;
  }, {}), [nodes]);

  const lastSeenLabel = (value?: string | null) => value ? formatRelativeTime(value) : '从未上报';
  const displayedConnectionStatus = (node: VirtualNode) => refreshFailed ? 'unknown' : node.connection_status;

  return (
    <div className="space-y-5">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-slate-900">虚拟节点管理</h1>
          <p className="mt-1 text-xs text-slate-500">管理 Client Agent、Gateway、Relay 和 Connector 的身份与生命周期。</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => void loadNodes(true)} isLoading={refreshing} className="gap-1.5">
            <RefreshCw className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`} />
            刷新
          </Button>
          <Button variant="outline" size="sm" onClick={() => navigate('/enrollment-tokens')} className="gap-1.5">
            <Key className="h-3.5 w-3.5" />
            注册凭证
          </Button>
        </div>
      </div>

      <Card className={`flex flex-col gap-3 border p-4 sm:flex-row sm:items-center sm:justify-between ${approvalCount > 0 ? 'border-amber-200 bg-amber-50' : 'border-slate-200 bg-white'}`}>
        <div>
          <div className={`text-sm font-semibold ${approvalCount > 0 ? 'text-amber-900' : 'text-slate-800'}`}>网关注册审批</div>
          <p className={`mt-1 text-xs ${approvalCount > 0 ? 'text-amber-800' : 'text-slate-500'}`}>
            {approvalCount > 0 ? `有 ${approvalCount} 个网关已提交注册，正在等待审批。` : '需要审批的网关完成注册后会显示在这里。'}
          </p>
        </div>
        <Button
          variant={approvalCount > 0 ? 'primary' : 'outline'}
          size="sm"
          className="shrink-0"
          onClick={() => {
            setTypeFilter(approvalOnly ? 'all' : 'gateway');
            setStatusFilter('all');
            setSearchParams((previous) => {
              const next = new URLSearchParams(previous);
              if (approvalOnly) next.delete('approval');
              else next.set('approval', 'pending');
              return next;
            });
          }}
        >
          {approvalOnly ? '查看全部节点' : `查看待审批网关（${approvalCount}）`}
        </Button>
      </Card>

      <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
        {(['pending', 'active', 'disabled', 'quarantined', 'revoked'] as VirtualNodeStatus[]).map((status) => (
          <Card key={status} className="p-3.5">
            <div className="text-xs text-slate-500">{statusLabels[status]}</div>
            <div className="mt-1 text-xl font-semibold text-slate-900">{countByStatus[status] ?? 0}</div>
          </Card>
        ))}
      </div>

      <Card className="p-4">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <div className="sm:col-span-1">
            <Input
              placeholder="搜索节点名称、编码、端点或网络..."
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              leftIcon={<Search className="h-4 w-4 text-slate-400" />}
            />
          </div>
          <Select value={typeFilter} onChange={(event) => setTypeFilter(event.target.value)}>
            <option value="all">全部节点类型</option>
            {Object.entries(typeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </Select>
          <Select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
            <option value="all">全部生命周期状态</option>
            {Object.entries(statusLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </Select>
        </div>
      </Card>

      <Card>
        {refreshFailed && (
          <div role="alert" className="border-b border-amber-200 bg-amber-50 px-5 py-3 text-sm text-amber-900">
            节点列表刷新失败，当前连接状态已标记为未知。请检查管理服务连接后重试。
          </div>
        )}
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>节点</TableHead>
              <TableHead>类型</TableHead>
              <TableHead>IPAM 地址</TableHead>
              <TableHead>网络</TableHead>
              <TableHead>生命周期</TableHead>
              <TableHead>连接状态</TableHead>
              <TableHead>运行健康</TableHead>
              <TableHead>最后心跳</TableHead>
              <TableHead>端点</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow><TableCell colSpan={10} className="py-12 text-center text-slate-400"><RefreshCw className="mx-auto mb-2 h-5 w-5 animate-spin" />正在加载节点...</TableCell></TableRow>
            ) : filteredNodes.length === 0 ? (
              <TableRow>
                <TableCell colSpan={10} className="p-8">
                  <EmptyState
                    icon={Server}
                    title={nodes.length === 0 ? '尚无虚拟节点' : '没有匹配的虚拟节点'}
                    description={nodes.length === 0 ? '网关身份通过注册凭证预建；其他节点需由对应注册流程写入节点目录。' : '调整搜索条件或清除筛选后重试。'}
                    actionText="清除筛选"
                    onAction={() => {
                      setSearch('');
                      setTypeFilter('all');
                      setStatusFilter('all');
                      setSearchParams((previous) => {
                        const next = new URLSearchParams(previous);
                        next.delete('approval');
                        return next;
                      });
                    }}
                  />
                </TableCell>
              </TableRow>
            ) : filteredNodes.map((node) => (
              <TableRow key={node.id} className="group">
                <TableCell>
                  <div className="font-medium text-slate-900">{node.node_name}</div>
                  <div className="mt-0.5 font-mono text-[11px] text-slate-500">{node.node_code} · #{node.id}</div>
                </TableCell>
                <TableCell><Badge variant={node.node_type === 'gateway' ? 'primary' : 'outline'}>{typeLabels[node.node_type]}</Badge></TableCell>
                <TableCell className="font-mono text-xs text-slate-700">{node.ipam_address ?? <span className="font-sans text-slate-400">未分配</span>}</TableCell>
                <TableCell className="max-w-[220px]">
                  {node.networks.length === 0 ? <span className="text-xs text-slate-400">未关联网络</span> : (
                    <div className="space-y-1">
                      {node.networks.slice(0, 2).map((network) => <div key={network.id} className="truncate text-xs text-slate-700" title={`${network.network_name} · ${network.cidr}`}>{network.network_name} <span className="font-mono text-slate-400">{network.cidr}</span></div>)}
                      {node.networks.length > 2 && <div className="text-[11px] text-slate-400">另有 {node.networks.length - 2} 个网络</div>}
                    </div>
                  )}
                </TableCell>
                <TableCell><StatusBadge status={node.node_status} label={isGatewayApprovalPending(node) ? '待审批' : statusLabels[node.node_status]} /></TableCell>
                <TableCell>
                  <Badge variant={displayedConnectionStatus(node) === 'online' ? 'success' : displayedConnectionStatus(node) === 'offline' ? 'danger' : 'outline'}>
                    {connectionLabels[displayedConnectionStatus(node)]}
                  </Badge>
                </TableCell>
                <TableCell>
                  <span className={`inline-flex items-center gap-1.5 text-xs ${node.health_status === 'healthy' ? 'text-emerald-700' : node.health_status === 'degraded' ? 'text-amber-700' : node.health_status === 'unhealthy' ? 'text-rose-700' : 'text-slate-500'}`}>
                    <Activity className="h-3.5 w-3.5" />{healthLabels[node.health_status]}
                  </span>
                </TableCell>
                <TableCell className="text-xs text-slate-600">{lastSeenLabel(node.last_heartbeat_at)}</TableCell>
                <TableCell className="max-w-[180px] truncate font-mono text-[11px] text-slate-600" title={node.public_endpoint ?? node.private_endpoint ?? ''}>{node.public_endpoint ?? node.private_endpoint ?? '—'}</TableCell>
                <TableCell className="text-right">
                  <Button variant="ghost" size="sm" onClick={() => navigate(`/nodes/${node.id}`)} className="gap-1 text-blue-600">
                    详情 <ExternalLink className="h-3 w-3" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {!loading && <div className="border-t border-slate-100 px-5 py-3 text-xs text-slate-500">显示 {filteredNodes.length} / {nodes.length} 个节点</div>}
      </Card>
    </div>
  );
};
