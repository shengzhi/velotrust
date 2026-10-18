import React, { useEffect, useRef, useState } from 'react';
import { KeyRound, ListChecks, Plus, RefreshCw } from 'lucide-react';
import { iamApi, IAMMenu, IAMRole } from '../../services/iam';
import { useAuth } from '../../contexts/AuthContext';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Badge } from '../../components/ui/badge';
import { Card } from '../../components/ui/card';
import { Dialog } from '../../components/ui/dialog';
import { EmptyState } from '../../components/EmptyState';
import { useToast } from '../../components/ui/toast';
import { formatDateTime } from '../../utils/format';

const emptyRole = { role_name: '', role_code: '', description: '' };

interface MenuNode extends IAMMenu {
  children: MenuNode[];
}

function buildMenuTree(menus: IAMMenu[]): MenuNode[] {
  const nodes = new Map<number, MenuNode>();
  menus.forEach((menu) => nodes.set(menu.id, { ...menu, children: [] }));
  const roots: MenuNode[] = [];
  nodes.forEach((node) => {
    const parent = node.parent_id == null ? undefined : nodes.get(node.parent_id);
    if (parent) parent.children.push(node);
    else roots.push(node);
  });
  const sort = (items: MenuNode[]) => {
    items.sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);
    items.forEach((item) => sort(item.children));
  };
  sort(roots);
  return roots;
}

function descendantIDs(node: MenuNode): number[] {
  return node.children.flatMap((child) => [child.id, ...descendantIDs(child)]);
}

const MenuCheckbox: React.FC<{
  checked: boolean;
  indeterminate: boolean;
  onChange: (checked: boolean) => void;
}> = ({ checked, indeterminate, onChange }) => {
  const inputRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (inputRef.current) inputRef.current.indeterminate = indeterminate;
  }, [indeterminate]);
  return (
    <input
      ref={inputRef}
      type="checkbox"
      checked={checked && !indeterminate}
      onChange={(event) => onChange(event.target.checked)}
      className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
    />
  );
};

