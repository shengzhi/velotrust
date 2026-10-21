import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  AlertTriangle,
  Check,
  Clipboard,
  Clock3,
  KeyRound,
  Plus,
  RefreshCw,
  RotateCw,
  Search,
  ShieldCheck,
  X,
} from 'lucide-react';
import { Button } from '../../components/ui/button';
import { Card } from '../../components/ui/card';
import { Dialog } from '../../components/ui/dialog';
import { Input } from '../../components/ui/input';
import { useToast } from '../../components/ui/toast';
import {
  CredentialGrant,
  RegistrationCredential,
  RegistrationCredentialStatus,
  SelectableSiteNetwork,
  enrollmentApi,
} from '../../services/enrollment';

const statusLabels: Record<RegistrationCredentialStatus, string> = {
  pending: '待使用',
  used: '已使用',
  expired: '已过期',
  revoked: '已撤销',
};

const statusStyles: Record<RegistrationCredentialStatus, string> = {
  pending: 'bg-blue-50 text-blue-700 ring-blue-200',
  used: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  expired: 'bg-amber-50 text-amber-700 ring-amber-200',
  revoked: 'bg-slate-100 text-slate-600 ring-slate-200',
};

const gatewayTypeOptions = [
  { value: 'site_gateway', label: '站点接入网关' },
  { value: 'service_gateway', label: '服务代理网关' },
  { value: 'relay_gateway', label: '边缘中继网关' },
  { value: 'hub_gateway', label: '核心中枢网关' },
  { value: 'datacenter_gateway', label: 'IDC 机房网关' },
  { value: 'branch_gateway', label: '分支机构网关' },
];

function gatewayTypeLabel(value: string): string {
  return gatewayTypeOptions.find((option) => option.value === value)?.label ?? value;
}

function formatDate(value?: string | null): string {
  if (!value) return '—';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString();
}

