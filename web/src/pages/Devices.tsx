import React, { useState, useEffect, useMemo } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import {
  Search,
  Plus,
  Monitor,
  Smartphone,
  Tablet,
  Server,
  Ban,
  Trash2,
  Eye,
  Copy,
  Check,
  QrCode,
  ShieldAlert,
  Compass,
} from 'lucide-react';
import { mockApi } from '../data/mock';
import { Device, DeviceType, User, Site } from '../types';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Select } from '../components/ui/select';
import { Badge } from '../components/ui/badge';
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
import { Dialog } from '../components/ui/dialog';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { EmptyState } from '../components/EmptyState';
import { useToast } from '../components/ui/toast';
import { formatRelativeTime, formatDateTime } from '../utils/format';

export const Devices: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const { success, error } = useToast();

  const [devices, setDevices] = useState<Device[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [loading, setLoading] = useState(true);

  // Filters
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const userParam = searchParams.get('userId');

  // Register Device 2-Step Modal
  const [isRegisterOpen, setIsRegisterOpen] = useState(false);
  const [registerStep, setRegisterStep] = useState<1 | 2>(1);
  const [regForm, setRegForm] = useState({
    userId: '',
    siteId: '',
    type: 'PC' as DeviceType,
    deviceName: '',
  });
  const [generatedDevice, setGeneratedDevice] = useState<Device | null>(null);
  const [tokenCopied, setTokenCopied] = useState(false);
  const [registering, setRegistering] = useState(false);

  // View Device Detail Modal
  const [viewDevice, setViewDevice] = useState<Device | null>(null);

  // Revoke & Delete modals
  const [deviceToRevoke, setDeviceToRevoke] = useState<Device | null>(null);
  const [revoking, setRevoking] = useState(false);

  const [deviceToDelete, setDeviceToDelete] = useState<Device | null>(null);
  const [deleting, setDeleting] = useState(false);

  const loadData = async () => {
    try {
      const [d, u, s] = await Promise.all([
        mockApi.getDevices(),
        mockApi.getUsers(),
        mockApi.getSites(),
      ]);
      setDevices(d);
      setUsers(u);
      setSites(s);
      if (u.length > 0 && s.length > 0) {
        setRegForm((prev) => ({
          ...prev,
          userId: prev.userId || u[0].id,
          siteId: prev.siteId || s[0].id,
        }));
      }
    } catch {
      error('加载设备数据失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const filteredDevices = useMemo(() => {
    return devices.filter((dev) => {
      const matchSearch =
        !search ||
        dev.name.toLowerCase().includes(search.toLowerCase()) ||
        dev.userName.toLowerCase().includes(search.toLowerCase()) ||
        dev.virtualIp.includes(search);
      const matchType = typeFilter === 'all' || dev.type === typeFilter;
      const matchStatus = statusFilter === 'all' || dev.status === statusFilter;
      const matchUser = !userParam || dev.userId === userParam;
      return matchSearch && matchType && matchStatus && matchUser;
    });
  }, [devices, search, typeFilter, statusFilter, userParam]);

  const openRegister = () => {
    setRegisterStep(1);
    setRegForm({
      userId: users[0]?.id || '',
      siteId: sites[0]?.id || '',
      type: 'PC',
      deviceName: '',
    });
    setGeneratedDevice(null);
    setTokenCopied(false);
    setIsRegisterOpen(true);
  };

  const handleGenerateToken = async (e: React.FormEvent) => {
    e.preventDefault();
    setRegistering(true);
    try {
      const newDev = await mockApi.registerDevice(regForm);
      setGeneratedDevice(newDev);
      setRegisterStep(2);
      success('设备接入凭证已成功生成');
      await loadData();
    } catch {
      error('生成设备令牌失败');
    } finally {
      setRegistering(false);
    }
  };

  const handleCopyToken = () => {
    if (!generatedDevice?.token) return;
    navigator.clipboard.writeText(generatedDevice.token);
    setTokenCopied(true);
    success('令牌已复制到剪贴板');
    setTimeout(() => setTokenCopied(false), 2000);
  };

  const handleRevokeConfirm = async () => {
    if (!deviceToRevoke) return;
    setRevoking(true);
    try {
      await mockApi.revokeDevice(deviceToRevoke.id);
      success(`设备 "${deviceToRevoke.name}" 权限已成功撤销`);
      setDeviceToRevoke(null);
      await loadData();
    } catch {
      error('撤销设备失败');
    } finally {
      setRevoking(false);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!deviceToDelete) return;
    setDeleting(true);
    try {
      await mockApi.deleteDevice(deviceToDelete.id);
      success(`设备 "${deviceToDelete.name}" 已删除`);
      setDeviceToDelete(null);
      await loadData();
    } catch {
      error('删除设备失败');
    } finally {
      setDeleting(false);
    }
  };

  const getTypeIcon = (type: DeviceType) => {
    switch (type) {
      case 'PC':
        return <Monitor className="w-3.5 h-3.5" />;
      case 'Phone':
        return <Smartphone className="w-3.5 h-3.5" />;
      case 'Tablet':
        return <Tablet className="w-3.5 h-3.5" />;
      case 'Server':
      default:
        return <Server className="w-3.5 h-3.5" />;
    }
  };

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight">终端设备管理</h1>
          <p className="text-xs text-slate-500 mt-0.5">
            管理接入 WireGuard 虚拟内网的笔记本、移动终端与测试服务器凭证
          </p>
        </div>
        <Button onClick={openRegister} className="gap-1.5 shadow-xs">
          <Plus className="w-4 h-4" />
          <span>注册新设备</span>
        </Button>
      </div>

      {/* Filter / Search Bar */}
      <Card className="p-4">
        <div className="flex flex-col sm:flex-row items-center gap-3">
          <div className="flex-1 w-full">
            <Input
              placeholder="搜索设备名称、用户名或虚拟 IP..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              leftIcon={<Search className="w-4 h-4 text-slate-400" />}
            />
          </div>

          <div className="w-full sm:w-44">
            <Select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}>
              <option value="all">设备类型：全部</option>
              <option value="PC">PC 电脑</option>
              <option value="Phone">手机</option>
              <option value="Tablet">平板</option>
              <option value="Server">服务器</option>
            </Select>
          </div>

          <div className="w-full sm:w-44">
            <Select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
              <option value="all">设备状态：全部</option>
              <option value="active">正常活跃 (active)</option>
              <option value="offline">离线 (offline)</option>
              <option value="revoked">已撤销 (revoked)</option>
            </Select>
          </div>

          {userParam && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setSearchParams({})}
              className="text-xs text-blue-600 border-blue-200"
            >
              清除用户筛选
            </Button>
          )}
        </div>
      </Card>

      {/* Devices Table */}
      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>设备名称</TableHead>
              <TableHead>绑定员工</TableHead>
              <TableHead>类型</TableHead>
              <TableHead>操作系统</TableHead>
              <TableHead>分配虚拟 IP</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>最后在线</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredDevices.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="p-8">
                  <EmptyState
                    icon={Monitor}
                    title="暂无设备"
                    description="未检索到符合条件的接入设备，请注册新设备或清除搜索条件。"
                  />
                </TableCell>
              </TableRow>
            ) : (
              filteredDevices.map((dev) => (
                <TableRow key={dev.id}>
                  <TableCell className="font-semibold text-slate-900">
                    {dev.name}
                  </TableCell>
                  <TableCell className="text-xs text-slate-700 font-medium">
                    {dev.userName}
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className="gap-1">
                      {getTypeIcon(dev.type)}
                      <span>{dev.type}</span>
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs text-slate-600">
                    {dev.os}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-800 font-medium">
                    {dev.virtualIp}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={dev.status} />
                  </TableCell>
                  <TableCell className="font-mono text-xs text-slate-500">
                    {formatRelativeTime(dev.lastSeen)}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        title="透视该设备可达资源 (Access Matrix)"
                        onClick={() => navigate(`/access-matrix?type=device&id=${dev.id}`)}
                        className="h-8 w-8 p-0 text-slate-500 hover:text-indigo-600 hover:bg-indigo-50"
                      >
                        <Compass className="w-3.5 h-3.5 text-indigo-600" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        title="查看设备详情"
                        onClick={() => setViewDevice(dev)}
                        className="h-8 w-8 p-0 text-slate-500 hover:text-blue-600"
                      >
                        <Eye className="w-3.5 h-3.5" />
                      </Button>
                      {dev.status !== 'revoked' && (
                        <Button
                          variant="ghost"
                          size="sm"
                          title="撤销设备接入授权"
                          onClick={() => setDeviceToRevoke(dev)}
                          className="h-8 w-8 p-0 text-slate-400 hover:text-amber-600"
                        >
                          <Ban className="w-3.5 h-3.5" />
                        </Button>
                      )}
                      <Button
                        variant="ghost"
                        size="sm"
                        title="删除设备记录"
                        onClick={() => setDeviceToDelete(dev)}
                        className="h-8 w-8 p-0 text-slate-400 hover:text-rose-600"
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
      </Card>

      {/* Register Device 2-Step Dialog */}
      <Dialog
        isOpen={isRegisterOpen}
        onClose={() => setIsRegisterOpen(false)}
        title={registerStep === 1 ? '第一步：选择用户与设备类型' : '第二步：获取设备入网令牌与配置二维码'}
        description={
          registerStep === 1
            ? '配置将要接入 WireGuard Mesh 网络的设备所有者与归属站点'
            : '将生成的 Token 注入设备或使用客户端扫码一键配置'
        }
        maxWidth="lg"
      >
        {registerStep === 1 ? (
          <form onSubmit={handleGenerateToken} className="space-y-4">
            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                选择设备归属用户 <span className="text-rose-500">*</span>
              </label>
              <Select
                value={regForm.userId}
                onChange={(e) => setRegForm({ ...regForm, userId: e.target.value })}
              >
                {users.map((u) => (
                  <option key={u.id} value={u.id}>
                    {u.name} ({u.username} • {u.department})
                  </option>
                ))}
              </Select>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-700 mb-1">
                选择首选接入站点网关 <span className="text-rose-500">*</span>
              </label>
              <Select
                value={regForm.siteId}
                onChange={(e) => setRegForm({ ...regForm, siteId: e.target.value })}
              >
                {sites.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name} ({s.localSubnet})
                  </option>
                ))}
              </Select>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-medium text-slate-700 mb-1">
                  设备类型
                </label>
                <Select
                  value={regForm.type}
                  onChange={(e) =>
                    setRegForm({ ...regForm, type: e.target.value as DeviceType })
                  }
                >
                  <option value="PC">PC 电脑 (macOS / Win / Linux)</option>
                  <option value="Phone">手机 (iOS / Android)</option>
                  <option value="Tablet">平板 (iPadOS)</option>
                  <option value="Server">服务器 / 容器节点</option>
                </Select>
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-700 mb-1">
                  自定义设备备注名称 (可选)
                </label>
                <Input
                  value={regForm.deviceName}
                  onChange={(e) => setRegForm({ ...regForm, deviceName: e.target.value })}
                  placeholder="例如: 张伟-MacBook-M3"
                />
              </div>
            </div>

            <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-100">
              <Button type="button" variant="outline" onClick={() => setIsRegisterOpen(false)}>
                取消
              </Button>
              <Button type="submit" isLoading={registering}>
                生成令牌与凭据
              </Button>
            </div>
          </form>
        ) : (
          <div className="space-y-5">
            {generatedDevice && (
              <>
                <div className="bg-slate-50 border border-slate-200 rounded-xl p-4 space-y-3">
                  <div className="flex items-center justify-between text-xs">
                    <span className="text-slate-500">设备名称</span>
                    <strong className="text-slate-800">{generatedDevice.name}</strong>
                  </div>
                  <div className="flex items-center justify-between text-xs">
                    <span className="text-slate-500">分配虚拟 IP</span>
                    <strong className="font-mono text-blue-600">{generatedDevice.virtualIp}</strong>
                  </div>
                  <div className="flex items-center justify-between text-xs">
                    <span className="text-slate-500">归属用户</span>
                    <span className="text-slate-700">{generatedDevice.userName}</span>
                  </div>
                </div>

                {/* Token Box */}
                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                    WireGuard 快速注册令牌 (Token)
                  </label>
                  <div className="flex items-center gap-2">
                    <Input
                      readOnly
                      value={generatedDevice.token}
                      className="font-mono text-xs bg-slate-50"
                    />
                    <Button
                      variant="outline"
                      onClick={handleCopyToken}
                      className="shrink-0 gap-1.5"
                    >
                      {tokenCopied ? (
                        <>
                          <Check className="w-3.5 h-3.5 text-emerald-600" />
                          <span className="text-emerald-700">已复制</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3.5 h-3.5" />
                          <span>复制</span>
                        </>
                      )}
                    </Button>
                  </div>
                </div>

                {/* QR Code SVG Display for mobile WireGuard onboarding */}
                <div className="flex flex-col items-center justify-center p-4 bg-white border border-slate-200 rounded-xl text-center">
                  <div className="p-3 bg-white border border-slate-200 shadow-xs rounded-xl mb-2">
                    {/* Simulated High-Res WireGuard QR Code Pattern */}
                    <svg
                      width="160"
                      height="160"
                      viewBox="0 0 160 160"
                      className="fill-slate-900"
                    >
                      <rect width="160" height="160" fill="#ffffff" />
                      {/* Top-left marker */}
                      <rect x="15" y="15" width="40" height="40" fill="#0f172a" rx="4" />
                      <rect x="23" y="23" width="24" height="24" fill="#ffffff" />
                      <rect x="29" y="29" width="12" height="12" fill="#0f172a" />
                      {/* Top-right marker */}
                      <rect x="105" y="15" width="40" height="40" fill="#0f172a" rx="4" />
                      <rect x="113" y="23" width="24" height="24" fill="#ffffff" />
                      <rect x="119" y="29" width="12" height="12" fill="#0f172a" />
                      {/* Bottom-left marker */}
                      <rect x="15" y="105" width="40" height="40" fill="#0f172a" rx="4" />
                      <rect x="23" y="113" width="24" height="24" fill="#ffffff" />
                      <rect x="29" y="119" width="12" height="12" fill="#0f172a" />
                      {/* Random data grid pattern */}
                      <rect x="65" y="20" width="8" height="8" />
                      <rect x="80" y="25" width="8" height="15" />
                      <rect x="65" y="45" width="15" height="8" />
                      <rect x="90" y="50" width="8" height="8" />
                      <rect x="20" y="65" width="10" height="10" />
                      <rect x="40" y="70" width="15" height="15" />
                      <rect x="65" y="70" width="10" height="10" />
                      <rect x="85" y="75" width="20" height="10" />
                      <rect x="115" y="70" width="15" height="10" />
                      <rect x="135" y="80" width="10" height="10" />
                      <rect x="65" y="95" width="15" height="15" />
                      <rect x="90" y="100" width="10" height="20" />
                      <rect x="110" y="110" width="20" height="10" />
                      <rect x="135" y="120" width="10" height="20" />
                      <rect x="65" y="125" width="15" height="15" />
                      <rect x="85" y="130" width="15" height="10" />
                    </svg>
                  </div>
                  <p className="text-xs text-slate-600 font-medium">
                    移动端扫码：使用 WireGuard 官方客户端扫描直接导入隧道配置
                  </p>
                </div>

                <div className="flex justify-end pt-3 border-t border-slate-100">
                  <Button onClick={() => setIsRegisterOpen(false)}>完成并关闭</Button>
                </div>
              </>
            )}
          </div>
        )}
      </Dialog>

      {/* Device Detail View Modal */}
      {viewDevice && (
        <Dialog
          isOpen={!!viewDevice}
          onClose={() => setViewDevice(null)}
          title={`设备详情 — ${viewDevice.name}`}
          description="查看设备硬件系统、连接参数与入网状态"
          maxWidth="md"
        >
          <div className="space-y-3 text-xs">
            <div className="flex justify-between py-1.5 border-b border-slate-100">
              <span className="text-slate-500">设备唯一标识</span>
              <span className="font-mono text-slate-800">{viewDevice.id}</span>
            </div>
            <div className="flex justify-between py-1.5 border-b border-slate-100">
              <span className="text-slate-500">归属用户</span>
              <span className="font-medium text-slate-900">{viewDevice.userName}</span>
            </div>
            <div className="flex justify-between py-1.5 border-b border-slate-100">
              <span className="text-slate-500">设备类型 / OS</span>
              <span className="text-slate-700">
                {viewDevice.type} • {viewDevice.os}
              </span>
            </div>
            <div className="flex justify-between py-1.5 border-b border-slate-100">
              <span className="text-slate-500">分配虚拟 IP</span>
              <span className="font-mono font-semibold text-blue-600">
                {viewDevice.virtualIp}
              </span>
            </div>
            <div className="flex justify-between py-1.5 border-b border-slate-100">
              <span className="text-slate-500">当前接入状态</span>
              <StatusBadge status={viewDevice.status} />
            </div>
            <div className="flex justify-between py-1.5 border-b border-slate-100">
              <span className="text-slate-500">注册归属网络站点</span>
              <span className="text-slate-700">{viewDevice.siteName}</span>
            </div>
            <div className="flex justify-between py-1.5 border-b border-slate-100">
              <span className="text-slate-500">最后活跃连接</span>
              <span className="font-mono text-slate-700">
                {formatRelativeTime(viewDevice.lastSeen)} ({formatDateTime(viewDevice.lastSeen)})
              </span>
            </div>
            {viewDevice.token && (
              <div className="pt-2">
                <span className="text-slate-500 block mb-1">入网凭据 Token</span>
                <p className="p-2 bg-slate-50 rounded border border-slate-200 font-mono text-[11px] text-slate-700 select-all break-all">
                  {viewDevice.token}
                </p>
              </div>
            )}
            <div className="pt-4 flex justify-end">
              <Button variant="outline" onClick={() => setViewDevice(null)}>
                关闭
              </Button>
            </div>
          </div>
        </Dialog>
      )}

      {/* Revoke Confirmation */}
      <ConfirmDialog
        isOpen={!!deviceToRevoke}
        onClose={() => setDeviceToRevoke(null)}
        onConfirm={handleRevokeConfirm}
        title="确认撤销设备入网权限"
        description={`确定要撤销设备 "${deviceToRevoke?.name}" 的网络访问权限吗？撤销后，该设备的 WireGuard 握手密钥将立刻被各网关拉黑并切断连接。`}
        danger={true}
        confirmText="确认撤销"
        isLoading={revoking}
      />

      {/* Delete Confirmation */}
      <ConfirmDialog
        isOpen={!!deviceToDelete}
        onClose={() => setDeviceToDelete(null)}
        onConfirm={handleDeleteConfirm}
        title="确认删除设备记录"
        description={`确定要彻底删除设备 "${deviceToDelete?.name}" 吗？此操作将清除该设备的所有历史档案。`}
        danger={true}
        isLoading={deleting}
      />
    </div>
  );
};