export const Roles: React.FC = () => {
  const { hasPermission, refreshCurrentUser } = useAuth();
  const { success, error } = useToast();
  const [roles, setRoles] = useState<IAMRole[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState(emptyRole);
  const [saving, setSaving] = useState(false);
  const [menuRole, setMenuRole] = useState<IAMRole | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuLoading, setMenuLoading] = useState(false);
  const [menuSaving, setMenuSaving] = useState(false);
  const [allMenus, setAllMenus] = useState<IAMMenu[]>([]);
  const [selectedMenuIDs, setSelectedMenuIDs] = useState<Set<number>>(new Set());

  const loadRoles = async () => {
    setLoading(true);
    try {
      setRoles(await iamApi.listRoles());
    } catch (err) {
      error(err instanceof Error ? err.message : '加载角色失败。');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadRoles(); }, []);

  const createRole = async (event: React.FormEvent) => {
    event.preventDefault();
    setSaving(true);
    try {
      await iamApi.createRole({
        role_name: form.role_name.trim(),
        role_code: form.role_code.trim(),
        description: form.description.trim(),
      });
      setForm(emptyRole);
      setCreateOpen(false);
      success('角色已创建。请继续配置菜单和权限。');
      await loadRoles();
    } catch (err) {
      error(err instanceof Error ? err.message : '创建角色失败。');
    } finally {
      setSaving(false);
    }
  };

  const openMenuEditor = async (role: IAMRole) => {
    setMenuRole(role);
    setMenuOpen(true);
    setMenuLoading(true);
    try {
      const [catalog, assigned] = await Promise.all([
        iamApi.listMenus(),
        iamApi.listRoleMenus(role.id),
      ]);
      setAllMenus(catalog);
      setSelectedMenuIDs(new Set(assigned.map((menu) => menu.id)));
    } catch (err) {
      error(err instanceof Error ? err.message : '加载角色菜单失败。');
      setMenuOpen(false);
    } finally {
      setMenuLoading(false);
    }
  };

  const toggleMenu = (node: MenuNode, checked: boolean) => {
    const descendants = descendantIDs(node);
    const byID = new Map(allMenus.map((menu) => [menu.id, menu]));
    setSelectedMenuIDs((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(node.id);
        descendants.forEach((id) => next.add(id));
        let parentID = node.parent_id;
        while (parentID != null) {
          next.add(parentID);
          parentID = byID.get(parentID)?.parent_id ?? null;
        }
      } else {
        next.delete(node.id);
        descendants.forEach((id) => next.delete(id));
        let parentID = node.parent_id;
        while (parentID != null) {
          const parent = byID.get(parentID);
          if (!parent) break;
          const parentNode = treeNodes.get(parentID);
          const parentDescendants = parentNode ? descendantIDs(parentNode) : [];
          if (parentDescendants.some((id) => next.has(id))) next.add(parentID);
          else next.delete(parentID);
          parentID = parent.parent_id;
        }
      }
      return next;
    });
  };

  const tree = buildMenuTree(allMenus);
  const treeNodes = new Map<number, MenuNode>();
  const indexNodes = (nodes: MenuNode[]) => nodes.forEach((node) => {
    treeNodes.set(node.id, node);
    indexNodes(node.children);
  });
  indexNodes(tree);

  const renderMenuNodes = (nodes: MenuNode[], depth = 0): React.ReactNode => nodes.map((node) => {
    const descendants = descendantIDs(node);
    const checkedDescendants = descendants.filter((id) => selectedMenuIDs.has(id)).length;
    const isSelected = selectedMenuIDs.has(node.id);
    const indeterminate = (isSelected || checkedDescendants > 0) && checkedDescendants < descendants.length;
    return (
      <React.Fragment key={node.id}>
        <div className="flex min-h-11 items-center gap-3 border-b border-slate-100 px-4 py-2 last:border-0" style={{ paddingLeft: `${16 + depth * 24}px` }}>
          <MenuCheckbox checked={isSelected} indeterminate={indeterminate} onChange={(checked) => toggleMenu(node, checked)} />
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-medium text-slate-800">{node.menu_name}</div>
            <div className="truncate font-mono text-[10px] text-slate-400">{node.menu_code}</div>
          </div>
          <Badge variant="outline">{{ directory: '目录', page: '页面', external: '外链', button: '按钮' }[node.menu_type]}</Badge>
          {node.status !== 'active' && <Badge variant="slate">已停用</Badge>}
        </div>
        {node.children.length > 0 && renderMenuNodes(node.children, depth + 1)}
      </React.Fragment>
    );
  });

  const saveRoleMenus = async () => {
    if (!menuRole) return;
    setMenuSaving(true);
    try {
      await iamApi.setRoleMenus(menuRole.id, [...selectedMenuIDs].sort((a, b) => a - b));
      success('角色菜单已保存。菜单控制导航显示，具体操作仍由权限策略校验。');
      setMenuOpen(false);
      await refreshCurrentUser();
    } catch (err) {
      error(err instanceof Error ? err.message : '保存角色菜单失败。');
    } finally {
      setMenuSaving(false);
    }
  };

  const canWrite = hasPermission('iam.role.write');

  return (
    <div className="space-y-5">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-slate-900">角色管理</h1>
          <p className="mt-1 text-xs text-slate-500">为角色配置可见菜单；菜单授权决定导航范围，实际操作仍由 IAM 权限码校验。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void loadRoles()} isLoading={loading} aria-label="刷新角色">
            <RefreshCw className="h-4 w-4" />
          </Button>
          {canWrite && <Button onClick={() => setCreateOpen(true)}><Plus className="h-4 w-4" />新建角色</Button>}
        </div>
      </div>

      <Card className="overflow-hidden">
        <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1.3fr)_auto_auto_auto] gap-4 border-b border-slate-200 bg-slate-50 px-5 py-3 text-[11px] font-semibold uppercase tracking-wide text-slate-500">
          <span>名称</span><span>编码</span><span>说明</span><span>类型</span><span>状态</span><span className="text-right">操作</span>
        </div>
        {loading ? (
          <div className="p-10 text-center text-sm text-slate-500">正在从 Management 加载角色…</div>
        ) : roles.length === 0 ? (
          <div className="p-8"><EmptyState icon={KeyRound} title="暂无角色" description="有权限的管理员可以创建角色。" /></div>
        ) : roles.map((role) => (
          <div key={role.id} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1.3fr)_auto_auto_auto] items-center gap-4 border-b border-slate-100 px-5 py-4 last:border-0">
            <div className="min-w-0"><div className="truncate text-sm font-semibold text-slate-800">{role.role_name}</div><div className="mt-1 text-[11px] text-slate-400">创建于 {formatDateTime(role.created_at)}</div></div>
            <span className="truncate font-mono text-xs text-slate-600">{role.role_code}</span>
            <span className="truncate text-xs text-slate-500">{role.description || '—'}</span>
            <Badge variant={role.is_system ? 'primary' : 'outline'}>{role.is_system ? '系统' : '自定义'}</Badge>
            <Badge variant={role.status === 'active' ? 'success' : 'slate'}>{role.status}</Badge>
            {canWrite ? (
              <Button variant="outline" size="sm" onClick={() => void openMenuEditor(role)} disabled={role.is_system} title={role.is_system ? '系统角色菜单不可修改' : '配置角色菜单'}>
                <ListChecks className="h-4 w-4" />配置菜单
              </Button>
            ) : <span className="text-right text-xs text-slate-400">—</span>}
          </div>
        ))}
      </Card>

      <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-xs leading-5 text-blue-800">
        菜单授权只影响角色可见的导航和页面入口。JRPC 方法与业务操作仍由 IAM 权限码独立校验；系统角色菜单保持只读。
      </div>

      <Dialog isOpen={createOpen} onClose={() => setCreateOpen(false)} title="创建 IAM 角色" description="名称与唯一编码保存至 Management。" maxWidth="lg">
        <form className="space-y-4" onSubmit={createRole}>
          <label className="block space-y-1.5 text-xs font-medium text-slate-700">角色名称<Input required maxLength={64} value={form.role_name} onChange={(event) => setForm({ ...form, role_name: event.target.value })} placeholder="例如：网络运维" /></label>
          <label className="block space-y-1.5 text-xs font-medium text-slate-700">角色编码<Input required maxLength={64} pattern="[A-Za-z0-9_.-]+" value={form.role_code} onChange={(event) => setForm({ ...form, role_code: event.target.value })} placeholder="例如：network_operator" /></label>
          <label className="block space-y-1.5 text-xs font-medium text-slate-700">说明<Input maxLength={512} value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} placeholder="可选" /></label>
          <div className="flex justify-end gap-2 pt-2"><Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>取消</Button><Button type="submit" isLoading={saving}>创建角色</Button></div>
        </form>
      </Dialog>

      <Dialog
        isOpen={menuOpen}
        onClose={() => setMenuOpen(false)}
        title={menuRole ? `配置菜单 · ${menuRole.role_name}` : '配置角色菜单'}
        description="选择后端导航目录和页面；勾选父项会同步选择其全部子项。"
        maxWidth="3xl"
      >
        <div className="space-y-4">
          <div className="rounded-lg border border-amber-100 bg-amber-50 px-4 py-3 text-xs leading-5 text-amber-800">
            菜单只控制导航可见范围。若要授权页面中的读取、创建或修改操作，还需在“受控资源与权限”中配置对应权限。
          </div>
          <div className="overflow-hidden rounded-lg border border-slate-200">
            <div className="flex items-center justify-between border-b border-slate-200 bg-slate-50 px-4 py-3 text-xs font-semibold text-slate-600">
              <span>菜单树</span><span>已选 {selectedMenuIDs.size} 项</span>
            </div>
            <div className="max-h-[55vh] overflow-y-auto">
              {menuLoading ? (
                <div className="p-8 text-center text-sm text-slate-500">正在加载菜单…</div>
              ) : allMenus.length === 0 ? (
                <div className="p-8 text-center text-sm text-slate-500">当前没有可分配的菜单。</div>
              ) : renderMenuNodes(tree)}
            </div>
          </div>
          <div className="flex justify-end gap-2 pt-1">
            <Button type="button" variant="outline" onClick={() => setMenuOpen(false)}>取消</Button>
            <Button type="button" onClick={() => void saveRoleMenus()} isLoading={menuSaving} disabled={menuLoading || !!menuRole?.is_system}>保存菜单</Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
};
