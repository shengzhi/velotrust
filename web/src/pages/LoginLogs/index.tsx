import React, { useState, useEffect, useMemo } from 'react';
import {
  History,
  Search,
  Download,
  Trash2,
  RefreshCw,
  ShieldAlert,
  ShieldCheck,
  CheckCircle2,
  AlertTriangle,
  Lock,
  Smartphone,
  Laptop,
  Globe,
  Terminal,
  KeyRound,
  Fingerprint,
  RotateCcw,
  Sparkles,
  ExternalLink,
  ChevronRight,
  Filter,
  Eye,
  Calendar,
} from 'lucide-react';
import { mockApi } from '../../data/mock';
import { LoginLog, LoginAuthType, LoginStatus } from '../../types';
import { useAuth } from '../../contexts/AuthContext';
import { useToast } from '../../components/ui/toast';
import { Dialog } from '../../components/ui/dialog';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Select } from '../../components/ui/select';
import { Card } from '../../components/ui/card';
import { EmptyState } from '../../components/EmptyState';
import { formatDateTime } from '../../utils/format';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '../../components/ui/table';

const AUTH_TYPE_LABELS: Record<LoginAuthType, { label: string; badgeClass: string }> = {
  password: { label: '账号密码', badgeClass: 'bg-slate-100 text-slate-700 border-slate-200' },
  idp_feishu: { label: '飞书 SSO', badgeClass: 'bg-blue-50 text-blue-700 border-blue-200' },
  idp_dingtalk: { label: '钉钉 SSO', badgeClass: 'bg-amber-50 text-amber-700 border-amber-200' },
  idp_okta: { label: 'Okta SAML', badgeClass: 'bg-indigo-50 text-indigo-700 border-indigo-200' },
  mfa_totp: { label: 'MFA 动态码', badgeClass: 'bg-purple-50 text-purple-700 border-purple-200' },
  wireguard_cert: { label: 'WG客户端证书', badgeClass: 'bg-emerald-50 text-emerald-700 border-emerald-200' },
};

const STATUS_CONFIG: Record<LoginStatus, { label: string; badgeClass: string; icon: React.ElementType }> = {
  success: {
    label: '登录成功',
    badgeClass: 'bg-emerald-50 text-emerald-700 border-emerald-200',
    icon: CheckCircle2,
  },
  failed: {
    label: '密码错误',
    badgeClass: 'bg-amber-50 text-amber-700 border-amber-200',
    icon: AlertTriangle,
  },
  locked: {
    label: '账号锁定',
    badgeClass: 'bg-rose-50 text-rose-700 border-rose-200',
    icon: Lock,
  },
  mfa_rejected: {
    label: 'MFA核验拒绝',
    badgeClass: 'bg-orange-50 text-orange-700 border-orange-200',
    icon: ShieldAlert,
  },
  risk_blocked: {
    label: '高危阻断拦截',
    badgeClass: 'bg-red-50 text-red-700 border-red-200 font-bold',
    icon: ShieldAlert,
  },
};

