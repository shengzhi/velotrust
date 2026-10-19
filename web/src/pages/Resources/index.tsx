import React, { useState, useEffect } from 'react';
import {
  ShieldCheck,
  Server,
  Database,
  Globe,
  Terminal,
  Monitor,
  Search,
  Plus,
  Activity,
  CheckCircle2,
  AlertTriangle,
  Lock,
  ExternalLink,
  RefreshCw,
  Edit2,
  Trash2,
  Sliders,
  Layers,
  ChevronRight,
  Send,
  Zap,
  Info,
  Copy,
  Cpu,
  Boxes,
  Users,
} from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { mockApi } from '../../data/mock';
import {
  ProtectedResource,
  ResourceCategory,
  ResourceProtocol,
  SensitivityLevel,
  Site,
  Node,
  ApiResource,
  ApiRequestMethod,
  ResourceRiskLevel,
} from '../../types';
import { useAuth } from '../../contexts/AuthContext';
import { useToast } from '../../components/ui/toast';
import { Dialog } from '../../components/ui/dialog';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';

export const Resources: React.FC = () => {
  const { hasPermission } = useAuth();
  const { addToast } = useToast();
  const navigate = useNavigate();

  // Active top tab
  const [activeTab, setActiveTab] = useState<'ztna_assets' | 'api_endpoints' | 'data_scopes'>('ztna_assets');

  // --- ZTNA Business Assets State ---
  const [resources, setResources] = useState<ProtectedResource[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);

  // Filters for ZTNA assets
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [selectedSensitivity, setSelectedSensitivity] = useState<string>('all');
  const [selectedSite, setSelectedSite] = useState<string>('all');

  // Edit / Create ZTNA Asset Modal State
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingResource, setEditingResource] = useState<Partial<ProtectedResource>>({
    name: '',
    code: '',
    category: 'web_portal',
    protocol: 'HTTPS',
    host: '',
    port: 443,
    url: '',
    siteId: '',
    siteName: '',
    gatewayNodeId: '',
    gatewayNodeName: '',
    sensitivity: 'Internal',
    department: '',
    owner: '',
    healthCheckType: 'HTTP_GET',
    tags: [],
    description: '',
  });

  // Probe Simulation State
  const [probingId, setProbingId] = useState<string | null>(null);
  const [probeResult, setProbeResult] = useState<{
    resource: ProtectedResource;
    latencyMs: number;
    details: string;
    statusText: string;
  } | null>(null);

  // --- API Resources State ---
  const [apiResources, setApiResources] = useState<ApiResource[]>([]);
  const [apiSearchQuery, setApiSearchQuery] = useState('');
  const [apiMethodFilter, setApiMethodFilter] = useState<string>('all');
  const [apiRiskFilter, setApiRiskFilter] = useState<string>('all');

  // Edit / Create API Resource Modal State
  const [isApiModalOpen, setIsApiModalOpen] = useState(false);
  const [editingApiResource, setEditingApiResource] = useState<Partial<ApiResource>>({
    name: '',
    code: '',
    type: 'API',
    url: '',
    method: 'GET',
    serviceModule: 'network',
    rateLimit: '1000 req/min',
    authRequired: true,
    riskLevel: 'Medium',
    boundPermissionCode: '',
    status: 'active',
    description: '',
  });

  const loadData = async () => {
    setLoading(true);
    try {
      const [resList, sList, nList, apiList] = await Promise.all([
        mockApi.getProtectedResources(),
        mockApi.getSites(),
        mockApi.getNodes(),
        mockApi.getApiResources(),
      ]);
      setResources(resList);
      setSites(sList);
      setNodes(nList.filter((n) => n.type === 'gateway'));
      setApiResources(apiList);
    } catch {
      addToast('加载资源数据失败', 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  // Filter ZTNA resources
  const filteredResources = resources.filter((res) => {
    const matchesSearch =
      res.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      res.host.toLowerCase().includes(searchQuery.toLowerCase()) ||
      res.code.toLowerCase().includes(searchQuery.toLowerCase()) ||
      res.owner.toLowerCase().includes(searchQuery.toLowerCase()) ||
      res.department.toLowerCase().includes(searchQuery.toLowerCase()) ||
      res.tags.some((t) => t.toLowerCase().includes(searchQuery.toLowerCase()));

    const matchesCategory = selectedCategory === 'all' || res.category === selectedCategory;
    const matchesSensitivity = selectedSensitivity === 'all' || res.sensitivity === selectedSensitivity;
    const matchesSite = selectedSite === 'all' || res.siteId === selectedSite;

    return matchesSearch && matchesCategory && matchesSensitivity && matchesSite;
  });

  // Filter API resources
  const filteredApiResources = apiResources.filter((api) => {
    const matchesSearch =
      !apiSearchQuery ||
      api.name.toLowerCase().includes(apiSearchQuery.toLowerCase()) ||
      api.code.toLowerCase().includes(apiSearchQuery.toLowerCase()) ||
      (api.url || (api as any).path || '').toLowerCase().includes(apiSearchQuery.toLowerCase()) ||
      ((api.boundPermissionCode || (api as any).mappedPermissionCode || '')).toLowerCase().includes(apiSearchQuery.toLowerCase());

    const matchesMethod = apiMethodFilter === 'all' || api.method === apiMethodFilter;
    const matchesRisk = apiRiskFilter === 'all' || api.riskLevel === apiRiskFilter;

    return matchesSearch && matchesMethod && matchesRisk;
  });

  // Helpers
  const getCategoryIcon = (category: ResourceCategory) => {
    switch (category) {
      case 'web_portal':
        return <Globe className="w-4 h-4 text-blue-600" />;
      case 'database':
        return <Database className="w-4 h-4 text-emerald-600" />;
      case 'server_ssh':
        return <Terminal className="w-4 h-4 text-slate-700" />;
      case 'rdp_desktop':
        return <Monitor className="w-4 h-4 text-indigo-600" />;
      case 'api_gateway':
        return <Server className="w-4 h-4 text-amber-600" />;
      case 'cloud_console':
        return <Layers className="w-4 h-4 text-cyan-600" />;
    }
  };

  const getSensitivityBadge = (level: SensitivityLevel) => {
    switch (level) {
      case 'TopSecret':
        return (
          <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-rose-50 text-rose-700 border border-rose-200 flex items-center gap-1">
            <Lock className="w-3 h-3 text-rose-600" />
            绝密 (TopSecret)
          </span>
        );
      case 'Confidential':
        return (
          <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-amber-50 text-amber-700 border border-amber-200 flex items-center gap-1">
            <Lock className="w-3 h-3 text-amber-600" />
            机密 (Confidential)
          </span>
        );
      case 'Internal':
        return (
          <span className="px-2 py-0.5 rounded text-[10px] font-medium bg-blue-50 text-blue-700 border border-blue-200">
            内部通用 (Internal)
          </span>
        );
      case 'Public':
        return (
          <span className="px-2 py-0.5 rounded text-[10px] font-medium bg-slate-50 text-slate-700 border border-slate-200">
            公开 (Public)
          </span>
        );
    }
  };

  // Handlers for ZTNA Assets
  const handleOpenCreate = () => {
    const defaultSite = sites[0] || { id: 'site-bj', name: '北京总部核心区' };
    const defaultGateway = nodes[0] || { id: 'node-hub-bj', name: 'gw-bj-hub-01' };

    setEditingResource({
      name: '',
      code: 'SVC-' + Math.floor(1000 + Math.random() * 9000),
      category: 'web_portal',
      protocol: 'HTTPS',
      host: '',
      port: 443,
      url: '',
      siteId: defaultSite.id,
      siteName: defaultSite.name,
      gatewayNodeId: defaultGateway.id,
      gatewayNodeName: defaultGateway.name,
      sensitivity: 'Confidential',
      department: '基础平台架构组',
      owner: '管理员',
      healthCheckType: 'HTTP_GET',
      tags: ['生产就绪', 'ZTNA受保护'],
      description: '',
    });
    setIsModalOpen(true);
  };

  const handleOpenEdit = (res: ProtectedResource) => {
    setEditingResource({ ...res, tags: [...res.tags] });
    setIsModalOpen(true);
  };

  const handleSaveResource = async () => {
    if (!editingResource.name || !editingResource.host) {
      addToast('资产名称和内部主机地址(Host/IP)为必填项', 'warning');
      return;
    }

    try {
      const saved = await mockApi.saveProtectedResource(editingResource as any);
      addToast(`受保护业务资产「${saved.name}」已成功保存并同步至网关`, 'success');
      setIsModalOpen(false);
      loadData();
    } catch (err: any) {
      addToast(err.message || '保存业务资产失败', 'error');
    }
  };

  const handleDeleteResource = async (id: string, name: string) => {
    if (confirm(`确定要从受保护业务目录中移除「${name}」吗？`)) {
      try {
        await mockApi.deleteProtectedResource(id);
        addToast(`业务资产「${name}」已成功移除`, 'success');
        loadData();
      } catch (err: any) {
        addToast(err.message || '删除失败', 'error');
      }
    }
  };

  const handleProbeHealth = async (res: ProtectedResource) => {
    setProbingId(res.id);
    try {
      const result = await mockApi.probeResourceHealth(res.id);
      setProbeResult({
        resource: res,
        latencyMs: result.latencyMs,
        details: result.details,
        statusText: result.statusText,
      });
      loadData();
    } catch {
      addToast('连通性健康探测执行异常', 'error');
    } finally {
      setProbingId(null);
    }
  };

  // Handlers for API Resources
  const handleOpenCreateApi = () => {
    setEditingApiResource({
      name: '',
      code: 'API-' + Math.floor(1000 + Math.random() * 9000),
      type: 'API',
      url: '/api/v1/services/custom',
      method: 'GET',
      serviceModule: 'network',
      rateLimit: '1000 req/min',
      authRequired: true,
      riskLevel: 'Medium',
      boundPermissionCode: 'menu:settings',
      status: 'active',
      description: '',
    });
    setIsApiModalOpen(true);
  };

  const handleOpenEditApi = (api: ApiResource) => {
    setEditingApiResource({ ...api });
    setIsApiModalOpen(true);
  };

  const handleSaveApiResource = async () => {
    if (!editingApiResource.name?.trim() || !editingApiResource.url?.trim()) {
      addToast('请完整填写接口名称与 URL 路径', 'warning');
      return;
    }
    try {
      const saved = await mockApi.saveApiResource(editingApiResource);
      addToast(`API 资源 [${saved.name}] 已保存`, 'success');
      setIsApiModalOpen(false);
      loadData();
    } catch {
      addToast('保存 API 资源失败', 'error');
    }
  };

  const handleDeleteApiResource = async (id: string, name: string) => {
    if (!confirm(`确定要删除 API 资源「${name}」吗？`)) return;
    try {
      await mockApi.deleteApiResource(id);
      addToast(`API 资源「${name}」已删除`, 'success');
      loadData();
    } catch {
      addToast('删除失败', 'error');
    }
  };

  // Metrics computation for ZTNA assets
  const totalCount = resources.length;
  const topSecretCount = resources.filter((r) => r.sensitivity === 'TopSecret').length;
  const avgLatency =
    totalCount > 0
      ? Math.round(resources.reduce((sum, r) => sum + r.latencyMs, 0) / totalCount)
      : 0;
  const distinctGatewaysCount = new Set(resources.map((r) => r.gatewayNodeId)).size;

  return (
    <div className="space-y-4 max-w-7xl mx-auto">
      {/* Clean Header - No Top Summary Banner / No KPI Stats Cards */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3 pb-2 border-b border-slate-200">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl font-bold text-slate-900 tracking-tight">企业资源管理中心</h1>
            <Badge variant="outline" className="text-blue-700 bg-blue-50 border-blue-200 text-xs">
              资源访问控制目录
            </Badge>
            <span className="text-xs text-slate-400">（与菜单访问权限解耦独立设计）</span>
          </div>
          <p className="text-xs text-slate-500 mt-0.5">
            统一维护零信任受保护业务资产、后端系统 API 接口资源及数据隔离边界
          </p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <button
            onClick={() => loadData()}
            className="p-2 text-slate-500 hover:text-slate-800 hover:bg-slate-100 rounded-lg transition-colors border border-slate-200"
            title="刷新数据"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
          {activeTab === 'ztna_assets' && (
            <button
              onClick={handleOpenCreate}
              className="flex items-center gap-1.5 px-3.5 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-semibold shadow-xs transition-colors"
            >
              <Plus className="w-3.5 h-3.5" />
              <span>注册新业务资源</span>
            </button>
          )}
          {activeTab === 'api_endpoints' && (
            <button
              onClick={handleOpenCreateApi}
              className="flex items-center gap-1.5 px-3.5 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg text-xs font-semibold shadow-xs transition-colors"
            >
              <Plus className="w-3.5 h-3.5" />
              <span>注册新 API 接口</span>
            </button>
          )}
        </div>
      </div>

      {/* Tab switch buttons */}
      <div className="flex items-center gap-2 border-b border-slate-200">
        <button
          onClick={() => setActiveTab('ztna_assets')}
          className={`flex items-center gap-2 px-4 py-2 text-xs font-bold border-b-2 transition-all ${
            activeTab === 'ztna_assets'
              ? 'border-blue-600 text-blue-600 bg-blue-50/40 rounded-t-lg'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          }`}
        >
          <ShieldCheck className="h-3.5 w-3.5" />
          <span>受保护业务网络资产 ({resources.length})</span>
        </button>
        <button
          onClick={() => setActiveTab('api_endpoints')}
          className={`flex items-center gap-2 px-4 py-2 text-xs font-bold border-b-2 transition-all ${
            activeTab === 'api_endpoints'
              ? 'border-purple-600 text-purple-600 bg-purple-50/40 rounded-t-lg'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          }`}
        >
          <Cpu className="h-3.5 w-3.5" />
          <span>系统与 API 接口资源 ({apiResources.length})</span>
        </button>
        <button
          onClick={() => setActiveTab('data_scopes')}
          className={`flex items-center gap-2 px-4 py-2 text-xs font-bold border-b-2 transition-all ${
            activeTab === 'data_scopes'
              ? 'border-emerald-600 text-emerald-600 bg-emerald-50/40 rounded-t-lg'
              : 'border-transparent text-slate-500 hover:text-slate-800'
          }`}
        >
          <Database className="h-3.5 w-3.5" />
          <span>数据维度与隔离规则</span>
        </button>
      </div>

      {/* ================= TAB 1: ZTNA PROTECTED ASSETS ================= */}
      {activeTab === 'ztna_assets' && (
        <div className="space-y-4">

          {/* Filter and Search Bar */}
          <div className="bg-white rounded-2xl border border-slate-200/80 p-4 shadow-2xs space-y-3">
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
              <div className="relative flex-1">
                <Search className="w-3.5 h-3.5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
                <input
                  type="text"
                  placeholder="搜索资产名称、代码、内网域名/IP、负责人、标签..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="w-full pl-9 pr-4 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-800 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                />
              </div>

              {/* Site selector */}
              <div className="flex items-center gap-2">
                <span className="text-xs text-slate-400 shrink-0">网络站点:</span>
                <select
                  value={selectedSite}
                  onChange={(e) => setSelectedSite(e.target.value)}
                  className="py-1.5 px-3 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-700 focus:outline-none"
                >
                  <option value="all">全部网络站点 ({resources.length})</option>
                  {sites.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} ({s.localSubnet})
                    </option>
                  ))}
                </select>
              </div>
            </div>

            {/* Category tags */}
            <div className="flex flex-wrap items-center gap-1.5 pt-1">
              <span className="text-[11px] text-slate-400 mr-1">分类类型:</span>
              {[
                { id: 'all', label: '全部资产' },
                { id: 'web_portal', label: 'Web 应用门户' },
                { id: 'database', label: '数据库服务' },
                { id: 'server_ssh', label: 'SSH 远程主机' },
                { id: 'rdp_desktop', label: 'Windows RDP' },
                { id: 'api_gateway', label: '内部 API 网关' },
                { id: 'cloud_console', label: '多云管理底座' },
              ].map((c) => (
                <button
                  key={c.id}
                  onClick={() => setSelectedCategory(c.id)}
                  className={`px-2.5 py-1 rounded-lg text-xs font-medium transition-colors ${
                    selectedCategory === c.id
                      ? 'bg-blue-600 text-white font-semibold shadow-xs'
                      : 'bg-slate-50 text-slate-600 hover:bg-slate-100 hover:text-slate-900 border border-slate-200/60'
                  }`}
                >
                  {c.label}
                </button>
              ))}

              <div className="ml-auto flex items-center gap-1.5">
                <span className="text-[11px] text-slate-400">敏感级别:</span>
                {(['all', 'TopSecret', 'Confidential', 'Internal', 'Public'] as const).map((lvl) => (
                  <button
                    key={lvl}
                    onClick={() => setSelectedSensitivity(lvl)}
                    className={`px-2 py-0.5 rounded text-[10px] font-medium transition-colors ${
                      selectedSensitivity === lvl
                        ? 'bg-slate-900 text-white font-semibold'
                        : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                    }`}
                  >
                    {lvl === 'all'
                      ? '全部'
                      : lvl === 'TopSecret'
                      ? '绝密'
                      : lvl === 'Confidential'
                      ? '机密'
                      : lvl === 'Internal'
                      ? '内部'
                      : '公开'}
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/* Cards Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {filteredResources.map((res) => {
              const isProbing = probingId === res.id;
              return (
                <div
                  key={res.id}
                  className="bg-white rounded-2xl border border-slate-200/80 p-5 shadow-2xs hover:shadow-md transition-all flex flex-col justify-between group"
                >
                  <div className="space-y-3">
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex items-center gap-2.5">
                        <div className="p-2.5 bg-slate-50 border border-slate-100 rounded-xl">
                          {getCategoryIcon(res.category)}
                        </div>
                        <div>
                          <h3 className="text-sm font-bold text-slate-900 tracking-tight group-hover:text-blue-600 transition-colors">
                            {res.name}
                          </h3>
                          <div className="flex items-center gap-1.5 mt-0.5">
                            <span className="font-mono text-[10px] font-bold text-slate-500 bg-slate-100 px-1.5 py-0.2 rounded">
                              {res.code}
                            </span>
                            <span className="text-[10px] text-slate-400">·</span>
                            <span className="text-[10px] text-slate-500">{res.department}</span>
                          </div>
                        </div>
                      </div>
                      {getSensitivityBadge(res.sensitivity)}
                    </div>

                    <p className="text-xs text-slate-500 line-clamp-2 leading-relaxed">{res.description}</p>

                    <div className="p-3 bg-slate-50/70 border border-slate-100 rounded-xl space-y-1.5 text-xs">
                      <div className="flex items-center justify-between">
                        <span className="text-slate-400">内部寻址 (Host:Port):</span>
                        <span className="font-mono font-medium text-slate-800">
                          {res.protocol}://{res.host}:{res.port}
                        </span>
                      </div>
                      <div className="flex items-center justify-between">
                        <span className="text-slate-400">归属网络站点:</span>
                        <span className="text-slate-700 truncate max-w-[180px]">{res.siteName}</span>
                      </div>
                      <div className="flex items-center justify-between">
                        <span className="text-slate-400">负责出向网关节点:</span>
                        <span className="font-mono text-blue-600 font-semibold">{res.gatewayNodeName}</span>
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center gap-1.5 pt-1">
                      {res.tags.map((tag, tIdx) => (
                        <span
                          key={tIdx}
                          className="px-2 py-0.5 rounded-md text-[10px] font-medium bg-slate-100 text-slate-600"
                        >
                          #{tag}
                        </span>
                      ))}
                    </div>
                  </div>

                  <div className="pt-4 mt-4 border-t border-slate-100 space-y-3">
                    <div className="flex items-center justify-between text-xs">
                      <div className="flex items-center gap-1.5">
                        <span
                          className={`w-2 h-2 rounded-full ${
                            res.status === 'healthy'
                              ? 'bg-emerald-500'
                              : res.status === 'degraded'
                              ? 'bg-amber-500'
                              : 'bg-rose-500'
                          }`}
                        />
                        <span className="text-slate-600 font-medium">
                          {res.status === 'healthy' ? '连通健康' : res.status === 'degraded' ? '告警延迟' : '不可达'}
                        </span>
                        <span className="text-slate-400 font-mono text-[11px]">({res.latencyMs}ms)</span>
                      </div>

                      <span className="text-[11px] text-blue-600 font-medium bg-blue-50 px-2 py-0.5 rounded">
                        {res.associatedPoliciesCount} 条关联 ACL 策略
                      </span>
                    </div>

                    <div className="flex items-center justify-between gap-2">
                      <button
                        onClick={() => handleProbeHealth(res)}
                        disabled={isProbing}
                        className="flex-1 py-1.5 px-2 bg-slate-100 hover:bg-slate-200 text-slate-700 rounded-lg text-xs font-semibold transition-colors flex items-center justify-center gap-1.5 disabled:opacity-50"
                      >
                        <Activity className={`w-3.5 h-3.5 ${isProbing ? 'animate-spin' : ''}`} />
                        <span>{isProbing ? '探测中...' : '网关探针测试'}</span>
                      </button>

                      <button
                        onClick={() => navigate('/policies')}
                        title="配置与查看该资产的访问策略"
                        className="p-1.5 text-slate-500 hover:text-blue-600 hover:bg-slate-50 rounded-lg border border-slate-200 transition-colors"
                      >
                        <Sliders className="w-3.5 h-3.5" />
                      </button>

                      <button
                        onClick={() => handleOpenEdit(res)}
                        title="编辑业务资产"
                        className="p-1.5 text-slate-500 hover:text-blue-600 hover:bg-slate-50 rounded-lg border border-slate-200 transition-colors"
                      >
                        <Edit2 className="w-3.5 h-3.5" />
                      </button>

                      <button
                        onClick={() => handleDeleteResource(res.id, res.name)}
                        title="移除资产"
                        className="p-1.5 text-slate-500 hover:text-rose-600 hover:bg-rose-50 rounded-lg border border-slate-200 transition-colors"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* ================= TAB 2: SYSTEM API & SERVICE ENDPOINTS ================= */}
      {activeTab === 'api_endpoints' && (
        <div className="space-y-4">

          {/* Filter Bar */}
          <div className="flex flex-col sm:flex-row gap-3 items-center justify-between bg-slate-900/60 p-3.5 border border-slate-800 rounded-xl">
            <div className="flex flex-1 items-center gap-3 w-full sm:w-auto">
              <div className="relative flex-1 sm:w-80">
                <Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
                <Input
                  value={apiSearchQuery}
                  onChange={(e) => setApiSearchQuery(e.target.value)}
                  placeholder="搜索接口名称、代码、URL路径或权限码..."
                  className="pl-9 bg-slate-950 border-slate-800 text-xs h-9 text-slate-200"
                />
              </div>

              <div className="flex items-center gap-1.5 bg-slate-950 p-1 border border-slate-800 rounded-lg">
                {(['all', 'GET', 'POST', 'PUT', 'DELETE'] as const).map((m) => (
                  <button
                    key={m}
                    onClick={() => setApiMethodFilter(m)}
                    className={`px-2.5 py-1 text-xs rounded font-medium transition-colors ${
                      apiMethodFilter === m
                        ? 'bg-purple-600 text-white font-semibold'
                        : 'text-slate-400 hover:text-slate-200'
                    }`}
                  >
                    {m === 'all' ? '全部方法' : m}
                  </button>
                ))}
              </div>
            </div>

            <div className="flex items-center gap-2 self-end sm:self-auto">
              <span className="text-xs text-slate-400">风险等级：</span>
              <select
                value={apiRiskFilter}
                onChange={(e) => setApiRiskFilter(e.target.value)}
                className="bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1 text-xs text-slate-300 focus:outline-none"
              >
                <option value="all">全部等级</option>
                <option value="Critical">极高危 (Critical)</option>
                <option value="High">高敏感 (High)</option>
                <option value="Medium">中敏感 (Medium)</option>
                <option value="Low">低敏感 (Low)</option>
              </select>
            </div>
          </div>

          {/* Table */}
          <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
            <div className="overflow-x-auto">
              <table className="w-full text-left border-collapse text-xs">
                <thead>
                  <tr className="border-b border-slate-800 bg-slate-950/60 text-slate-400 font-semibold uppercase tracking-wider">
                    <th className="py-3 px-4 min-w-[200px]">接口资源标识</th>
                    <th className="py-3 px-3">请求方式</th>
                    <th className="py-3 px-3 min-w-[220px]">URL 路由路径</th>
                    <th className="py-3 px-3">所属服务模块</th>
                    <th className="py-3 px-3">绑定权限代码</th>
                    <th className="py-3 px-3">限速策略</th>
                    <th className="py-3 px-3 text-center">风险等级</th>
                    <th className="py-3 px-4 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 font-medium">
                  {filteredApiResources.length === 0 ? (
                    <tr>
                      <td colSpan={8} className="py-12 text-center text-slate-400">
                        未检索到符合条件的系统 API 资源
                      </td>
                    </tr>
                  ) : (
                    filteredApiResources.map((api) => (
                      <tr key={api.id} className="hover:bg-slate-800/40 transition-colors group">
                        <td className="py-3 px-4">
                          <div>
                            <span className="font-semibold text-slate-200">{api.name}</span>
                            <span className="block font-mono text-[10px] text-slate-400">{api.code}</span>
                          </div>
                        </td>

                        <td className="py-3 px-3">
                          <span
                            className={`font-mono font-bold text-[10px] px-2 py-0.5 rounded border ${
                              api.method === 'GET'
                                ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                                : api.method === 'POST'
                                ? 'bg-blue-500/10 text-blue-400 border-blue-500/20'
                                : api.method === 'PUT'
                                ? 'bg-amber-500/10 text-amber-400 border-amber-500/20'
                                : 'bg-rose-500/10 text-rose-400 border-rose-500/20'
                            }`}
                          >
                            {api.method}
                          </span>
                        </td>

                        <td className="py-3 px-3">
                          <span className="font-mono text-slate-300 bg-slate-950 px-2 py-0.5 rounded border border-slate-800">
                            {api.url}
                          </span>
                        </td>

                        <td className="py-3 px-3 text-slate-300">
                          {api.serviceModule === 'network'
                            ? '网络互联'
                            : api.serviceModule === 'policy'
                            ? '安全策略'
                            : api.serviceModule === 'identity'
                            ? '身份体系'
                            : api.serviceModule === 'audit'
                            ? '审计追溯'
                            : '系统底座'}
                        </td>

                        <td className="py-3 px-3">
                          <span className="font-mono text-[11px] text-purple-400 bg-purple-500/10 px-2 py-0.5 rounded border border-purple-500/20">
                            {api.boundPermissionCode}
                          </span>
                        </td>

                        <td className="py-3 px-3 text-slate-400 font-mono text-[11px]">
                          {api.rateLimit}
                        </td>

                        <td className="py-3 px-3 text-center">
                          <span
                            className={`px-2 py-0.5 rounded text-[10px] font-bold border ${
                              api.riskLevel === 'Critical'
                                ? 'bg-red-500/20 text-red-400 border-red-500/30'
                                : api.riskLevel === 'High'
                                ? 'bg-rose-500/10 text-rose-400 border-rose-500/20'
                                : api.riskLevel === 'Medium'
                                ? 'bg-amber-500/10 text-amber-400 border-amber-500/20'
                                : 'bg-slate-800 text-slate-300 border-slate-700'
                            }`}
                          >
                            {api.riskLevel}
                          </span>
                        </td>

                        <td className="py-3 px-4 text-right">
                          <div className="flex items-center justify-end gap-1.5 opacity-90 group-hover:opacity-100">
                            <button
                              onClick={() => handleOpenEditApi(api)}
                              title="编辑接口"
                              className="p-1.5 rounded hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition-colors"
                            >
                              <Edit2 className="h-3.5 w-3.5" />
                            </button>
                            <button
                              onClick={() => handleDeleteApiResource(api.id, api.name)}
                              title="删除接口"
                              className="p-1.5 rounded hover:bg-rose-500/20 text-rose-400 hover:text-rose-300 transition-colors"
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* ================= TAB 3: DATA SCOPE & ISOLATION ================= */}
      {activeTab === 'data_scopes' && (
        <div className="space-y-6">
          <div className="bg-slate-900 border border-slate-800 p-5 rounded-2xl">
            <h3 className="text-lg font-bold text-white tracking-tight">数据维度与多租户隔离资源规则</h3>
            <p className="text-xs text-slate-400 mt-1">
              定义不同角色在检索审计、节点与策略时的数据可见性范围，严守数据安全边界
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="bg-slate-900 border border-slate-800 p-4 rounded-xl space-y-3">
              <div className="flex items-center justify-between">
                <Badge variant="outline" className="bg-emerald-500/10 text-emerald-400 border-emerald-500/30">
                  全量租户数据 (all)
                </Badge>
                <Lock className="h-4 w-4 text-emerald-400" />
              </div>
              <h4 className="font-bold text-slate-200 text-sm">全局无限制数据透视</h4>
              <p className="text-xs text-slate-400">
                穿透所有多云 VPC、私有 IDC 及跨域网段。仅限「超级管理员」及授权特权身份。
              </p>
              <div className="pt-2 border-t border-slate-800 text-[11px] text-slate-400 font-mono">
                适用范围: 全局 100% 资产与日志
              </div>
            </div>

            <div className="bg-slate-900 border border-slate-800 p-4 rounded-xl space-y-3">
              <div className="flex items-center justify-between">
                <Badge variant="outline" className="bg-blue-500/10 text-blue-400 border-blue-500/30">
                  部门及下属数据 (dept)
                </Badge>
                <Users className="h-4 w-4 text-blue-400" />
              </div>
              <h4 className="font-bold text-slate-200 text-sm">部门组织边界隔离</h4>
              <p className="text-xs text-slate-400">
                通过 IdP 自动获取用户所属部门与下级组织，仅返回归属于本部门资产的配置与操作流水。
              </p>
              <div className="pt-2 border-t border-slate-800 text-[11px] text-slate-400 font-mono">
                适用角色: 安全保密员、部门管理员
              </div>
            </div>

            <div className="bg-slate-900 border border-slate-800 p-4 rounded-xl space-y-3">
              <div className="flex items-center justify-between">
                <Badge variant="outline" className="bg-amber-500/10 text-amber-400 border-amber-500/30">
                  本人自建数据 (self)
                </Badge>
                <Monitor className="h-4 w-4 text-amber-400" />
              </div>
              <h4 className="font-bold text-slate-200 text-sm">最小特权终端访问</h4>
              <p className="text-xs text-slate-400">
                普通研发人员仅能查看本人名下接入的 PC/手机设备状态与拥有明确授权的业务系统目录。
              </p>
              <div className="pt-2 border-t border-slate-800 text-[11px] text-slate-400 font-mono">
                适用角色: 普通员工 (Employee)
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ================= MODAL: Create / Edit Protected Resource ================= */}
      {isModalOpen && (
        <Dialog
          isOpen={isModalOpen}
          onClose={() => setIsModalOpen(false)}
          title={`注册/配置受保护业务资产 · ${editingResource.name || '新建资产'}`}
          maxWidth="2xl"
        >
          <div className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">
                  业务资产名称 <span className="text-rose-500">*</span>
                </label>
                <input
                  type="text"
                  placeholder="例如：财务核心 SAP S/4HANA"
                  value={editingResource.name || ''}
                  onChange={(e) => setEditingResource({ ...editingResource, name: e.target.value })}
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-800 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">唯一资产编码 (Asset Code)</label>
                <input
                  type="text"
                  placeholder="例如：FIN-SAP-PROD"
                  value={editingResource.code || ''}
                  onChange={(e) => setEditingResource({ ...editingResource, code: e.target.value.toUpperCase() })}
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-mono text-slate-800 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                />
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">分类类型</label>
                <select
                  value={editingResource.category}
                  onChange={(e) =>
                    setEditingResource({ ...editingResource, category: e.target.value as ResourceCategory })
                  }
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-700 focus:outline-none"
                >
                  <option value="web_portal">Web 应用门户</option>
                  <option value="database">数据库服务</option>
                  <option value="server_ssh">SSH 远程终端</option>
                  <option value="rdp_desktop">Windows RDP 桌面</option>
                  <option value="api_gateway">内部 API 网关</option>
                  <option value="cloud_console">多云管理底座</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">传输协议</label>
                <select
                  value={editingResource.protocol}
                  onChange={(e) =>
                    setEditingResource({ ...editingResource, protocol: e.target.value as ResourceProtocol })
                  }
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-700 focus:outline-none"
                >
                  <option value="HTTPS">HTTPS (443)</option>
                  <option value="HTTP">HTTP (80)</option>
                  <option value="SSH">SSH (22)</option>
                  <option value="RDP">RDP (3389)</option>
                  <option value="MySQL">MySQL (3306)</option>
                  <option value="PostgreSQL">PostgreSQL (5432)</option>
                  <option value="TCP">通用 TCP</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">敏感等级</label>
                <select
                  value={editingResource.sensitivity}
                  onChange={(e) =>
                    setEditingResource({ ...editingResource, sensitivity: e.target.value as SensitivityLevel })
                  }
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-700 focus:outline-none"
                >
                  <option value="TopSecret">绝密 (TopSecret)</option>
                  <option value="Confidential">机密 (Confidential)</option>
                  <option value="Internal">内部通用 (Internal)</option>
                  <option value="Public">公开 (Public)</option>
                </select>
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div className="sm:col-span-2">
                <label className="block text-xs font-semibold text-slate-700 mb-1">
                  内部目标主机 (Host / IP) <span className="text-rose-500">*</span>
                </label>
                <input
                  type="text"
                  placeholder="例如：10.100.2.18 或 sap-core.vpc.internal"
                  value={editingResource.host || ''}
                  onChange={(e) => setEditingResource({ ...editingResource, host: e.target.value })}
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-mono text-slate-800 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">端口</label>
                <input
                  type="number"
                  placeholder="443"
                  value={editingResource.port || 443}
                  onChange={(e) => setEditingResource({ ...editingResource, port: parseInt(e.target.value, 10) || 80 })}
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-mono text-slate-800 focus:outline-none"
                />
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">归属网络站点</label>
                <select
                  value={editingResource.siteId}
                  onChange={(e) => {
                    const st = sites.find((s) => s.id === e.target.value);
                    setEditingResource({
                      ...editingResource,
                      siteId: e.target.value,
                      siteName: st ? st.name : '',
                    });
                  }}
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-700 focus:outline-none"
                >
                  {sites.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} ({s.localSubnet})
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">出向代理网关 (Gateway)</label>
                <select
                  value={editingResource.gatewayNodeId}
                  onChange={(e) => {
                    const nd = nodes.find((n) => n.id === e.target.value);
                    setEditingResource({
                      ...editingResource,
                      gatewayNodeId: e.target.value,
                      gatewayNodeName: nd ? nd.name : '',
                    });
                  }}
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-700 focus:outline-none"
                >
                  {nodes.map((n) => (
                    <option key={n.id} value={n.id}>
                      {n.name} ({n.virtualIp})
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">业务归属部门</label>
                <input
                  type="text"
                  placeholder="例如：财务中心与合规部"
                  value={editingResource.department || ''}
                  onChange={(e) => setEditingResource({ ...editingResource, department: e.target.value })}
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-800 focus:outline-none"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">资产技术负责人</label>
                <input
                  type="text"
                  placeholder="例如：张伟 (基础运维组)"
                  value={editingResource.owner || ''}
                  onChange={(e) => setEditingResource({ ...editingResource, owner: e.target.value })}
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-800 focus:outline-none"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1">资产功能描述与用途</label>
              <textarea
                placeholder="详细说明该受保护业务资源的服务内容..."
                value={editingResource.description || ''}
                onChange={(e) => setEditingResource({ ...editingResource, description: e.target.value })}
                rows={2}
                className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-800 focus:outline-none"
              />
            </div>

            <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-100">
              <button
                type="button"
                onClick={() => setIsModalOpen(false)}
                className="px-4 py-2 border border-slate-200 text-slate-700 hover:bg-slate-50 rounded-xl text-xs font-medium"
              >
                取消
              </button>
              <button
                type="button"
                onClick={handleSaveResource}
                className="px-5 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-xl text-xs font-semibold shadow-md shadow-blue-600/20"
              >
                保存并生效业务资产
              </button>
            </div>
          </div>
        </Dialog>
      )}

      {/* ================= MODAL: Create / Edit API Resource ================= */}
      {isApiModalOpen && (
        <Dialog
          isOpen={isApiModalOpen}
          onClose={() => setIsApiModalOpen(false)}
          title={editingApiResource.id ? `编辑 API 接口 · ${editingApiResource.name}` : '注册新 API / 系统服务接口'}
          maxWidth="2xl"
        >
          <div className="space-y-4 text-xs">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-slate-400 font-semibold mb-1">接口资源名称 *</label>
                <Input
                  value={editingApiResource.name || ''}
                  onChange={(e) => setEditingApiResource({ ...editingApiResource, name: e.target.value })}
                  placeholder="例如：网关配置下发 RPC"
                  className="bg-slate-950 border-slate-800 text-slate-200 text-xs"
                />
              </div>
              <div>
                <label className="block text-slate-400 font-semibold mb-1">接口资源唯一代码 *</label>
                <Input
                  value={editingApiResource.code || ''}
                  onChange={(e) => setEditingApiResource({ ...editingApiResource, code: e.target.value })}
                  placeholder="例如：API-GW-PEER-SYNC"
                  className="bg-slate-950 border-slate-800 text-slate-200 text-xs font-mono"
                />
              </div>
            </div>

            <div className="grid grid-cols-3 gap-4">
              <div>
                <label className="block text-slate-400 font-semibold mb-1">请求动作</label>
                <select
                  value={editingApiResource.method || 'GET'}
                  onChange={(e) =>
                    setEditingApiResource({ ...editingApiResource, method: e.target.value as ApiRequestMethod })
                  }
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2 text-slate-200 focus:outline-none"
                >
                  <option value="GET">GET (只读查询)</option>
                  <option value="POST">POST (提交/下发)</option>
                  <option value="PUT">PUT (更新)</option>
                  <option value="DELETE">DELETE (注销)</option>
                  <option value="*">* (所有动作)</option>
                </select>
              </div>

              <div>
                <label className="block text-slate-400 font-semibold mb-1">所属服务模块</label>
                <select
                  value={editingApiResource.serviceModule || 'network'}
                  onChange={(e) => setEditingApiResource({ ...editingApiResource, serviceModule: e.target.value as any })}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2 text-slate-200 focus:outline-none"
                >
                  <option value="network">网络互联</option>
                  <option value="policy">零信任策略</option>
                  <option value="identity">身份权限</option>
                  <option value="audit">合规审计</option>
                  <option value="system">系统灾备</option>
                </select>
              </div>

              <div>
                <label className="block text-slate-400 font-semibold mb-1">风险敏感度</label>
                <select
                  value={editingApiResource.riskLevel || 'Medium'}
                  onChange={(e) =>
                    setEditingApiResource({ ...editingApiResource, riskLevel: e.target.value as ResourceRiskLevel })
                  }
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2 text-slate-200 focus:outline-none"
                >
                  <option value="Critical">极高危 (Critical)</option>
                  <option value="High">高敏感 (High)</option>
                  <option value="Medium">中敏感 (Medium)</option>
                  <option value="Low">低敏感 (Low)</option>
                </select>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-slate-400 font-semibold mb-1">URL 路由路径 *</label>
                <Input
                  value={editingApiResource.url || ''}
                  onChange={(e) => setEditingApiResource({ ...editingApiResource, url: e.target.value })}
                  placeholder="例如：/api/v1/gateways/:id/sync"
                  className="bg-slate-950 border-slate-800 text-slate-200 text-xs font-mono"
                />
              </div>

              <div>
                <label className="block text-slate-400 font-semibold mb-1">绑定权限码 (Permission Code)</label>
                <Input
                  value={editingApiResource.boundPermissionCode || ''}
                  onChange={(e) => setEditingApiResource({ ...editingApiResource, boundPermissionCode: e.target.value })}
                  placeholder="例如：action:node:create"
                  className="bg-slate-950 border-slate-800 text-slate-200 text-xs font-mono"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-slate-400 font-semibold mb-1">限速频率 (Rate Limit)</label>
                <Input
                  value={editingApiResource.rateLimit || ''}
                  onChange={(e) => setEditingApiResource({ ...editingApiResource, rateLimit: e.target.value })}
                  placeholder="例如：1000 req/min"
                  className="bg-slate-950 border-slate-800 text-slate-200 text-xs font-mono"
                />
              </div>

              <div className="flex items-center gap-2 pt-6">
                <label className="flex items-center gap-2 cursor-pointer text-slate-300">
                  <input
                    type="checkbox"
                    checked={editingApiResource.authRequired !== false}
                    onChange={(e) => setEditingApiResource({ ...editingApiResource, authRequired: e.target.checked })}
                    className="rounded bg-slate-950 border-slate-800 text-purple-600 focus:ring-0"
                  />
                  <span>强制需要 JWT 认证令牌</span>
                </label>
              </div>
            </div>

            <div>
              <label className="block text-slate-400 font-semibold mb-1">接口作用描述</label>
              <textarea
                value={editingApiResource.description || ''}
                onChange={(e) => setEditingApiResource({ ...editingApiResource, description: e.target.value })}
                rows={2}
                placeholder="详细说明此接口调用的数据范围与业务逻辑..."
                className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2 text-slate-200 focus:outline-none"
              />
            </div>

            <div className="flex justify-end gap-2.5 pt-3 border-t border-slate-800">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setIsApiModalOpen(false)}
                className="border-slate-700 bg-slate-800 text-slate-200"
              >
                取消
              </Button>
              <Button
                size="sm"
                onClick={handleSaveApiResource}
                className="bg-purple-600 hover:bg-purple-500 text-white shadow-md shadow-purple-600/20"
              >
                保存 API 接口
              </Button>
            </div>
          </div>
        </Dialog>
      )}

      {/* ================= MODAL: Health Probe Inspection Result ================= */}
      {probeResult && (
        <Dialog
          isOpen={!!probeResult}
          onClose={() => setProbeResult(null)}
          title={`网关实时探针回执 · ${probeResult.resource.name}`}
          maxWidth="md"
        >
          <div className="space-y-4">
            <div className="p-4 rounded-xl bg-slate-50 border border-slate-100 space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs text-slate-500 font-medium">目标协议与地址:</span>
                <span className="font-mono text-xs font-bold text-slate-800">
                  {probeResult.resource.protocol}://{probeResult.resource.host}:{probeResult.resource.port}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-xs text-slate-500 font-medium">执行探测网关:</span>
                <span className="font-mono text-xs text-blue-600 font-semibold">
                  {probeResult.resource.gatewayNodeName}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-xs text-slate-500 font-medium">探针响应往返时延:</span>
                <span className="font-mono text-sm font-bold text-emerald-600">{probeResult.latencyMs} ms</span>
              </div>
            </div>

            <div className="p-3.5 rounded-xl bg-slate-900 text-slate-300 font-mono text-xs leading-relaxed space-y-1">
              <div className="text-slate-400 text-[11px] mb-1">--- Mesh Enforcer Probe Log ---</div>
              <div>{probeResult.details}</div>
            </div>

            <div className="flex justify-end pt-2">
              <button
                type="button"
                onClick={() => setProbeResult(null)}
                className="px-4 py-2 bg-slate-100 hover:bg-slate-200 text-slate-700 rounded-lg text-xs font-semibold"
              >
                关闭报告
              </button>
            </div>
          </div>
        </Dialog>
      )}
    </div>
  );
};
