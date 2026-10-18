import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Server,
  CheckCircle,
  XCircle,
  Users,
  Monitor,
  Globe,
  RefreshCw,
  ArrowRight,
  ShieldAlert,
  Layers,
  Compass,
  Fingerprint,
  ShieldCheck,
} from 'lucide-react';
import { mockApi } from '../data/mock';
import { DashboardStats } from '../types';
import { StatCard } from '../components/StatCard';
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card';
import { PieChart } from '../components/PieChart';
import { LineChart } from '../components/LineChart';
import { Table, TableHeader, TableBody, TableHead, TableRow, TableCell } from '../components/ui/table';
import { StatusBadge } from '../components/StatusBadge';
import { Button } from '../components/ui/button';
import { formatBytes, formatRelativeTime } from '../utils/format';

export const Dashboard: React.FC = () => {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const navigate = useNavigate();

  const loadData = useCallback(async (showRefreshing = false) => {
    if (showRefreshing) setIsRefreshing(true);
    try {
      const data = await mockApi.getDashboardStats();
      setStats(data);
      setLastUpdated(new Date());
    } finally {
      setLoading(false);
      setIsRefreshing(false);
    }
  }, []);

  useEffect(() => {
    loadData();
    // 30 seconds auto-refresh interval
    const interval = setInterval(() => {
      loadData();
    }, 30000);
    return () => clearInterval(interval);
  }, [loadData]);

  if (loading || !stats) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="flex flex-col items-center gap-3">
          <RefreshCw className="w-8 h-8 text-blue-600 animate-spin" />
          <p className="text-sm text-slate-500 font-medium">正在加载 Mesh 控制平面监控数据...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Top Banner / Actions */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">控制台概览</h1>
          <p className="text-xs text-slate-500 mt-0.5">
            WireGuard 骨干专网实时状态 · 每 30 秒自动同步
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
            size="sm"
            onClick={() => navigate('/topology')}
            className="gap-1.5"
          >
            <span>全景拓扑</span>
            <ArrowRight className="w-3.5 h-3.5" />
          </Button>
        </div>
      </div>

      {/* 6 Statistic Cards Grid */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3.5">
        <StatCard
          title="节点总数"
          value={stats.totalNodes}
          icon={<Server className="w-4 h-4" />}
          color="blue"
          onClick={() => navigate('/nodes')}
        />
        <StatCard
          title="在线节点"
          value={stats.onlineNodes}
          icon={<CheckCircle className="w-4 h-4" />}
          color="green"
          onClick={() => navigate('/nodes?status=online')}
        />
        <StatCard
          title="离线节点"
          value={stats.offlineNodes}
          icon={<XCircle className="w-4 h-4" />}
          color="gray"
          onClick={() => navigate('/nodes?status=offline')}
        />
        <StatCard
          title="授权用户"
          value={stats.totalUsers}
          icon={<Users className="w-4 h-4" />}
          color="purple"
          onClick={() => navigate('/users')}
        />
        <StatCard
          title="纳管设备"
          value={stats.totalDevices}
          icon={<Monitor className="w-4 h-4" />}
          color="orange"
          onClick={() => navigate('/devices')}
        />
        <StatCard
          title="网络站点"
          value={stats.totalSites}
          icon={<Globe className="w-4 h-4" />}
          color="cyan"
          onClick={() => navigate('/sites')}
        />
      </div>

      {/* Quick Nav Feature Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3.5">
        <div
          onClick={() => navigate('/ipam')}
          className="bg-white p-4 rounded-xl border border-slate-200 hover:border-blue-400 hover:shadow-xs transition-all cursor-pointer group"
        >
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <div className="h-7 w-7 rounded-lg bg-blue-50 text-blue-600 flex items-center justify-center group-hover:bg-blue-600 group-hover:text-white transition-colors">
                <Layers className="h-4 w-4" />
              </div>
              <span className="text-xs font-semibold text-slate-800">IPAM 网段规划</span>
            </div>
            <span className="text-[11px] font-mono text-slate-400">10.100.0.0/16</span>
          </div>
          <p className="text-xs text-slate-500">
            虚拟大网段层级划分、子网池管理与节点租约绑定
          </p>
        </div>

        <div
          onClick={() => navigate('/dns')}
          className="bg-white p-4 rounded-xl border border-slate-200 hover:border-emerald-400 hover:shadow-xs transition-all cursor-pointer group"
        >
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <div className="h-7 w-7 rounded-lg bg-emerald-50 text-emerald-600 flex items-center justify-center group-hover:bg-emerald-600 group-hover:text-white transition-colors">
                <Compass className="h-4 w-4" />
              </div>
              <span className="text-xs font-semibold text-slate-800">内网 Split DNS</span>
            </div>
            <span className="text-[11px] text-slate-400">3 个活跃域</span>
          </div>
          <p className="text-xs text-slate-500">
            内部域名加密隧道解析与视界隔离防护
          </p>
        </div>

        <div
          onClick={() => navigate('/access-matrix')}
          className="bg-white p-4 rounded-xl border border-slate-200 hover:border-purple-400 hover:shadow-xs transition-all cursor-pointer group"
        >
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <div className="h-7 w-7 rounded-lg bg-purple-50 text-purple-600 flex items-center justify-center group-hover:bg-purple-600 group-hover:text-white transition-colors">
                <ShieldCheck className="h-4 w-4" />
              </div>
              <span className="text-xs font-semibold text-slate-800">访问权限透视</span>
            </div>
            <span className="text-[11px] text-slate-400">可达性透视</span>
          </div>
          <p className="text-xs text-slate-500">
            多维度主体权限穿透、服务白名单与链路验证
          </p>
        </div>

        <div
          onClick={() => navigate('/idp')}
          className="bg-white p-4 rounded-xl border border-slate-200 hover:border-indigo-400 hover:shadow-xs transition-all cursor-pointer group"
        >
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <div className="h-7 w-7 rounded-lg bg-indigo-50 text-indigo-600 flex items-center justify-center group-hover:bg-indigo-600 group-hover:text-white transition-colors">
                <Fingerprint className="h-4 w-4" />
              </div>
              <span className="text-xs font-semibold text-slate-800">身份源集成</span>
            </div>
            <span className="text-[11px] text-slate-400">OIDC / SAML</span>
          </div>
          <p className="text-xs text-slate-500">
            飞书、钉钉、Okta 联合认证与动态策略下发
          </p>
        </div>
      </div>

      {/* Charts Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Left: Node status distribution pie chart */}
        <Card>
          <CardHeader>
            <CardTitle>节点健康与状态分布</CardTitle>
            <p className="text-xs text-slate-500">当前注册节点在线与降级率统计</p>
          </CardHeader>
          <CardContent>
            <PieChart data={stats.statusDistribution} />
          </CardContent>
        </Card>

        {/* Right: Traffic trends */}
        <Card>
          <CardHeader>
            <CardTitle>Mesh 网络吞吐趋势 (24小时)</CardTitle>
            <p className="text-xs text-slate-500">全网 WireGuard 隧道双向流量实时波动</p>
          </CardHeader>
          <CardContent>
            <LineChart data={stats.trafficTrend} />
          </CardContent>
        </Card>
      </div>

      {/* Recent Audit Events Table */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>最近安全审计事件</CardTitle>
            <p className="text-xs text-slate-500">最近 10 条防火墙 ACL 与端点接入动作记录</p>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => navigate('/audit')}
            className="text-xs text-blue-600 hover:text-blue-700 font-medium"
          >
            查看全部审计日志 →
          </Button>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>时间</TableHead>
                <TableHead>发起用户</TableHead>
                <TableHead>设备</TableHead>
                <TableHead>途径节点</TableHead>
                <TableHead>源 IP → 目标 IP</TableHead>
                <TableHead>端口</TableHead>
                <TableHead>动作</TableHead>
                <TableHead>流量大小</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {stats.recentAudit.map((record) => (
                <TableRow key={record.id}>
                  <TableCell className="font-mono text-xs text-slate-500">
                    {formatRelativeTime(record.timestamp)}
                  </TableCell>
                  <TableCell className="font-medium text-slate-900">
                    {record.user}
                  </TableCell>
                  <TableCell className="text-slate-600 text-xs">
                    {record.device}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-600">
                    {record.node}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-700">
                    {record.sourceIp} → {record.destIp}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-600">
                    {record.port}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={record.action} />
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-600">
                    {formatBytes(record.bytes)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
};
