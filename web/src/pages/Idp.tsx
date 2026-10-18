import React, { useState, useEffect, useMemo } from 'react';
import {
  Fingerprint,
  CheckCircle2,
  XCircle,
  RefreshCw,
  Plus,
  Trash2,
  KeyRound,
  ShieldCheck,
  Lock,
  Eye,
  EyeOff,
  Terminal,
  ExternalLink,
  Users,
  Clock,
  Sparkles,
  ArrowRight,
  Layers,
  Check,
} from 'lucide-react';
import { mockApi } from '../data/mock';
import { IdpProvider, IdpGroupMapping, Group } from '../types';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { Badge } from '../components/ui/badge';
import { StatusBadge } from '../components/StatusBadge';
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
import { useToast } from '../components/ui/toast';

export const Idp: React.FC = () => {
  const { success, error } = useToast();

  const [providers, setProviders] = useState<IdpProvider[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedProviderId, setSelectedProviderId] = useState<string>('');

  // Editing state for selected provider
  const [activeForm, setActiveForm] = useState<IdpProvider | null>(null);
  const [showSecret, setShowSecret] = useState(false);
  const [testingConn, setTestingConn] = useState(false);
  const [testResult, setTestResult] = useState<any>(null);
  const [syncing, setSyncing] = useState(false);
  const [savingConfig, setSavingConfig] = useState(false);

  // New Group Mapping row
  const [newIdpGroup, setNewIdpGroup] = useState('');
  const [newVeloGroup, setNewVeloGroup] = useState('');

  // JWT Token Simulator Modal
  const [isSimulatorOpen, setIsSimulatorOpen] = useState(false);
  const [simSub, setSimSub] = useState('feishu_emp_98214');
  const [simEmail, setSimEmail] = useState('lina.zhang@mesh.internal');
  const [simGroupsInput, setSimGroupsInput] = useState('基础架构部/后端开发, 质量保障与测试中心');
  const [simulating, setSimulating] = useState(false);
  const [simulatedResult, setSimulatedResult] = useState<any>(null);

  const loadData = async () => {
    try {
      const [pList, gList] = await Promise.all([
        mockApi.getIdpConfigs(),
        mockApi.getGroups(),
      ]);
      setProviders(pList);
      setGroups(gList);
      if (pList.length > 0 && !selectedProviderId) {
        setSelectedProviderId(pList[0].id);
        setActiveForm({ ...pList[0] });
      } else if (selectedProviderId) {
        const curr = pList.find((p) => p.id === selectedProviderId);
        if (curr) setActiveForm({ ...curr });
      }
      if (gList.length > 0 && !newVeloGroup) {
        setNewVeloGroup(gList[0].name);
      }
    } catch {
      error('加载身份源集成配置失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleSelectProvider = (prov: IdpProvider) => {
    setSelectedProviderId(prov.id);
    setActiveForm({ ...prov });
    setTestResult(null);
    setShowSecret(false);
  };

  const handleAddMapping = () => {
    if (!newIdpGroup.trim()) {
      error('请输入 IdP 部门或角色全路径');
      return;
    }
    if (!activeForm) return;

    const newMapping: IdpGroupMapping = {
      id: `gm-${Date.now()}`,
      idp_group_name: newIdpGroup.trim(),
      velotrust_group_id: newVeloGroup || groups[0]?.name || '研发核心组',
    };

    setActiveForm({
      ...activeForm,
      group_mapping: [...activeForm.group_mapping, newMapping],
    });
    setNewIdpGroup('');
  };

  const handleRemoveMapping = (mapId: string) => {
    if (!activeForm) return;
    setActiveForm({
      ...activeForm,
      group_mapping: activeForm.group_mapping.filter((m) => m.id !== mapId),
    });
  };

  const handleSaveProviderConfig = async () => {
    if (!activeForm) return;
    setSavingConfig(true);
    try {
      await mockApi.updateIdpConfig(activeForm.id, activeForm);
      success(`认证源 [${activeForm.name}] 配置与动态组映射已成功保存并同步！`);
      await loadData();
    } catch {
      error('保存认证源配置失败');
    } finally {
      setSavingConfig(false);
    }
  };

  const handleTestConnection = async () => {
    if (!activeForm) return;
    setTestingConn(true);
    setTestResult(null);
    try {
      const res = await mockApi.testIdpConnection(activeForm.id);
      setTestResult(res);
      if (res.success) {
        success('身份源端点连通性测试通过');
      } else {
        error('身份源端点连接异常');
      }
    } catch {
      error('测试请求超时');
    } finally {
      setTestingConn(false);
    }
  };

  const handleSyncUsers = async () => {
    if (!activeForm) return;
    setSyncing(true);
    try {
      const updated = await mockApi.syncIdpUsers(activeForm.id);
      setActiveForm(updated);
      success(`成功从 [${activeForm.name}] 增量同步最新的企业组织架构与用户凭证！`);
      await loadData();
    } catch {
      error('同步组织架构失败');
    } finally {
      setSyncing(false);
    }
  };

  const handleRunJwtSimulation = async () => {
    if (!activeForm) return;
    setSimulating(true);
    try {
      const gList = simGroupsInput.split(',').map((s) => s.trim()).filter(Boolean);
      const res = await mockApi.simulateIdpToken({
        providerId: activeForm.id,
        sub: simSub,
        email: simEmail,
        idpGroups: gList,
      });
      setSimulatedResult(res);
    } catch {
      error('JWT 模拟推导失败');
    } finally {
      setSimulating(false);
    }
  };

  const getBrandLogo = (type: IdpProvider['type']) => {
    switch (type) {
      case 'Feishu':
        return (
          <div className="w-10 h-10 rounded-xl bg-blue-500/10 text-blue-600 flex items-center justify-center font-bold text-base border border-blue-200">
            飞
          </div>
        );
      case 'DingTalk':
        return (
          <div className="w-10 h-10 rounded-xl bg-cyan-500/10 text-cyan-600 flex items-center justify-center font-bold text-base border border-cyan-200">
            钉
          </div>
        );
      case 'Okta':
        return (
          <div className="w-10 h-10 rounded-xl bg-indigo-500/10 text-indigo-600 flex items-center justify-center font-bold text-base border border-indigo-200">
            Ok
          </div>
        );
      case 'WeCom':
        return (
          <div className="w-10 h-10 rounded-xl bg-emerald-500/10 text-emerald-600 flex items-center justify-center font-bold text-base border border-emerald-200">
            微
          </div>
        );
      default:
        return (
          <div className="w-10 h-10 rounded-xl bg-slate-100 text-slate-700 flex items-center justify-center">
            <KeyRound className="w-5 h-5" />
          </div>
        );
    }
  };

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">
            身份源集成 (IdP)
          </h1>
          <p className="text-xs text-slate-500 mt-0.5">
            对接企业飞书、钉钉、Okta 等身份基础设施，支持单点登录 (SSO) 与动态安全策略划分
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setIsSimulatorOpen(true);
              handleRunJwtSimulation();
            }}
            className="gap-1.5"
          >
            <Sparkles className="w-3.5 h-3.5 text-purple-600" />
            <span>JWT 策略推导演示</span>
          </Button>
        </div>
      </div>

      {/* Identity Provider Cards Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {providers.map((prov) => {
          const isSelected = selectedProviderId === prov.id;
          return (
            <Card
              key={prov.id}
              onClick={() => handleSelectProvider(prov)}
              className={`p-4 cursor-pointer transition-all duration-150 border ${
                isSelected
                  ? 'border-blue-500 ring-2 ring-blue-500/10 shadow-md bg-white'
                  : 'hover:border-slate-300 hover:shadow-xs bg-white/80'
              }`}
            >
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  {getBrandLogo(prov.type)}
                  <div>
                    <h3 className="text-sm font-bold text-slate-900">{prov.name}</h3>
                    <span className="text-[10px] font-mono font-medium text-slate-400">
                      {prov.provider_type} 协议
                    </span>
                  </div>
                </div>
                <StatusBadge
                  status={prov.status === 'Connected' ? 'active' : 'disabled'}
                  label={prov.status === 'Connected' ? '已联通' : '未连接'}
                />
              </div>

              <div className="mt-4 pt-3 border-t border-slate-100 flex items-center justify-between text-xs">
                <div className="flex items-center gap-1.5 text-slate-500">
                  <Users className="w-3.5 h-3.5 text-slate-400" />
                  <span>已纳管 {prov.user_count} 员工</span>
                </div>
                {prov.mfa_required && (
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
                    强制 MFA
                  </span>
                )}
              </div>
            </Card>
          );
        })}
      </div>

      {/* Main Configuration & Dynamic Group Mapping Area */}
      {activeForm && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Left 1-Col: Protocol Parameters & Connectivity */}
          <div className="space-y-6">
            <Card>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <CardTitle className="text-sm font-semibold flex items-center gap-2">
                    <Lock className="w-4 h-4 text-blue-600" />
                    <span>认证源协议参数</span>
                  </CardTitle>
                  <Badge variant="primary">{activeForm.provider_type}</Badge>
                </div>
              </CardHeader>
              <CardContent className="space-y-3.5 text-xs">
                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    发行方发现端点 (Issuer URL)
                  </label>
                  <Input
                    value={activeForm.issuer_url}
                    onChange={(e) => setActiveForm({ ...activeForm, issuer_url: e.target.value })}
                    className="font-mono text-xs"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    客户端 Client ID
                  </label>
                  <Input
                    value={activeForm.client_id}
                    onChange={(e) => setActiveForm({ ...activeForm, client_id: e.target.value })}
                    className="font-mono text-xs"
                  />
                </div>

                <div>
                  <div className="flex items-center justify-between mb-1">
                    <label className="text-xs font-medium text-slate-700">
                      机密密钥 (Client Secret)
                    </label>
                    <span className="text-[10px] text-emerald-600 font-mono flex items-center gap-1">
                      <ShieldCheck className="w-3 h-3" />
                      KMS 加密托管
                    </span>
                  </div>
                  <div className="relative">
                    <Input
                      type={showSecret ? 'text' : 'password'}
                      value={activeForm.client_secret}
                      onChange={(e) =>
                        setActiveForm({ ...activeForm, client_secret: e.target.value })
                      }
                      className="font-mono text-xs pr-10"
                    />
                    <button
                      type="button"
                      onClick={() => setShowSecret(!showSecret)}
                      className="absolute right-2.5 top-2.5 text-slate-400 hover:text-slate-600"
                    >
                      {showSecret ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-3 pt-1">
                  <div>
                    <label className="block text-xs font-medium text-slate-700 mb-1">
                      会话生命周期 (TTL)
                    </label>
                    <Select
                      value={activeForm.session_ttl_hours}
                      onChange={(e) =>
                        setActiveForm({
                          ...activeForm,
                          session_ttl_hours: Number(e.target.value),
                        })
                      }
                    >
                      <option value={8}>8 小时 (严格)</option>
                      <option value={12}>12 小时 (推荐)</option>
                      <option value={24}>24 小时 (标准)</option>
                      <option value={72}>72 小时 (长期)</option>
                    </Select>
                  </div>

                  <div>
                    <label className="block text-xs font-medium text-slate-700 mb-1">
                      MFA 多因素认证
                    </label>
                    <Select
                      value={activeForm.mfa_required ? 'true' : 'false'}
                      onChange={(e) =>
                        setActiveForm({
                          ...activeForm,
                          mfa_required: e.target.value === 'true',
                        })
                      }
                    >
                      <option value="true">强制开启 (Required)</option>
                      <option value="false">根据设备策略自适应</option>
                    </Select>
                  </div>
                </div>

                {/* Connection Test & Sync Actions */}
                <div className="pt-2 border-t border-slate-100 flex flex-col gap-2">
                  <div className="flex items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleTestConnection}
                      isLoading={testingConn}
                      className="flex-1 text-xs gap-1"
                    >
                      <CheckCircle2 className="w-3.5 h-3.5 text-blue-600" />
                      <span>测试端点连通性</span>
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={handleSyncUsers}
                      isLoading={syncing}
                      className="flex-1 text-xs gap-1"
                    >
                      <RefreshCw className="w-3.5 h-3.5 text-slate-600" />
                      <span>立即增量同步</span>
                    </Button>
                  </div>

                  {testResult && (
                    <div
                      className={`p-2.5 rounded-lg text-[11px] leading-relaxed border ${
                        testResult.success
                          ? 'bg-emerald-50 text-emerald-800 border-emerald-200'
                          : 'bg-rose-50 text-rose-800 border-rose-200'
                      }`}
                    >
                      <p className="font-semibold">{testResult.message}</p>
                      {testResult.success && (
                        <p className="font-mono mt-0.5 text-emerald-600">
                          往返延迟: {testResult.latencyMs} ms
                        </p>
                      )}
                    </div>
                  )}

                  <div className="flex items-center justify-between text-[11px] text-slate-400 font-mono pt-1">
                    <span>最近同步：{activeForm.sync_time}</span>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Right 2-Cols: Dynamic Group Mapping Table */}
          <div className="lg:col-span-2 space-y-6">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <div>
                  <CardTitle className="text-base font-semibold flex items-center gap-2">
                    <Users className="w-4 h-4 text-purple-600" />
                    <span>动态组映射规则 (Dynamic Group Mapping)</span>
                  </CardTitle>
                  <p className="text-xs text-slate-500 mt-1">
                    读取 IdP 返回的 JWT Token 中的 <code>groups</code> 声明，无缝映射至 VeloTrust 安全组，动态生成 WireGuard 允许路由。
                  </p>
                </div>
                <Button size="sm" onClick={handleSaveProviderConfig} isLoading={savingConfig} className="gap-1.5 shadow-xs">
                  <Check className="w-3.5 h-3.5" />
                  <span>保存所有变更</span>
                </Button>
              </CardHeader>
              <CardContent className="space-y-4">
                {/* Mapping Table */}
                <div className="border border-slate-200 rounded-xl overflow-hidden">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>IdP 身份源部门 / 角色路径</TableHead>
                        <TableHead className="w-12 text-center">→</TableHead>
                        <TableHead>VeloTrust 本地网络安全组</TableHead>
                        <TableHead className="w-16 text-right">操作</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {activeForm.group_mapping.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={4} className="text-center py-8 text-xs text-slate-400">
                            暂无映射规则，来自该 IdP 的登录员工将仅获得默认基础策略。
                          </TableCell>
                        </TableRow>
                      ) : (
                        activeForm.group_mapping.map((mapping) => (
                          <TableRow key={mapping.id}>
                            <TableCell className="font-mono text-xs font-semibold text-slate-800">
                              <span className="p-1 px-2 rounded bg-slate-100 border border-slate-200">
                                {mapping.idp_group_name}
                              </span>
                            </TableCell>
                            <TableCell className="text-center text-slate-400 font-bold">→</TableCell>
                            <TableCell>
                              <Badge variant="primary" className="font-medium text-xs">
                                <ShieldCheck className="w-3 h-3 mr-1" />
                                {mapping.velotrust_group_id}
                              </Badge>
                            </TableCell>
                            <TableCell className="text-right">
                              <button
                                onClick={() => handleRemoveMapping(mapping.id)}
                                className="text-slate-400 hover:text-rose-600 p-1 transition-colors"
                                title="删除该映射"
                              >
                                <Trash2 className="w-3.5 h-3.5" />
                              </button>
                            </TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </div>

                {/* Add new mapping row */}
                <div className="flex flex-col sm:flex-row items-center gap-2 p-3 bg-slate-50 border border-slate-200 rounded-xl">
                  <div className="flex-1 w-full">
                    <Input
                      placeholder="输入 IdP 中的部门或角色 (如: 基础架构部/后端开发)"
                      value={newIdpGroup}
                      onChange={(e) => setNewIdpGroup(e.target.value)}
                      className="h-9 text-xs"
                    />
                  </div>
                  <div className="w-full sm:w-60">
                    <Select
                      value={newVeloGroup}
                      onChange={(e) => setNewVeloGroup(e.target.value)}
                      className="h-9 text-xs"
                    >
                      {groups.map((g) => (
                        <option key={g.id} value={g.name}>
                          映射到：{g.name}
                        </option>
                      ))}
                    </Select>
                  </div>
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    onClick={handleAddMapping}
                    className="h-9 text-xs shrink-0 gap-1 w-full sm:w-auto"
                  >
                    <Plus className="w-3.5 h-3.5" />
                    <span>+ 添加映射行</span>
                  </Button>
                </div>

                <div className="p-3 bg-blue-50/70 border border-blue-200 rounded-xl text-xs text-blue-900 leading-relaxed flex items-start gap-2.5">
                  <Sparkles className="w-4 h-4 text-blue-600 shrink-0 mt-0.5" />
                  <div>
                    <strong>零信任动态降权机制：</strong>
                    员工在飞书/钉钉中调岗或离职时，IdP Webhook
                    将即时推送变更通知。控制平面在3秒内刷新该员工的 WireGuard AllowedIPs 与
                    iptables ACL 规则，无需手动回收设备。
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {/* JWT Token & Dynamic Policy Simulator Modal */}
      <Dialog
        isOpen={isSimulatorOpen}
        onClose={() => setIsSimulatorOpen(false)}
        title="JWT Token 解析与 WireGuard 策略动态生成试算器"
        description="模拟员工通过飞书/Okta 扫码返回的 OIDC ID Token Payload，实时演算 VeloTrust 路由权限下发过程"
        maxWidth="2xl"
      >
        <div className="space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 bg-slate-50 p-3.5 rounded-xl border border-slate-200">
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                模拟员工主体标识 (sub)
              </label>
              <Input
                value={simSub}
                onChange={(e) => setSimSub(e.target.value)}
                className="font-mono text-xs"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                员工工作邮箱 (email)
              </label>
              <Input
                value={simEmail}
                onChange={(e) => setSimEmail(e.target.value)}
                className="font-mono text-xs"
              />
            </div>
            <div className="sm:col-span-2">
              <label className="block text-xs font-medium text-slate-700 mb-1">
                JWT Token 携带的 IdP 部门数组 (groups 声明，逗号分隔)
              </label>
              <Input
                value={simGroupsInput}
                onChange={(e) => setSimGroupsInput(e.target.value)}
                className="font-mono text-xs"
                placeholder="例如: 基础架构部/后端开发, 质量保障与测试中心"
              />
            </div>
          </div>

          <div className="flex justify-end">
            <Button
              size="sm"
              onClick={handleRunJwtSimulation}
              isLoading={simulating}
              className="gap-1.5"
            >
              <Sparkles className="w-3.5 h-3.5" />
              <span>执行 JWT 动态策略推导</span>
            </Button>
          </div>

          {/* Simulation Output Dashboard */}
          {simedResultRender(simulatedResult)}

          <div className="flex justify-end pt-2 border-t border-slate-100">
            <Button variant="outline" onClick={() => setIsSimulatorOpen(false)}>
              关闭
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
};

// Sub-render for JWT simulation output
function simedResultRender(res: any) {
  if (!res) return null;

  return (
    <div className="p-4 rounded-xl border border-slate-200 bg-slate-900 text-slate-200 space-y-3 font-mono text-xs">
      <div className="flex items-center justify-between border-b border-slate-800 pb-2">
        <span className="text-slate-400">OIDC 凭据校验结果:</span>
        <span className="text-emerald-400 font-bold flex items-center gap-1">
          <CheckCircle2 className="w-3.5 h-3.5" />
          JWT 签名合法 (256-bit ECDSA OK)
        </span>
      </div>

      <div className="space-y-2 text-[11px]">
        <div className="flex justify-between">
          <span className="text-slate-400">认证主体:</span>
          <span className="text-white font-bold">{res.authenticatedUser.email}</span>
        </div>

        <div>
          <span className="text-slate-400 block mb-1">动态命中的 VeloTrust 安全组:</span>
          <div className="flex flex-wrap gap-1.5">
            {res.resolvedGroups && res.resolvedGroups.length > 0 ? (
              res.resolvedGroups.map((g: string) => (
                <span
                  key={g}
                  className="px-2 py-0.5 rounded bg-blue-500/20 text-blue-300 border border-blue-400/30 font-bold"
                >
                  {g}
                </span>
              ))
            ) : (
              <span className="text-slate-500">未命中任何映射组（仅限公共访客策略）</span>
            )}
          </div>
        </div>

        <div>
          <span className="text-slate-400 block mb-1">下发至 WireGuard 客户端的允许路由策略:</span>
          <div className="space-y-1">
            {res.matchedPolicies.map((p: any, idx: number) => (
              <div
                key={idx}
                className="flex items-center justify-between p-1.5 rounded bg-slate-800/80 border border-slate-700 text-[10px]"
              >
                <span className="text-slate-300">{p.name}</span>
                <div className="flex items-center gap-2">
                  <span className="text-blue-400 font-bold">{p.targetIp}</span>
                  <span className="text-slate-400">:{p.ports}</span>
                  <span className={p.action === 'allow' ? 'text-emerald-400 font-bold' : 'text-rose-400 font-bold'}>
                    [{p.action.toUpperCase()}]
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div>
          <span className="text-slate-400 block mb-1">动态加载的内网 Split DNS 视界:</span>
          <div className="flex flex-wrap gap-1">
            {res.accessibleDnsZones.map((z: string) => (
              <span key={z} className="px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-300 border border-purple-400/30 text-[10px]">
                {z}
              </span>
            ))}
          </div>
        </div>

        <div className="flex justify-between pt-1 border-t border-slate-800 text-[10px]">
          <span className="text-slate-400">会话有效期至 (Session Expiry):</span>
          <span className="text-amber-400 font-semibold">{res.expiryTimestamp}</span>
        </div>
      </div>
    </div>
  );
}