export const EnrollmentTokens: React.FC = () => {
  const navigate = useNavigate();
  const { success, error } = useToast();
  const [credentials, setCredentials] = useState<RegistrationCredential[]>([]);
  const [selectableSiteNetworks, setSelectableSiteNetworks] = useState<SelectableSiteNetwork[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');

  const [issueOpen, setIssueOpen] = useState(false);
  const [gatewayName, setGatewayName] = useState('');
  const [gatewayType, setGatewayType] = useState('site_gateway');
  const [networkName, setNetworkName] = useState('');
  const [selectedNetworkIDs, setSelectedNetworkIDs] = useState<number[]>([]);
  const [requireApproval, setRequireApproval] = useState(true);
  const [expiresInHours, setExpiresInHours] = useState(24);
  const [notes, setNotes] = useState('');
  const [issuing, setIssuing] = useState(false);

  const [grant, setGrant] = useState<CredentialGrant | null>(null);
  const [copied, setCopied] = useState(false);
  const [details, setDetails] = useState<RegistrationCredential | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<RegistrationCredential | null>(null);
  const [revokeReason, setRevokeReason] = useState('');
  const [revoking, setRevoking] = useState(false);

  const reload = async (refresh = false) => {
    if (refresh) setRefreshing(true);
    try {
      const [credentialRows, networkRows] = await Promise.all([
        enrollmentApi.listCredentials(),
        enrollmentApi.listSelectableSiteNetworks(),
      ]);
      setCredentials(credentialRows);
      setSelectableSiteNetworks(networkRows);
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '读取注册凭证失败');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    void reload();
  }, []);

  const filteredCredentials = useMemo(() => {
    const query = search.trim().toLocaleLowerCase();
    return credentials.filter((item) => {
      const matchesStatus = statusFilter === 'all' || item.status === statusFilter;
      const searchText = [
        item.gateway_name,
        item.gateway_id,
        item.network_name,
        item.created_by_name,
        ...item.allowed_subnets,
      ].join(' ').toLocaleLowerCase();
      return matchesStatus && (!query || searchText.includes(query));
    });
  }, [credentials, search, statusFilter]);

  const stats = useMemo(() => ({
    total: credentials.length,
    pending: credentials.filter((item) => item.status === 'pending').length,
    used: credentials.filter((item) => item.status === 'used').length,
    inactive: credentials.filter((item) => item.status === 'expired' || item.status === 'revoked').length,
  }), [credentials]);

  const openIssue = () => {
    setGatewayName('');
    setGatewayType('site_gateway');
    setNetworkName('');
    setSelectedNetworkIDs([]);
    setRequireApproval(true);
    setExpiresInHours(24);
    setNotes('');
    setIssueOpen(true);
  };

  const handleIssue = async (event: React.FormEvent) => {
    event.preventDefault();
    if (selectedNetworkIDs.length === 0) {
      error('请至少选择一个 IPAM 业务网段');
      return;
    }
    setIssuing(true);
    try {
      const result = await enrollmentApi.createGateway({
        gateway_name: gatewayName.trim(),
        gateway_type: gatewayType.trim(),
        network_name: networkName.trim(),
        allowed_network_ids: selectedNetworkIDs,
        require_approval: requireApproval,
        expires_in_hours: Number(expiresInHours),
        notes: notes.trim(),
      });
      setIssueOpen(false);
      setGrant(result);
      setCopied(false);
      success('注册凭证已签发。请立即复制并交付给网关节点。');
      await reload();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '签发注册凭证失败');
    } finally {
      setIssuing(false);
    }
  };

  const copyToken = async () => {
    if (!grant) return;
    try {
      await navigator.clipboard.writeText(grant.token);
      setCopied(true);
      success('已复制注册凭证');
    } catch {
      error('浏览器未允许访问剪贴板，请手动复制凭证');
    }
  };

  const closeGrant = () => {
    setGrant(null);
    setCopied(false);
  };

  const handleExtend = async (item: RegistrationCredential) => {
    try {
      await enrollmentApi.extendCredential(item.id, 24);
      success('凭证有效期已延长 24 小时');
      await reload();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '延长凭证有效期失败');
    }
  };

  const handleRevoke = async () => {
    if (!revokeTarget) return;
    setRevoking(true);
    try {
      await enrollmentApi.revokeCredential(revokeTarget.id, revokeReason.trim());
      success('注册凭证已撤销');
      setRevokeTarget(null);
      setRevokeReason('');
      await reload();
    } catch (cause) {
      error(cause instanceof Error ? cause.message : '撤销注册凭证失败');
    } finally {
      setRevoking(false);
    }
  };

  return (
    <div className="mx-auto max-w-7xl space-y-6 px-5 py-6 lg:px-8">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-start">
        <div>
          <div className="flex items-center gap-2 text-sm text-slate-500"><KeyRound className="h-4 w-4" /> 网关管理</div>
          <h1 className="mt-2 text-2xl font-semibold tracking-tight text-slate-900">注册凭证</h1>
          <p className="mt-1 max-w-3xl text-sm text-slate-500">创建网关身份、签发一次性入网凭证，并管理有效期和撤销状态。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void reload(true)} isLoading={refreshing}>
            <RefreshCw className="h-4 w-4" /> 刷新
          </Button>
          <Button onClick={openIssue}><Plus className="h-4 w-4" /> 创建网关并签发</Button>
        </div>
      </div>

      <div className="flex flex-col gap-3 rounded-xl border border-blue-200 bg-blue-50 p-4 text-sm text-blue-900 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex gap-3">
          <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0" />
          <p>若凭证要求审批，Gateway 完成注册后会进入虚拟节点管理的待审批队列。批准后 HUB 会继续注册流程，网关随后重新进行身份认证。</p>
        </div>
        <Button variant="outline" size="sm" className="shrink-0" onClick={() => navigate('/nodes?approval=pending')}>
          查看待审批网关
        </Button>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {[
          { label: '注册凭证总数', value: stats.total, icon: KeyRound, tone: 'text-blue-600 bg-blue-50' },
          { label: '待使用', value: stats.pending, icon: Clock3, tone: 'text-indigo-600 bg-indigo-50' },
          { label: '已使用', value: stats.used, icon: ShieldCheck, tone: 'text-emerald-600 bg-emerald-50' },
          { label: '过期或撤销', value: stats.inactive, icon: X, tone: 'text-slate-600 bg-slate-100' },
        ].map(({ label, value, icon: Icon, tone }) => (
          <Card key={label} className="flex items-center gap-3 p-4">
            <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${tone}`}><Icon className="h-5 w-5" /></div>
            <div><div className="text-xl font-semibold text-slate-900">{value}</div><div className="text-xs text-slate-500">{label}</div></div>
          </Card>
        ))}
      </div>

      <Card className="overflow-hidden">
        <div className="flex flex-col gap-3 border-b border-slate-200 p-4 sm:flex-row sm:items-center sm:justify-between">
          <div><h2 className="font-semibold text-slate-900">凭证列表</h2><p className="mt-1 text-xs text-slate-500">列表仅展示凭证元数据，不保存或回显凭证明文。</p></div>
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索网关、网络或网段" leftIcon={<Search className="h-4 w-4" />} className="sm:w-64" />
            <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)} className="h-10 rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-700">
              <option value="all">全部状态</option>
              <option value="pending">待使用</option>
              <option value="used">已使用</option>
              <option value="expired">已过期</option>
              <option value="revoked">已撤销</option>
            </select>
          </div>
        </div>
        {loading ? (
          <div className="flex items-center justify-center gap-2 p-14 text-sm text-slate-500"><RefreshCw className="h-4 w-4 animate-spin" /> 正在加载凭证</div>
        ) : filteredCredentials.length === 0 ? (
          <div className="p-14 text-center">
            <KeyRound className="mx-auto h-8 w-8 text-slate-300" />
            <p className="mt-3 text-sm font-medium text-slate-700">没有找到注册凭证</p>
            <p className="mt-1 text-xs text-slate-500">创建网关并签发一次性凭证后，会在这里显示。</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[980px] text-left text-sm">
              <thead className="border-b border-slate-200 bg-slate-50 text-xs text-slate-500">
                <tr><th className="px-4 py-3 font-medium">网关 / 网络</th><th className="px-4 py-3 font-medium">允许网段</th><th className="px-4 py-3 font-medium">状态</th><th className="px-4 py-3 font-medium">使用次数</th><th className="px-4 py-3 font-medium">有效期</th><th className="px-4 py-3 font-medium">签发人</th><th className="px-4 py-3 text-right font-medium">操作</th></tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {filteredCredentials.map((item) => {
                  const mayManage = item.status === 'pending' || item.status === 'expired';
                  return (
                    <tr key={item.id} className="hover:bg-slate-50/70">
                      <td className="px-4 py-3">
                        <div className="font-medium text-slate-900">{item.gateway_name}</div>
                        <div className="mt-0.5 text-xs text-slate-500">{item.network_name} · {item.gateway_id}</div>
                      </td>
                      <td className="max-w-64 px-4 py-3 text-xs text-slate-600"><div className="truncate" title={item.allowed_subnets.join(', ')}>{item.allowed_subnets.join(', ') || '—'}</div></td>
                      <td className="px-4 py-3"><span className={`inline-flex rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset ${statusStyles[item.status]}`}>{statusLabels[item.status]}</span></td>
                      <td className="px-4 py-3 text-slate-700">{item.used_count} / {item.max_uses}</td>
                      <td className="px-4 py-3 text-xs text-slate-600">{formatDate(item.expires_at)}</td>
                      <td className="px-4 py-3 text-slate-600">{item.created_by_name || '—'}</td>
                      <td className="px-4 py-3">
                        <div className="flex justify-end gap-1">
                          <Button variant="ghost" size="sm" onClick={() => setDetails(item)}>详情</Button>
                          {mayManage && <Button variant="ghost" size="sm" onClick={() => void handleExtend(item)} title="延长 24 小时"><RotateCw className="h-4 w-4" /><span className="sr-only">延长</span></Button>}
                          {mayManage && item.used_count === 0 && <Button variant="ghost" size="sm" className="text-rose-600 hover:text-rose-700" onClick={() => { setRevokeTarget(item); setRevokeReason(''); }}>撤销</Button>}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
        <div className="flex items-center justify-between border-t border-slate-100 px-4 py-3 text-xs text-slate-500">
          <span>共 {filteredCredentials.length} 条</span><span>凭证明文仅在签发时展示一次</span>
        </div>
      </Card>

      <Dialog isOpen={issueOpen} onClose={() => !issuing && setIssueOpen(false)} title="创建网关并签发注册凭证" description="明文只会在签发完成后展示一次。" maxWidth="2xl">
        <form className="space-y-5" onSubmit={(event) => void handleIssue(event)}>
          <div className="grid gap-4 sm:grid-cols-2">
              <label className="space-y-1.5 text-sm"><span className="font-medium text-slate-700">网关名称</span><Input required maxLength={128} value={gatewayName} onChange={(event) => setGatewayName(event.target.value)} placeholder="例如：上海生产网关 01" /></label>
              <label className="space-y-1.5 text-sm"><span className="font-medium text-slate-700">网关类型</span><select required value={gatewayType} onChange={(event) => setGatewayType(event.target.value)} className="h-10 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500">{gatewayTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>
              <label className="space-y-1.5 text-sm sm:col-span-2"><span className="font-medium text-slate-700">网络名称</span><Input required maxLength={128} value={networkName} onChange={(event) => setNetworkName(event.target.value)} placeholder="例如：上海生产网" /></label>
              <div className="space-y-2 text-sm sm:col-span-2">
                <div className="flex items-center justify-between gap-3"><span className="font-medium text-slate-700">允许网段</span><span className="text-xs text-slate-500">从网络站点选择内网网段</span></div>
                {selectableSiteNetworks.length === 0 ? (
                  <div className="rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-5 text-center">
                    <p className="text-sm text-slate-600">当前没有已启用且未绑定网关的网络站点内网网段。</p>
                    <p className="mt-1 text-xs text-slate-500">请先配置并启用站点内网网段；已绑定其他网关的站点需先解除绑定。</p>
                  </div>
                ) : (
                  <div className="max-h-56 space-y-1 overflow-y-auto rounded-lg border border-slate-200 p-2">
                    {selectableSiteNetworks.map((network) => {
                      const checked = selectedNetworkIDs.includes(network.id);
                      return (
                        <label key={network.id} className={`flex cursor-pointer items-start gap-3 rounded-md px-3 py-2.5 hover:bg-slate-50 ${checked ? 'bg-blue-50/70' : ''}`}>
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={(event) => setSelectedNetworkIDs((current) => event.target.checked ? [...current, network.id] : current.filter((id) => id !== network.id))}
                            className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600"
                          />
                          <span className="min-w-0 flex-1">
                            <span className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
                              <span className="font-medium text-slate-800">{network.network_name}</span>
                              <code className="font-mono text-xs text-slate-700">{network.cidr}</code>
                            </span>
                            <span className="mt-1 block text-xs text-slate-500">{network.network_type}{network.region ? ` · ${network.region}` : ''}{network.environment ? ` · ${network.environment}` : ''}</span>
                          </span>
                        </label>
                      );
                    })}
                  </div>
                )}
                {selectedNetworkIDs.length > 0 && <span className="block text-xs text-blue-700">已选择 {selectedNetworkIDs.length} 个网段；提交时服务端会再次确认站点仍然启用且未被其他网关占用。</span>}
              </div>
              <label className="flex items-center gap-2 text-sm text-slate-700 sm:col-span-2"><input type="checkbox" checked={requireApproval} onChange={(event) => setRequireApproval(event.target.checked)} className="h-4 w-4 rounded border-slate-300 text-blue-600" /> 网关注册后需要管理员审批</label>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <label className="space-y-1.5 text-sm"><span className="font-medium text-slate-700">凭证有效期（小时）</span><Input required type="number" min={1} max={168} value={expiresInHours} onChange={(event) => setExpiresInHours(Number(event.target.value))} /></label>
            <label className="space-y-1.5 text-sm"><span className="font-medium text-slate-700">备注</span><Input maxLength={512} value={notes} onChange={(event) => setNotes(event.target.value)} placeholder="可选" /></label>
          </div>
          <div className="flex justify-end gap-2 border-t border-slate-100 pt-4"><Button type="button" variant="outline" onClick={() => setIssueOpen(false)} disabled={issuing}>取消</Button><Button type="submit" isLoading={issuing} disabled={selectableSiteNetworks.length === 0}><KeyRound className="h-4 w-4" />创建并签发</Button></div>
        </form>
      </Dialog>

      <Dialog isOpen={!!grant} onClose={closeGrant} title="凭证已签发" description="请立即复制并安全交付。关闭窗口后，系统无法再次显示这段凭证。" maxWidth="lg">
        {grant && <div className="space-y-5">
          <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900"><div className="font-semibold">{grant.credential.gateway_name}</div><div className="mt-1 text-xs">网关 ID：{grant.credential.gateway_id} · 有效期至 {formatDate(grant.credential.expires_at)}</div></div>
          <div><label className="mb-2 block text-xs font-semibold uppercase tracking-wide text-slate-500">一次性注册凭证</label><div className="flex items-stretch gap-2"><code className="min-w-0 flex-1 break-all rounded-lg border border-slate-300 bg-slate-50 p-3 text-sm text-slate-800">{grant.token}</code><Button variant="outline" className="shrink-0" onClick={() => void copyToken()}>{copied ? <Check className="h-4 w-4" /> : <Clipboard className="h-4 w-4" />}{copied ? '已复制' : '复制'}</Button></div></div>
          <div className="flex gap-2 rounded-lg bg-amber-50 p-3 text-xs text-amber-900"><AlertTriangle className="h-4 w-4 shrink-0" /><span>该凭证明文不会写入数据库，也不会再次显示。妥善保存后再关闭此窗口。</span></div>
          <div className="flex justify-end"><Button onClick={closeGrant}>完成并关闭</Button></div>
        </div>}
      </Dialog>

      <Dialog isOpen={!!details} onClose={() => setDetails(null)} title="注册凭证详情" maxWidth="lg">
        {details && <div className="space-y-4 text-sm">
          <div className="flex items-start justify-between gap-3"><div><div className="font-semibold text-slate-900">{details.gateway_name}</div><div className="mt-1 text-xs text-slate-500">{details.gateway_id} · {details.network_name}</div></div><span className={`rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset ${statusStyles[details.status]}`}>{statusLabels[details.status]}</span></div>
          <dl className="grid grid-cols-2 gap-x-4 gap-y-3 rounded-lg bg-slate-50 p-4 text-xs">
            <dt className="text-slate-500">网关类型</dt><dd className="text-slate-800">{gatewayTypeLabel(details.gateway_type)}</dd>
            <dt className="text-slate-500">允许网段</dt><dd className="text-slate-800">{details.allowed_subnets.join(', ') || '—'}</dd>
            <dt className="text-slate-500">使用次数</dt><dd className="text-slate-800">{details.used_count} / {details.max_uses}</dd>
            <dt className="text-slate-500">需要审批</dt><dd className="text-slate-800">{details.require_approval ? '是' : '否'}</dd>
            <dt className="text-slate-500">签发时间</dt><dd className="text-slate-800">{formatDate(details.created_at)}</dd>
            <dt className="text-slate-500">有效期至</dt><dd className="text-slate-800">{formatDate(details.expires_at)}</dd>
            <dt className="text-slate-500">签发人</dt><dd className="text-slate-800">{details.created_by_name || '—'}</dd>
            <dt className="text-slate-500">备注</dt><dd className="text-slate-800">{details.notes || '—'}</dd>
            {details.used_at && <><dt className="text-slate-500">使用时间</dt><dd className="text-slate-800">{formatDate(details.used_at)}</dd></>}
            {details.used_ip && <><dt className="text-slate-500">来源地址</dt><dd className="text-slate-800">{details.used_ip}</dd></>}
            {details.revocation_reason && <><dt className="text-slate-500">撤销原因</dt><dd className="text-slate-800">{details.revocation_reason}</dd></>}
          </dl>
          <div className="flex justify-end"><Button variant="outline" onClick={() => setDetails(null)}>关闭</Button></div>
        </div>}
      </Dialog>

      <Dialog isOpen={!!revokeTarget} onClose={() => !revoking && setRevokeTarget(null)} title="撤销注册凭证" description="撤销后，这张凭证不能再用于网关注册。" maxWidth="sm">
        <div className="space-y-4"><label className="block space-y-1.5 text-sm"><span className="font-medium text-slate-700">撤销原因</span><Input autoFocus required maxLength={512} value={revokeReason} onChange={(event) => setRevokeReason(event.target.value)} placeholder="填写原因" /></label><div className="flex justify-end gap-2"><Button variant="outline" onClick={() => setRevokeTarget(null)} disabled={revoking}>取消</Button><Button variant="danger" onClick={() => void handleRevoke()} isLoading={revoking} disabled={!revokeReason.trim()}>确认撤销</Button></div></div>
      </Dialog>
    </div>
  );
};

export default EnrollmentTokens;
