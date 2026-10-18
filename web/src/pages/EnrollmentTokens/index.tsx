import React, { useState, useEffect, useMemo } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import {
  Key,
  Plus,
  Search,
  RefreshCw,
  Copy,
  Check,
  Shield,
  ShieldAlert,
  Clock,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  Server,
  Network,
  Terminal,
  ExternalLink,
  Ban,
  RotateCw,
  Info,
  ChevronRight,
  Sparkles,
  ArrowRight,
  Globe,
  Sliders,
  Play,
  Lock,
  Layers,
  Cpu,
} from 'lucide-react';
import { mockApi } from '../../data/mock';
import {
  EnrollmentToken,
  EnrollmentTokenStatus,
  Node,
  PreCreateNodeInput,
  GenerateTokenInput,
} from '../../types';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Select } from '../../components/ui/select';
import { Badge } from '../../components/ui/badge';
import { Card } from '../../components/ui/card';
import { Dialog } from '../../components/ui/dialog';
import { EmptyState } from '../../components/EmptyState';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useToast } from '../../components/ui/toast';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '../../components/ui/table';

export const EnrollmentTokens: React.FC = () => {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { success, error, info } = useToast();

  const [tokens, setTokens] = useState<EnrollmentToken[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [copiedTokenId, setCopiedTokenId] = useState<string | null>(null);

  // Filters
  const [search, setSearch] = useState(searchParams.get('q') || '');
  const [statusFilter, setStatusFilter] = useState(searchParams.get('status') || 'all');
  const [typeFilter, setTypeFilter] = useState(searchParams.get('type') || 'all');

  // Modals state
  const [isCreateWizardOpen, setIsCreateWizardOpen] = useState(false);
  const [isExistingNodeTokenOpen, setIsExistingNodeTokenOpen] = useState(false);
  const [selectedTokenDetail, setSelectedTokenDetail] = useState<EnrollmentToken | null>(null);
  const [tokenToRevoke, setTokenToRevoke] = useState<EnrollmentToken | null>(null);
  const [revokeReason, setRevokeReason] = useState('业务部署调整，废弃原入网凭证');
  const [revoking, setRevoking] = useState(false);

  // Simulation modal state
  const [isSimulateModalOpen, setIsSimulateModalOpen] = useState(false);
  const [simulateTokenId, setSimulateTokenId] = useState('');
  const [simulateClientIp, setSimulateClientIp] = useState('180.168.41.88');
  const [simulateHostname, setSimulateHostname] = useState('gateway-appliance-sh01');
  const [isSimulating, setIsSimulating] = useState(false);
  const [simulationLogs, setSimulationLogs] = useState<string[]>([]);
  const [simulationResult, setSimulationResult] = useState<any>(null);

  // Pre-creation wizard state (Step 1 -> Step 2 -> Success)
  const [wizardStep, setWizardStep] = useState<1 | 2 | 3>(1);
  const [wizardNodeName, setWizardNodeName] = useState('shanghai-prod-gateway-01');
  const [wizardNodeType, setWizardNodeType] = useState('站点接入网关');
  const [wizardNetwork, setWizardNetwork] = useState('上海生产网');
  const [wizardSubnets, setWizardSubnets] = useState('10.20.0.0/16');
  const [wizardRoles, setWizardRoles] = useState<string[]>(['流量转发', '站点互联']);
  const [wizardRequireApproval, setWizardRequireApproval] = useState(true);
  const [wizardExpiresHours, setWizardExpiresHours] = useState(24);
  const [wizardNotes, setWizardNotes] = useState('上海生产网网关节点预建与一次性入网凭证');
  const [creating, setCreating] = useState(false);
  const [createdResult, setCreatedResult] = useState<{ node: Node; token: EnrollmentToken } | null>(null);

  // Existing node token generation state
  const [selectedExistingNodeId, setSelectedExistingNodeId] = useState('');
  const [existingNodeExpiresHours, setExistingNodeExpiresHours] = useState(24);
  const [existingNodeNotes, setExistingNodeNotes] = useState('');

  const loadData = async (showRefresh = false) => {
    if (showRefresh) setIsRefreshing(true);
    try {
      const [tokenList, nodeList] = await Promise.all([
        mockApi.getEnrollmentTokens(),
        mockApi.getNodes(),
      ]);
      setTokens(tokenList);
      setNodes(nodeList);
    } catch {
      error('获取注册凭证列表失败');
    } finally {
      setLoading(false);
      setIsRefreshing(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  // Sync search params
  useEffect(() => {
    const params: Record<string, string> = {};
    if (search) params.q = search;
    if (statusFilter !== 'all') params.status = statusFilter;
    if (typeFilter !== 'all') params.type = typeFilter;
    setSearchParams(params, { replace: true });
  }, [search, statusFilter, typeFilter, setSearchParams]);

  // Copy helper
  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedTokenId(id);
    success('已成功复制到剪贴板');
    setTimeout(() => setCopiedTokenId(null), 2500);
  };

  // Filtered tokens
  const filteredTokens = useMemo(() => {
    return tokens.filter((t) => {
      const matchesSearch =
        !search ||
        t.nodeName.toLowerCase().includes(search.toLowerCase()) ||
        t.token.toLowerCase().includes(search.toLowerCase()) ||
        t.networkName.toLowerCase().includes(search.toLowerCase()) ||
        t.allowedSubnets.some((s) => s.toLowerCase().includes(search.toLowerCase())) ||
        t.id.toLowerCase().includes(search.toLowerCase());

      const matchesStatus = statusFilter === 'all' || t.status === statusFilter;
      const matchesType = typeFilter === 'all' || t.nodeType === typeFilter;

      return matchesSearch && matchesStatus && matchesType;
    });
  }, [tokens, search, statusFilter, typeFilter]);

  // Statistics
  const stats = useMemo(() => {
    const total = tokens.length;
    const pending = tokens.filter((t) => t.status === 'PENDING').length;
    const used = tokens.filter((t) => t.status === 'USED').length;
    const expired = tokens.filter((t) => t.status === 'EXPIRED').length;
    const revoked = tokens.filter((t) => t.status === 'REVOKED').length;
    const pendingNodes = nodes.filter((n) => n.status === 'pending_enrollment' || n.enrollmentStatus === 'unregistered').length;
    return { total, pending, used, expired, revoked, pendingNodes };
  }, [tokens, nodes]);

  // Nodes without an active PENDING token
  const availableNodesForToken = useMemo(() => {
    const activeTokenNodeIds = new Set(
      tokens.filter((t) => t.status === 'PENDING').map((t) => t.nodeId)
    );
    return nodes.filter((n) => !activeTokenNodeIds.has(n.id));
  }, [nodes, tokens]);

  // Wizard submission (Two-stage creation)
  const handleCreateWizardSubmit = async () => {
    if (!wizardNodeName.trim()) {
      error('请输入网关节点名称');
      return;
    }
    const subnets = wizardSubnets
      .split(/[,;\s]+/)
      .map((s) => s.trim())
      .filter(Boolean);

    if (subnets.length === 0) {
      error('请至少指定一个允许访问的网段 CIDR');
      return;
    }

    setCreating(true);
    try {
      const nodeInput: PreCreateNodeInput = {
        name: wizardNodeName.trim(),
        nodeTypeLabel: wizardNodeType,
        networkName: wizardNetwork,
        allowedSubnets: subnets,
        nodeRoles: wizardRoles.length > 0 ? wizardRoles : ['流量转发'],
        registrationMethod: 'token',
        requireApproval: wizardRequireApproval,
      };

      const res = await mockApi.preCreateNodeAndToken(nodeInput, {
        expiresInHours: Number(wizardExpiresHours),
        notes: wizardNotes,
      });

      setCreatedResult(res);
      setWizardStep(3); // Success step
      success(`逻辑节点 [${res.node.name}] 及一次性注册凭证已创建成功！`);
      await loadData();
    } catch (err: any) {
      error(err.message || '创建节点与注册凭证失败');
    } finally {
      setCreating(false);
    }
  };

  // Generate token for existing node
  const handleGenerateExistingNodeToken = async () => {
    if (!selectedExistingNodeId) {
      error('请选择一个预创建节点');
      return;
    }
    setCreating(true);
    try {
      const newToken = await mockApi.generateEnrollmentToken({
        nodeId: selectedExistingNodeId,
        expiresInHours: Number(existingNodeExpiresHours),
        notes: existingNodeNotes,
      });
      success(`已为节点 [${newToken.nodeName}] 签发一次性入网凭证`);
      setIsExistingNodeTokenOpen(false);
      setSelectedTokenDetail(newToken);
      await loadData();
    } catch (err: any) {
      error(err.message || '生成凭证失败');
    } finally {
      setCreating(false);
    }
  };

  // Revoke token
  const handleRevokeConfirm = async () => {
    if (!tokenToRevoke) return;
    setRevoking(true);
    try {
      await mockApi.revokeEnrollmentToken(tokenToRevoke.id, revokeReason);
      success(`凭证 ${tokenToRevoke.id} 已作废`);
      setTokenToRevoke(null);
      await loadData();
    } catch (err: any) {
      error(err.message || '作废凭证失败');
    } finally {
      setRevoking(false);
    }
  };

  // Extend token
  const handleExtendToken = async (token: EnrollmentToken) => {
    try {
      await mockApi.extendEnrollmentToken(token.id, 24);
      success(`凭证 ${token.id} 有效期已延长 24 小时`);
      await loadData();
    } catch (err: any) {
      error(err.message || '延长有效期失败');
    }
  };

  // Run simulation
  const handleStartSimulation = async (targetTokenId?: string) => {
    const tokenTarget = targetTokenId || simulateTokenId;
    if (!tokenTarget) {
      error('请选择一个待入网的有效凭证');
      return;
    }

    setIsSimulating(true);
    setSimulationLogs([]);
    setSimulationResult(null);

    const target = tokens.find((t) => t.id === tokenTarget);
    const nodeName = target?.nodeName || 'gateway-node';

    const logSteps = [
      `[1/6] 网关客户端发起 TLS 握手，连接 VeloTrust 控制面认证服务 (https://mesh-control.internal)...`,
      `[2/6] 提交一次性注册凭证 Token: ${target?.token.substring(0, 16)}...`,
      `[3/6] 控制面校验凭证有效性：状态 [PENDING]，有效时长未过期，通过鉴权！`,
      `[4/6] 绑定逻辑节点记录 [${nodeName}]，类型 [${target?.nodeType}]，授权访问网段 [${target?.allowedSubnets.join(', ')}]...`,
      `[5/6] 强制执行安全边界：凭证权限约束为 [node:register_only] (无控制面与业务数据越权)...`,
      `[6/6] 注册成功！依据安全策略立即销毁一次性凭证 (状态翻转为 USED，使用计数 1/1)，节点已激活上线！`,
    ];

    for (let i = 0; i < logSteps.length; i++) {
      await new Promise((r) => setTimeout(r, 180));
      setSimulationLogs((prev) => [...prev, logSteps[i]]);
    }

    try {
      const res = await mockApi.simulateNodeEnrollment(tokenTarget, {
        clientIp: simulateClientIp,
        hostname: simulateHostname,
      });
      setSimulationResult(res);
      success(res.message);
      await loadData();
    } catch (err: any) {
      error(err.message || '模拟入网握手失败');
    } finally {
      setIsSimulating(false);
    }
  };

  const getStatusBadge = (status: EnrollmentTokenStatus) => {
    switch (status) {
      case 'PENDING':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
            有效待注册
          </span>
        );
      case 'USED':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-slate-100 text-slate-700 border border-slate-200">
            <CheckCircle2 className="w-3.5 h-3.5 text-slate-500" />
            已使用入网 (已失效)
          </span>
        );
      case 'EXPIRED':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-amber-50 text-amber-700 border border-amber-200">
            <Clock className="w-3.5 h-3.5 text-amber-500" />
            已过期失效
          </span>
        );
      case 'REVOKED':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-rose-50 text-rose-700 border border-rose-200">
            <Ban className="w-3.5 h-3.5 text-rose-500" />
            已撤动作废
          </span>
        );
      default:
        return null;
    }
  };

  const formatExpiresCountdown = (expiresAt: string, status: EnrollmentTokenStatus) => {
    if (status === 'USED') return '已完成入网消耗';
    if (status === 'REVOKED') return '已被管理员作废';

    const diffMs = new Date(expiresAt).getTime() - Date.now();
    if (diffMs <= 0) return '已超过有效期限';

    const hours = Math.floor(diffMs / (1000 * 3600));
    const mins = Math.floor((diffMs % (1000 * 3600)) / (1000 * 60));
    if (hours > 24) {
      const days = Math.floor(hours / 24);
      return `剩余 ${days} 天 ${hours % 24} 小时`;
    }
    return `剩余 ${hours} 小时 ${mins} 分钟`;
  };

  return (
    <div className="space-y-5">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">
            网关注册凭证管理
          </h1>
          <p className="text-xs text-slate-500 mt-0.5">
            两阶段安全入网：预先创建逻辑节点定义用途与管辖网段，按需签发一次性入网凭证
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => loadData(true)}
            isLoading={isRefreshing}
            className="gap-1.5"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? 'animate-spin' : ''}`} />
            <span>刷新</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              const pendingOne = tokens.find((t) => t.status === 'PENDING');
              if (pendingOne) {
                setSimulateTokenId(pendingOne.id);
              }
              setIsSimulateModalOpen(true);
            }}
            className="gap-1.5 text-slate-700 hover:text-slate-900"
          >
            <Play className="w-3.5 h-3.5 text-indigo-600 fill-current" />
            <span>模拟入网</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={() => setIsExistingNodeTokenOpen(true)}
            className="gap-1.5"
          >
            <Key className="w-3.5 h-3.5 text-slate-500" />
            <span>已有节点签发</span>
          </Button>

          <Button
            size="sm"
            onClick={() => {
              setWizardStep(1);
              setCreatedResult(null);
              setIsCreateWizardOpen(true);
            }}
            className="gap-1.5 bg-blue-600 hover:bg-blue-700 text-white"
          >
            <Plus className="w-3.5 h-3.5" />
            <span>预建节点并生成凭证</span>
          </Button>
        </div>
      </div>

      {/* Metrics Stat Strip */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <Card className="p-3.5">
          <div className="text-xs text-slate-500 font-medium">全部凭证</div>
          <div className="text-2xl font-bold text-slate-900 mt-1">{stats.total}</div>
        </Card>

        <Card className="p-3.5 border-emerald-200/80 bg-emerald-50/20">
          <div className="text-xs text-emerald-800 font-medium flex items-center justify-between">
            <span>待入网凭证</span>
            <span className="w-2 h-2 rounded-full bg-emerald-500" />
          </div>
          <div className="text-2xl font-bold text-emerald-800 mt-1">{stats.pending}</div>
        </Card>

        <Card className="p-3.5">
          <div className="text-xs text-slate-500 font-medium">已入网生效 (已失效)</div>
          <div className="text-2xl font-bold text-slate-800 mt-1">{stats.used}</div>
        </Card>

        <Card className="p-3.5">
          <div className="text-xs text-slate-500 font-medium">待入网逻辑节点</div>
          <div className="text-2xl font-bold text-slate-900 mt-1">{stats.pendingNodes}</div>
        </Card>
      </div>

      {/* Filter and Search Bar */}
      <Card className="p-3.5">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          <div className="lg:col-span-2">
            <Input
              placeholder="搜索节点名称、凭证 Token、所属网络或网段..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              leftIcon={<Search className="w-4 h-4 text-slate-400" />}
            />
          </div>

          <Select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          >
            <option value="all">凭证状态：全部</option>
            <option value="PENDING">有效待注册 (PENDING)</option>
            <option value="USED">已使用入网 (USED)</option>
            <option value="EXPIRED">已过期 (EXPIRED)</option>
            <option value="REVOKED">已作废 (REVOKED)</option>
          </Select>

          <Select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
          >
            <option value="all">节点类型：全部</option>
            <option value="站点接入网关">站点接入网关</option>
            <option value="服务代理网关">服务代理网关</option>
            <option value="边缘中继网关">边缘中继网关</option>
            <option value="核心汇聚网关">核心汇聚网关</option>
          </Select>
        </div>
      </Card>

      {/* Tokens Table Card */}
      <Card>
        <div className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-48">凭证标识 / Token</TableHead>
                <TableHead>绑定逻辑节点</TableHead>
                <TableHead>所属网络与授权网段</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>有效期限 / 消耗</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={6} className="text-center py-12 text-slate-400">
                    <RefreshCw className="w-5 h-5 animate-spin mx-auto mb-2 text-blue-600" />
                    <span>正在加载注册凭证数据...</span>
                  </TableCell>
                </TableRow>
              ) : filteredTokens.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="p-8">
                    <EmptyState
                      icon={Key}
                      title="暂无匹配的注册凭证"
                      description="未找到符合条件的网关注册凭证，您可以点击右上角【预建节点并生成凭证】发起两阶段入网流程。"
                      actionText="清除过滤条件"
                      onAction={() => {
                        setSearch('');
                        setStatusFilter('all');
                        setTypeFilter('all');
                      }}
                    />
                  </TableCell>
                </TableRow>
              ) : (
                filteredTokens.map((token) => (
                  <TableRow key={token.id} className="group hover:bg-slate-50/70 transition-colors">
                    {/* Token ID and value */}
                    <TableCell>
                      <div className="space-y-1">
                        <div className="font-mono text-xs font-semibold text-slate-900 flex items-center gap-1.5">
                          <Key className="w-3.5 h-3.5 text-blue-600" />
                          <span>{token.id}</span>
                        </div>
                        <div className="flex items-center gap-1.5">
                          <code className="text-[11px] font-mono text-slate-600 bg-slate-100 px-1.5 py-0.5 rounded border border-slate-200 truncate max-w-[130px]" title={token.token}>
                            {token.token.substring(0, 10)}...{token.token.substring(token.token.length - 4)}
                          </code>
                          <button
                            onClick={() => handleCopy(token.token, token.id)}
                            title="复制完整 Token"
                            className="p-1 text-slate-400 hover:text-blue-600 hover:bg-blue-50 rounded transition-colors"
                          >
                            {copiedTokenId === token.id ? (
                              <Check className="w-3 h-3 text-emerald-600" />
                            ) : (
                              <Copy className="w-3 h-3" />
                            )}
                          </button>
                        </div>
                      </div>
                    </TableCell>

                    {/* Node and Type */}
                    <TableCell>
                      <div className="space-y-1">
                        <div className="flex items-center gap-1.5 font-medium text-slate-900 text-xs">
                          <Server className="w-3.5 h-3.5 text-slate-400" />
                          <span>{token.nodeName}</span>
                          {token.requireApproval && (
                            <Badge variant="outline" className="text-[10px] py-0 px-1 border-amber-300 text-amber-700 bg-amber-50">
                              需审批
                            </Badge>
                          )}
                        </div>
                        <div className="flex items-center gap-1 text-[11px] text-slate-500">
                          <span className="px-1.5 py-0.2 rounded bg-slate-100 text-slate-700 font-medium">
                            {token.nodeType}
                          </span>
                          {token.nodeRoles && token.nodeRoles.map((role) => (
                            <span key={role} className="text-[10px] px-1 py-0.2 rounded bg-slate-100 text-slate-600">
                              {role}
                            </span>
                          ))}
                        </div>
                      </div>
                    </TableCell>

                    {/* Network and Subnets */}
                    <TableCell>
                      <div className="space-y-1">
                        <div className="text-xs text-slate-700 font-medium flex items-center gap-1">
                          <Network className="w-3 h-3 text-slate-400" />
                          <span>{token.networkName}</span>
                        </div>
                        <div className="flex flex-wrap gap-1">
                          {token.allowedSubnets.map((subnet) => (
                            <code
                              key={subnet}
                              className="text-[11px] font-mono bg-slate-50 text-slate-700 px-1.5 py-0.5 rounded border border-slate-200"
                            >
                              {subnet}
                            </code>
                          ))}
                        </div>
                      </div>
                    </TableCell>

                    {/* Status */}
                    <TableCell>
                      {getStatusBadge(token.status)}
                    </TableCell>

                    {/* Expiration and Audit */}
                    <TableCell>
                      <div className="space-y-0.5 text-xs">
                        <div className="text-slate-800 font-medium">
                          {formatExpiresCountdown(token.expiresAt, token.status)}
                        </div>
                        {token.status === 'USED' && token.usedAt && (
                          <div className="text-[11px] text-slate-500 font-mono">
                            对端: {token.usedIp || '180.168.41.88'}
                          </div>
                        )}
                        {token.status === 'REVOKED' && token.revokeReason && (
                          <div className="text-[11px] text-rose-600 truncate max-w-[150px]" title={token.revokeReason}>
                            {token.revokeReason}
                          </div>
                        )}
                      </div>
                    </TableCell>

                    {/* Actions */}
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setSelectedTokenDetail(token)}
                          className="h-7 px-2 text-xs text-blue-600 hover:text-blue-700 hover:bg-blue-50"
                        >
                          配置指令
                        </Button>

                        {token.status === 'PENDING' && (
                          <>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => {
                                setSimulateTokenId(token.id);
                                setIsSimulateModalOpen(true);
                              }}
                              className="h-7 px-1.5 text-xs text-indigo-600 hover:text-indigo-700 hover:bg-indigo-50"
                              title="模拟网关使用该凭证入网"
                            >
                              <Play className="w-3 h-3 fill-current" />
                            </Button>

                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => setTokenToRevoke(token)}
                              className="h-7 px-1.5 text-xs text-rose-600 hover:text-rose-700 hover:bg-rose-50"
                              title="作废凭证"
                            >
                              <Ban className="w-3 h-3" />
                            </Button>
                          </>
                        )}

                        {(token.status === 'EXPIRED' || token.status === 'REVOKED') && (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleExtendToken(token)}
                            className="h-7 px-1.5 text-xs text-amber-600 hover:text-amber-700 hover:bg-amber-50"
                            title="延期激活 24 小时"
                          >
                            <RotateCw className="w-3 h-3" />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </Card>

      {/* ================= MODAL: Two-Stage Pre-Create Wizard Modal ================= */}
      <Dialog
        isOpen={isCreateWizardOpen}
        onClose={() => {
          if (!creating) {
            setIsCreateWizardOpen(false);
            setCreatedResult(null);
          }
        }}
        maxWidth="2xl"
        title={
          <div className="flex items-center gap-2">
            <Sparkles className="w-5 h-5 text-blue-600" />
            <span>网关节点两阶段入网向导</span>
          </div>
        }
        description="阶段一：在控制台预创建逻辑节点；阶段二：签发限定有效期的一次性入网凭证"
      >
        <div className="space-y-5">
          {/* Step Indicator Header */}
          <div className="grid grid-cols-3 gap-2 bg-slate-50 p-1.5 rounded-lg border border-slate-200 text-xs">
            <div
              className={`flex items-center justify-center gap-1.5 py-1.5 rounded-md font-medium transition-colors ${
                wizardStep === 1
                  ? 'bg-white text-blue-700 shadow-2xs font-semibold'
                  : 'text-slate-500'
              }`}
            >
              <span className={`w-4 h-4 rounded-full flex items-center justify-center text-[10px] ${wizardStep === 1 ? 'bg-blue-600 text-white' : 'bg-slate-200 text-slate-600'}`}>
                1
              </span>
              <span>阶段一：预建节点</span>
            </div>
            <div
              className={`flex items-center justify-center gap-1.5 py-1.5 rounded-md font-medium transition-colors ${
                wizardStep === 2
                  ? 'bg-white text-blue-700 shadow-2xs font-semibold'
                  : 'text-slate-500'
              }`}
            >
              <span className={`w-4 h-4 rounded-full flex items-center justify-center text-[10px] ${wizardStep === 2 ? 'bg-blue-600 text-white' : 'bg-slate-200 text-slate-600'}`}>
                2
              </span>
              <span>阶段二：凭证安全</span>
            </div>
            <div
              className={`flex items-center justify-center gap-1.5 py-1.5 rounded-md font-medium transition-colors ${
                wizardStep === 3
                  ? 'bg-white text-emerald-700 shadow-2xs font-semibold'
                  : 'text-slate-500'
              }`}
            >
              <span className={`w-4 h-4 rounded-full flex items-center justify-center text-[10px] ${wizardStep === 3 ? 'bg-emerald-600 text-white' : 'bg-slate-200 text-slate-600'}`}>
                3
              </span>
              <span>入网指令下发</span>
            </div>
          </div>

          {/* STEP 1: Pre-create logical node record */}
          {wizardStep === 1 && (
            <div className="space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    节点名称 <span className="text-rose-500">*</span>
                  </label>
                  <Input
                    placeholder="例如 shanghai-prod-gateway-01"
                    value={wizardNodeName}
                    onChange={(e) => setWizardNodeName(e.target.value)}
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    节点类型 <span className="text-rose-500">*</span>
                  </label>
                  <Select
                    value={wizardNodeType}
                    onChange={(e) => setWizardNodeType(e.target.value)}
                  >
                    <option value="站点接入网关">站点接入网关</option>
                    <option value="服务代理网关">服务代理网关</option>
                    <option value="边缘中继网关">边缘中继网关</option>
                    <option value="核心汇聚网关">核心汇聚网关</option>
                  </Select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    所属网络 <span className="text-rose-500">*</span>
                  </label>
                  <Input
                    placeholder="例如 上海生产网"
                    value={wizardNetwork}
                    onChange={(e) => setWizardNetwork(e.target.value)}
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    允许访问的网段 (CIDR) <span className="text-rose-500">*</span>
                  </label>
                  <Input
                    placeholder="例如 10.20.0.0/16"
                    value={wizardSubnets}
                    onChange={(e) => setWizardSubnets(e.target.value)}
                  />
                </div>
              </div>

              {/* Node Roles */}
              <div>
                <label className="block text-xs font-medium text-slate-700 mb-1.5">
                  节点角色
                </label>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                  {['流量转发', '站点互联', 'DNS中继', '边界隔离'].map((role) => {
                    const isChecked = wizardRoles.includes(role);
                    return (
                      <button
                        key={role}
                        type="button"
                        onClick={() => {
                          if (isChecked) {
                            setWizardRoles(wizardRoles.filter((r) => r !== role));
                          } else {
                            setWizardRoles([...wizardRoles, role]);
                          }
                        }}
                        className={`flex items-center justify-between p-2 rounded-lg border text-xs text-left transition-colors ${
                          isChecked
                            ? 'border-blue-500 bg-blue-50/50 text-blue-900 font-medium'
                            : 'border-slate-200 bg-white text-slate-600 hover:bg-slate-50'
                        }`}
                      >
                        <span>{role}</span>
                        {isChecked && <Check className="w-3.5 h-3.5 text-blue-600" />}
                      </button>
                    );
                  })}
                </div>
              </div>

              {/* Registration Method & Approval */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2 border-t border-slate-100">
                <div className="p-2.5 rounded-lg border border-slate-200 bg-slate-50/50">
                  <div className="text-xs font-medium text-slate-700">注册方式</div>
                  <div className="text-xs text-blue-700 font-medium mt-1 flex items-center gap-1.5">
                    <Key className="w-3.5 h-3.5" />
                    <span>一次性注册凭证</span>
                  </div>
                </div>

                <div className="p-2.5 rounded-lg border border-slate-200 bg-slate-50/50 flex flex-col justify-between">
                  <div className="text-xs font-medium text-slate-700">是否需要审批</div>
                  <div className="flex items-center gap-4 mt-1.5">
                    <label className="flex items-center gap-1.5 text-xs text-slate-700 cursor-pointer">
                      <input
                        type="radio"
                        name="requireApproval"
                        checked={wizardRequireApproval === true}
                        onChange={() => setWizardRequireApproval(true)}
                        className="text-blue-600 focus:ring-blue-500"
                      />
                      <span>是</span>
                    </label>
                    <label className="flex items-center gap-1.5 text-xs text-slate-700 cursor-pointer">
                      <input
                        type="radio"
                        name="requireApproval"
                        checked={wizardRequireApproval === false}
                        onChange={() => setWizardRequireApproval(false)}
                        className="text-blue-600 focus:ring-blue-500"
                      />
                      <span>否</span>
                    </label>
                  </div>
                </div>
              </div>

              {/* Wizard Actions */}
              <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
                <Button
                  variant="outline"
                  onClick={() => setIsCreateWizardOpen(false)}
                >
                  取消
                </Button>
                <Button
                  onClick={() => {
                    if (!wizardNodeName.trim()) {
                      error('请输入节点名称');
                      return;
                    }
                    setWizardStep(2);
                  }}
                  className="gap-1.5 bg-blue-600 hover:bg-blue-700 text-white"
                >
                  <span>下一步：配置凭证</span>
                  <ChevronRight className="w-4 h-4" />
                </Button>
              </div>
            </div>
          )}

          {/* STEP 2: Configure Token Security Attributes */}
          {wizardStep === 2 && (
            <div className="space-y-4">
              {/* Node Summary */}
              <div className="p-3 rounded-lg border border-slate-200 bg-slate-50 text-xs">
                <div className="font-medium text-slate-800 mb-1.5">待绑定逻辑节点</div>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-slate-600">
                  <div>节点: <span className="font-semibold text-slate-900">{wizardNodeName}</span></div>
                  <div>类型: <span className="text-slate-800">{wizardNodeType}</span></div>
                  <div>网络: <span className="text-slate-800">{wizardNetwork}</span></div>
                  <div>网段: <code className="text-emerald-700 font-mono">{wizardSubnets}</code></div>
                </div>
              </div>

              {/* Expiration Selection */}
              <div>
                <label className="block text-xs font-medium text-slate-700 mb-1.5">
                  凭证有效时长 <span className="text-rose-500">*</span>
                </label>
                <div className="grid grid-cols-4 gap-2">
                  {[
                    { label: '1 小时', value: 1 },
                    { label: '24 小时 (推荐)', value: 24 },
                    { label: '72 小时', value: 72 },
                    { label: '7 天', value: 168 },
                  ].map((item) => (
                    <button
                      key={item.value}
                      type="button"
                      onClick={() => setWizardExpiresHours(item.value)}
                      className={`p-2 rounded-lg border text-xs text-center transition-colors ${
                        wizardExpiresHours === item.value
                          ? 'border-blue-600 bg-blue-50 text-blue-900 font-medium'
                          : 'border-slate-200 bg-white text-slate-600 hover:bg-slate-50'
                      }`}
                    >
                      {item.label}
                    </button>
                  ))}
                </div>
              </div>

              {/* Clean 3-Item Security Constraints */}
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2.5">
                <div className="p-2.5 rounded-lg border border-slate-200 bg-slate-50 text-xs">
                  <div className="flex items-center gap-1.5 font-semibold text-slate-800">
                    <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" />
                    <span>单次使用</span>
                  </div>
                  <div className="text-[11px] text-slate-500 mt-1">入网握手成功后凭证立即失效</div>
                </div>
                <div className="p-2.5 rounded-lg border border-slate-200 bg-slate-50 text-xs">
                  <div className="flex items-center gap-1.5 font-semibold text-slate-800">
                    <Server className="w-3.5 h-3.5 text-blue-600" />
                    <span>绑定预建节点</span>
                  </div>
                  <div className="text-[11px] text-slate-500 mt-1">仅限预建节点及类型接入</div>
                </div>
                <div className="p-2.5 rounded-lg border border-slate-200 bg-slate-50 text-xs">
                  <div className="flex items-center gap-1.5 font-semibold text-slate-800">
                    <ShieldAlert className="w-3.5 h-3.5 text-amber-600" />
                    <span>最小权限隔离</span>
                  </div>
                  <div className="text-[11px] text-slate-500 mt-1">仅限入网握手，无管理与业务越权</div>
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-700 mb-1">
                  备注说明 (选填)
                </label>
                <Input
                  placeholder="例如：上海浦东一号机房网关入网凭证"
                  value={wizardNotes}
                  onChange={(e) => setWizardNotes(e.target.value)}
                />
              </div>

              {/* Wizard Actions */}
              <div className="flex items-center justify-between pt-3 border-t border-slate-100">
                <Button
                  variant="outline"
                  onClick={() => setWizardStep(1)}
                  disabled={creating}
                >
                  上一步
                </Button>
                <Button
                  onClick={handleCreateWizardSubmit}
                  isLoading={creating}
                  className="gap-1.5 bg-blue-600 hover:bg-blue-700 text-white"
                >
                  <Sparkles className="w-3.5 h-3.5" />
                  <span>完成预建并生成注册凭证</span>
                </Button>
              </div>
            </div>
          )}

          {/* STEP 3: Created Result Presentation */}
          {wizardStep === 3 && createdResult && (
            <div className="space-y-4">
              <div className="text-center py-3">
                <div className="w-12 h-12 rounded-full bg-emerald-100 text-emerald-600 flex items-center justify-center mx-auto mb-2 shadow-xs">
                  <Check className="w-6 h-6 stroke-[3]" />
                </div>
                <h3 className="text-base font-bold text-slate-900">
                  逻辑节点与注册凭证创建成功！
                </h3>
                <p className="text-xs text-slate-500 mt-1">
                  逻辑节点已登记，请复制以下凭证或一键入网指令前往网关主机执行初始化。
                </p>
              </div>

              {/* Token Display Box */}
              <div className="p-3.5 bg-slate-900 text-white rounded-lg space-y-2">
                <div className="flex items-center justify-between text-xs text-slate-400">
                  <span>一次性注册 Token</span>
                  <button
                    onClick={() => handleCopy(createdResult.token.token, 'created-token')}
                    className="flex items-center gap-1 text-blue-400 hover:text-blue-300 font-medium"
                  >
                    <Copy className="w-3.5 h-3.5" />
                    <span>复制 Token</span>
                  </button>
                </div>
                <div className="font-mono text-xs text-emerald-400 break-all select-all bg-slate-800/80 p-2.5 rounded border border-slate-700">
                  {createdResult.token.token}
                </div>
                <div className="flex items-center justify-between text-[11px] text-slate-400 pt-1">
                  <span>绑定节点: <strong>{createdResult.node.name}</strong></span>
                  <span>有效期限: {createdResult.token.expiresAt.substring(0, 16).replace('T', ' ')} (单次使用)</span>
                </div>
              </div>

              {/* CLI Command Box */}
              <div className="space-y-1.5">
                <div className="flex items-center justify-between text-xs text-slate-700 font-medium">
                  <span className="flex items-center gap-1.5">
                    <Terminal className="w-3.5 h-3.5 text-slate-500" />
                    <span>网关主机一键入网命令 (推荐在网关 Shell 执行)</span>
                  </span>
                  <button
                    onClick={() => handleCopy(createdResult.token.installCommand, 'created-cli')}
                    className="text-xs text-blue-600 hover:text-blue-700 font-medium flex items-center gap-1"
                  >
                    <Copy className="w-3 h-3" />
                    <span>复制命令</span>
                  </button>
                </div>
                <pre className="p-2.5 bg-slate-950 text-slate-200 rounded-lg text-[11px] font-mono overflow-x-auto border border-slate-800 leading-relaxed">
                  {createdResult.token.installCommand}
                </pre>
              </div>

              <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
                <Button
                  variant="outline"
                  onClick={() => {
                    setIsCreateWizardOpen(false);
                    setCreatedResult(null);
                  }}
                >
                  关闭
                </Button>
                <Button
                  onClick={() => {
                    setIsCreateWizardOpen(false);
                    setSimulateTokenId(createdResult.token.id);
                    setIsSimulateModalOpen(true);
                  }}
                  className="gap-1.5 bg-indigo-600 hover:bg-indigo-700 text-white"
                >
                  <Play className="w-3.5 h-3.5 fill-current" />
                  <span>立即执行入网握手仿真测试</span>
                </Button>
              </div>
            </div>
          )}
        </div>
      </Dialog>

      {/* ================= MODAL: Existing Node Token Issue Modal ================= */}
      <Dialog
        isOpen={isExistingNodeTokenOpen}
        onClose={() => setIsExistingNodeTokenOpen(false)}
        maxWidth="md"
        title={
          <div className="flex items-center gap-2">
            <Key className="w-5 h-5 text-blue-600" />
            <span>为已有逻辑节点签发一次性凭证</span>
          </div>
        }
        description="选择未生成有效凭证或凭证已过期的预创建节点，为其签发新的入网凭证"
      >
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1">
              选择逻辑节点 <span className="text-rose-500">*</span>
            </label>
            <Select
              value={selectedExistingNodeId}
              onChange={(e) => setSelectedExistingNodeId(e.target.value)}
            >
              <option value="">-- 请选择逻辑节点 --</option>
              {availableNodesForToken.map((n) => (
                <option key={n.id} value={n.id}>
                  {n.name} ({n.nodeTypeLabel || (n.type === 'gateway' ? '网关' : '客户端')} · {n.networkName || n.siteName})
                </option>
              ))}
            </Select>
            <p className="text-[11px] text-slate-400 mt-1">
              仅列出当前没有有效 PENDING 凭证的节点
            </p>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1">
              凭证有效时长
            </label>
            <Select
              value={existingNodeExpiresHours}
              onChange={(e) => setExistingNodeExpiresHours(Number(e.target.value))}
            >
              <option value={1}>1 小时 (高安全时效)</option>
              <option value={24}>24 小时 (默认推荐)</option>
              <option value={72}>72 小时 (跨周末/调试)</option>
              <option value={168}>7 天 (长期工程排期)</option>
            </Select>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1">
              备注说明
            </label>
            <Input
              placeholder="例如：补充签发入网凭证"
              value={existingNodeNotes}
              onChange={(e) => setExistingNodeNotes(e.target.value)}
            />
          </div>

          <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 text-xs text-slate-600 space-y-1">
            <div className="font-semibold text-slate-800">安全准则提醒</div>
            <p className="text-[11px]">
              该凭证严格限制为仅单次使用，并在入网认证完成后立即作废，避免未销毁凭证留存安全隐患。
            </p>
          </div>

          <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
            <Button
              variant="outline"
              onClick={() => setIsExistingNodeTokenOpen(false)}
            >
              取消
            </Button>
            <Button
              onClick={handleGenerateExistingNodeToken}
              isLoading={creating}
              disabled={!selectedExistingNodeId}
              className="bg-blue-600 hover:bg-blue-700 text-white"
            >
              签发入网凭证
            </Button>
          </div>
        </div>
      </Dialog>

      {/* ================= MODAL: Token Detail & Config Inspection ================= */}
      {selectedTokenDetail && (
        <Dialog
          isOpen={!!selectedTokenDetail}
          onClose={() => setSelectedTokenDetail(null)}
          maxWidth="2xl"
          title={
            <div className="flex items-center gap-2">
              <Key className="w-5 h-5 text-blue-600" />
              <span>注册凭证详情与配置指令 ({selectedTokenDetail.id})</span>
            </div>
          }
          description="查看一次性凭证密文、网关自动化部署脚本与 WireGuard 配置文件模版"
        >
          <div className="space-y-4">
            {/* Status overview */}
            <div className="flex items-center justify-between p-3 rounded-lg bg-slate-50 border border-slate-200 text-xs">
              <div>
                <span className="text-slate-500">凭证状态：</span>
                {getStatusBadge(selectedTokenDetail.status)}
              </div>
              <div className="text-slate-500">
                绑定节点：<strong className="text-slate-900">{selectedTokenDetail.nodeName}</strong> ({selectedTokenDetail.nodeType})
              </div>
              <div className="text-slate-500">
                网络：<span className="text-slate-800">{selectedTokenDetail.networkName}</span>
              </div>
            </div>

            {/* Token Copy Block */}
            <div className="p-3 bg-slate-900 text-white rounded-lg space-y-1.5">
              <div className="flex items-center justify-between text-xs text-slate-400">
                <span>注册凭证 Token (一次性使用)</span>
                <button
                  onClick={() => handleCopy(selectedTokenDetail.token, 'detail-token')}
                  className="flex items-center gap-1 text-blue-400 hover:text-blue-300 font-medium"
                >
                  <Copy className="w-3 h-3" />
                  <span>复制完整 Token</span>
                </button>
              </div>
              <div className="font-mono text-xs text-emerald-400 break-all bg-slate-800/80 p-2 rounded border border-slate-700 select-all">
                {selectedTokenDetail.token}
              </div>
            </div>

            {/* One-click shell join command */}
            <div className="space-y-1">
              <div className="flex items-center justify-between text-xs text-slate-700 font-medium">
                <span className="flex items-center gap-1">
                  <Terminal className="w-3.5 h-3.5 text-slate-500" />
                  <span>网关 Shell 一键入网命令 (推荐)</span>
                </span>
                <button
                  onClick={() => handleCopy(selectedTokenDetail.installCommand, 'detail-cli')}
                  className="text-xs text-blue-600 hover:text-blue-700 font-medium flex items-center gap-1"
                >
                  <Copy className="w-3 h-3" />
                  <span>复制</span>
                </button>
              </div>
              <pre className="p-2.5 bg-slate-950 text-slate-200 rounded-lg text-[11px] font-mono overflow-x-auto border border-slate-800 leading-relaxed">
                {selectedTokenDetail.installCommand}
              </pre>
            </div>

            {/* WireGuard Config Snippet */}
            <div className="space-y-1">
              <div className="flex items-center justify-between text-xs text-slate-700 font-medium">
                <span className="flex items-center gap-1">
                  <Layers className="w-3.5 h-3.5 text-slate-500" />
                  <span>WireGuard 节点接口配置模版 (wg0.conf)</span>
                </span>
                <button
                  onClick={() => handleCopy(selectedTokenDetail.configSnippet, 'detail-wg')}
                  className="text-xs text-blue-600 hover:text-blue-700 font-medium flex items-center gap-1"
                >
                  <Copy className="w-3 h-3" />
                  <span>复制</span>
                </button>
              </div>
              <pre className="p-2.5 bg-slate-900 text-slate-200 rounded-lg text-[11px] font-mono overflow-x-auto border border-slate-800 leading-relaxed">
                {selectedTokenDetail.configSnippet}
              </pre>
            </div>

            {/* Detailed Metadata Grid */}
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 p-3 bg-slate-50 rounded-lg border border-slate-200 text-xs">
              <div>
                <span className="text-slate-400 block text-[11px]">创建人员</span>
                <span className="font-medium text-slate-800">{selectedTokenDetail.createdBy}</span>
              </div>
              <div>
                <span className="text-slate-400 block text-[11px]">有效截止时间</span>
                <span className="font-medium text-slate-800 font-mono">
                  {new Date(selectedTokenDetail.expiresAt).toLocaleString()}
                </span>
              </div>
              <div>
                <span className="text-slate-400 block text-[11px]">安全权限边界</span>
                <span className="font-mono text-emerald-700 font-medium">node:register_only</span>
              </div>
              {selectedTokenDetail.status === 'USED' && (
                <>
                  <div>
                    <span className="text-slate-400 block text-[11px]">消耗时间</span>
                    <span className="font-medium text-slate-800 font-mono">
                      {selectedTokenDetail.usedAt ? new Date(selectedTokenDetail.usedAt).toLocaleString() : '-'}
                    </span>
                  </div>
                  <div>
                    <span className="text-slate-400 block text-[11px]">握手来源 IP</span>
                    <span className="font-mono text-slate-800 font-medium">
                      {selectedTokenDetail.usedIp || '180.168.41.88'}
                    </span>
                  </div>
                  <div>
                    <span className="text-slate-400 block text-[11px]">设备硬件特征</span>
                    <span className="text-slate-700 truncate block" title={selectedTokenDetail.deviceFingerprint}>
                      {selectedTokenDetail.deviceFingerprint || 'Linux x86_64'}
                    </span>
                  </div>
                </>
              )}
            </div>

            <div className="flex items-center justify-end gap-2 pt-2 border-t border-slate-100">
              <Button
                variant="outline"
                onClick={() => setSelectedTokenDetail(null)}
              >
                关闭
              </Button>
            </div>
          </div>
        </Dialog>
      )}

      {/* ================= MODAL: Simulation Playground ================= */}
      <Dialog
        isOpen={isSimulateModalOpen}
        onClose={() => {
          if (!isSimulating) {
            setIsSimulateModalOpen(false);
          }
        }}
        maxWidth="2xl"
        title={
          <div className="flex items-center gap-2">
            <Play className="w-5 h-5 text-indigo-600 fill-current" />
            <span>网关节点入网握手仿真测试</span>
          </div>
        }
        description="模拟外部新安装的网关节点使用一次性凭证发起 TLS 握手入网，验证凭证立即失效与节点状态翻转闭环"
      >
        <div className="space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div className="sm:col-span-2">
              <label className="block text-xs font-medium text-slate-700 mb-1">
                选择测试入网的凭证 <span className="text-rose-500">*</span>
              </label>
              <Select
                value={simulateTokenId}
                onChange={(e) => setSimulateTokenId(e.target.value)}
                disabled={isSimulating}
              >
                <option value="">-- 请选择有效待注册凭证 --</option>
                {tokens
                  .filter((t) => t.status === 'PENDING')
                  .map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.id} ({t.nodeName} · {t.networkName} · 剩{formatExpiresCountdown(t.expiresAt, t.status)})
                    </option>
                  ))}
              </Select>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                模拟网关出口 IP
              </label>
              <Input
                value={simulateClientIp}
                onChange={(e) => setSimulateClientIp(e.target.value)}
                disabled={isSimulating}
              />
            </div>
          </div>

          {/* Terminal Console Logs */}
          <div className="space-y-1">
            <div className="flex items-center justify-between text-xs text-slate-600 font-medium">
              <span className="flex items-center gap-1.5">
                <Terminal className="w-3.5 h-3.5" />
                <span>控制面握手终端日志</span>
              </span>
              {isSimulating && (
                <span className="flex items-center gap-1 text-indigo-600 text-xs font-medium">
                  <RefreshCw className="w-3 h-3 animate-spin" />
                  <span>正在执行 WireGuard 隧道与零信任凭证校验...</span>
                </span>
              )}
            </div>

            <div className="bg-slate-950 text-slate-200 p-3 rounded-lg font-mono text-[11px] min-h-[160px] max-h-[220px] overflow-y-auto space-y-1 border border-slate-800">
              {simulationLogs.length === 0 ? (
                <div className="text-slate-500 italic py-6 text-center">
                  点击下方【开始入网握手仿真】执行网关节点接入流程...
                </div>
              ) : (
                simulationLogs.map((log, index) => (
                  <div
                    key={index}
                    className={`leading-relaxed ${
                      log.includes('通过鉴权') || log.includes('激活上线')
                        ? 'text-emerald-400 font-medium'
                        : log.includes('强制执行安全边界')
                        ? 'text-indigo-300'
                        : 'text-slate-300'
                    }`}
                  >
                    {log}
                  </div>
                ))
              )}
            </div>
          </div>

          {/* Result Alert */}
          {simulationResult && (
            <div className="p-3 bg-emerald-50 rounded-lg border border-emerald-200 text-xs text-emerald-900 space-y-1">
              <div className="flex items-center gap-1.5 font-bold text-emerald-800">
                <CheckCircle2 className="w-4 h-4 text-emerald-600" />
                <span>全链路验证通过：节点已激活，凭证已销毁</span>
              </div>
              <p className="text-[11px] text-emerald-700">
                已生成审计操作日志并持久化入库，您可以前往【虚拟节点管理】或【操作审计日志】查看详情。
              </p>
            </div>
          )}

          <div className="flex items-center justify-between pt-3 border-t border-slate-100">
            <Button
              variant="outline"
              onClick={() => setIsSimulateModalOpen(false)}
              disabled={isSimulating}
            >
              关闭
            </Button>
            <div className="flex items-center gap-2">
              {simulationResult && (
                <Button
                  variant="outline"
                  onClick={() => navigate('/nodes')}
                  className="gap-1 text-blue-600 border-blue-200 hover:bg-blue-50"
                >
                  <Server className="w-3.5 h-3.5" />
                  <span>前往虚拟节点列表查看</span>
                </Button>
              )}
              <Button
                onClick={() => handleStartSimulation()}
                isLoading={isSimulating}
                disabled={!simulateTokenId}
                className="gap-1.5 bg-indigo-600 hover:bg-indigo-700 text-white"
              >
                <Play className="w-3.5 h-3.5 fill-current" />
                <span>开始入网握手仿真</span>
              </Button>
            </div>
          </div>
        </div>
      </Dialog>

      {/* ================= MODAL: Revoke Token ================= */}
      {tokenToRevoke && (
        <Dialog
          isOpen={!!tokenToRevoke}
          onClose={() => setTokenToRevoke(null)}
          maxWidth="md"
          title={
            <div className="flex items-center gap-2 text-rose-600">
              <Ban className="w-5 h-5" />
              <span>作废一次性注册凭证</span>
            </div>
          }
          description={`确定要撤回作废凭证 "${tokenToRevoke.id}" 吗？作废后该凭证将彻底失效，网关节点无法再以此入网。`}
        >
          <div className="space-y-4">
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                作废原因说明 <span className="text-rose-500">*</span>
              </label>
              <Input
                value={revokeReason}
                onChange={(e) => setRevokeReason(e.target.value)}
                placeholder="请输入作废原因..."
              />
            </div>

            <div className="flex items-center justify-end gap-2 pt-2 border-t border-slate-100">
              <Button
                variant="outline"
                onClick={() => setTokenToRevoke(null)}
                disabled={revoking}
              >
                取消
              </Button>
              <Button
                onClick={handleRevokeConfirm}
                isLoading={revoking}
                className="bg-rose-600 hover:bg-rose-700 text-white"
              >
                确认作废
              </Button>
            </div>
          </div>
        </Dialog>
      )}
    </div>
  );
};

export default EnrollmentTokens;
