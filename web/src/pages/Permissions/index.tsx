import React, { useEffect, useMemo, useState } from 'react';
import { useLocation, useSearchParams } from 'react-router-dom';
import { Braces, ChevronRight, Code2, Layers3, RefreshCw, Search, ShieldCheck } from 'lucide-react';
import { iamApi, IAMMethodCatalogEntry, IAMPermission, IAMResource } from '../../services/iam';
import { Button } from '../../components/ui/button';
import { Badge } from '../../components/ui/badge';
import { Card } from '../../components/ui/card';
import { EmptyState } from '../../components/EmptyState';
import { Input } from '../../components/ui/input';
import { useToast } from '../../components/ui/toast';

type CatalogView = 'resources' | 'methods';

const actionNames: Record<string, string> = {
  read: '读取',
  write: '写入',
  create: '创建',
  update: '修改',
  delete: '删除',
  execute: '执行',
  grant: '授权',
  manage: '管理',
};

const policyNames: Record<IAMMethodCatalogEntry['policy_type'], string> = {
  public: '公开',
  authenticated: '需登录',
  permission: '权限控制',
};

const includesQuery = (query: string, values: Array<string | null | undefined>) => {
  const normalized = query.trim().toLocaleLowerCase();
  return !normalized || values.some((value) => value?.toLocaleLowerCase().includes(normalized));
};

