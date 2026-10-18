import React, { useState, useEffect, useMemo } from 'react';
import {
  Search,
  Download,
  Filter,
  FileText,
  Calendar,
  RotateCcw,
  History,
  FileSpreadsheet,
} from 'lucide-react';

import { useNavigate } from 'react-router-dom';
import { mockApi } from '../data/mock';
import { AuditRecord } from '../types';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { StatusBadge } from '../components/StatusBadge';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '../components/ui/table';
import { Card } from '../components/ui/card';
import { EmptyState } from '../components/EmptyState';
import { useToast } from '../components/ui/toast';
import { formatBytes, formatDateTime } from '../utils/format';

export const Audit: React.FC = () => {
  const { success, error } = useToast();
  const navigate = useNavigate();

  const [records, setRecords] = useState<AuditRecord[]>([]);
  const [loading, setLoading] = useState(true);

  // Filters
  const [searchIp, setSearchIp] = useState('');
  const [userFilter, setUserFilter] = useState('all');
  const [nodeFilter, setNodeFilter] = useState('all');
  const [actionFilter, setActionFilter] = useState<'all' | 'allow' | 'deny'>('all');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');

  // Pagination
  const [currentPage, setCurrentPage] = useState(1);
  const pageSize = 20;

  const loadData = async () => {
    try {
      const data = await mockApi.getAudit();
      setRecords(data);
    } catch {
      error('加载审计日志失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  // Distinct users and nodes for dropdown options
  const uniqueUsers = useMemo(() => {
    const s = new Set<string>();
    records.forEach((r) => r.user && s.add(r.user));
    return Array.from(s);
  }, [records]);

  const uniqueNodes = useMemo(() => {
    const s = new Set<string>();
    records.forEach((r) => r.node && s.add(r.node));
    return Array.from(s);
  }, [records]);

  // Filtered records
  const filteredRecords = useMemo(() => {
    return records.filter((r) => {
      const matchIp =
        !searchIp ||
        r.sourceIp.includes(searchIp) ||
        r.destIp.includes(searchIp);
      const matchUser = userFilter === 'all' || r.user === userFilter;
      const matchNode = nodeFilter === 'all' || r.node === nodeFilter;
      const matchAction = actionFilter === 'all' || r.action === actionFilter;

      let matchDate = true;
      if (startDate) {
        matchDate = matchDate && new Date(r.timestamp) >= new Date(startDate);
      }
      if (endDate) {
        // Set to end of the day
        const end = new Date(endDate);
        end.setHours(23, 59, 59, 999);
        matchDate = matchDate && new Date(r.timestamp) <= end;
      }

      return matchIp && matchUser && matchNode && matchAction && matchDate;
    });
  }, [records, searchIp, userFilter, nodeFilter, actionFilter, startDate, endDate]);

  // Paginated records
  const totalPages = Math.ceil(filteredRecords.length / pageSize) || 1;
  const paginatedRecords = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredRecords.slice(start, start + pageSize);
  }, [filteredRecords, currentPage, pageSize]);

  // Export CSV function with UTF-8 BOM for Microsoft Excel compatibility
  const handleExportCSV = () => {
    if (filteredRecords.length === 0) {
      error('当前无符合条件的审计日志可供导出');
      return;
    }

    const headers = [
      '时间戳',
      '用户',
      '终端设备',
      '途径节点',
      '源IP',
      '目的IP',
      '端口',
      '动作',
      '字节数',
    ];

    const rows = filteredRecords.map((r) => [
      formatDateTime(r.timestamp),
      r.user,
      r.device,
      r.node,
      r.sourceIp,
      r.destIp,
      r.port,
      r.action === 'allow' ? '允许' : '阻断拒绝',
      r.bytes,
    ]);

    const csvContent =
      '\uFEFF' +
      [headers.join(','), ...rows.map((row) => row.map((cell) => `"${cell}"`).join(','))].join(
        '\n'
      );

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.setAttribute('href', url);
    link.setAttribute(
      'download',
      `wireguard_audit_log_${new Date().toISOString().slice(0, 10)}.csv`
    );
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);

    success(`已导出 ${filteredRecords.length} 条审计日志为 CSV 文件`);
  };

  const handleResetFilters = () => {
    setSearchIp('');
    setUserFilter('all');
    setNodeFilter('all');
    setActionFilter('all');
    setStartDate('');
    setEndDate('');
    setCurrentPage(1);
  };

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">安全审计日志</h1>
          <p className="text-xs text-slate-500 mt-0.5">
            记录全网 Mesh 节点的网络会话、连接握手与防火墙 ACL 放行/拦截事件
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate('/operation-logs')}
            className="gap-1.5"
          >
            <FileSpreadsheet className="w-3.5 h-3.5 text-slate-500" />
            <span>操作日志</span>
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => navigate('/login-logs')}
            className="gap-1.5"
          >
            <History className="w-3.5 h-3.5 text-slate-500" />
            <span>登录日志</span>
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleExportCSV}
            className="gap-1.5"
          >
            <Download className="w-3.5 h-3.5 text-slate-500" />
            <span>导出 CSV</span>
          </Button>
        </div>

      </div>

      {/* Filter and Search Bar */}
      <Card className="p-4 space-y-3">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          <div>
            <Input
              placeholder="搜索源 IP 或目的 IP..."
              value={searchIp}
              onChange={(e) => {
                setSearchIp(e.target.value);
                setCurrentPage(1);
              }}
              leftIcon={<Search className="w-4 h-4 text-slate-400" />}
            />
          </div>

          <div>
            <Select
              value={userFilter}
              onChange={(e) => {
                setUserFilter(e.target.value);
                setCurrentPage(1);
              }}
            >
              <option value="all">用户筛选：全部用户</option>
              {uniqueUsers.map((u) => (
                <option key={u} value={u}>
                  {u}
                </option>
              ))}
            </Select>
          </div>

          <div>
            <Select
              value={nodeFilter}
              onChange={(e) => {
                setNodeFilter(e.target.value);
                setCurrentPage(1);
              }}
            >
              <option value="all">节点筛选：全部节点</option>
              {uniqueNodes.map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </Select>
          </div>

          <div>
            <Select
              value={actionFilter}
              onChange={(e) => {
                setActionFilter(e.target.value as any);
                setCurrentPage(1);
              }}
            >
              <option value="all">判定动作：全部</option>
              <option value="allow">允许通行 (allow)</option>
              <option value="deny">阻断拦截 (deny)</option>
            </Select>
          </div>
        </div>

        {/* Date range picker line */}
        <div className="flex flex-wrap items-center justify-between gap-3 pt-1 border-t border-slate-100">
          <div className="flex flex-wrap items-center gap-2 text-xs text-slate-600">
            <span className="flex items-center gap-1 font-medium text-slate-500">
              <Calendar className="w-3.5 h-3.5" />
              <span>时间范围:</span>
            </span>
            <input
              type="date"
              value={startDate}
              onChange={(e) => {
                setStartDate(e.target.value);
                setCurrentPage(1);
              }}
              className="rounded-lg border border-slate-300 bg-white px-2.5 py-1 text-xs text-slate-800 shadow-2xs focus:border-blue-500 focus:outline-none"
            />
            <span className="text-slate-400">至</span>
            <input
              type="date"
              value={endDate}
              onChange={(e) => {
                setEndDate(e.target.value);
                setCurrentPage(1);
              }}
              className="rounded-lg border border-slate-300 bg-white px-2.5 py-1 text-xs text-slate-800 shadow-2xs focus:border-blue-500 focus:outline-none"
            />
          </div>

          <Button
            variant="ghost"
            size="sm"
            onClick={handleResetFilters}
            className="text-xs text-slate-500 hover:text-slate-800 gap-1"
          >
            <RotateCcw className="w-3 h-3" />
            <span>重置筛选</span>
          </Button>
        </div>
      </Card>

      {/* Audit Table */}
      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>时间戳</TableHead>
              <TableHead>触发用户</TableHead>
              <TableHead>发起终端</TableHead>
              <TableHead>途径节点</TableHead>
              <TableHead>源 IP 地址</TableHead>
              <TableHead>目的 IP 地址</TableHead>
              <TableHead>目标端口</TableHead>
              <TableHead>动作判定</TableHead>
              <TableHead>传输字节</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredRecords.length === 0 ? (
              <TableRow>
                <TableCell colSpan={9} className="p-8">
                  <EmptyState
                    icon={FileText}
                    title="暂无审计事件"
                    description="未检索到满足条件的日志记录，请调整筛选条件或时间范围。"
                  />
                </TableCell>
              </TableRow>
            ) : (
              paginatedRecords.map((r) => (
                <TableRow key={r.id}>
                  <TableCell className="font-mono text-xs text-slate-500">
                    {formatDateTime(r.timestamp)}
                  </TableCell>
                  <TableCell className="font-medium text-slate-900 text-xs">
                    {r.user}
                  </TableCell>
                  <TableCell className="text-xs text-slate-600">
                    {r.device}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-600 font-medium">
                    {r.node}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-700">
                    {r.sourceIp}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-700">
                    {r.destIp}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-600">
                    {r.port}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={r.action} />
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-600">
                    {r.bytes > 0 ? formatBytes(r.bytes) : '-'}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>

        {/* Pagination */}
        {filteredRecords.length > 0 && (
          <div className="flex items-center justify-between px-6 py-4 border-t border-slate-100 text-xs text-slate-500">
            <div>
              共 <strong className="text-slate-800 font-semibold">{filteredRecords.length}</strong> 条审计记录，
              当前第 {currentPage} / {totalPages} 页 (每页 20 条)
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
    </div>
  );
};
