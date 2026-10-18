import React, { useState, useEffect, useMemo } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import {
  Search,
  RefreshCw,
  Trash2,
  ExternalLink,
  Server,
  ArrowUpDown,
  Filter,
  Key,
  Sparkles,
} from 'lucide-react';
import { mockApi } from '../../data/mock';
import { Node, NodeType, NodeStatus } from '../../types';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Select } from '../../components/ui/select';
import { Badge } from '../../components/ui/badge';
import { StatusBadge } from '../../components/StatusBadge';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '../../components/ui/table';
import { Card } from '../../components/ui/card';
import { EmptyState } from '../../components/EmptyState';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useToast } from '../../components/ui/toast';
import { formatBytes, formatRelativeTime } from '../../utils/format';

export const NodeList: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const { success, error } = useToast();

  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Filters state from query params or defaults
  const [search, setSearch] = useState(searchParams.get('q') || '');
  const [siteFilter, setSiteFilter] = useState(searchParams.get('site') || 'all');
  const [typeFilter, setTypeFilter] = useState(searchParams.get('type') || 'all');
  const [statusFilter, setStatusFilter] = useState(searchParams.get('status') || 'all');

  // Sorting
  const [sortBy, setSortBy] = useState<'name' | 'lastHandshake' | 'traffic'>('lastHandshake');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc');

  // Pagination
  const [currentPage, setCurrentPage] = useState(1);
  const pageSize = 10;

  // Deletion modal
  const [nodeToDelete, setNodeToDelete] = useState<Node | null>(null);
  const [deleting, setDeleting] = useState(false);

  const loadNodes = async (showRefresh = false) => {
    if (showRefresh) setIsRefreshing(true);
    try {
      const data = await mockApi.getNodes();
      setNodes(data);
    } catch {
      error('获取节点列表失败');
    } finally {
      setLoading(false);
      setIsRefreshing(false);
    }
  };

  useEffect(() => {
    loadNodes();
  }, []);

  // Sync filter changes with query string
  useEffect(() => {
    const params: Record<string, string> = {};
    if (search) params.q = search;
    if (siteFilter !== 'all') params.site = siteFilter;
    if (typeFilter !== 'all') params.type = typeFilter;
    if (statusFilter !== 'all') params.status = statusFilter;
    setSearchParams(params, { replace: true });
    setCurrentPage(1);
  }, [search, siteFilter, typeFilter, statusFilter, setSearchParams]);

  // Unique sites for filter dropdown
  const sitesList = useMemo(() => {
    const sMap = new Map<string, string>();
    nodes.forEach((n) => sMap.set(n.siteId, n.siteName));
    return Array.from(sMap.entries()).map(([id, name]) => ({ id, name }));
  }, [nodes]);

  // Filtered and sorted nodes
  const filteredNodes = useMemo(() => {
    return nodes
      .filter((node) => {
        const matchesSearch =
          !search ||
          node.name.toLowerCase().includes(search.toLowerCase()) ||
          node.virtualIp.toLowerCase().includes(search.toLowerCase());
        const matchesSite = siteFilter === 'all' || node.siteId === siteFilter;
        const matchesType = typeFilter === 'all' || node.type === typeFilter;
        const matchesStatus = statusFilter === 'all' || node.status === statusFilter;
        return matchesSearch && matchesSite && matchesType && matchesStatus;
      })
      .sort((a, b) => {
        let valA: string | number = '';
        let valB: string | number = '';

        if (sortBy === 'traffic') {
          valA = a.rxBytes + a.txBytes;
          valB = b.rxBytes + b.txBytes;
        } else if (sortBy === 'lastHandshake') {
          valA = new Date(a.lastHandshake).getTime();
          valB = new Date(b.lastHandshake).getTime();
        } else {
          valA = a.name;
          valB = b.name;
        }

        if (valA < valB) return sortOrder === 'asc' ? -1 : 1;
        if (valA > valB) return sortOrder === 'asc' ? 1 : -1;
        return 0;
      });
  }, [nodes, search, siteFilter, typeFilter, statusFilter, sortBy, sortOrder]);

  // Paginated nodes
  const totalPages = Math.ceil(filteredNodes.length / pageSize) || 1;
  const paginatedNodes = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredNodes.slice(start, start + pageSize);
  }, [filteredNodes, currentPage, pageSize]);

  const handleDeleteConfirm = async () => {
    if (!nodeToDelete) return;
    setDeleting(true);
    try {
      await mockApi.deleteNode(nodeToDelete.id);
      success(`节点 ${nodeToDelete.name} 已成功删除`);
      setNodeToDelete(null);
      await loadNodes();
    } catch {
      error('删除节点失败');
    } finally {
      setDeleting(false);
    }
  };

  const handleSort = (field: 'name' | 'lastHandshake' | 'traffic') => {
    if (sortBy === field) {
      setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
    } else {
      setSortBy(field);
      setSortOrder('desc');
    }
  };

  return (
    <div className="space-y-5">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">虚拟节点管理</h1>
          <p className="text-xs text-slate-500 mt-0.5">
            WireGuard Mesh 拓扑网络中的边缘网关与接入客户端节点
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => loadNodes(true)}
            isLoading={isRefreshing}
            className="gap-1.5"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? 'animate-spin' : ''}`} />
            <span>刷新</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate('/enrollment-tokens')}
            className="gap-1.5"
          >
            <Key className="w-3.5 h-3.5 text-slate-500" />
            <span>注册凭证</span>
          </Button>

          <Button
            size="sm"
            onClick={() => navigate('/enrollment-tokens')}
            className="gap-1.5 bg-blue-600 hover:bg-blue-700 text-white"
          >
            <Sparkles className="w-3.5 h-3.5" />
            <span>预建网关节点</span>
          </Button>
        </div>
      </div>

      {/* Filter and Search Bar */}
      <Card className="p-4">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3">
          <div className="lg:col-span-2">
            <Input
              placeholder="搜索节点名称或虚拟 IP..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              leftIcon={<Search className="w-4 h-4 text-slate-400" />}
            />
          </div>

          <Select
            value={siteFilter}
            onChange={(e) => setSiteFilter(e.target.value)}
          >
            <option value="all">所属站点：全部站点</option>
            {sitesList.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>

          <Select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
          >
            <option value="all">节点类型：全部</option>
            <option value="gateway">网关 (gateway)</option>
            <option value="client">客户端 (client)</option>
          </Select>

          <Select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          >
            <option value="all">运行状态：全部</option>
            <option value="online">在线 (online)</option>
            <option value="pending_enrollment">待注册入网 (pending_enrollment)</option>
            <option value="degraded">降级 (degraded)</option>
            <option value="offline">离线 (offline)</option>
          </Select>
        </div>
      </Card>

      {/* Nodes Table Card */}
      <Card>
        <div className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>
                  <button
                    onClick={() => handleSort('name')}
                    className="flex items-center gap-1 hover:text-slate-900 transition-colors"
                  >
                    <span>节点名称</span>
                    <ArrowUpDown className="w-3 h-3" />
                  </button>
                </TableHead>
                <TableHead>类型</TableHead>
                <TableHead>虚拟 IP</TableHead>
                <TableHead>所属物理站点</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>
                  <button
                    onClick={() => handleSort('lastHandshake')}
                    className="flex items-center gap-1 hover:text-slate-900 transition-colors"
                  >
                    <span>最后握手</span>
                    <ArrowUpDown className="w-3 h-3" />
                  </button>
                </TableHead>
                <TableHead>
                  <button
                    onClick={() => handleSort('traffic')}
                    className="flex items-center gap-1 hover:text-slate-900 transition-colors"
                  >
                    <span>接收 / 发送流量</span>
                    <ArrowUpDown className="w-3 h-3" />
                  </button>
                </TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center py-12 text-slate-400">
                    <RefreshCw className="w-6 h-6 animate-spin mx-auto mb-2 text-blue-600" />
                    <span>正在加载节点数据...</span>
                  </TableCell>
                </TableRow>
              ) : paginatedNodes.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="p-8">
                    <EmptyState
                      icon={Server}
                      title="暂无匹配节点"
                      description="没有找到符合筛选条件的 WireGuard 节点，请尝试清空或重置搜索条件。"
                      actionText="清除所有过滤"
                      onAction={() => {
                        setSearch('');
                        setSiteFilter('all');
                        setTypeFilter('all');
                        setStatusFilter('all');
                      }}
                    />
                  </TableCell>
                </TableRow>
              ) : (
                paginatedNodes.map((node) => (
                  <TableRow key={node.id} className="group">
                    <TableCell>
                      <button
                        onClick={() => navigate(`/nodes/${node.id}`)}
                        className="font-medium text-blue-600 hover:text-blue-800 hover:underline text-left flex items-center gap-1.5"
                      >
                        <span>{node.name}</span>
                        <ExternalLink className="w-3 h-3 opacity-0 group-hover:opacity-100 transition-opacity" />
                      </button>
                    </TableCell>
                    <TableCell>
                      {node.type === 'gateway' ? (
                        <div className="space-y-0.5">
                          <Badge variant="primary">Gateway</Badge>
                          {node.nodeTypeLabel && (
                            <div className="text-[10px] text-slate-500 font-normal">
                              {node.nodeTypeLabel}
                            </div>
                          )}
                        </div>
                      ) : (
                        <Badge variant="success">Client</Badge>
                      )}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-slate-800">
                      {node.virtualIp}
                    </TableCell>
                    <TableCell className="text-slate-600 text-xs">
                      <div className="font-medium text-slate-800">{node.siteName}</div>
                      {node.allowedSubnets && node.allowedSubnets.length > 0 ? (
                        <div className="text-[11px] font-mono text-blue-600 mt-0.5 flex items-center gap-1">
                          <span className="text-slate-400">代理:</span>
                          <span className="truncate max-w-[180px]" title={node.allowedSubnets.join(', ')}>
                            {node.allowedSubnets.join(', ')}
                          </span>
                        </div>
                      ) : node.networkName ? (
                        <div className="text-[10px] text-slate-400">{node.networkName}</div>
                      ) : null}
                    </TableCell>
                    <TableCell>
                      <StatusBadge status={node.status} />
                    </TableCell>
                    <TableCell className="text-xs text-slate-500 font-mono">
                      {node.status === 'pending_enrollment' ? (
                        <span className="text-blue-600 font-sans text-xs flex items-center gap-1">
                          <Key className="w-3 h-3" />
                          待首次入网握手
                        </span>
                      ) : (
                        formatRelativeTime(node.lastHandshake)
                      )}
                    </TableCell>
                    <TableCell className="text-xs text-slate-600 font-mono">
                      {node.status === 'pending_enrollment' ? (
                        <span className="text-slate-400 font-sans text-xs">未激活 (0 B)</span>
                      ) : (
                        <>
                          <span className="text-blue-600 font-medium">
                            ↓ {formatBytes(node.rxBytes)}
                          </span>
                          <span className="mx-1 text-slate-300">/</span>
                          <span className="text-emerald-600 font-medium">
                            ↑ {formatBytes(node.txBytes)}
                          </span>
                        </>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1.5">
                        {node.status === 'pending_enrollment' && (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => navigate(`/enrollment-tokens?q=${encodeURIComponent(node.name)}`)}
                            className="h-8 px-2 text-xs text-indigo-600 hover:text-indigo-700 hover:bg-indigo-50 font-medium flex items-center gap-1"
                            title="查看/签发该节点的一次性入网凭证"
                          >
                            <Key className="w-3.5 h-3.5" />
                            <span>入网凭证</span>
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => navigate(`/nodes/${node.id}`)}
                          className="h-8 px-2 text-xs text-blue-600 hover:text-blue-700 hover:bg-blue-50"
                        >
                          详情
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setNodeToDelete(node)}
                          className="h-8 w-8 p-0 text-slate-400 hover:text-rose-600 hover:bg-rose-50"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>

        {/* Pagination Bar */}
        {filteredNodes.length > 0 && (
          <div className="flex items-center justify-between px-6 py-4 border-t border-slate-100 text-xs text-slate-500">
            <div>
              共 <strong className="text-slate-800 font-semibold">{filteredNodes.length}</strong> 条记录，
              当前第 {currentPage} / {totalPages} 页
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={currentPage === 1}
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
              >
                上一页
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={currentPage >= totalPages}
                onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
              >
                下一页
              </Button>
            </div>
          </div>
        )}
      </Card>

      {/* Delete Confirmation Modal */}
      {nodeToDelete && (
        <ConfirmDialog
          isOpen={!!nodeToDelete}
          onClose={() => setNodeToDelete(null)}
          onConfirm={handleDeleteConfirm}
          title="确认删除节点"
          description={`确定要删除虚拟节点 "${nodeToDelete.name}" (${nodeToDelete.virtualIp}) 吗？删除后该节点的 WireGuard 密钥与连接凭据将永久失效，所有 Mesh Peer 连接将被立即切断。`}
          confirmText="确认删除"
          danger={true}
          isLoading={deleting}
        />
      )}
    </div>
  );
};
