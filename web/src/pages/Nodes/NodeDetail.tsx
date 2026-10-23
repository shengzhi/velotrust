import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ArrowLeft, CheckCircle2, Copy, Layers, Network, RefreshCw, Server, ShieldAlert, XCircle } from 'lucide-react';
import { Button } from '../../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card';
import { Badge } from '../../components/ui/badge';
import { StatusBadge } from '../../components/StatusBadge';
import { Select } from '../../components/ui/select';
import { Input } from '../../components/ui/input';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useToast } from '../../components/ui/toast';
import { VirtualNode, VirtualNodeStatus, virtualNodeApi } from '../../services/node';
import { IPAMLease, IPAMPool, ipamApi } from '../../services/ipam';
import { formatDateTime } from '../../utils/format';

const statusLabels: Record<VirtualNodeStatus, string> = {
  pending: '待注册',
  active: '已启用',
  disabled: '已禁用',
  quarantined: '已隔离',
  revoked: '已撤销',
};

const typeLabels: Record<VirtualNode['node_type'], string> = {
  client_agent: '客户端 Agent',
  gateway: 'Gateway',
  relay: 'Relay',
  connector: 'Connector',
};

function statusOptions(status: VirtualNodeStatus): VirtualNodeStatus[] {
  switch (status) {
    case 'pending': return ['active', 'disabled', 'quarantined', 'revoked'];
    case 'active': return ['disabled', 'quarantined', 'revoked'];
    case 'disabled': return ['active', 'quarantined', 'revoked'];
    case 'quarantined': return ['active', 'disabled', 'revoked'];
    case 'revoked': return [];
  }
}

