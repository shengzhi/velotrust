import React, { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Monitor, RefreshCw, Search, Smartphone, Tablet, Power } from 'lucide-react';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Badge } from '../components/ui/badge';
import { StatusBadge } from '../components/StatusBadge';
import { Card } from '../components/ui/card';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { EmptyState } from '../components/EmptyState';
import { useToast } from '../components/ui/toast';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../components/ui/table';
import { AccessChangeStatus, VirtualNode, virtualNodeApi } from '../services/node';
import { formatRelativeTime } from '../utils/format';

const effectiveStatus = (node: VirtualNode) => node.device_status ?? node.node_status;

export const Devices: React.FC = () => {
  const [searchParams] = useSearchParams();
  const { success, error } = useToast();
  const [nodes, setNodes] = useState<VirtualNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [target, setTarget] = useState<VirtualNode | null>(null);
  const [saving, setSaving] = useState(false);
  const [accessNotice, setAccessNotice] = useState<AccessChangeStatus | null>(null);
  const userParam = searchParams.get('userId');

  const loadNodes = async () => {
    setLoading(true);
    try {
      setNodes((await virtualNodeApi.list()).filter((node) => node.node_type === 'client_agent'));
    } catch (err) {
      error(err instanceof Error ? err.message : '加载客户端设备失败。');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadNodes(); }, []);

  const filteredNodes = useMemo(() => {
    const query = search.trim().toLocaleLowerCase();
    return nodes.filter((node) => {
      const currentStatus = effectiveStatus(node);
      const matchesSearch = !query || [
        node.device_name ?? node.node_name,
        node.owner_username ?? '',
        node.device_platform ?? '',
        node.peers[0]?.tunnel_ip ?? '',
      ].some((value) => value.toLocaleLowerCase().includes(query));
      return matchesSearch && (statusFilter === 'all' || currentStatus === statusFilter) &&
        (!userParam || String(node.owner_user_id ?? '') === userParam);
    });
  }, [nodes, search, statusFilter, userParam]);

  const refreshChange = async (nodeID: number) => {
    const started = Date.now();
    let latest: AccessChangeStatus | null = null;
    do {
      try {
        latest = await virtualNodeApi.getAccessChangeStatus('node', nodeID);
      } catch {
        return latest;
      }
      if (latest) setAccessNotice(latest);
      if (latest?.status === 'completed' || Date.now() - started >= 5000) return latest;
      await new Promise((resolve) => window.setTimeout(resolve, 500));
    } while (true);
  };

  const updateStatus = async () => {
    if (!target) return;
    const nextStatus = effectiveStatus(target) === 'active' ? 'disabled' : 'active';
    setAccessNotice(null);
    setSaving(true);
    try {
      await virtualNodeApi.setStatus(target, nextStatus);
      const change = await refreshChange(target.id);
      success(nextStatus === 'disabled'
        ? change?.status === 'completed'
          ? `设备已禁用，${change.acknowledged_count}/${change.target_count} 个 Gateway 已确认更新。`
          : '设备已禁用，Gateway 配置正在同步。'
        : '设备已启用，Gateway 将同步恢复授权配置。');
      setTarget(null);
      await loadNodes();
    } catch (err) {
      error(err instanceof Error ? err.message : '更新设备状态失败。');
    } finally {
      setSaving(false);
    }
  };

  const getDeviceIcon = (type?: string | null) => {
    const value = (type ?? '').toLowerCase();
    if (value.includes('phone') || value.includes('mobile')) return <Smartphone className="h-3.5 w-3.5" />;
    if (value.includes('tablet')) return <Tablet className="h-3.5 w-3.5" />;
    return <Monitor className="h-3.5 w-3.5" />;
  };

  return (
    <div className="space-y-5">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-slate-900">终端设备管理</h1>
          <p className="mt-1 text-xs text-slate-500">员工使用账号密码登录 Client Agent 并注册设备。此处管理已注册设备；禁用后 HUB 会推动 Gateway 移除对应 WireGuard Peer。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void loadNodes()} isLoading={loading} aria-label="刷新设备">
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <Card className="p-4">
        <div className="flex flex-col gap-3 sm:flex-row">
          <Input
            placeholder="搜索设备、用户名、平台或隧道 IP"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            leftIcon={<Search className="h-4 w-4 text-slate-400" />}
          />
          <select className="h-10 rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-700" value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
            <option value="all">全部状态</option>
            <option value="active">启用</option>
            <option value="disabled">已禁用</option>
            <option value="pending">待注册</option>
            <option value="quarantined">隔离</option>
            <option value="revoked">已撤销</option>
          </select>
        </div>
      </Card>

      {accessNotice?.status === 'pending' && (
        <Card className="flex flex-col gap-3 border-amber-200 bg-amber-50/70 p-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-sm font-semibold text-amber-900">网络访问变更仍在同步</p>
            <p className="mt-1 text-xs text-amber-800">已确认 {accessNotice.acknowledged_count}/{accessNotice.target_count} 个 Gateway。离线 Gateway 会在重新连接并应用当前配置后确认。</p>
            {accessNotice.last_error && <p className="mt-2 text-xs text-red-700">最近一次配置失败：{accessNotice.last_error}</p>}
          </div>
          <Button variant="outline" size="sm" onClick={() => void refreshChange(accessNotice.subject_id)}>
            <RefreshCw className="h-3.5 w-3.5" />再次检查
          </Button>
        </Card>
      )}

      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>设备</TableHead>
              <TableHead>所属用户</TableHead>
              <TableHead>平台</TableHead>
              <TableHead>隧道 IP</TableHead>
              <TableHead>连接状态</TableHead>
              <TableHead>最后在线</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow><TableCell colSpan={7} className="py-10 text-center text-slate-500">正在读取 HUB 节点和设备凭证…</TableCell></TableRow>
            ) : filteredNodes.length === 0 ? (
              <TableRow><TableCell colSpan={7} className="p-8"><EmptyState icon={Monitor} title="没有已注册的客户端设备" description="客户端完成 HUB 注册并关联设备凭证后会显示在这里。" /></TableCell></TableRow>
            ) : filteredNodes.map((node) => {
              const status = effectiveStatus(node);
              const canToggle = ['active', 'disabled'].includes(status) && ['active', 'disabled'].includes(node.node_status);
              const online = node.health_status === 'healthy' && node.last_heartbeat_at && Date.now() - new Date(node.last_heartbeat_at).getTime() < 60000;
              return (
                <TableRow key={node.id}>
                  <TableCell>
                    <div className="font-semibold text-slate-900">{node.device_name ?? node.node_name}</div>
                    <div className="mt-0.5 font-mono text-[11px] text-slate-500">{node.node_code}</div>
                  </TableCell>
                  <TableCell className="text-xs">{node.owner_username ?? '未关联用户'}</TableCell>
                  <TableCell>
                    <Badge variant="outline" className="gap-1">{getDeviceIcon(node.device_type)}{node.device_platform ?? node.device_type ?? '客户端'}</Badge>
                  </TableCell>
                  <TableCell className="font-mono text-xs">{node.peers[0]?.tunnel_ip ?? '—'}</TableCell>
                  <TableCell><StatusBadge status={status} label={online ? '在线' : status === 'active' ? '离线 · 已授权' : undefined} /></TableCell>
                  <TableCell className="text-xs text-slate-500">{node.last_heartbeat_at ? formatRelativeTime(node.last_heartbeat_at) : '从未连接'}</TableCell>
                  <TableCell className="text-right">
                    <Button variant={status === 'active' ? 'danger' : 'outline'} size="sm" disabled={!canToggle} onClick={() => setTarget(node)}>
                      <Power className="h-3.5 w-3.5" />{status === 'active' ? '立即禁用' : '启用'}
                    </Button>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </Card>

      <ConfirmDialog
        isOpen={target !== null}
        onClose={() => setTarget(null)}
        onConfirm={updateStatus}
        isLoading={saving}
        title={target && effectiveStatus(target) === 'active' ? '立即禁用此设备？' : '重新启用此设备？'}
        description={target && effectiveStatus(target) === 'active'
          ? 'HUB 会通知 Gateway 拉取当前授权配置并移除此设备的 WireGuard Peer。在线 Gateway 目标为尽力 5 秒内确认；未确认时会保留待同步状态。'
          : '设备会重新获得配置中允许的访问权限。Gateway 将在重新应用当前配置后恢复此 Peer。'}
        confirmText={target && effectiveStatus(target) === 'active' ? '禁用设备' : '启用设备'}
        danger={target !== null && effectiveStatus(target) === 'active'}
      />
    </div>
  );
};
