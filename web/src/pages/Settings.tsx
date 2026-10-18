import React, { useState, useEffect } from 'react';
import {
  ShieldAlert,
  ShieldCheck,
  AlertTriangle,
  PlayCircle,
  Clock,
  Database,
  RefreshCw,
  Server,
  CheckCircle2,
  KeyRound,
  Flame,
  Sliders,
  ArrowLeftRight,
  Cpu,
} from 'lucide-react';
import { mockApi } from '../data/mock';
import { FailSafeConfig } from '../types';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { Card } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { useToast } from '../components/ui/toast';

export const Settings: React.FC = () => {
  const { success, error, info } = useToast();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [drilling, setDrilling] = useState(false);
  const [drillOutput, setDrillOutput] = useState<string[]>([]);
  const [config, setConfig] = useState<FailSafeConfig>({
    heartbeatIntervalSec: 15,
    offlineThresholdSec: 60,
    mode: 'ALLOW_EXISTING_CACHE',
    localCacheTtlHours: 72,
    deadMansSwitchEnabled: true,
    deadMansTimeoutMinutes: 30,
    emergencyAdminBypass: true,
    autoRecovery: true,
    lastSyncAt: new Date().toISOString(),
    gatewaysInFailSafe: 0,
    lastDrillRun: '尚未执行演练',
  });

  useEffect(() => {
    fetchConfig();
  }, []);

  const fetchConfig = async () => {
    try {
      setLoading(true);
      const res = await mockApi.getFailSafeConfig();
      setConfig(res);
    } catch {
      error('加载灾备配置失败');
    } finally {
      setLoading(false);
    }
  };

  const handleSaveConfig = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setSaving(true);
      const updated = await mockApi.updateFailSafeConfig(config);
      setConfig(updated);
      success('系统全局灾备与逃生参数已更新并推送到全球网关节点！');
    } catch {
      error('保存配置失败');
    } finally {
      setSaving(false);
    }
  };

  const handleRunDrill = async () => {
    setDrilling(true);
    setDrillOutput([
      `[${new Date().toLocaleTimeString()}] 🚀 发起控制面失联逃生演练 (Chaos Simulation Drill)...`,
      `[${new Date().toLocaleTimeString()}] 📡 模拟向全球网关群发下线心跳中断探测报文...`,
    ]);

    setTimeout(() => {
      setDrillOutput((prev) => [
        ...prev,
        `[${new Date().toLocaleTimeString()}] ⚠️ 网关 gw-bj-hub-01 触发死信计时器 (Dead Man's Switch Activated)`,
        `[${new Date().toLocaleTimeString()}] 🛡️ 启用本地 /var/lib/velotrust/offline_acl_cache.db 规则快照`,
        `[${new Date().toLocaleTimeString()}] 🛑 拒绝一切新终端设备握手 (New Handshake REJECT)`,
        `[${new Date().toLocaleTimeString()}] 🔄 保持 14 个在线核心长连接隧道正常直通`,
      ]);
    }, 600);

    setTimeout(() => {
      setDrillOutput((prev) => [
        ...prev,
        `[${new Date().toLocaleTimeString()}] 🌐 网关 gw-sh-branch-01 (腾讯云 NAT) 与 gw-sz-branch-01 均成功进入离线自决模式`,
        `[${new Date().toLocaleTimeString()}] ✅ 演练成功完成：控制面中断情况下无任何未授权渗透，现有业务连续性保障率 100%`,
      ]);
      setDrilling(false);
      mockApi.triggerFailSafeDrill().then((res) => {
        setConfig((prev) => ({ ...prev, lastDrillRun: res.log }));
        success('网关离线断开逃生演练完成，数据面自愈表现优异！');
      });
    }, 1400);
  };

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">系统与容灾设置</h1>
          <p className="text-xs text-slate-500 mt-0.5">
            配置控制面失联时网关防御行为、容灾熔断阈值与本地缓存保护机制
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={fetchConfig}
            disabled={loading}
            className="text-xs"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1 ${loading ? 'animate-spin' : ''}`} />
            刷新状态
          </Button>
          <Button
            variant="primary"
            size="sm"
            onClick={handleRunDrill}
            disabled={drilling}
            className="text-xs bg-amber-600 hover:bg-amber-700 text-white border-amber-700"
          >
            <PlayCircle className={`w-3.5 h-3.5 mr-1 ${drilling ? 'animate-spin' : ''}`} />
            {drilling ? '演练执行中...' : '发起离线逃生演练'}
          </Button>
        </div>
      </div>

      {/* Overview Metric Banner */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card className="p-4 border-slate-200 bg-white shadow-xs space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs text-slate-500 font-medium">死信定时器 (Switch)</span>
            <span className="h-2 w-2 rounded-full bg-emerald-500 ring-4 ring-emerald-100" />
          </div>
          <div className="text-lg font-bold text-slate-900 flex items-center gap-1.5">
            <Clock className="w-5 h-5 text-indigo-600" />
            {config.deadMansSwitchEnabled ? `${config.deadMansTimeoutMinutes} 分钟` : '已禁用'}
          </div>
          <p className="text-[11px] text-slate-500">超阈值失联将强行激活隔离模式</p>
        </Card>

        <Card className="p-4 border-slate-200 bg-white shadow-xs space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs text-slate-500 font-medium">离线自决模式</span>
            <Badge variant="outline" className="text-[10px] font-mono text-blue-700 bg-blue-50 border-blue-200">
              Active
            </Badge>
          </div>
          <div className="text-sm font-bold text-slate-900 truncate">
            {config.mode === 'ALLOW_EXISTING_CACHE' && '保存量 / 阻新连 (推荐)'}
            {config.mode === 'BLOCK_ALL_EMERGENCY' && '全面熔断隔离 (极度严密)'}
            {config.mode === 'READ_ONLY_FALLBACK' && '降级只读模式 (核心放行)'}
          </div>
          <p className="text-[11px] text-slate-500">控制面板挂掉后的网关数据面规则</p>
        </Card>

        <Card className="p-4 border-slate-200 bg-white shadow-xs space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs text-slate-500 font-medium">本地旁路 ACL 缓存</span>
            <CheckCircle2 className="w-4 h-4 text-emerald-600" />
          </div>
          <div className="text-lg font-bold text-slate-900 flex items-center gap-1.5">
            <Database className="w-5 h-5 text-emerald-600" />
            {config.localCacheTtlHours} 小时有效
          </div>
          <p className="text-[11px] text-slate-500">网关本地持久化安全黑白名单</p>
        </Card>

        <Card className="p-4 border-slate-200 bg-white shadow-xs space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs text-slate-500 font-medium">失联演练历史</span>
            <Flame className="w-4 h-4 text-amber-500" />
          </div>
          <div className="text-xs font-semibold text-slate-800 line-clamp-1">
            {config.lastDrillRun || '正常无演练'}
          </div>
          <p className="text-[11px] text-slate-500">验证网关脱网自愈能力指标</p>
        </Card>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main Settings Form */}
        <div className="lg:col-span-2 space-y-6">
          <Card className="p-6 border-slate-200 bg-white shadow-sm space-y-6">
            <div className="flex items-center gap-2 border-b border-slate-100 pb-3">
              <Sliders className="w-5 h-5 text-indigo-600" />
              <div>
                <h2 className="text-sm font-bold text-slate-900">高级灾备与网关离线兜底策略 (Fail-Safe Parameters)</h2>
                <p className="text-xs text-slate-500">
                  确保中央控制面板遭遇 DDOS、光缆中断或意外故障时，内网不会因指令中断造成数据真空或业务瘫痪。
                </p>
              </div>
            </div>

            <form onSubmit={handleSaveConfig} className="space-y-6">
              {/* Dead Man's Switch Section */}
              <div className="p-4 rounded-xl border border-slate-200 bg-slate-50/70 space-y-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <ShieldAlert className="w-4 h-4 text-indigo-600" />
                    <span className="text-xs font-bold text-slate-800">
                      死信定时器 (Dead Man's Switch) 自动熔断机制
                    </span>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={config.deadMansSwitchEnabled}
                      onChange={(e) => setConfig({ ...config, deadMansSwitchEnabled: e.target.checked })}
                      className="sr-only peer"
                    />
                    <div className="w-9 h-5 bg-slate-300 peer-focus:outline-hidden rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-slate-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-indigo-600"></div>
                  </label>
                </div>
                <p className="text-xs text-slate-500 leading-relaxed">
                  若中央控制面板异常宕机，全球网关在持续达到指定失联时长后，将判定中心被物理摧毁或受攻陷，自动切入离线隔离行为，防止未撤销的恶意凭据长驱直入。
                </p>

                {config.deadMansSwitchEnabled && (
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2">
                    <div>
                      <label className="block text-xs font-medium text-slate-700 mb-1">
                        失联触发熔断阈值 (分钟)
                      </label>
                      <Input
                        type="number"
                        min="5"
                        max="180"
                        value={config.deadMansTimeoutMinutes}
                        onChange={(e) =>
                          setConfig({ ...config, deadMansTimeoutMinutes: parseInt(e.target.value) || 30 })
                        }
                        className="h-8 text-xs font-mono"
                      />
                      <span className="text-[10px] text-slate-400 mt-1 block">推荐 30 分钟，生产环境严禁低于 5 分钟以防误触发</span>
                    </div>

                    <div>
                      <label className="block text-xs font-medium text-slate-700 mb-1">
                        网关主动心跳探活周期 (秒)
                      </label>
                      <Input
                        type="number"
                        min="5"
                        max="120"
                        value={config.heartbeatIntervalSec}
                        onChange={(e) =>
                          setConfig({ ...config, heartbeatIntervalSec: parseInt(e.target.value) || 15 })
                        }
                        className="h-8 text-xs font-mono"
                      />
                      <span className="text-[10px] text-slate-400 mt-1 block">网关 Agent 向中心发送 Keep-Alive 广播的周期</span>
                    </div>
                  </div>
                )}
              </div>

              {/* Fail-Safe Mode Radios */}
              <div className="space-y-3">
                <label className="block text-xs font-bold text-slate-800">
                  离线自决行为模式 (Gateways Offline Decision Mode)
                </label>
                <div className="space-y-2">
                  <div
                    onClick={() => setConfig({ ...config, mode: 'ALLOW_EXISTING_CACHE' })}
                    className={`p-3.5 rounded-xl border cursor-pointer transition-all ${
                      config.mode === 'ALLOW_EXISTING_CACHE'
                        ? 'border-indigo-500 bg-indigo-50/50 shadow-xs ring-1 ring-indigo-500'
                        : 'border-slate-200 bg-white hover:bg-slate-50'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <input
                          type="radio"
                          name="failSafeMode"
                          checked={config.mode === 'ALLOW_EXISTING_CACHE'}
                          onChange={() => setConfig({ ...config, mode: 'ALLOW_EXISTING_CACHE' })}
                          className="text-indigo-600"
                        />
                        <span className="text-xs font-bold text-slate-900">
                          维持存量在线会话，阻断一切新设备握手 (推荐)
                        </span>
                      </div>
                      <Badge variant="primary" className="text-[10px] bg-emerald-100 text-emerald-800 border-none">
                        生产首选
                      </Badge>
                    </div>
                    <p className="text-xs text-slate-600 mt-1.5 ml-6">
                      已建立 WireGuard 安全隧道的合法工作会话不受影响（保证业务平稳不中断），但关闭公钥注册握手，防止黑客趁乱注入新未授权设备。
                    </p>
                  </div>

                  <div
                    onClick={() => setConfig({ ...config, mode: 'BLOCK_ALL_EMERGENCY' })}
                    className={`p-3.5 rounded-xl border cursor-pointer transition-all ${
                      config.mode === 'BLOCK_ALL_EMERGENCY'
                        ? 'border-rose-500 bg-rose-50/50 shadow-xs ring-1 ring-rose-500'
                        : 'border-slate-200 bg-white hover:bg-slate-50'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <input
                          type="radio"
                          name="failSafeMode"
                          checked={config.mode === 'BLOCK_ALL_EMERGENCY'}
                          onChange={() => setConfig({ ...config, mode: 'BLOCK_ALL_EMERGENCY' })}
                          className="text-rose-600"
                        />
                        <span className="text-xs font-bold text-slate-900">
                          全面安全熔断隔离 (Block All Emergency)
                        </span>
                      </div>
                      <Badge variant="outline" className="text-[10px] text-rose-700 bg-rose-50 border-rose-200">
                        高密资产专用
                      </Badge>
                    </div>
                    <p className="text-xs text-slate-600 mt-1.5 ml-6">
                      立即向内核驱动（wg0）下发清空 Peer 指令，切断所有现有流量并关闭监听端口。适用于金融结算系统、核心密钥机房等对未决状态零容忍的环境。
                    </p>
                  </div>

                  <div
                    onClick={() => setConfig({ ...config, mode: 'READ_ONLY_FALLBACK' })}
                    className={`p-3.5 rounded-xl border cursor-pointer transition-all ${
                      config.mode === 'READ_ONLY_FALLBACK'
                        ? 'border-blue-500 bg-blue-50/50 shadow-xs ring-1 ring-blue-500'
                        : 'border-slate-200 bg-white hover:bg-slate-50'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <input
                          type="radio"
                          name="failSafeMode"
                          checked={config.mode === 'READ_ONLY_FALLBACK'}
                          onChange={() => setConfig({ ...config, mode: 'READ_ONLY_FALLBACK' })}
                          className="text-blue-600"
                        />
                        <span className="text-xs font-bold text-slate-900">
                          降级只读旁路模式 (Read-Only Fallback & Local ACL)
                        </span>
                      </div>
                      <Badge variant="outline" className="text-[10px] text-blue-700 bg-blue-50 border-blue-200">
                        自主运算
                      </Badge>
                    </div>
                    <p className="text-xs text-slate-600 mt-1.5 ml-6">
                      网关依据最后一次在本地固化持久化的策略规则快照与黑名单，自主执行 4 层 ACL 与进程过滤。
                    </p>
                  </div>
                </div>
              </div>

              {/* Local Storage & Cache Options */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    本地 ACL 快照持久化留存期限 (小时)
                  </label>
                  <Select
                    value={String(config.localCacheTtlHours)}
                    onChange={(e) =>
                      setConfig({ ...config, localCacheTtlHours: parseInt(e.target.value) || 72 })
                    }
                    className="h-8 text-xs"
                  >
                    <option value="24">24 小时 (严格)</option>
                    <option value="48">48 小时</option>
                    <option value="72">72 小时 (标准企业默认)</option>
                    <option value="168">7 天 (野外或海缆高延迟节点)</option>
                  </Select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-700 mb-1">
                    离线应急救援密码 (Emergency Break-Glass PIN)
                  </label>
                  <div className="flex items-center gap-2">
                    <Input
                      type="password"
                      value="••••••••••••"
                      readOnly
                      className="h-8 text-xs font-mono bg-slate-50"
                    />
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-8 text-xs"
                      onClick={() => info('已生成离线特权救援指令：velo-cli break-glass --seed=9f4a...')}
                    >
                      <KeyRound className="w-3.5 h-3.5 mr-1" />
                      生成命令
                    </Button>
                  </div>
                </div>
              </div>

              {/* Submit Buttons */}
              <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-100">
                <Button
                  type="button"
                  variant="outline"
                  onClick={fetchConfig}
                  className="text-xs"
                >
                  重置为初始
                </Button>
                <Button
                  type="submit"
                  variant="primary"
                  disabled={saving}
                  className="text-xs bg-indigo-600 hover:bg-indigo-700 text-white"
                >
                  {saving ? '保存并推送到全球网关...' : '保存并下发全局灾备策略'}
                </Button>
              </div>
            </form>
          </Card>
        </div>

        {/* Drill Console & Subsystem Status */}
        <div className="space-y-6">
          {/* Chaos Simulator Console */}
          <Card className="p-4 border-slate-800 bg-slate-950 text-slate-200 shadow-sm space-y-3 font-mono">
            <div className="flex items-center justify-between border-b border-slate-800 pb-2">
              <div className="flex items-center gap-2">
                <span className="h-2 w-2 rounded-full bg-emerald-400 animate-pulse" />
                <span className="text-xs font-bold text-white">断网逃生演练控制台 (Drill Console)</span>
              </div>
              <Badge variant="outline" className="text-[9px] border-slate-700 text-slate-400 font-mono">
                eBPF Driver Live
              </Badge>
            </div>

            <div className="min-h-[190px] max-h-[260px] overflow-y-auto space-y-1.5 text-[11px] leading-relaxed text-slate-300">
              {drillOutput.length === 0 ? (
                <div className="text-slate-500 py-6 text-center">
                  点击上方「发起离线逃生演练」按钮，可模拟控制面板宕机、全球网关触发死信熔断的实时行为日志。
                </div>
              ) : (
                drillOutput.map((log, index) => (
                  <div key={index} className="break-all font-mono">
                    {log}
                  </div>
                ))
              )}
            </div>

            <div className="pt-2 border-t border-slate-800/80 flex items-center justify-between text-[10px] text-slate-400 font-sans">
              <span>状态: 全球 3 个网关准备就绪</span>
              <span className="text-emerald-400 font-mono">100% 旁路缓存覆盖</span>
            </div>
          </Card>

          {/* Connected Subsystems Card */}
          <Card className="p-5 border-slate-200 bg-white shadow-xs space-y-4">
            <h3 className="text-xs font-bold text-slate-900 flex items-center gap-2">
              <ShieldCheck className="w-4 h-4 text-emerald-600" />
              关联子系统生产状态巡检
            </h3>

            <div className="space-y-3 text-xs">
              <div className="p-2.5 rounded-lg border border-slate-100 bg-slate-50 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <ArrowLeftRight className="w-4 h-4 text-indigo-600" />
                  <div>
                    <span className="font-semibold text-slate-800">跨云 VPC 重叠 NAT 映射</span>
                    <p className="text-[10px] text-slate-400">腾讯云影子网段: 172.31.10.0/24</p>
                  </div>
                </div>
                <Badge variant="primary" className="text-[10px] bg-emerald-100 text-emerald-700">
                  双向 1:1 NAT 活跃
                </Badge>
              </div>

              <div className="p-2.5 rounded-lg border border-slate-100 bg-slate-50 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Cpu className="w-4 h-4 text-blue-600" />
                  <div>
                    <span className="font-semibold text-slate-800">分应用/进程级防火墙</span>
                    <p className="text-[10px] text-slate-400">Windows WFP / macOS NetExt</p>
                  </div>
                </div>
                <Badge variant="primary" className="text-[10px] bg-blue-100 text-blue-700">
                  进程过滤生效
                </Badge>
              </div>

              <div className="p-2.5 rounded-lg border border-slate-100 bg-slate-50 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Server className="w-4 h-4 text-purple-600" />
                  <div>
                    <span className="font-semibold text-slate-800">Overlay 自动化 IPAM</span>
                    <p className="text-[10px] text-slate-400">全局: 10.100.0.0/16</p>
                  </div>
                </div>
                <Badge variant="outline" className="text-[10px] text-purple-700 border-purple-200">
                  自动化级联
                </Badge>
              </div>
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
};