export const Permissions: React.FC = () => {
  const { error } = useToast();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const apiMappingsRoute = location.pathname.includes('api-mappings') || searchParams.get('tab') === 'mappings';
  const [view, setView] = useState<CatalogView>(apiMappingsRoute ? 'methods' : 'resources');
  const [resources, setResources] = useState<IAMResource[]>([]);
  const [permissions, setPermissions] = useState<IAMPermission[]>([]);
  const [methods, setMethods] = useState<IAMMethodCatalogEntry[]>([]);
  const [selectedResourceID, setSelectedResourceID] = useState<number | null>(null);
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (apiMappingsRoute) setView('methods');
  }, [apiMappingsRoute]);

  const loadCatalog = async () => {
    setLoading(true);
    try {
      const [resourceList, permissionList, methodList] = await Promise.all([
        iamApi.listResourceCatalog(),
        iamApi.listPermissionCatalog(),
        iamApi.listMethodCatalog(),
      ]);
      setResources(resourceList);
      setPermissions(permissionList);
      setMethods(methodList);
      setSelectedResourceID((current) => current && resourceList.some((item) => item.id === current)
        ? current
        : resourceList[0]?.id ?? null);
    } catch (err) {
      error(err instanceof Error ? err.message : '加载 IAM 资源与方法目录失败。');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadCatalog(); }, []);

  const resourceByID = useMemo(() => new Map(resources.map((resource) => [resource.id, resource])), [resources]);
  const permissionsByCode = useMemo(() => new Map(permissions.map((permission) => [permission.permission_code, permission])), [permissions]);
  const selectedResource = resources.find((resource) => resource.id === selectedResourceID) ?? null;
  const selectedPermissions = useMemo(
    () => permissions.filter((permission) => permission.resource_id === selectedResourceID)
      .sort((a, b) => a.action_code.localeCompare(b.action_code) || a.permission_code.localeCompare(b.permission_code)),
    [permissions, selectedResourceID],
  );
  const visibleResources = useMemo(() => resources.filter((resource) => {
    const resourcePermissions = permissions.filter((permission) => permission.resource_id === resource.id);
    return includesQuery(query, [
      resource.resource_name,
      resource.resource_code,
      resource.resource_type,
      resource.description,
      ...resourcePermissions.flatMap((permission) => [permission.permission_name, permission.permission_code, permission.action_code]),
    ]);
  }), [permissions, query, resources]);
  const visibleMethods = useMemo(() => methods.filter((method) => {
    const permission = method.permission_code ? permissionsByCode.get(method.permission_code) : undefined;
    const resource = permission ? resourceByID.get(permission.resource_id) : undefined;
    return includesQuery(query, [
      method.method_code,
      method.service_name,
      method.method_name,
      method.policy_type,
      method.permission_code,
      permission?.permission_name,
      resource?.resource_name,
      resource?.resource_code,
    ]);
  }), [methods, permissionsByCode, query, resourceByID]);

  const methodsForPermission = (permissionCode: string) => methods.filter((method) => method.permission_code === permissionCode);

  const catalogView = apiMappingsRoute ? 'methods' : view;

  return (
    <div className="mx-auto max-w-7xl space-y-5">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-xl font-bold tracking-tight text-slate-900">{apiMappingsRoute ? 'API / JRPC 权限映射' : '资源与权限管理'}</h1>
            <Badge variant="outline" className="text-blue-700">IAM</Badge>
          </div>
          <p className="mt-1 max-w-3xl text-xs leading-5 text-slate-500">
            查看 Management 登记的受控资源、资源操作权限和 JRPC 方法策略。权限目录由代码与数据库种子维护；角色授权在角色配置中完成。
          </p>
        </div>
        <Button variant="outline" onClick={() => void loadCatalog()} isLoading={loading} aria-label="刷新 IAM 目录">
          <RefreshCw className="h-4 w-4" />刷新目录
        </Button>
      </div>

      <div className="flex flex-col gap-3 border-b border-slate-200 sm:flex-row sm:items-center sm:justify-between">
        {!apiMappingsRoute && (
          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => setView('resources')}
              className={`border-b-2 px-4 py-2.5 text-xs font-semibold transition-colors ${catalogView === 'resources' ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-slate-800'}`}
            >
              <span className="inline-flex items-center gap-2"><Layers3 className="h-3.5 w-3.5" />资源与权限</span>
            </button>
            <button
              type="button"
              onClick={() => setView('methods')}
              className={`border-b-2 px-4 py-2.5 text-xs font-semibold transition-colors ${catalogView === 'methods' ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-slate-800'}`}
            >
              <span className="inline-flex items-center gap-2"><Braces className="h-3.5 w-3.5" />API / JRPC 映射</span>
            </button>
          </div>
        )}
        <div className="relative pb-2 sm:w-80 sm:pb-0">
          <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
          <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={catalogView === 'methods' ? '搜索方法、权限或资源' : '搜索资源、权限名称或权限码'} className="pl-9" />
        </div>
      </div>

      {loading ? (
        <Card className="p-10 text-center text-sm text-slate-500">正在加载 IAM 目录…</Card>
      ) : catalogView === 'resources' ? (
        resources.length === 0 ? (
          <Card className="p-8"><EmptyState icon={Layers3} title="资源目录为空" description="请通过 IAM 数据库种子或迁移初始化受控资源。" /></Card>
        ) : (
          <div className="grid gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
            <Card className="h-fit overflow-hidden">
              <div className="border-b border-slate-100 bg-slate-50 px-4 py-3">
                <div className="text-sm font-semibold text-slate-800">受控资源</div>
                <div className="mt-1 text-[11px] text-slate-500">{visibleResources.length} / {resources.length} 个资源</div>
              </div>
              <div className="max-h-[68vh] overflow-y-auto p-2">
                {visibleResources.length === 0 ? (
                  <div className="p-5 text-center text-xs text-slate-500">没有匹配的资源。</div>
                ) : visibleResources.map((resource) => {
                  const count = permissions.filter((permission) => permission.resource_id === resource.id).length;
                  const active = resource.id === selectedResourceID;
                  return (
                    <button
                      key={resource.id}
                      type="button"
                      onClick={() => setSelectedResourceID(resource.id)}
                      className={`mb-1 flex w-full items-center gap-3 rounded-lg px-3 py-3 text-left transition-colors last:mb-0 ${active ? 'bg-blue-50 text-blue-800' : 'text-slate-700 hover:bg-slate-50'}`}
                    >
                      <span className={`rounded-lg p-2 ${active ? 'bg-white text-blue-600' : 'bg-slate-100 text-slate-500'}`}><Layers3 className="h-4 w-4" /></span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-xs font-semibold">{resource.resource_name}</span>
                        <span className="mt-1 block truncate font-mono text-[10px] opacity-70">{resource.resource_code}</span>
                      </span>
                      <span className="text-[10px] text-slate-400">{count}</span>
                      <ChevronRight className="h-3.5 w-3.5 shrink-0 opacity-50" />
                    </button>
                  );
                })}
              </div>
            </Card>

            {selectedResource ? (
              <div className="space-y-4">
                <Card className="overflow-hidden">
                  <div className="flex flex-wrap items-start justify-between gap-3 border-b border-slate-100 bg-slate-50 px-5 py-4">
                    <div className="flex min-w-0 items-start gap-3">
                      <div className="rounded-lg bg-white p-2.5 text-indigo-600 shadow-sm"><Layers3 className="h-4 w-4" /></div>
                      <div className="min-w-0">
                        <h2 className="truncate text-base font-semibold text-slate-900">{selectedResource.resource_name}</h2>
                        <div className="mt-1 flex flex-wrap items-center gap-2 font-mono text-[11px] text-slate-500">
                          <span>{selectedResource.resource_code}</span>
                          <Badge variant="outline">{selectedResource.resource_type}</Badge>
                          <Badge variant={selectedResource.status === 'active' ? 'success' : 'slate'}>{selectedResource.status === 'active' ? '启用' : '停用'}</Badge>
                        </div>
                      </div>
                    </div>
                    <Badge variant="outline">{selectedPermissions.length} 项操作权限</Badge>
                  </div>
                  {selectedResource.description && <p className="px-5 py-3 text-xs leading-5 text-slate-600">{selectedResource.description}</p>}
                </Card>

                <Card className="overflow-hidden">
                  <div className="border-b border-slate-100 px-5 py-3">
                    <h3 className="text-sm font-semibold text-slate-800">资源操作与 JRPC 关联</h3>
                    <p className="mt-1 text-[11px] text-slate-500">操作权限由资源目录定义；具体方法策略以 Management 启动时校验的代码声明为准。</p>
                  </div>
                  {selectedPermissions.length === 0 ? (
                    <div className="p-8"><EmptyState icon={ShieldCheck} title="该资源尚无操作权限" description="此资源目前没有可分配的 active 权限项。" /></div>
                  ) : (
                    <div className="divide-y divide-slate-100">
                      {selectedPermissions.map((permission) => {
                        const mappedMethods = methodsForPermission(permission.permission_code);
                        return (
                          <div key={permission.id} className="px-5 py-4">
                            <div className="flex flex-wrap items-start justify-between gap-3">
                              <div className="min-w-0">
                                <div className="flex flex-wrap items-center gap-2 text-sm font-medium text-slate-800">
                                  <span>{permission.permission_name}</span>
                                  <Badge variant="outline">{actionNames[permission.action_code] ?? permission.action_code}</Badge>
                                  <Badge variant={permission.status === 'active' ? 'success' : 'slate'}>{permission.status === 'active' ? '启用' : '停用'}</Badge>
                                </div>
                                <div className="mt-1 font-mono text-[11px] text-slate-500">{permission.permission_code}</div>
                                {permission.description && <p className="mt-1 text-xs leading-5 text-slate-500">{permission.description}</p>}
                              </div>
                              <Badge variant="outline">{mappedMethods.length} 个方法</Badge>
                            </div>
                            <div className="mt-3 space-y-2 border-l-2 border-slate-100 pl-3">
                              {mappedMethods.length === 0 ? (
                                <div className="text-[11px] text-slate-400">目录中没有声明依赖此权限码的 JRPC 方法。</div>
                              ) : mappedMethods.map((method) => (
                                <div key={method.method_code} className="flex flex-wrap items-center gap-2 text-[11px]">
                                  <Code2 className="h-3.5 w-3.5 text-slate-400" />
                                  <code className="font-mono text-slate-700">{method.method_code}</code>
                                  <Badge variant={method.policy_type === 'permission' ? 'primary' : 'outline'}>{policyNames[method.policy_type]}</Badge>
                                </div>
                              ))}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </Card>
              </div>
            ) : (
              <Card className="p-8"><EmptyState icon={Search} title="没有选中的资源" description="从左侧资源列表选择一个资源。" /></Card>
            )}
          </div>
        )
      ) : (
        <Card className="overflow-hidden">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 bg-slate-50 px-5 py-3">
            <div>
              <h2 className="text-sm font-semibold text-slate-800">JRPC 方法授权目录</h2>
              <p className="mt-1 text-[11px] text-slate-500">只读展示实际注册的方法与其显式访问策略；未登记的方法不会被授权门放行。</p>
            </div>
            <Badge variant="outline">{visibleMethods.length} / {methods.length} 个方法</Badge>
          </div>
          {methods.length === 0 ? (
            <div className="p-8"><EmptyState icon={Braces} title="没有已注册的方法" description="Management 启动后会从 JRPC 注册表生成方法目录。" /></div>
          ) : visibleMethods.length === 0 ? (
            <div className="p-8"><EmptyState icon={Search} title="没有匹配的方法" description="调整搜索内容后重试。" /></div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[760px] text-left text-xs">
                <thead className="border-b border-slate-200 bg-white text-[10px] uppercase tracking-wide text-slate-500">
                  <tr><th className="px-5 py-3 font-semibold">JRPC 方法</th><th className="px-4 py-3 font-semibold">访问策略</th><th className="px-4 py-3 font-semibold">所需权限</th><th className="px-5 py-3 font-semibold">资源</th></tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {visibleMethods.map((method) => {
                    const permission = method.permission_code ? permissionsByCode.get(method.permission_code) : undefined;
                    const resource = permission ? resourceByID.get(permission.resource_id) : undefined;
                    return (
                      <tr key={method.method_code} className="align-top hover:bg-slate-50/70">
                        <td className="px-5 py-3.5"><code className="font-mono font-medium text-slate-800">{method.method_code}</code></td>
                        <td className="px-4 py-3.5"><Badge variant={method.policy_type === 'permission' ? 'primary' : 'outline'}>{policyNames[method.policy_type]}</Badge></td>
                        <td className="px-4 py-3.5">
                          {method.permission_code ? (
                            <div><code className="font-mono text-slate-700">{method.permission_code}</code>{!permission && <div className="mt-1 text-[10px] text-amber-700">未找到 active 权限目录项</div>}</div>
                          ) : <span className="text-slate-400">—</span>}
                        </td>
                        <td className="px-5 py-3.5">{resource ? <div><div className="font-medium text-slate-700">{resource.resource_name}</div><code className="text-[10px] text-slate-400">{resource.resource_code}</code></div> : <span className="text-slate-400">—</span>}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      )}

      <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-xs leading-5 text-blue-800">
        此页为只读目录。资源与权限由数据库目录维护，JRPC 授权策略由代码显式声明并在启动时与注册方法核对；角色权限关系在角色管理中配置，缺少授权或状态不可用时服务端默认拒绝。
      </div>
    </div>
  );
};