export const LoginLogs: React.FC = () => {
  const { addToast } = useToast();

  const [logs, setLogs] = useState<LoginLog[]>([]);
  const [loading, setLoading] = useState(true);

  // Filters
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedType, setSelectedType] = useState<'all' | LoginAuthType>('all');
  const [selectedStatus, setSelectedStatus] = useState<'all' | LoginStatus>('all');
  const [riskFilter, setRiskFilter] = useState<'all' | 'high' | 'medium' | 'low'>('all');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');

  // Selected Log Detail Modal
  const [selectedLog, setSelectedLog] = useState<LoginLog | null>(null);

  // Clear Confirmation Modal
  const [showClearConfirm, setShowClearConfirm] = useState(false);

  // Pagination
  const [currentPage, setCurrentPage] = useState(1);
  const pageSize = 15;

  const loadLogs = async () => {
    try {
      setLoading(true);
      const data = await mockApi.getLoginLogs();
      setLogs(data);
    } catch {
      addToast('加载登录日志失败', 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadLogs();
  }, []);

  // Filter logic
  const filteredLogs = useMemo(() => {
    return logs.filter((log) => {
      const q = searchQuery.toLowerCase().trim();
      const matchSearch =
        !q ||
        log.username.toLowerCase().includes(q) ||
        log.name.toLowerCase().includes(q) ||
        log.ip.includes(q) ||
        log.location.toLowerCase().includes(q) ||
        log.message.toLowerCase().includes(q);

      const matchType = selectedType === 'all' || log.loginType === selectedType;
      const matchStatus = selectedStatus === 'all' || log.status === selectedStatus;

      let matchRisk = true;
      if (riskFilter === 'high') matchRisk = log.riskScore >= 70;
      else if (riskFilter === 'medium') matchRisk = log.riskScore >= 30 && log.riskScore < 70;
      else if (riskFilter === 'low') matchRisk = log.riskScore < 30;

      let matchDate = true;
      if (startDate) {
        matchDate = matchDate && new Date(log.timestamp) >= new Date(startDate);
      }
      if (endDate) {
        const end = new Date(endDate);
        end.setHours(23, 59, 59, 999);
        matchDate = matchDate && new Date(log.timestamp) <= end;
      }

      return matchSearch && matchType && matchStatus && matchRisk && matchDate;
    });
  }, [logs, searchQuery, selectedType, selectedStatus, riskFilter, startDate, endDate]);

  // Paginated records
  const totalPages = Math.ceil(filteredLogs.length / pageSize) || 1;
  const paginatedLogs = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredLogs.slice(start, start + pageSize);
  }, [filteredLogs, currentPage, pageSize]);

  const handleExportCsv = () => {
    if (filteredLogs.length === 0) {
      addToast('当前没有可导出的日志数据', 'warning');
      return;
    }
    const headers = [
      '日志编号',
      '时间戳',
      '登录账号',
      '用户姓名',
      '认证方式',
      '认证状态',
      '客户端IP',
      '地理位置',
      '终端设备',
      '浏览器UA',
      '风险分值',
      'MFA双因子',
      '审计消息',
    ];

    const rows = filteredLogs.map((l) => [
      l.id,
      l.timestamp,
      l.username,
      l.name,
      l.loginType,
      l.status,
      l.ip,
      `"${l.location}"`,
      `"${l.device}"`,
      `"${l.browser}"`,
      l.riskScore,
      l.mfaUsed ? '是' : '否',
      `"${l.message.replace(/"/g, '""')}"`,
    ]);

    const csvContent =
      '\uFEFF' + [headers.join(','), ...rows.map((e) => e.join(','))].join('\n');
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `login_audit_logs_${new Date().toISOString().slice(0, 10)}.csv`;
    link.click();
    URL.revokeObjectURL(url);
    addToast(`已导出 ${filteredLogs.length} 条登录审计记录为 CSV`, 'success');
  };

  const handleClearLogs = async () => {
    try {
      await mockApi.clearLoginLogs();
      setLogs([]);
      setShowClearConfirm(false);
      addToast('登录审计日志已彻底清空并生成归档记录', 'success');
    } catch {
      addToast('清空日志失败', 'error');
    }
  };

  const handleResetFilters = () => {
    setSearchQuery('');
    setSelectedType('all');
    setSelectedStatus('all');
    setRiskFilter('all');
    setStartDate('');
    setEndDate('');
    setCurrentPage(1);
  };

  const triggerSimulatedLogin = async (type: LoginStatus) => {
    try {
      let message = '管理员控制台正常登录验证通过';
      let risk = 10;
      let mfa = true;
      let ip = '192.168.1.100';
      let location = '北京 · 海淀区 [办公专线内网]';

      if (type === 'failed') {
        message = '连续输入错误凭据口令，触发单日密码错误告警';
        risk = 45;
        mfa = false;
      } else if (type === 'mfa_rejected') {
        message = 'TOTP 动态密码超时或私钥不匹配，MFA 核验失败被拒';
        risk = 60;
        mfa = false;
      } else if (type === 'risk_blocked') {
        message = '命中匿名 Tor 代理或境外高危黑名单网段，自适应零信任风控即时阻断';
        risk = 95;
        mfa = false;
        ip = '185.220.101.5';
        location = '德国 · 汉堡 [Tor Exit Node]';
      }

      const item = await mockApi.addLoginLog({
        status: type,
        message,
        riskScore: risk,
        mfaUsed: mfa,
        ip,
        location,
      });

      setLogs((prev) => [item, ...prev]);
      addToast(`已模拟触发【${STATUS_CONFIG[type].label}】事件并即时留痕`, 'info');
    } catch {
      addToast('模拟登录事件失败', 'error');
    }
  };

  return (
    <div className="space-y-5">
      {/* Clean Header - Unified with System Default Theme */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl font-bold text-slate-900 tracking-tight">登录审计日志</h1>
            <Badge variant="outline" className="text-amber-700 border-amber-200 bg-amber-50 text-xs">
              等保三级 · 身份认证留痕
            </Badge>
          </div>
          <p className="text-xs text-slate-500 mt-0.5">
            全量记录系统用户与管理员的控制平面身份认证、双因子校验与异常登录阻断事件
          </p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          {/* Quick Simulation Dropdown */}
          <div className="relative group">
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5 shadow-xs"
            >
              <Sparkles className="h-3.5 w-3.5 text-blue-600" />
              <span>模拟登录事件</span>
            </Button>
            <div className="absolute right-0 top-full mt-1 w-48 bg-white border border-slate-200 rounded-lg shadow-lg p-1.5 hidden group-hover:block z-30">
              <button
                onClick={() => triggerSimulatedLogin('success')}
                className="w-full text-left px-3 py-1.5 text-xs text-slate-700 hover:bg-slate-50 rounded flex items-center gap-2 transition-colors"
              >
                <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />
                模拟正常成功登录
              </button>
              <button
                onClick={() => triggerSimulatedLogin('failed')}
                className="w-full text-left px-3 py-1.5 text-xs text-slate-700 hover:bg-slate-50 rounded flex items-center gap-2 transition-colors"
              >
                <AlertTriangle className="h-3.5 w-3.5 text-amber-600" />
                模拟密码错误告警
              </button>
              <button
                onClick={() => triggerSimulatedLogin('mfa_rejected')}
                className="w-full text-left px-3 py-1.5 text-xs text-slate-700 hover:bg-slate-50 rounded flex items-center gap-2 transition-colors"
              >
                <ShieldAlert className="h-3.5 w-3.5 text-orange-600" />
                模拟 MFA 校验失败
              </button>
              <button
                onClick={() => triggerSimulatedLogin('risk_blocked')}
                className="w-full text-left px-3 py-1.5 text-xs text-slate-700 hover:bg-slate-50 rounded flex items-center gap-2 transition-colors"
              >
                <Lock className="h-3.5 w-3.5 text-rose-600" />
                模拟境外高危 IP 拦截
              </button>
            </div>
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={handleExportCsv}
            className="gap-1.5 shadow-xs"
          >
            <Download className="h-3.5 w-3.5 text-slate-600" />
            <span>导出 CSV</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={() => setShowClearConfirm(true)}
            className="gap-1.5 shadow-xs text-rose-600 border-rose-200 hover:bg-rose-50"
          >
            <Trash2 className="h-3.5 w-3.5" />
            <span>清空日志</span>
          </Button>
        </div>
      </div>

      {/* Filter and Search Bar */}
      <Card className="p-4 space-y-3">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          <Input
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              setCurrentPage(1);
            }}
            placeholder="搜索账号、姓名、IP地址或提示信息..."
            leftIcon={<Search className="w-4 h-4 text-slate-400" />}
          />

          <Select
            value={selectedType}
            onChange={(e) => {
              setSelectedType(e.target.value as any);
              setCurrentPage(1);
            }}
          >
            <option value="all">认证方式：全部方式</option>
            <option value="password">账号密码</option>
            <option value="idp_feishu">飞书 SSO</option>
            <option value="idp_dingtalk">钉钉 SSO</option>
            <option value="idp_okta">Okta SAML</option>
            <option value="mfa_totp">TOTP 动态令牌</option>
            <option value="wireguard_cert">WireGuard 客户端证书</option>
          </Select>

          <Select
            value={selectedStatus}
            onChange={(e) => {
              setSelectedStatus(e.target.value as any);
              setCurrentPage(1);
            }}
          >
            <option value="all">认证状态：全部状态</option>
            <option value="success">登录成功 (success)</option>
            <option value="failed">口令错误 (failed)</option>
            <option value="mfa_rejected">MFA失败 (mfa_rejected)</option>
            <option value="risk_blocked">高危阻断 (risk_blocked)</option>
            <option value="locked">账号锁定 (locked)</option>
          </Select>

          <Select
            value={riskFilter}
            onChange={(e) => {
              setRiskFilter(e.target.value as any);
              setCurrentPage(1);
            }}
          >
            <option value="all">风险评估：全部评级</option>
            <option value="low">低风险 (0-29)</option>
            <option value="medium">中风险 (30-69)</option>
            <option value="high">高风险 (70-100)</option>
          </Select>
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
            {(searchQuery || selectedType !== 'all' || selectedStatus !== 'all' || riskFilter !== 'all' || startDate || endDate) && (
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

      {/* Login Logs Table */}
      <Card className="overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-44">登录时间</TableHead>
              <TableHead className="w-44">登录账号 / 姓名</TableHead>
              <TableHead className="w-32">认证方式</TableHead>
              <TableHead className="w-28 text-center">状态</TableHead>
              <TableHead className="w-48">客户端 IP / 归属地</TableHead>
              <TableHead className="w-40">终端设备 / 浏览器</TableHead>
              <TableHead className="w-24 text-center">风险分值</TableHead>
              <TableHead className="min-w-[200px]">安全审计留痕与诊断消息</TableHead>
              <TableHead className="w-20 text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={9} className="py-12 text-center text-slate-400">
                  <div className="flex items-center justify-center gap-2">
                    <div className="w-4 h-4 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
                    <span>正在载入登录留痕流水...</span>
                  </div>
                </TableCell>
              </TableRow>
            ) : paginatedLogs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={9} className="py-12 text-center">
                  <EmptyState
                    title="未检索到符合条件的登录留痕"
                    description="请尝试调整搜索关键词、认证方式或风险等级筛选条件"
                    actionText="重置搜索"
                    onAction={handleResetFilters}
                  />

                </TableCell>
              </TableRow>
            ) : (
              paginatedLogs.map((log) => {
                const statusMeta = STATUS_CONFIG[log.status] || STATUS_CONFIG.failed;
                const StatusIcon = statusMeta.icon;
                const authMeta = AUTH_TYPE_LABELS[log.loginType] || AUTH_TYPE_LABELS.password;

                return (
                  <TableRow
                    key={log.id}
                    className="hover:bg-slate-50/80 transition-colors group"
                  >
                    {/* Timestamp */}
                    <TableCell className="font-mono text-xs text-slate-600 whitespace-nowrap">
                      {formatDateTime(log.timestamp)}
                    </TableCell>

                    {/* Account & Name */}
                    <TableCell>
                      <div className="flex flex-col">
                        <span className="text-xs font-semibold text-slate-900">{log.name}</span>
                        <span className="text-[11px] font-mono text-slate-500">@{log.username}</span>
                      </div>
                    </TableCell>

                    {/* Auth Method */}
                    <TableCell>
                      <Badge variant="outline" className={authMeta.badgeClass}>
                        {authMeta.label}
                      </Badge>
                    </TableCell>

                    {/* Status Badge */}
                    <TableCell className="text-center">
                      <span
                        className={`inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded border font-medium ${statusMeta.badgeClass}`}
                      >
                        <StatusIcon className="h-3 w-3" />
                        {statusMeta.label}
                      </span>
                    </TableCell>

                    {/* IP & Location */}
                    <TableCell>
                      <div className="flex flex-col">
                        <span className="font-mono text-xs text-slate-800">{log.ip}</span>
                        <span className="text-[11px] text-slate-500 truncate max-w-[170px]" title={log.location}>
                          {log.location}
                        </span>
                      </div>
                    </TableCell>

                    {/* Device & Browser */}
                    <TableCell>
                      <div className="flex flex-col">
                        <span className="text-xs text-slate-700 truncate max-w-[140px]" title={log.device}>
                          {log.device}
                        </span>
                        <span className="text-[10px] text-slate-400 font-mono truncate max-w-[140px]" title={log.browser}>
                          {log.browser}
                        </span>
                      </div>
                    </TableCell>

                    {/* Risk Score */}
                    <TableCell className="text-center">
                      <span
                        className={`font-mono text-xs px-2 py-0.5 rounded font-bold border ${
                          log.riskScore >= 70
                            ? 'text-rose-700 bg-rose-50 border-rose-200'
                            : log.riskScore >= 30
                            ? 'text-amber-700 bg-amber-50 border-amber-200'
                            : 'text-emerald-700 bg-emerald-50 border-emerald-200'
                        }`}
                      >
                        {log.riskScore}
                      </span>
                    </TableCell>

                    {/* Security Message */}
                    <TableCell>
                      <p className="text-xs text-slate-600 line-clamp-2 leading-relaxed" title={log.message}>
                        {log.message}
                      </p>
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
                        详情
                      </Button>
                    </TableCell>
                  </TableRow>
                );
              })
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

      {/* Log Detail Modal */}
      {selectedLog && (
        <Dialog
          isOpen={!!selectedLog}
          onClose={() => setSelectedLog(null)}
          title={
            <div className="flex items-center gap-2">
              <span>登录审计留痕详细档案</span>
              <span className="text-xs font-mono font-normal text-slate-500">#{selectedLog.id}</span>
            </div>
          }
          maxWidth="2xl"
        >
          <div className="space-y-4 text-xs">
            {/* Meta Grid */}
            <div className="grid grid-cols-2 gap-3 p-3.5 bg-slate-50 rounded-lg border border-slate-200">
              <div>
                <span className="text-slate-400">登录账号：</span>
                <span className="font-semibold text-slate-900 ml-1">
                  {selectedLog.name} (@{selectedLog.username})
                </span>
              </div>
              <div>
                <span className="text-slate-400">登录状态：</span>
                <span className="ml-1 font-semibold text-slate-800">
                  {STATUS_CONFIG[selectedLog.status].label}
                </span>
              </div>
              <div>
                <span className="text-slate-400">认证协议方式：</span>
                <span className="ml-1 font-medium text-slate-800">
                  {AUTH_TYPE_LABELS[selectedLog.loginType].label}
                </span>
              </div>
              <div>
                <span className="text-slate-400">双因子 (MFA)：</span>
                <span className="ml-1 font-medium text-slate-800">
                  {selectedLog.mfaUsed ? '已核验并通过 TOTP/WebAuthn' : '未开启或未完成核验'}
                </span>
              </div>
              <div>
                <span className="text-slate-400">操作时间：</span>
                <span className="font-mono text-slate-700 ml-1">
                  {formatDateTime(selectedLog.timestamp)}
                </span>
              </div>
              <div>
                <span className="text-slate-400">风险评分：</span>
                <span className="font-mono font-bold text-slate-900 ml-1">
                  {selectedLog.riskScore} / 100
                </span>
              </div>
              <div>
                <span className="text-slate-400">客户端 IP：</span>
                <span className="font-mono text-slate-800 ml-1">{selectedLog.ip}</span>
              </div>
              <div>
                <span className="text-slate-400">地理位置归属：</span>
                <span className="text-slate-800 ml-1">{selectedLog.location}</span>
              </div>
              <div className="col-span-2">
                <span className="text-slate-400">终端环境：</span>
                <span className="text-slate-800 ml-1 font-mono">{selectedLog.device}</span>
              </div>
              <div className="col-span-2">
                <span className="text-slate-400">用户代理 (UA)：</span>
                <span className="text-slate-800 ml-1 font-mono text-[11px]">{selectedLog.browser}</span>
              </div>
              <div className="col-span-2">
                <span className="text-slate-400">会话跟踪 Session ID：</span>
                <span className="font-mono text-slate-800 ml-1 bg-white px-2 py-0.5 rounded border border-slate-200">
                  {selectedLog.sessionId}
                </span>
              </div>
            </div>

            {/* Audit Message */}
            <div className="p-3 bg-slate-50 border border-slate-200 rounded-lg">
              <span className="font-semibold text-slate-700 block mb-1">系统审计留痕诊断摘要：</span>
              <p className="text-slate-600 leading-relaxed">{selectedLog.message}</p>
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
        title="清空登录审计日志"
        description="确定要清空当前的系统登录审计日志吗？清空后将生成合规归档快照，本操作不可撤销。"
        confirmText="确认清空并归档"
        danger
        onConfirm={handleClearLogs}
        onClose={() => setShowClearConfirm(false)}
      />
    </div>
  );
};
export default LoginLogs;

