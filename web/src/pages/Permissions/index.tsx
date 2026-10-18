import React, { useEffect, useMemo, useState } from 'react';
import { useLocation, useSearchParams } from 'react-router-dom';
import { Code2, RefreshCw, ShieldCheck } from 'lucide-react';
import { iamApi, IAMPermission, IAMRole } from '../../services/iam';
import { useAuth } from '../../contexts/AuthContext';
import { Button } from '../../components/ui/button';
import { Badge } from '../../components/ui/badge';
import { Card } from '../../components/ui/card';
import { EmptyState } from '../../components/EmptyState';
import { useToast } from '../../components/ui/toast';

export const Permissions: React.FC = () => {
  const { hasPermission } = useAuth();
  const { success, error } = useToast();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const apiMappingsRoute = location.pathname.includes('api-mappings');
  const [roles, setRoles] = useState<IAMRole[]>([]);
  const [catalog, setCatalog] = useState<IAMPermission[]>([]);
  const [assigned, setAssigned] = useState<IAMPermission[]>([]);
  const [selectedRoleId, setSelectedRoleId] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [assignmentLoading, setAssignmentLoading] = useState(false);
  const [assignmentFailed, setAssignmentFailed] = useState(false);
  const [savingPermissionId, setSavingPermissionId] = useState<number | null>(null);
  const [assignmentReload, setAssignmentReload] = useState(0);

  const loadBase = async () => {
    setLoading(true);
    try {
      const [roleList, permissionList] = await Promise.all([iamApi.listRoles(), iamApi.listPermissionCatalog()]);
      setRoles(roleList);
      setCatalog(permissionList);
      setSelectedRoleId((current) => current && roleList.some((role) => role.id === current) ? current : roleList[0]?.id ?? null);
      setAssignmentReload((revision) => revision + 1);
    } catch (err) {
      error(err instanceof Error ? err.message : '加载角色权限失败。');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadBase(); }, []);

  useEffect(() => {
    if (apiMappingsRoute || searchParams.get('tab') === 'mappings') return;
    if (!selectedRoleId) {
      setAssigned([]);
      setAssignmentFailed(false);
      return;
    }
    let active = true;
    setAssigned([]);
    setAssignmentFailed(false);
    setAssignmentLoading(true);
    iamApi.listRolePermissions(selectedRoleId)
      .then((items) => { if (active) setAssigned(items); })
      .catch((err) => {
        if (active) {
          setAssignmentFailed(true);
          error(err instanceof Error ? err.message : '加载角色授权失败。');
        }
      })
      .finally(() => { if (active) setAssignmentLoading(false); });
    return () => { active = false; };
  }, [selectedRoleId, apiMappingsRoute, searchParams, assignmentReload]);

  const assignedIds = useMemo(() => new Set(assigned.map((item) => item.id)), [assigned]);
  const groups = useMemo(() => {
    const map = new Map<number, IAMPermission[]>();
    for (const permission of catalog) {
      const list = map.get(permission.resource_id) ?? [];
      list.push(permission);
      map.set(permission.resource_id, list);
    }
    return Array.from(map.entries()).sort(([a], [b]) => a - b);
  }, [catalog]);

  const togglePermission = async (permission: IAMPermission) => {
    if (!selectedRoleId) return;
    const wasAssigned = assignedIds.has(permission.id);
    setSavingPermissionId(permission.id);
    try {
      if (wasAssigned) await iamApi.revokePermission(selectedRoleId, permission.id);
      else await iamApi.grantPermission(selectedRoleId, permission.id);
      setAssigned((current) => wasAssigned ? current.filter((item) => item.id !== permission.id) : [...current, permission]);
      success(wasAssigned ? `已撤销「${permission.permission_name}」` : `已授予「${permission.permission_name}」`);
    } catch (err) {
      error(err instanceof Error ? err.message : '更新权限失败。');
    } finally {
      setSavingPermissionId(null);
    }
  };

  const canWrite = hasPermission('iam.role.write');

  if (apiMappingsRoute || searchParams.get('tab') === 'mappings') {
    return (
      <div className="space-y-5">
        <div><h1 className="text-xl font-bold tracking-tight text-slate-900">JRPC 权限映射</h1><p className="mt-1 text-xs text-slate-500">JRPC 方法的访问控制由 Management 服务端配置。</p></div>
        <Card className="p-8"><EmptyState icon={Code2} title="接口映射管理暂未开放" description="当前 IAM JRPC 使用服务端显式权限映射及默认拒绝策略。前端尚无接口映射查询或编辑方法。" /></Card>
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
        <div><h1 className="text-xl font-bold tracking-tight text-slate-900">受控资源与权限</h1><p className="mt-1 text-xs text-slate-500">权限目录和角色授权均读取 Management IAM；每次变更都会经过服务端鉴权。</p></div>
        <Button variant="outline" onClick={() => void loadBase()} isLoading={loading}><RefreshCw className="h-4 w-4" />刷新</Button>
      </div>

      <Card className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
        <label className="flex items-center gap-3 text-xs font-medium text-slate-600">正在配置的角色
          <select className="min-w-56 rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-800" value={selectedRoleId ?? ''} onChange={(event) => setSelectedRoleId(Number(event.target.value) || null)} disabled={loading || roles.length === 0}>
            {roles.length === 0 && <option value="">暂无角色</option>}
            {roles.map((role) => <option key={role.id} value={role.id}>{role.role_name} ({role.role_code})</option>)}
          </select>
        </label>
        <div className="text-xs text-slate-500">已分配 {assigned.length} / {catalog.length} 项权限</div>
      </Card>

      {!canWrite && <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800">当前账号只有读取权限；修改授权需要 iam.role.write。</div>}
      {assignmentFailed && <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-800">无法读取当前角色的授权，已停用权限修改。刷新重试后再操作。</div>}

      {loading ? <Card className="p-10 text-center text-sm text-slate-500">正在加载权限目录…</Card> : catalog.length === 0 ? (
        <Card className="p-8"><EmptyState icon={ShieldCheck} title="权限目录为空" description="请先在 Management 数据库中配置 IAM 资源与权限目录。" /></Card>
      ) : groups.map(([resourceID, permissions]) => (
        <Card key={resourceID} className="overflow-hidden">
          <div className="flex items-center justify-between border-b border-slate-100 bg-slate-50 px-5 py-3">
            <div><h2 className="text-sm font-semibold text-slate-800">资源 ID {resourceID}</h2><p className="mt-0.5 text-[11px] text-slate-500">{permissions.length} 项可用权限</p></div>
            <Badge variant="outline">resource_id: {resourceID}</Badge>
          </div>
          <div className="divide-y divide-slate-100">
            {permissions.map((permission) => {
              const checked = assignedIds.has(permission.id);
              return (
                <label key={permission.id} className="flex cursor-pointer items-start gap-3 px-5 py-3.5 hover:bg-slate-50">
                  <input type="checkbox" className="mt-1 h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" checked={checked} disabled={!canWrite || assignmentLoading || assignmentFailed || savingPermissionId !== null || permission.status !== 'active'} onChange={() => void togglePermission(permission)} />
                  <span className="min-w-0 flex-1"><span className="flex flex-wrap items-center gap-2 text-sm font-medium text-slate-800">{permission.permission_name}<Badge variant={checked ? 'success' : 'outline'}>{permission.action_code}</Badge><Badge variant={permission.status === 'active' ? 'slate' : 'warning'}>{permission.status}</Badge></span><span className="mt-1 block font-mono text-[11px] text-slate-500">{permission.permission_code}</span>{permission.description && <span className="mt-1 block text-xs text-slate-500">{permission.description}</span>}</span>
                  {savingPermissionId === permission.id && <span className="text-xs text-slate-400">保存中…</span>}
                </label>
              );
            })}
          </div>
        </Card>
      ))}
      <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-xs leading-5 text-blue-800">
        当前版本提供权限目录读取与角色授权。资源和权限目录由数据库迁移/初始化数据管理，前端不直接创建目录项。
      </div>
    </div>
  );
};