export const NodeDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { success, error } = useToast();
  const [node, setNode] = useState<VirtualNode | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [refreshFailed, setRefreshFailed] = useState(false);
  const [saving, setSaving] = useState(false);
  const [targetStatus, setTargetStatus] = useState<VirtualNodeStatus | ''>('');
  const [confirmRevoke, setConfirmRevoke] = useState(false);
  const [copied, setCopied] = useState(false);
  const [gatewayPools, setGatewayPools] = useState<IPAMPool[]>([]);
  const [gatewayLease, setGatewayLease] = useState<IPAMLease | null>(null);
  const [gatewayIPAMLoading, setGatewayIPAMLoading] = useState(false);
  const [gatewayIPAMError, setGatewayIPAMError] = useState('');
  const [selectedGatewayPool, setSelectedGatewayPool] = useState('');
  const [requestedGatewayIP, setRequestedGatewayIP] = useState('');
  const [gatewayAddressSaving, setGatewayAddressSaving] = useState(false);
  const [publicEndpointInput, setPublicEndpointInput] = useState('');
  const [publicEndpointEditing, setPublicEndpointEditing] = useState(false);
  const [publicEndpointSaving, setPublicEndpointSaving] = useState(false);
  const loadGatewayAddress = async (gateway: VirtualNode) => {
    setGatewayIPAMLoading(true);
    setGatewayIPAMError('');
    try {
      const [pools, leases] = await Promise.all([
        ipamApi.listPools(),
        ipamApi.listLeases(),
      ]);
      const activeGatewayPools = pools.filter((pool) => pool.type === 'Gateway' && pool.status === 'active');
      const activeGatewayLeases = leases
        .filter((lease) => lease.node_type === 'Gateway' && lease.status !== 'Released')
        .sort((left, right) => right.allocated_at.localeCompare(left.allocated_at));
      const lease = gateway.public_key
        ? activeGatewayLeases.find((item) => item.wireguard_pubkey === gateway.public_key)
          ?? activeGatewayLeases.find((item) => item.target_name === gateway.node_name && !item.wireguard_pubkey)
        : activeGatewayLeases.find((item) => item.target_name === gateway.node_name);
      setGatewayPools(activeGatewayPools);
      setGatewayLease(lease ?? null);
      setSelectedGatewayPool((current) => activeGatewayPools.some((pool) => String(pool.id) === current)
        ? current
        : String(activeGatewayPools[0]?.id ?? ''));
      setRequestedGatewayIP('');
    } catch (cause) {
      setGatewayIPAMError(cause instanceof Error ? cause.message : '加载 Gateway 地址分配信息失败');
    } finally {
      setGatewayIPAMLoading(false);
    }
  };

  const loadNode = async (manualRefresh = false, notifyError = manualRefresh, refreshGatewayDetails = true) => {
    if (manualRefresh) setRefreshing(true);
    const nodeID = Number(id);
    if (!Number.isSafeInteger(nodeID) || nodeID <= 0) {
      setNode(null);
      setLoading(false);
      setRefreshing(false);
      return;
    }
    try {
      const result = await virtualNodeApi.get(nodeID);
      setNode(result);
      setRefreshFailed(false);
      if (refreshGatewayDetails) {
        setPublicEndpointInput(result.public_endpoint ?? '');
        if (!manualRefresh) setTargetStatus('');
      }
      if (refreshGatewayDetails && result.node_type === 'gateway') {
        const detailLoad = loadGatewayAddress(result);
        if (manualRefresh) await detailLoad;
      }
    } catch (err) {
      setRefreshFailed(true);
      if (notifyError) error(err instanceof Error ? err.message : '加载虚拟节点失败');
      if (!manualRefresh && refreshGatewayDetails) setNode(null);
    } finally {
      setLoading(false);
      if (manualRefresh) setRefreshing(false);
    }
  };

  useEffect(() => {
    void loadNode(false, true);
    const timer = window.setInterval(() => void loadNode(false, false, false), 15000);
    return () => window.clearInterval(timer);
  }, [id]);

  const saveStatus = async (status: VirtualNodeStatus) => {
    if (!node) return;
    setSaving(true);
    try {
      await virtualNodeApi.setStatus(node, status);
      success(`节点状态已更新为“${statusLabels[status]}”`);
      setConfirmRevoke(false);
      await loadNode();
    } catch (err) {
      error(err instanceof Error ? err.message : '更新节点状态失败');
    } finally {
      setSaving(false);
    }
  };

  const savePublicEndpoint = async () => {
    if (!node) return;
    setPublicEndpointSaving(true);
    try {
      await virtualNodeApi.setPublicEndpoint(node, publicEndpointInput.trim());
      setPublicEndpointEditing(false);
      success(publicEndpointInput.trim() ? '公网端点已更新' : '公网端点已清除');
      await loadNode();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '保存公网端点失败');
    } finally {
      setPublicEndpointSaving(false);
    }
  };

  const assignGatewayAddress = async () => {
    if (!node || node.node_type !== 'gateway') return;
    const pool = gatewayPools.find((item) => String(item.id) === selectedGatewayPool);
    if (!pool) {
      error('请先选择一个启用的 Gateway 地址池');
      return;
    }
    if (pool.allocation_mode === 'static' && !requestedGatewayIP.trim()) {
      error('静态地址池需要填写要分配的 IPv4 地址');
      return;
    }
    setGatewayAddressSaving(true);
    try {
      const allocatedLease = await ipamApi.allocateAddress({
        pool_id: pool.id,
        target_name: node.node_name,
        node_type: 'Gateway',
        ...(pool.allocation_mode === 'static' ? { requested_ip: requestedGatewayIP.trim() } : {}),
        wireguard_pubkey: node.public_key ?? undefined,
        endpoint: node.public_endpoint ?? undefined,
      });
      setGatewayLease(allocatedLease);
      success(`已分配 ${allocatedLease.virtual_ip}/32；HUB 会从 IPAM 租约生成隧道配置`);
      await loadGatewayAddress(node);
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '分配 Gateway 地址失败');
    } finally {
      setGatewayAddressSaving(false);
    }
  };

  const copyPublicKey = async () => {
    if (!node?.public_key) return;
    try {
      await navigator.clipboard.writeText(node.public_key);
      setCopied(true);
      success('公钥已复制');
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      error('复制失败，请手动选择公钥内容');
    }
  };

  const options = useMemo(() => {
    if (!node) return [];
    const registered = Boolean(node.public_key) && (node.node_type !== 'gateway' || Boolean(node.registered_at));
    return statusOptions(node.node_status).filter((status) => status !== 'active' || registered);
  }, [node]);

  if (loading) {
    return <div className="flex h-80 items-center justify-center text-sm text-slate-500"><RefreshCw className="mr-2 h-4 w-4 animate-spin" />正在加载节点信息...</div>;
  }

  if (!node) {
    return (
      <div className="py-20 text-center">
        <p className="font-semibold text-slate-800">未找到该虚拟节点</p>
        <Button className="mt-4" onClick={() => navigate('/nodes')}>返回虚拟节点列表</Button>
      </div>
    );
  }

  const approvalPending = node.node_type === 'gateway'
    && node.node_status === 'pending'
    && node.require_approval
    && Boolean(node.public_key && node.registered_at);
  const displayValue = (value?: string | null) => value || '—';
  const dateValue = (value?: string | null) => value ? formatDateTime(value) : '从未上报';
  const connectionStatus = refreshFailed ? 'unknown' : node.connection_status;
  const leasedGatewayAddress = gatewayLease ? `${gatewayLease.virtual_ip}/32` : '';
  const selectedPool = gatewayPools.find((pool) => String(pool.id) === selectedGatewayPool);
  const gatewayGroups = node.gateway_groups;

  return (
    <div className="space-y-5">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex items-start gap-3">
          <Button variant="outline" size="sm" onClick={() => navigate('/nodes')} className="mt-0.5 gap-1.5">
            <ArrowLeft className="h-3.5 w-3.5" />返回
          </Button>
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-xl font-bold tracking-tight text-slate-900">{node.node_name}</h1>
              <StatusBadge status={node.node_status} label={approvalPending ? '待审批' : statusLabels[node.node_status]} />
              <Badge variant={node.node_type === 'gateway' ? 'primary' : 'outline'}>{typeLabels[node.node_type]}</Badge>
            </div>
            <p className="mt-1 font-mono text-xs text-slate-500">{node.node_code} · 节点 ID {node.id} · 版本 {node.version_no}</p>
          </div>
        </div>
        <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:items-center">
          <Button variant="outline" size="sm" disabled={refreshing || saving || gatewayAddressSaving} onClick={() => void loadNode(true)} className="gap-1.5">
            <RefreshCw className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`} />刷新
          </Button>
          {approvalPending ? (
            <div className="flex w-full gap-2 sm:w-auto">
              <Button size="sm" disabled={saving} isLoading={saving} onClick={() => void saveStatus('active')} className="gap-1.5">
                <CheckCircle2 className="h-4 w-4" />批准注册
              </Button>
              <Button variant="outline" size="sm" disabled={saving} onClick={() => setConfirmRevoke(true)} className="gap-1.5 text-rose-700 hover:text-rose-800">
                <XCircle className="h-4 w-4" />拒绝注册
              </Button>
            </div>
          ) : options.length > 0 && (
            <div className="flex w-full items-center gap-2 sm:w-auto">
              <Select className="min-w-40" value={targetStatus} onChange={(event) => setTargetStatus(event.target.value as VirtualNodeStatus | '')}>
                <option value="">选择生命周期操作</option>
                {options.map((status) => <option key={status} value={status}>{statusLabels[status]}</option>)}
              </Select>
              <Button size="sm" disabled={!targetStatus || saving} isLoading={saving} onClick={() => targetStatus === 'revoked' ? setConfirmRevoke(true) : targetStatus && void saveStatus(targetStatus)}>
                应用
              </Button>
            </div>
          )}
        </div>
      </div>

      {refreshFailed && (
        <div role="alert" className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">
          节点状态刷新失败，当前连接状态已标记为未知。请检查管理服务连接后重试。
        </div>
      )}

      {node.node_status === 'revoked' && (
        <div className="flex items-start gap-2 rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-800">
          <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0" />
          <span>此节点已撤销，撤销是终态。若需要重新接入，请通过相应注册流程创建新的节点身份。</span>
        </div>
      )}

      {approvalPending && (
        <div className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">
          <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0" />
          <span>Gateway 已使用注册凭证完成注册并绑定公钥，当前仍处于待审批状态。批准后 HUB 会结束等待并要求网关重新进行身份认证；拒绝会撤销此节点身份。</span>
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader><CardTitle className="flex items-center gap-2 text-sm"><Server className="h-4 w-4 text-blue-600" />节点身份与运行状态</CardTitle></CardHeader>
          <CardContent className="space-y-3 text-xs">
            <InfoRow label="节点类型" value={typeLabels[node.node_type]} />
            <InfoRow label="生命周期状态" value={<StatusBadge status={node.node_status} label={approvalPending ? '待审批' : statusLabels[node.node_status]} />} />
            <InfoRow label="HUB 连接状态" value={<Badge variant={connectionStatus === 'online' ? 'success' : connectionStatus === 'offline' ? 'danger' : 'outline'}>{connectionStatus === 'online' ? '在线' : connectionStatus === 'offline' ? '离线' : '未知'}</Badge>} />
            <InfoRow label="健康状态" value={node.health_status} />
            <InfoRow label="Agent 版本" value={displayValue(node.agent_version)} />
            <InfoRow label="最近心跳" value={dateValue(node.last_heartbeat_at)} />
            <InfoRow label="注册时间" value={dateValue(node.registered_at)} />
            <InfoRow label="创建时间" value={dateValue(node.created_at)} />
            {node.node_type === 'gateway' && <>
              <InfoRow label="网关类型" value={displayValue(node.gateway_type)} />
              <InfoRow label="注册网络标签" value={displayValue(node.gateway_network_name)} />
              <InfoRow label="注册后需审批" value={node.require_approval ? '是' : '否'} />
            </>}
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle className="text-sm">端点与公钥</CardTitle></CardHeader>
          <CardContent className="space-y-3 text-xs">
            <InfoRow label="控制端点" value={displayValue(node.control_endpoint)} mono />
            <div className="space-y-1.5">
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="text-slate-500">公网 WireGuard 端点</div>
                  {publicEndpointEditing ? (
                    <div className="mt-1.5 flex flex-col gap-2 sm:flex-row">
                      <Input
                        id="node-public-endpoint"
                        value={publicEndpointInput}
                        onChange={(event) => setPublicEndpointInput(event.target.value)}
                        placeholder="gateway.example.com:51820"
                        maxLength={255}
                        className="h-8 min-w-0 flex-1 font-mono text-xs"
                        aria-label="公网 WireGuard 端点"
                        autoFocus
                      />
                      <div className="flex shrink-0 gap-2">
                        <Button
                          size="sm"
                          disabled={publicEndpointSaving || refreshing || publicEndpointInput.trim() === (node.public_endpoint ?? '')}
                          isLoading={publicEndpointSaving}
                          onClick={() => void savePublicEndpoint()}
                        >保存</Button>
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={publicEndpointSaving}
                          onClick={() => {
                            setPublicEndpointInput(node.public_endpoint ?? '');
                            setPublicEndpointEditing(false);
                          }}
                        >取消</Button>
                      </div>
                    </div>
                  ) : (
                    <div className="mt-1 break-all font-mono font-medium text-slate-800">{node.public_endpoint || '未设置'}</div>
                  )}
                  {publicEndpointEditing && <p className="mt-1.5 text-[11px] leading-4 text-slate-500">填写其他节点可访问的 UDP 地址和端口；NAT 后请填写公网映射端口。留空并保存可清除。</p>}
                </div>
                {!publicEndpointEditing && (
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={refreshing || saving || gatewayAddressSaving}
                    onClick={() => {
                      setPublicEndpointInput(node.public_endpoint ?? '');
                      setPublicEndpointEditing(true);
                    }}
                    className="shrink-0"
                  >编辑</Button>
                )}
              </div>
            </div>
            <InfoRow label="私网端点" value={displayValue(node.private_endpoint)} mono />
            <div className="border-t border-slate-100 pt-3">
              <div className="mb-1 text-slate-500">WireGuard 公钥</div>
              <div className="flex items-start gap-2">
                <code className="min-w-0 flex-1 break-all rounded bg-slate-50 p-2 font-mono text-[11px] text-slate-700">{displayValue(node.public_key)}</code>
                {node.public_key && <Button variant="outline" size="sm" onClick={() => void copyPublicKey()} className="shrink-0 gap-1"><Copy className="h-3 w-3" />{copied ? '已复制' : '复制'}</Button>}
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {node.node_type === 'gateway' && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm"><Network className="h-4 w-4 text-blue-600" />Gateway 隧道 IP</CardTitle>
            <p className="text-xs text-slate-500">从 IPAM 的 Gateway 地址池分配地址；HUB 根据租约自动生成并同步隧道配置。</p>
          </CardHeader>
          <CardContent className="space-y-4 text-xs">
            {gatewayIPAMLoading ? (
              <div className="flex items-center text-slate-500"><RefreshCw className="mr-2 h-4 w-4 animate-spin" />正在读取 IPAM 租约…</div>
            ) : gatewayIPAMError ? (
              <div className="rounded-lg border border-rose-200 bg-rose-50 p-3 text-rose-800">{gatewayIPAMError}</div>
            ) : gatewayLease ? (
              <div className="space-y-3">
                <div className="grid grid-cols-1 gap-3 rounded-lg border border-slate-200 bg-slate-50 p-4 sm:grid-cols-3">
                  <InfoRow label="IPAM 地址" value={<span className="font-mono font-semibold">{leasedGatewayAddress}</span>} />
                  <InfoRow label="所属地址池" value={gatewayLease.pool_name || '—'} />
                  <InfoRow label="租约状态" value={<Badge variant={gatewayLease.status === 'Active' ? 'success' : 'primary'}>{gatewayLease.status === 'Active' ? '活动' : '保留'}</Badge>} />
                </div>
                <div className="flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-emerald-800"><CheckCircle2 className="h-4 w-4 shrink-0" />该 IPAM 租约是 HUB 生成 Gateway 隧道地址的唯一来源。</div>
              </div>
            ) : (
              <div className="space-y-3">
                {gatewayPools.length ? (
                  <div className="grid grid-cols-1 items-end gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
                    <div>
                      <label className="mb-1.5 block text-xs font-medium text-slate-700">Gateway 地址池</label>
                      <Select value={selectedGatewayPool} onChange={(event) => { setSelectedGatewayPool(event.target.value); setRequestedGatewayIP(''); }}>
                        <option value="">选择地址池</option>
                        {gatewayPools.map((pool) => <option key={pool.id} value={pool.id}>{pool.pool_name} · {pool.cidr} · {pool.allocation_mode === 'dynamic' ? '动态' : '静态'}</option>)}
                      </Select>
                    </div>
                    {selectedPool?.allocation_mode === 'static' ? (
                      <div>
                        <label className="mb-1.5 block text-xs font-medium text-slate-700">指定 IPv4 地址</label>
                        <Input value={requestedGatewayIP} onChange={(event) => setRequestedGatewayIP(event.target.value)} placeholder="例如 10.100.0.1" className="font-mono" />
                      </div>
                    ) : <div className="flex items-center gap-2 rounded-lg bg-slate-50 p-3 text-slate-500"><Layers className="h-4 w-4" />动态池会自动选择空闲地址</div>}
                    <Button onClick={() => void assignGatewayAddress()} disabled={!selectedPool || gatewayAddressSaving || (selectedPool.allocation_mode === 'static' && !requestedGatewayIP.trim())} isLoading={gatewayAddressSaving} className="gap-1.5">
                      <Network className="h-4 w-4" />分配地址
                    </Button>
                  </div>
                ) : (
                  <div className="flex flex-col gap-3 rounded-lg border border-slate-200 bg-slate-50 p-4 sm:flex-row sm:items-center sm:justify-between">
                    <span className="text-slate-600">尚无启用的 Gateway 地址池。请先在 IPAM 中创建类型为 Gateway 的 Overlay 地址池。</span>
                    <Button variant="outline" size="sm" onClick={() => navigate('/ipam')} className="shrink-0">打开 IPAM</Button>
                  </div>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {node.node_type === 'gateway' && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Gateway 安全用户组准入</CardTitle>
            <p className="text-xs text-slate-500">安全用户组绑定控制哪些 Client 可以建立到此 Gateway 的隧道连接。</p>
          </CardHeader>
          <CardContent className="space-y-2 text-xs">
            {gatewayGroups === undefined ? (
              <p className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-amber-900">当前 Management 服务未返回安全用户组准入信息，请更新并重启 Management 服务后刷新此页面。</p>
            ) : gatewayGroups.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {gatewayGroups.map((group) => (
                  <Badge key={group.id} variant={group.status === 'active' ? 'success' : 'default'}>
                    {group.name}{group.status !== 'active' ? ' · 已停用' : ''}
                  </Badge>
                ))}
              </div>
            ) : (
              <p className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-amber-900">尚未绑定安全用户组。请到「零信任安全策略 → 安全用户组 → Gateway 准入」设置。</p>
            )}
          </CardContent>
        </Card>
      )}

      {node.node_type === 'client_agent' && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm"><Network className="h-4 w-4 text-blue-600" />Client 隧道地址</CardTitle>
            <p className="text-xs text-slate-500">Client 登录 HUB 后，根据员工所属用户组的自动分配规则取得隧道 IP，并从 HUB 接收 Gateway Peer 配置。</p>
          </CardHeader>
          <CardContent className="space-y-3 text-xs">
            {node.ipam_address ? (
              <div className="space-y-3">
                <div className="grid grid-cols-1 gap-3 rounded-lg border border-emerald-200 bg-emerald-50 p-4 sm:grid-cols-2">
                  <InfoRow label="IPAM 隧道地址" value={<span className="font-mono font-semibold">{node.ipam_address}/32</span>} />
                  <InfoRow label="Gateway 准入" value="由员工所属安全用户组绑定决定" />
                </div>
              </div>
            ) : (
              <div className="flex flex-col gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4 text-amber-900 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <div className="font-semibold">HUB 尚未为此 Client 分配隧道 IP</div>
                  <p className="mt-1 leading-relaxed">请确认员工所属的活动安全用户组已关联动态 User-Device 地址池。地址池负责分配 Overlay IP；Gateway 接入由安全用户组页面中的 Gateway 准入绑定决定。</p>
                </div>
                <Button variant="outline" size="sm" onClick={() => navigate('/ipam')} className="shrink-0">检查 IPAM 规则</Button>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">关联网络</CardTitle>
          <p className="text-xs text-slate-500">此处展示当前直接绑定到该 Gateway 的网络；其他节点按控制面分配的 Peer 路由呈现。</p>
        </CardHeader>
        <CardContent>
          {node.networks.length === 0 ? <p className="text-xs text-slate-400">当前没有直接绑定的网络。</p> : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead><tr className="border-b text-slate-500"><th className="py-2 pr-4 font-medium">网络名称</th><th className="py-2 pr-4 font-medium">编码</th><th className="py-2 pr-4 font-medium">类型</th><th className="py-2 pr-4 font-medium">CIDR</th><th className="py-2 font-medium">状态</th></tr></thead>
                <tbody>{node.networks.map((network) => <tr key={network.id} className="border-b last:border-0"><td className="py-3 pr-4 font-medium text-slate-800">{network.network_name}</td><td className="py-3 pr-4 font-mono text-slate-500">{network.network_code}</td><td className="py-3 pr-4">{network.network_type}</td><td className="py-3 pr-4 font-mono">{network.cidr}</td><td className="py-3"><StatusBadge status={network.status} label={network.status === 'active' ? '启用' : '已禁用'} /></td></tr>)}</tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">隧道 Peer</CardTitle>
          <p className="text-xs text-slate-500">隧道地址、AllowedIPs 与握手时间来自当前 Peer 记录。</p>
        </CardHeader>
        <CardContent>
          {node.peers.length === 0 ? <p className="text-xs text-slate-400">当前没有 Peer 记录。</p> : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead><tr className="border-b text-slate-500"><th className="py-2 pr-4 font-medium">隧道</th><th className="py-2 pr-4 font-medium">隧道地址</th><th className="py-2 pr-4 font-medium">AllowedIPs</th><th className="py-2 pr-4 font-medium">Peer 状态</th><th className="py-2 pr-4 font-medium">Endpoint</th><th className="py-2 font-medium">最后握手</th></tr></thead>
                <tbody>{node.peers.map((peer) => <tr key={peer.id} className="border-b last:border-0"><td className="py-3 pr-4 font-medium text-slate-800">{peer.tunnel_code || `隧道 #${peer.tunnel_id}`}</td><td className="py-3 pr-4 font-mono">{peer.tunnel_ip}</td><td className="max-w-sm py-3 pr-4 font-mono text-slate-600">{peer.allowed_ips?.join(', ') || '—'}</td><td className="py-3 pr-4">{peer.status}</td><td className="py-3 pr-4 font-mono text-slate-600">{peer.endpoint || '—'}</td><td className="py-3">{dateValue(peer.last_handshake_at)}</td></tr>)}</tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle className="text-sm">配置同步</CardTitle></CardHeader>
        <CardContent className="grid grid-cols-1 gap-3 text-xs md:grid-cols-2">
          <HashValue label="最近下发配置摘要" value={node.last_config_hash} />
          <HashValue label="节点回执配置摘要" value={node.actual_config_hash} />
        </CardContent>
      </Card>

      <ConfirmDialog
        isOpen={confirmRevoke}
        onClose={() => setConfirmRevoke(false)}
        onConfirm={() => void saveStatus('revoked')}
        title={approvalPending ? '确认拒绝网关注册' : '确认撤销虚拟节点'}
        description={approvalPending
          ? `拒绝后“${node.node_name}”的节点身份将被撤销，网关不能通过本次注册继续接入。`
          : `撤销后“${node.node_name}”将进入不可恢复的终态，后续需重新注册新的节点身份。`}
        confirmText={approvalPending ? '确认拒绝' : '确认撤销'}
        danger
        isLoading={saving}
      />
    </div>
  );
};

function InfoRow({ label, value, mono = false }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return <div className="flex items-start justify-between gap-4 border-b border-slate-100 py-2 last:border-0"><span className="shrink-0 text-slate-500">{label}</span><span className={`break-all text-right text-slate-800 ${mono ? 'font-mono' : ''}`}>{value}</span></div>;
}

function HashValue({ label, value }: { label: string; value?: string | null }) {
  return <div><div className="mb-1 text-slate-500">{label}</div><code className="block break-all rounded bg-slate-50 p-2 font-mono text-[11px] text-slate-700">{value || '—'}</code></div>;
}
