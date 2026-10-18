import React, { useState, useEffect, useMemo } from 'react';
import {
  Search,
  Download,
  Filter,
  FileSpreadsheet,
  Calendar,
  RotateCcw,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Eye,
  Trash2,
  Play,
  ShieldAlert,
  Clock,
  UserCheck,
  Server,
  Layers,
  Code2,
} from 'lucide-react';
import { mockApi } from '../../data/mock';
import { OperationLog, OperationActionType, OperationStatus } from '../../types';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Select } from '../../components/ui/select';
import { Card } from '../../components/ui/card';
import { Badge } from '../../components/ui/badge';
import { Dialog } from '../../components/ui/dialog';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { EmptyState } from '../../components/EmptyState';
import { useToast } from '../../components/ui/toast';
import { formatDateTime } from '../../utils/format';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '../../components/ui/table';

export const OperationLogs: React.FC = () => {
  const { addToast } = useToast();

  const [logs, setLogs] = useState<OperationLog[]>([]);
  const [loading, setLoading] = useState(true);

  // Filters
  const [searchQuery, setSearchQuery] = useState('');
  const [moduleFilter, setModuleFilter] = useState('all');
  const [actionTypeFilter, setActionTypeFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState<'all' | OperationStatus>('all');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');

  // Pagination
  const [currentPage, setCurrentPage] = useState(1);
  const pageSize = 15;

  // Detail Modal State
  const [selectedLog, setSelectedLog] = useState<OperationLog | null>(null);

  // Clear Confirm Dialog
  const [showClearConfirm, setShowClearConfirm] = useState(false);

  const loadData = async () => {
    try {
      setLoading(true);
      const data = await mockApi.getOperationLogs();
      setLogs(data);
    } catch {
      addToast('加载操作审计日志失败', 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  // Distinct modules for filter
  const uniqueModules = useMemo(() => {
    const s = new Set<string>();
    logs.forEach((item) => item.module && s.add(item.module));
    return Array.from(s);
  }, [logs]);

  // Filtered records
  const filteredLogs = useMemo(() => {
    return logs.filter((log) => {
      const q = searchQuery.toLowerCase().trim();
      const matchSearch =
        !q ||
        log.operatorUsername.toLowerCase().includes(q) ||
        log.operatorName.toLowerCase().includes(q) ||
        log.actionName.toLowerCase().includes(q) ||
        log.targetResource.toLowerCase().includes(q) ||
        log.ip.includes(q) ||
        log.requestUri.toLowerCase().includes(q);

      const matchModule = moduleFilter === 'all' || log.module === moduleFilter;
      const matchAction = actionTypeFilter === 'all' || log.actionType === actionTypeFilter;
      const matchStatus = statusFilter === 'all' || log.status === statusFilter;

      let matchDate = true;
      if (startDate) {
        matchDate = matchDate && new Date(log.timestamp) >= new Date(startDate);
      }
      if (endDate) {
        const end = new Date(endDate);
        end.setHours(23, 59, 59, 999);
        matchDate = matchDate && new Date(log.timestamp) <= end;
      }

      return matchSearch && matchModule && matchAction && matchStatus && matchDate;
    });
  }, [logs, searchQuery, moduleFilter, actionTypeFilter, statusFilter, startDate, endDate]);

  // Paginated records
  const totalPages = Math.ceil(filteredLogs.length / pageSize) || 1;
  const paginatedLogs = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredLogs.slice(start, start + pageSize);
  }, [filteredLogs, currentPage, pageSize]);

  const handleResetFilters = () => {
    setSearchQuery('');
    setModuleFilter('all');
    setActionTypeFilter('all');
    setStatusFilter('all');
    setStartDate('');
    setEndDate('');
    setCurrentPage(1);
  };

  const handleExportCSV = () => {
    if (filteredLogs.length === 0) {
      addToast('当前无符合筛选条件的操作日志可导出', 'warning');
      return;
    }

    const headers = [
      '日志编号',
      '操作时间',
      '操作账号',
      '姓名与角色',
      '业务模块',
      '操作动作类型',
      '行为描述',
      '目标资源标识',
      '请求方法',
      '请求接口URI',
      '客户端IP',
      '客户端位置',
      '执行状态',
      '耗时(ms)',
      '拦截或失败原因',
    ];

    const rows = filteredLogs.map((l) => [
      l.id,
      l.timestamp,
      l.operatorUsername,
      `${l.operatorName} (${l.operatorRole})`,
      l.module,
      l.actionType,
      `"${l.actionName.replace(/"/g, '""')}"`,
      l.targetResource,
      l.requestMethod,
      l.requestUri,
      l.ip,
      `"${l.location}"`,
      l.status,
      l.durationMs,
      l.errorMessage ? `"${l.errorMessage.replace(/"/g, '""')}"` : '',
    ]);

    const csvContent =
      '\uFEFF' + [headers.join(','), ...rows.map((r) => r.join(','))].join('\n');

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.setAttribute('href', url);
    link.setAttribute(
      'download',
      `velotrust_operation_audit_${new Date().toISOString().slice(0, 10)}.csv`
    );
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);

    addToast(`已成功导出 ${filteredLogs.length} 条操作审计日志报表`, 'success');
  };

  const handleClearLogs = async () => {
    try {
      await mockApi.clearOperationLogs();
      setLogs([]);
      setShowClearConfirm(false);
      addToast('操作审计日志已成功清空并归档', 'success');
    } catch {
      addToast('清空日志失败', 'error');
    }
  };

  // Simulate new operation for demo verification
  const triggerSimulatedOperation = async (type: 'policy' | 'user' | 'blocked' | 'drill') => {
    try {
      if (type === 'policy') {
        const item = await mockApi.addOperationLog({
          operatorUsername: 'sec_admin',
          operatorName: '李安全 (安全保密员)',
          operatorRole: '安全保密管理员',
          module: '访问控制策略',
          actionType: 'CREATE',
          actionName: '动态下发针对财务服务器端口 8443 的微隔离访问策略',
          targetResource: 'policy-fin-micro-seg',
          requestMethod: 'POST',
          requestUri: '/api/v1/policies',
          status: 'SUCCESS',
          durationMs: 34,
          diffSnapshot: {
            after: {
              targetIp: '10.88.30.10/32',
              port: '8443',
              action: 'allow',
              createdAt: new Date().toISOString(),
            },
          },
        });
        setLogs((prev) => [item, ...prev]);
        addToast('已成功模拟管理员新建访问控制策略操作', 'success');
      } else if (type === 'user') {
        const item = await mockApi.addOperationLog({
          operatorUsername: 'admin',
          operatorName: '张伟 (超级管理员)',
          operatorRole: '超级管理员',
          module: '系统用户管理',
          actionType: 'UPDATE',
          actionName: '重置研发人员多因子 TOTP 动态令牌绑定',
          targetResource: 'user-dev-chen',
          requestMethod: 'PUT',
          requestUri: '/api/v1/users/user-chen/mfa-reset',
          status: 'SUCCESS',
          durationMs: 52,
          diffSnapshot: {
            before: { mfaActive: true, secretRotated: false },
            after: { mfaActive: false, secretRotated: true, reason: '用户更换手机终端申请' },
          },
        });
        setLogs((prev) => [item, ...prev]);
        addToast('已成功模拟用户安全属性更新操作', 'success');
      } else if (type === 'blocked') {
        const item = await mockApi.addOperationLog({
          operatorUsername: 'sys_admin',
          operatorName: '赵网络 (系统管理员)',
          operatorRole: '系统与网络管理员',
          module: '角色与权限',
          actionType: 'GRANT',
          actionName: '越权尝试修改超级管理员角色权限码 (三权分立拦截)',
          targetResource: 'role-super-admin',
          requestMethod: 'PUT',
          requestUri: '/api/v1/roles/super_admin',
          status: 'BLOCKED',
          durationMs: 18,
          errorMessage: '【三权分立互斥越权防御】系统与网络管理员禁止变更角色授权与访问控制字典，已自动阻断并上报合规审计中心',
          diffSnapshot: {
            before: { permissionCodes: ['*'] },
            after: { permissionCodes: ['custom:hack'] },
          },
        });
        setLogs((prev) => [item, ...prev]);
        addToast('已成功模拟越权操作触发安全阻断留痕', 'warning');
      } else {
        const item = await mockApi.addOperationLog({
          operatorUsername: 'sys_admin',
          operatorName: '赵网络 (系统管理员)',
          operatorRole: '系统与网络管理员',
          module: '系统容灾与设置',
          actionType: 'DRILL_TEST',
          actionName: '执行 WireGuard 边缘网关断网熔断死信定时器测试演练',
          targetResource: 'failsafe-drill-worker',
          requestMethod: 'POST',
          requestUri: '/api/v1/system/failsafe/drill-run',
          status: 'SUCCESS',
          durationMs: 1200,
          diffSnapshot: {
            after: {
              heartbeatLostSimulated: true,
              failoverSeconds: 1.2,
              result: 'PASS',
            },
          },
        });
        setLogs((prev) => [item, ...prev]);
        addToast('已成功模拟高可用容灾演练操作', 'info');
      }
    } catch {
      addToast('模拟操作失败', 'error');
    }
  };

  const getActionTypeBadge = (type: OperationActionType) => {
    switch (type) {
      case 'CREATE':
        return <Badge variant="outline" className="bg-emerald-50 text-emerald-700 border-emerald-200">新建 CREATE</Badge>;
      case 'UPDATE':
        return <Badge variant="outline" className="bg-blue-50 text-blue-700 border-blue-200">修改 UPDATE</Badge>;
      case 'DELETE':
        return <Badge variant="outline" className="bg-rose-50 text-rose-700 border-rose-200">删除 DELETE</Badge>;
      case 'ENABLE':
        return <Badge variant="outline" className="bg-teal-50 text-teal-700 border-teal-200">启用 ENABLE</Badge>;
      case 'DISABLE':
        return <Badge variant="outline" className="bg-slate-100 text-slate-700 border-slate-300">禁用 DISABLE</Badge>;
      case 'EXPORT':
        return <Badge variant="outline" className="bg-purple-50 text-purple-700 border-purple-200">导出 EXPORT</Badge>;
      case 'GRANT':
        return <Badge variant="outline" className="bg-indigo-50 text-indigo-700 border-indigo-200">授权 GRANT</Badge>;
      case 'POLICY_ENFORCE':
        return <Badge variant="outline" className="bg-amber-50 text-amber-700 border-amber-200">生效 ENFORCE</Badge>;
      case 'DRILL_TEST':
        return <Badge variant="outline" className="bg-orange-50 text-orange-700 border-orange-200">演练 DRILL</Badge>;
      default:
        return <Badge variant="outline" className="bg-slate-50 text-slate-700 border-slate-200">{type}</Badge>;
    }
  };

  const getStatusBadge = (status: OperationStatus) => {
    switch (status) {
      case 'SUCCESS':
        return (
          <span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded border border-emerald-200">
            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" />
            成功
          </span>
        );
      case 'FAILED':
        return (
          <span className="inline-flex items-center gap-1 text-xs font-semibold text-rose-700 bg-rose-50 px-2 py-0.5 rounded border border-rose-200">
            <XCircle className="w-3.5 h-3.5 text-rose-600" />
            失败
          </span>
        );
      case 'BLOCKED':
        return (
          <span className="inline-flex items-center gap-1 text-xs font-semibold text-amber-700 bg-amber-50 px-2 py-0.5 rounded border border-amber-200">
            <AlertTriangle className="w-3.5 h-3.5 text-amber-600" />
            安全拦截
          </span>
        );
      default:
        return null;
    }
  };

  return (
    <div className="space-y-5">
      {/* Page Header - Clean, No Top Summary Banner / No KPI Stats Cards */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl font-bold text-slate-900 tracking-tight">操作审计日志</h1>
            <Badge variant="outline" className="bg-purple-50 text-purple-700 border-purple-200 text-xs">
              行为留痕 · 等保三级
            </Badge>
          </div>
          <p className="text-xs text-slate-500 mt-0.5">
            全量记录系统用户与管理员的控制平面配置变更、权限赋予、安全策略调整与管理行为留痕
          </p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          {/* Quick Simulate Dropdown */}
          <div className="relative group">
            <Button variant="outline" size="sm" className="gap-1.5 shadow-xs">
              <Play className="w-3.5 h-3.5 text-blue-600" />
              <span>模拟操作事件</span>
            </Button>
            <div className="absolute right-0 top-full mt-1 w-52 bg-white rounded-lg border border-slate-200 shadow-lg p-1.5 hidden group-hover:block z-30">
              <button
                onClick={() => triggerSimulatedOperation('policy')}
                className="w-full text-left px-3 py-1.5 text-xs text-slate-700 hover:bg-slate-50 rounded flex items-center gap-2 transition-colors"
              >
                <Layers className="h-3.5 w-3.5 text-emerald-600" />
                新建业务微隔离策略
              </button>
              <button
                onClick={() => triggerSimulatedOperation('user')}
                className="w-full text-left px-3 py-1.5 text-xs text-slate-700 hover:bg-slate-50 rounded flex items-center gap-2 transition-colors"
              >
                <UserCheck className="h-3.5 w-3.5 text-blue-600" />
                更新用户 MFA 绑定
              </button>
              <button
                onClick={() => triggerSimulatedOperation('blocked')}
                className="w-full text-left px-3 py-1.5 text-xs text-slate-700 hover:bg-slate-50 rounded flex items-center gap-2 transition-colors"
              >
                <ShieldAlert className="h-3.5 w-3.5 text-amber-600" />
                模拟三权分立越权拦截
              </button>
              <button
                onClick={() => triggerSimulatedOperation('drill')}
                className="w-full text-left px-3 py-1.5 text-xs text-slate-700 hover:bg-slate-50 rounded flex items-center gap-2 transition-colors"
              >
                <Server className="h-3.5 w-3.5 text-purple-600" />
                触发控制网关演练
              </button>
            </div>
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={handleExportCSV}
            className="gap-1.5 shadow-xs"
          >
            <Download className="w-3.5 h-3.5 text-slate-600" />
            <span>导出 CSV 报表</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={() => setShowClearConfirm(true)}
            className="gap-1.5 shadow-xs text-rose-600 border-rose-200 hover:bg-rose-50"
          >
            <Trash2 className="w-3.5 h-3.5" />
            <span>清空日志</span>
          </Button>
        </div>
      </div>

      {/* Filter and Search Bar */}
      <Card className="p-4 space-y-3">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          <div>
            <Input
              placeholder="搜索操作人、行为名称、目标资源、IP..."
              value={searchQuery}
              onChange={(e) => {
                setSearchQuery(e.target.value);
                setCurrentPage(1);
              }}
              leftIcon={<Search className="w-4 h-4 text-slate-400" />}
            />
          </div>

          <div>
            <Select
              value={moduleFilter}
              onChange={(e) => {
                setModuleFilter(e.target.value);
                setCurrentPage(1);
              }}
            >
              <option value="all">业务模块：全部模块</option>
              {uniqueModules.map((m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </Select>
          </div>

          <div>
            <Select
              value={actionTypeFilter}
              onChange={(e) => {
                setActionTypeFilter(e.target.value);
                setCurrentPage(1);
              }}
            >
              <option value="all">操作动作：全部动作</option>
              <option value="CREATE">新建 (CREATE)</option>
              <option value="UPDATE">修改 (UPDATE)</option>
              <option value="DELETE">删除 (DELETE)</option>
              <option value="ENABLE">启用 (ENABLE)</option>
              <option value="DISABLE">禁用 (DISABLE)</option>
              <option value="EXPORT">导出 (EXPORT)</option>
              <option value="GRANT">授权 (GRANT)</option>
              <option value="POLICY_ENFORCE">生效 (POLICY_ENFORCE)</option>
              <option value="DRILL_TEST">演练 (DRILL_TEST)</option>
            </Select>
          </div>

          <div>
            <Select
              value={statusFilter}
              onChange={(e) => {
                setStatusFilter(e.target.value as any);
                setCurrentPage(1);
              }}
            >
              <option value="all">执行结果：全部状态</option>
              <option value="SUCCESS">执行成功 (SUCCESS)</option>
              <option value="FAILED">执行失败 (FAILED)</option>
              <option value="BLOCKED">安全拦截 (BLOCKED)</option>
            </Select>
          </div>
        </div>

        {/* Date Filters Row */}
        <div className="flex flex-col sm:flex-row items-center justify-between gap-3 pt-2 border-t border-slate-100 text-xs text-slate-500">
          <div className="flex items-center gap-2 w-full sm:w-auto">
            <Calendar className="w-4 h-4 text-slate-400" />
            <span>时间范围：</span>
            <input
              type="date"
              value={startDate}
              onChange={(e) => {
                setStartDate(e.target.value);
                setCurrentPage(1);
              }}
              className="px-2.5 py-1 text-xs border border-slate-200 rounded-lg bg-white text-slate-700 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
            <span>至</span>
            <input
              type="date"
              value={endDate}
              onChange={(e) => {
                setEndDate(e.target.value);
                setCurrentPage(1);
              }}
              className="px-2.5 py-1 text-xs border border-slate-200 rounded-lg bg-white text-slate-700 focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          <div className="flex items-center gap-2 self-end sm:self-auto">
            <span>
              已检索到 <strong className="text-slate-800">{filteredLogs.length}</strong> 条记录
            </span>
            {(searchQuery || moduleFilter !== 'all' || actionTypeFilter !== 'all' || statusFilter !== 'all' || startDate || endDate) && (
              <Button
                variant="ghost"
                size="sm"
                onClick={handleResetFilters}
                className="gap-1 text-xs text-slate-500 hover:text-slate-800"
              >
                <RotateCcw className="w-3.5 h-3.5" />
                重置筛选
              </Button>
            )}
          </div>
        </div>
      </Card>

      {/* Operation Logs Table */}
      <Card className="overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-44">操作时间</TableHead>
              <TableHead className="w-48">操作账号 / 角色</TableHead>
              <TableHead className="w-32">业务模块</TableHead>
              <TableHead className="w-32">动作类型</TableHead>
              <TableHead className="min-w-[220px]">行为描述与目标资源</TableHead>
              <TableHead className="w-40">客户端 IP / 位置</TableHead>
              <TableHead className="w-24 text-center">执行结果</TableHead>
              <TableHead className="w-20 text-right">耗时</TableHead>
              <TableHead className="w-20 text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={9} className="py-12 text-center text-slate-400">
                  <div className="flex items-center justify-center gap-2">
                    <div className="w-4 h-4 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
                    <span>正在加载操作审计日志...</span>
                  </div>
                </TableCell>
              </TableRow>
            ) : paginatedLogs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={9} className="py-12 text-center">
                  <EmptyState
                    title="未检索到符合条件的操作审计日志"
                    description="请尝试调整搜索关键词、动作类型或时间范围筛选条件"
                    actionText="重置筛选条件"
                    onAction={handleResetFilters}
                  />

                </TableCell>
              </TableRow>
            ) : (
              paginatedLogs.map((log) => (
                <TableRow key={log.id} className="hover:bg-slate-50/80 transition-colors">
                  {/* Timestamp */}
                  <TableCell className="font-mono text-xs text-slate-600 whitespace-nowrap">
                    {formatDateTime(log.timestamp)}
                  </TableCell>

                  {/* Operator */}
                  <TableCell>
                    <div className="flex flex-col">
                      <span className="text-xs font-semibold text-slate-900">{log.operatorName}</span>
                      <span className="text-[11px] text-slate-500 font-mono">@{log.operatorUsername}</span>
                    </div>
                  </TableCell>

                  {/* Module */}
                  <TableCell>
                    <span className="text-xs text-slate-700 font-medium">{log.module}</span>
                  </TableCell>

                  {/* Action Type */}
                  <TableCell>
                    {getActionTypeBadge(log.actionType)}
                  </TableCell>

                  {/* Action Name & Target */}
                  <TableCell>
                    <div className="flex flex-col gap-0.5">
                      <span className="text-xs font-medium text-slate-900 leading-tight">
                        {log.actionName}
                      </span>
                      <div className="flex items-center gap-1.5 mt-0.5">
                        <span className="text-[11px] font-mono text-slate-500 bg-slate-100 px-1.5 py-0.2 rounded border border-slate-200">
                          {log.targetResource}
                        </span>
                        <span className="text-[10px] text-slate-400 font-mono">
                          {log.requestMethod} {log.requestUri}
                        </span>
                      </div>
                      {log.errorMessage && (
                        <p className="text-[11px] text-rose-600 mt-0.5 leading-snug">
                          {log.errorMessage}
                        </p>
                      )}
                    </div>
                  </TableCell>

                  {/* IP & Location */}
                  <TableCell>
                    <div className="flex flex-col">
                      <span className="font-mono text-xs text-slate-800">{log.ip}</span>
                      <span className="text-[11px] text-slate-500 truncate max-w-[150px]" title={log.location}>
                        {log.location}
                      </span>
                    </div>
                  </TableCell>

                  {/* Status */}
                  <TableCell className="text-center">
                    {getStatusBadge(log.status)}
                  </TableCell>

                  {/* Latency */}
                  <TableCell className="text-right font-mono text-xs text-slate-600 whitespace-nowrap">
                    {log.durationMs}ms
                  </TableCell>

                  {/* Action */}
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setSelectedLog(log)}
                      className="h-7 px-2 text-xs text-blue-600 hover:text-blue-700 hover:bg-blue-50"
                    >
                      <Eye className="w-3.5 h-3.5 mr-1" />
                      快照
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>

        {/* Pagination Bar */}
        {filteredLogs.length > pageSize && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-slate-100 bg-slate-50/50 text-xs text-slate-500">
            <div>
              显示第 {(currentPage - 1) * pageSize + 1} 至{' '}
              {Math.min(currentPage * pageSize, filteredLogs.length)} 条，共{' '}
              {filteredLogs.length} 条
            </div>
            <div className="flex items-center gap-1.5">
              <Button
                variant="outline"
                size="sm"
                disabled={currentPage === 1}
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                className="h-7 text-xs px-2.5"
              >
                上一页
              </Button>
              <span className="text-xs text-slate-600 font-medium px-2">
                {currentPage} / {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={currentPage === totalPages}
                onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                className="h-7 text-xs px-2.5"
              >
                下一页
              </Button>
            </div>
          </div>
        )}
      </Card>

      {/* Log Detail & Snapshot Dialog */}
      {selectedLog && (
        <Dialog
          isOpen={!!selectedLog}
          onClose={() => setSelectedLog(null)}
          title={
            <div className="flex items-center gap-2">
              <span>操作审计记录与参数变更快照</span>
              <span className="text-xs font-mono font-normal text-slate-500">#{selectedLog.id}</span>
            </div>
          }
          maxWidth="2xl"
        >
          <div className="space-y-4">
            {/* Meta Grid */}
            <div className="grid grid-cols-2 gap-3 p-3.5 bg-slate-50 rounded-lg border border-slate-200 text-xs">
              <div>
                <span className="text-slate-400">操作账号：</span>
                <span className="font-semibold text-slate-800 ml-1">
                  {selectedLog.operatorName} (@{selectedLog.operatorUsername})
                </span>
              </div>
              <div>
                <span className="text-slate-400">所属角色：</span>
                <span className="text-slate-700 ml-1 font-medium">{selectedLog.operatorRole}</span>
              </div>
              <div>
                <span className="text-slate-400">业务模块：</span>
                <span className="text-slate-700 ml-1 font-medium">{selectedLog.module}</span>
              </div>
              <div>
                <span className="text-slate-400">操作动作：</span>
                <span className="ml-1">{getActionTypeBadge(selectedLog.actionType)}</span>
              </div>
              <div>
                <span className="text-slate-400">操作时间：</span>
                <span className="font-mono text-slate-700 ml-1">
                  {formatDateTime(selectedLog.timestamp)}
                </span>
              </div>
              <div>
                <span className="text-slate-400">执行结果：</span>
                <span className="ml-1">{getStatusBadge(selectedLog.status)} ({selectedLog.durationMs}ms)</span>
              </div>
              <div>
                <span className="text-slate-400">客户端 IP：</span>
                <span className="font-mono text-slate-700 ml-1">{selectedLog.ip}</span>
              </div>
              <div>
                <span className="text-slate-400">归属地点：</span>
                <span className="text-slate-700 ml-1">{selectedLog.location}</span>
              </div>
              <div className="col-span-2">
                <span className="text-slate-400">终端环境：</span>
                <span className="text-slate-700 ml-1 font-mono text-[11px]">{selectedLog.device}</span>
              </div>
              <div className="col-span-2">
                <span className="text-slate-400">接口 URI：</span>
                <span className="font-mono text-slate-800 bg-white px-2 py-0.5 rounded border border-slate-200 ml-1 text-[11px]">
                  {selectedLog.requestMethod} {selectedLog.requestUri}
                </span>
              </div>
            </div>

            {/* Error or Warning if Blocked */}
            {selectedLog.errorMessage && (
              <div className="p-3 bg-rose-50 border border-rose-200 rounded-lg text-xs text-rose-800 flex items-start gap-2">
                <AlertTriangle className="w-4 h-4 text-rose-600 shrink-0 mt-0.5" />
                <div>
                  <strong className="font-semibold">阻断或失败原因：</strong>
                  <p className="mt-0.5 leading-relaxed">{selectedLog.errorMessage}</p>
                </div>
              </div>
            )}

            {/* Diff Snapshot View */}
            <div>
              <div className="flex items-center gap-1.5 text-xs font-semibold text-slate-800 mb-2">
                <Code2 className="w-4 h-4 text-blue-600" />
                <span>参数变更快照 (Diff Snapshot)</span>
              </div>

              {selectedLog.diffSnapshot ? (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs font-mono">
                  {/* Before */}
                  <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 overflow-x-auto max-h-60">
                    <div className="text-[11px] font-bold text-slate-500 mb-1.5 pb-1 border-b border-slate-200 flex items-center justify-between">
                      <span>变更前状态 (Before)</span>
                      <span className="text-slate-400 font-normal">历史版本</span>
                    </div>
                    {selectedLog.diffSnapshot.before ? (
                      <pre className="text-slate-700 leading-relaxed text-[11px]">
                        {JSON.stringify(selectedLog.diffSnapshot.before, null, 2)}
                      </pre>
                    ) : (
                      <span className="text-slate-400 italic">【初次创建，无前序记录】</span>
                    )}
                  </div>

                  {/* After */}
                  <div className="p-3 bg-emerald-50/50 rounded-lg border border-emerald-200 overflow-x-auto max-h-60">
                    <div className="text-[11px] font-bold text-emerald-800 mb-1.5 pb-1 border-b border-emerald-200 flex items-center justify-between">
                      <span>变更后状态 (After)</span>
                      <span className="text-emerald-600 font-normal">生效版本</span>
                    </div>
                    {selectedLog.diffSnapshot.after ? (
                      <pre className="text-emerald-900 leading-relaxed text-[11px]">
                        {JSON.stringify(selectedLog.diffSnapshot.after, null, 2)}
                      </pre>
                    ) : (
                      <span className="text-slate-400 italic">【资源已销毁删除】</span>
                    )}
                  </div>
                </div>
              ) : (
                <div className="p-4 text-center text-xs text-slate-400 bg-slate-50 rounded-lg border border-slate-200">
                  该操作未提交复杂结构体变更参数
                </div>
              )}
            </div>

            <div className="flex justify-end pt-2 border-t border-slate-100">
              <Button variant="outline" size="sm" onClick={() => setSelectedLog(null)}>
                关闭
              </Button>
            </div>
          </div>
        </Dialog>
      )}

      {/* Clear Logs Confirm Dialog */}
      <ConfirmDialog
        isOpen={showClearConfirm}
        title="清空操作审计日志"
        description="确定要清空当前的系统操作审计记录吗？清空后将生成审计归档快照，本操作不可撤销。"
        confirmText="确认清空并归档"
        danger
        onConfirm={handleClearLogs}
        onClose={() => setShowClearConfirm(false)}
      />
    </div>
  );
};

export default OperationLogs;
