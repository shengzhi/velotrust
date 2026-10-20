import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ArrowLeft, Copy, RefreshCw, Server, ShieldAlert } from 'lucide-react';
import { Button } from '../../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card';
import { Badge } from '../../components/ui/badge';
import { StatusBadge } from '../../components/StatusBadge';
import { Select } from '../../components/ui/select';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useToast } from '../../components/ui/toast';
import { VirtualNode, VirtualNodeStatus, virtualNodeApi } from '../../services/node';
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
  const [saving, setSaving] = useState(false);
  const [targetStatus, setTargetStatus] = useState<VirtualNodeStatus | ''>('');
  const [confirmRevoke, setConfirmRevoke] = useState(false);
  const [copied, setCopied] = useState(false);

  const loadNode = async () => {
    const nodeID = Number(id);
    if (!Number.isSafeInteger(nodeID) || nodeID <= 0) {
      setNode(null);
      setLoading(false);
      return;
    }
    try {
      const result = await virtualNodeApi.get(nodeID);
      setNode(result);
      setTargetStatus('');
    } catch (err) {
      error(err instanceof Error ? err.message : '加载虚拟节点失败');
      setNode(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadNode();
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

  const displayValue = (value?: string | null) => value || '—';
  const dateValue = (value?: string | null) => value ? formatDateTime(value) : '从未上报';

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
              <StatusBadge status={node.node_status} label={statusLabels[node.node_status]} />
              <Badge variant={node.node_type === 'gateway' ? 'primary' : 'outline'}>{typeLabels[node.node_type]}</Badge>
            </div>
            <p className="mt-1 font-mono text-xs text-slate-500">{node.node_code} · 节点 ID {node.id} · 版本 {node.version_no}</p>
          </div>
        </div>
        {options.length > 0 && (
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

      {node.node_status === 'revoked' && (
        <div className="flex items-start gap-2 rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-800">
          <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0" />
          <span>此节点已撤销，撤销是终态。若需要重新接入，请通过相应注册流程创建新的节点身份。</span>
        </div>
      )}

      <div className="rounded-lg border border-blue-200 bg-blue-50 p-3 text-xs text-blue-800">
        状态变更会写入管理数据库并记录审计。HUB 在线控制通道尚未接入，因此当前不会立即向运行节点推送配置或断开已建立的隧道。
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader><CardTitle className="flex items-center gap-2 text-sm"><Server className="h-4 w-4 text-blue-600" />节点身份与运行状态</CardTitle></CardHeader>
          <CardContent className="space-y-3 text-xs">
            <InfoRow label="节点类型" value={typeLabels[node.node_type]} />
            <InfoRow label="生命周期状态" value={<StatusBadge status={node.node_status} label={statusLabels[node.node_status]} />} />
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
            <InfoRow label="公网端点" value={displayValue(node.public_endpoint)} mono />
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
        title="确认撤销虚拟节点"
        description={`撤销后“${node.node_name}”将进入不可恢复的终态，后续需重新注册新的节点身份。`}
        confirmText="确认撤销"
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
