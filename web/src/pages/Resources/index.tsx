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
  Lock,
  RefreshCw,
  Edit2,
  Trash2,
  Sliders,
  Layers,
  Cpu,
  Users,
} from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { mockApi } from '../../data/mock';
import { protectedResourceApi } from '../../services/protectedResource';
import { siteApi } from '../../services/site';
import {
  ProtectedResource,
  ResourceCategory,
  ResourceProtocol,
  Site,
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

interface IPv4Range {
  first: number;
  last: number;
}

function parseIPv4Range(value: string): IPv4Range | null {
  const parts = value.trim().split('/');
  if (parts.length > 2 || !parts[0]) return null;
  const octetParts = parts[0].split('.');
  if (octetParts.length !== 4 || octetParts.some((part) => !/^\d{1,3}$/.test(part) || Number(part) > 255 || String(Number(part)) !== part)) return null;
  const octets = octetParts.map(Number);
  const address = octets.reduce((result, octet) => result * 256 + octet, 0);
  const bits = parts.length === 1 ? 32 : Number(parts[1]);
  if (!Number.isInteger(bits) || bits < 0 || bits > 32 || (parts.length === 2 && !/^\d{1,2}$/.test(parts[1]))) return null;
  const size = 2 ** (32 - bits);
  const first = Math.floor(address / size) * size;
  return { first, last: first + size - 1 };
}

function isRangeWithin(child: IPv4Range, parent: IPv4Range): boolean {
  return child.first >= parent.first && child.last <= parent.last;
}

export const Resources: React.FC = () => {
  const { hasPermission } = useAuth();
  const { addToast } = useToast();
  const navigate = useNavigate();

  // Active top tab
  const [activeTab, setActiveTab] = useState<'ztna_assets' | 'api_endpoints' | 'data_scopes'>('ztna_assets');

  // --- ZTNA Business Assets State ---
  const [resources, setResources] = useState<ProtectedResource[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [loading, setLoading] = useState(true);

  // Filters for ZTNA assets
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
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
    portEnd: 443,
    portMode: 'single',
    url: '',
    siteId: '',
    siteName: '',
    gatewayNodeId: '',
    gatewayNodeName: '',
    tags: [],
    description: '',
  });

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
      const [resList, sList, apiList] = await Promise.all([
        protectedResourceApi.list(),
        siteApi.listSites(),
        mockApi.getApiResources(),
      ]);
      setResources(resList);
      setSites(sList);
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
      res.description.toLowerCase().includes(searchQuery.toLowerCase()) ||
      res.tags.some((t) => t.toLowerCase().includes(searchQuery.toLowerCase()));

    const matchesCategory = selectedCategory === 'all' || res.category === selectedCategory;
    const matchesSite = selectedSite === 'all' || res.siteId === selectedSite;

    return matchesSearch && matchesCategory && matchesSite;
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

  // Handlers for ZTNA Assets
  const handleOpenCreate = () => {
    const defaultSite = sites.find((site) => site.status === 'active' && Boolean(site.gatewayNodeId));

    setEditingResource({
      name: '',
      code: 'SVC-' + Math.floor(1000 + Math.random() * 9000),
      category: 'web_portal',
      protocol: 'HTTPS',
      host: '',
      port: 443,
      portEnd: 443,
      portMode: 'single',
      url: '',
      siteId: defaultSite?.id || '',
      siteName: defaultSite?.name || '',
      gatewayNodeId: defaultSite?.gatewayNodeId || '',
      gatewayNodeName: defaultSite?.gatewayNodeName || '',
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
    if (!editingResource.name?.trim() || !editingResource.host?.trim() || !editingResource.code?.trim() || !editingResource.siteId || !editingResource.gatewayNodeId) {
      addToast('请填写资源名称、编码、IP/CIDR，并选择已绑定 Gateway 的网络站点', 'warning');
      return;
    }

    const selectedSite = sites.find((site) => site.id === editingResource.siteId);
    const targetRange = parseIPv4Range(editingResource.host);
    const siteRange = selectedSite ? parseIPv4Range(selectedSite.localSubnet) : null;
    const natRange = selectedSite?.natMapping?.enabled ? parseIPv4Range(selectedSite.natMapping.virtualCidr) : null;
    if (!selectedSite || !targetRange || !siteRange || !(
      isRangeWithin(targetRange, siteRange) || (natRange && isRangeWithin(targetRange, natRange))
    )) {
      addToast('目标 IPv4 地址或网段必须完整位于所选网络站点的真实网段或已启用的 NAT 影子网段内', 'warning');
      return;
    }

    const portMode = editingResource.portMode || 'single';
    if (portMode === 'unconfigured') {
      addToast('请为该资源选择单端口、端口范围或任意端口', 'warning');
      return;
    }
    if (portMode !== 'any') {
      const portStart = Number(editingResource.port);
      const portEnd = portMode === 'range' ? Number(editingResource.portEnd) : portStart;
      if (!Number.isInteger(portStart) || !Number.isInteger(portEnd) || portStart < 1 || portEnd < portStart || portEnd > 65535) {
        addToast('端口必须是 1–65535 之间的有效端口，且范围结束端口不能小于起始端口', 'warning');
        return;
      }
    }

    try {
      const saved = await protectedResourceApi.save(editingResource as any);
      addToast(`受保护业务资产「${saved.name}」已成功保存并同步至网关`, 'success');
      setIsModalOpen(false);
      loadData();
    } catch (err: any) {
      addToast(err.message || '保存业务资产失败', 'error');
    }
  };

  const handleDeleteResource = async (resource: ProtectedResource) => {
    if (confirm(`确定要从受保护业务目录中移除「${resource.name}」吗？`)) {
      try {
        await protectedResourceApi.delete(resource);
        addToast(`业务资产「${resource.name}」已成功移除`, 'success');
        loadData();
      } catch (err: any) {
        addToast(err.message || '删除失败', 'error');
      }
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
          {activeTab === 'ztna_assets' && hasPermission('ztna.resource.write') && (
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
                  placeholder="搜索资源名称、编码、地址、说明或标签..."
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
                  {sites.filter((site) => site.status === 'active' && Boolean(site.gatewayNodeId)).map((s) => (
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

            </div>
          </div>

          {/* Cards Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {loading ? <div className="col-span-full py-12 text-center text-sm text-slate-500">正在加载企业资源目录…</div>
              : filteredResources.length === 0 ? <div className="col-span-full rounded-xl border border-dashed border-slate-300 bg-white py-12 text-center text-sm text-slate-500">没有符合条件的受保护资源。请先登记服务，并关联已绑定 Gateway 的网络站点。</div>
              : filteredResources.map((res) => {
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
                          </div>
                        </div>
                      </div>
                    </div>

                    <p className="text-xs text-slate-500 line-clamp-2 leading-relaxed">{res.description}</p>

                    <div className="p-3 bg-slate-50/70 border border-slate-100 rounded-xl space-y-1.5 text-xs">
                      <div className="flex items-center justify-between gap-3">
                        <span className="shrink-0 text-slate-400">目标地址:</span>
                        <span className="truncate text-right font-mono font-medium text-slate-800">
                          {res.host}
                        </span>
                      </div>
                      <div className="flex items-center justify-between">
                        <span className="text-slate-400">开放服务:</span>
                        <span className="font-mono font-medium text-slate-800">
                          {res.protocol} / {res.portMode === 'any' ? '任意端口' : res.portMode === 'unconfigured' ? '未配置' : res.portMode === 'range' ? `${res.port}-${res.portEnd}` : res.port}
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
                        <span className={`w-2 h-2 rounded-full ${res.status === 'healthy' ? 'bg-emerald-500' : 'bg-slate-400'}`} />
                        <span className="text-slate-600 font-medium">
                          {res.status === 'healthy' ? '已启用' : '未启用'}
                        </span>
                      </div>

                      <span className="text-[11px] text-blue-600 font-medium bg-blue-50 px-2 py-0.5 rounded">
                        {res.associatedPoliciesCount} 条关联 ACL 策略
                      </span>
                    </div>

                    <div className="flex items-center justify-between gap-2">
                      <button
                        onClick={() => navigate('/policies')}
                        title="配置与查看该资产的访问策略"
                        className="p-1.5 text-slate-500 hover:text-blue-600 hover:bg-slate-50 rounded-lg border border-slate-200 transition-colors"
                      >
                        <Sliders className="w-3.5 h-3.5" />
                      </button>

                      {hasPermission('ztna.resource.write') && <button
                        onClick={() => handleOpenEdit(res)}
                        title="编辑业务资产"
                        className="p-1.5 text-slate-500 hover:text-blue-600 hover:bg-slate-50 rounded-lg border border-slate-200 transition-colors"
                      >
                        <Edit2 className="w-3.5 h-3.5" />
                      </button>}

                      {hasPermission('ztna.resource.write') && <button
                        onClick={() => handleDeleteResource(res)}
                        title="移除资产"
                        className="p-1.5 text-slate-500 hover:text-rose-600 hover:bg-rose-50 rounded-lg border border-slate-200 transition-colors"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>}
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
                  onChange={(e) => setEditingResource({ ...editingResource, protocol: e.target.value as ResourceProtocol })}
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-700 focus:outline-none"
                >
                  {(editingResource.portMode || 'single') === 'single' ? <>
                    <option value="HTTPS">HTTPS (443)</option>
                    <option value="HTTP">HTTP (80)</option>
                    <option value="SSH">SSH (22)</option>
                    <option value="RDP">RDP (3389)</option>
                    <option value="MySQL">MySQL (3306)</option>
                    <option value="PostgreSQL">PostgreSQL (5432)</option>
                    <option value="TCP">通用 TCP</option>
                    <option value="UDP">通用 UDP</option>
                  </> : <>
                    <option value="TCP">TCP</option>
                    <option value="UDP">UDP</option>
                  </>}
                </select>
              </div>

            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div className="sm:col-span-2">
                <label className="block text-xs font-semibold text-slate-700 mb-1">
                  目标地址（IPv4 主机或网段） <span className="text-rose-500">*</span>
                </label>
                <input
                  type="text"
                  placeholder="例如 172.31.2.5 或 172.31.0.0/16"
                  value={editingResource.host || ''}
                  onChange={(e) => setEditingResource({ ...editingResource, host: e.target.value })}
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs font-mono text-slate-800 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                />
                <p className="mt-1.5 text-[11px] leading-relaxed text-slate-500">
                  可填写 IPv4 单地址或 CIDR 网段，也可填写该站点 NAT 影子网段中的地址。填写网段时，策略将作用于整个网段的所选协议和端口；地址必须属于所选网络站点。
                </p>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">端口</label>
                <select
                  value={editingResource.portMode || 'single'}
                  onChange={(event) => {
                    const mode = event.target.value as 'single' | 'range' | 'any';
                    setEditingResource((current) => {
                      const start = current.port && current.port > 0 ? current.port : 443;
                      return {
                        ...current,
                        portMode: mode,
                        protocol: mode === 'single' || current.protocol === 'UDP' ? current.protocol : 'TCP',
                        port: mode === 'any' ? 0 : start,
                        portEnd: mode === 'any' ? 0 : mode === 'range' ? Math.max(start, current.portEnd || start) : start,
                      };
                    });
                  }}
                  className="mb-2 w-full rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-700 focus:outline-none"
                >
                  {editingResource.portMode === 'unconfigured' && <option value="unconfigured" disabled>未配置</option>}
                  <option value="single">单端口</option>
                  <option value="range">端口范围</option>
                  <option value="any">任意端口</option>
                </select>
                {(editingResource.portMode || 'single') === 'single' && <input
                  type="number"
                  min={1}
                  max={65535}
                  placeholder="443"
                  value={editingResource.port ?? ''}
                  onChange={(event) => setEditingResource({ ...editingResource, port: Number(event.target.value), portEnd: Number(event.target.value) })}
                  className="w-full rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-xs font-mono text-slate-800 focus:outline-none"
                />}
                {editingResource.portMode === 'range' && <div className="grid grid-cols-2 gap-2">
                  <input
                    type="number"
                    min={1}
                    max={65535}
                    aria-label="起始端口"
                    placeholder="起始"
                    value={editingResource.port ?? ''}
                    onChange={(event) => setEditingResource({ ...editingResource, port: Number(event.target.value) })}
                    className="min-w-0 w-full rounded-xl border border-slate-200 bg-slate-50 px-2 py-2 text-xs font-mono text-slate-800 focus:outline-none"
                  />
                  <input
                    type="number"
                    min={1}
                    max={65535}
                    aria-label="结束端口"
                    placeholder="结束"
                    value={editingResource.portEnd ?? ''}
                    onChange={(event) => setEditingResource({ ...editingResource, portEnd: Number(event.target.value) })}
                    className="min-w-0 w-full rounded-xl border border-slate-200 bg-slate-50 px-2 py-2 text-xs font-mono text-slate-800 focus:outline-none"
                  />
                </div>}
                {editingResource.portMode === 'any' && <p className="rounded-lg bg-amber-50 px-2.5 py-2 text-[11px] leading-relaxed text-amber-800">将允许所选 TCP 或 UDP 协议的所有端口。</p>}
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
                      gatewayNodeId: st ? st.gatewayNodeId : '',
                      gatewayNodeName: st ? st.gatewayNodeName : '',
                    });
                  }}
                  className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs text-slate-700 focus:outline-none"
                >
                  <option value="" disabled>请选择已绑定 Gateway 的网络站点</option>
                  {sites.filter((site) => site.status === 'active' && Boolean(site.gatewayNodeId)).map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} ({s.localSubnet})
                    </option>
                  ))}
                </select>
                {(() => {
                  const site = sites.find((item) => item.id === editingResource.siteId);
                  if (!site) return null;
                  const ranges = [site.localSubnet];
                  if (site.natMapping?.enabled && site.natMapping.virtualCidr) ranges.push(site.natMapping.virtualCidr);
                  return <p className="mt-1.5 text-[11px] text-slate-500">允许的目标范围：<span className="font-mono">{ranges.join('、')}</span></p>;
                })()}
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1">由网络站点关联的 Gateway</label>
                <input readOnly value={editingResource.gatewayNodeName || '先选择已绑定 Gateway 的网络站点'} className="w-full cursor-not-allowed rounded-xl border border-slate-200 bg-slate-100 px-3 py-2 text-xs text-slate-600" />
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

    </div>
  );
};
