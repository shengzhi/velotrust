import React, { useEffect, useMemo, useState } from 'react';
import {
  Activity, Compass, Download, Eye, EyeOff, FileSpreadsheet, FileText, Fingerprint,
  FolderTree, Globe, History, Key, KeyRound, Layers, LayoutDashboard, ListTree,
  Monitor, MousePointerClick, Network, Pencil, Plus, RefreshCw,
  Search, Server, Settings2, Shield, ShieldCheck, SlidersHorizontal, Trash2,
  Users, UsersRound, type LucideIcon,
} from 'lucide-react';
import { iamApi, IAMMenu, IAMMenuInput } from '../../services/iam';
import { useAuth } from '../../contexts/AuthContext';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Badge } from '../../components/ui/badge';
import { Card } from '../../components/ui/card';
import { Dialog } from '../../components/ui/dialog';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { EmptyState } from '../../components/EmptyState';
import { useToast } from '../../components/ui/toast';
import { formatDateTime } from '../../utils/format';

type MenuDraft = {
  parent_id: string;
  menu_name: string;
  menu_code: string;
  path: string;
  component: string;
  icon: string;
  sort_order: string;
  menu_type: IAMMenu['menu_type'];
  visible: boolean;
  status: IAMMenu['status'];
};

type MenuRow = { menu: IAMMenu; depth: number };
type MenuFilter = 'all' | IAMMenu['menu_type'];

const menuIcons: Record<string, LucideIcon> = {
  Activity, Compass, Download, FileSpreadsheet, FileText, Fingerprint, FolderTree,
  Globe, History, Key, KeyRound, Layers, LayoutDashboard, ListTree, Monitor,
  MousePointerClick, Network, Plus, Server, Settings2, Shield, ShieldCheck,
  SlidersHorizontal, Users, UsersRound,
};

const MenuIcon: React.FC<{ name: string | null | undefined; className?: string }> = ({ name, className }) => {
  const Icon = name ? menuIcons[name] ?? ListTree : ListTree;
  return <Icon className={className} aria-hidden="true" />;
};

const emptyDraft: MenuDraft = {
  parent_id: '', menu_name: '', menu_code: '', path: '', component: '', icon: '',
  sort_order: '0', menu_type: 'directory', visible: true, status: 'active',
};

const toDraft = (menu: IAMMenu): MenuDraft => ({
  parent_id: menu.parent_id ? String(menu.parent_id) : '',
  menu_name: menu.menu_name,
  menu_code: menu.menu_code,
  path: menu.path ?? '',
  component: menu.component ?? '',
  icon: menu.icon ?? '',
  sort_order: String(menu.sort_order),
  menu_type: menu.menu_type,
  visible: menu.visible,
  status: menu.status,
});

const makeInput = (draft: MenuDraft): IAMMenuInput => ({
  parent_id: draft.menu_type === 'directory' || !draft.parent_id ? null : Number(draft.parent_id),
  menu_name: draft.menu_name.trim(),
  menu_code: draft.menu_code.trim(),
  path: draft.path.trim() || null,
  component: draft.component.trim() || null,
  icon: draft.icon.trim() || null,
  sort_order: Number(draft.sort_order),
  menu_type: draft.menu_type,
  visible: draft.visible,
  status: draft.status,
});

const flattenMenus = (menus: IAMMenu[]): MenuRow[] => {
  const children = new Map<number | null, IAMMenu[]>();
  for (const menu of menus) {
    const list = children.get(menu.parent_id) ?? [];
    list.push(menu);
    children.set(menu.parent_id, list);
  }
  for (const list of children.values()) list.sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);

  const rows: MenuRow[] = [];
  const visited = new Set<number>();
  const visit = (parentID: number | null, depth: number) => {
    for (const menu of children.get(parentID) ?? []) {
      if (visited.has(menu.id)) continue;
      visited.add(menu.id);
      rows.push({ menu, depth });
      visit(menu.id, depth + 1);
    }
  };
  visit(null, 0);
  for (const menu of menus) {
    if (!visited.has(menu.id)) rows.push({ menu, depth: 0 });
  }
  return rows;
};

export const Menus: React.FC = () => {
  const { hasPermission } = useAuth();
  const { success, error } = useToast();
  const [menus, setMenus] = useState<IAMMenu[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<MenuFilter>('all');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<IAMMenu | null>(null);
  const [draft, setDraft] = useState<MenuDraft>(emptyDraft);
  const [deleting, setDeleting] = useState<IAMMenu | null>(null);

  const loadMenus = async () => {
    setLoading(true);
    try {
      setMenus(await iamApi.listMenus());
    } catch (err) {
      error(err instanceof Error ? err.message : '加载菜单失败。');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadMenus(); }, []);

  const directories = useMemo(() => menus.filter((menu) => menu.menu_type === 'directory' && menu.status === 'active'), [menus]);
  const rows = useMemo(() => {
    const query = search.trim().toLocaleLowerCase();
    return flattenMenus(menus).filter(({ menu }) => {
      const matchesFilter = filter === 'all' || menu.menu_type === filter;
      const matchesSearch = !query || [menu.menu_name, menu.menu_code, menu.path ?? '', menu.component ?? '']
        .some((value) => value.toLocaleLowerCase().includes(query));
      return matchesFilter && matchesSearch;
    });
  }, [menus, search, filter]);

  const canWrite = hasPermission('iam.role.write');
  const pageCount = menus.filter((menu) => menu.menu_type === 'page' || menu.menu_type === 'external').length;
  const hiddenCount = menus.filter((menu) => !menu.visible).length;
  const actionCount = menus.filter((menu) => menu.menu_type === 'button').length;

  const openCreate = () => {
    setEditing(null);
    setDraft(emptyDraft);
    setDialogOpen(true);
  };

  const openEdit = (menu: IAMMenu) => {
    setEditing(menu);
    setDraft(toDraft(menu));
    setDialogOpen(true);
  };

  const saveMenu = async (event: React.FormEvent) => {
    event.preventDefault();
    setSaving(true);
    try {
      const input = makeInput(draft);
      if (editing) {
        await iamApi.updateMenu({ ...editing, ...input, version_no: editing.version_no });
        success('菜单已更新。');
      } else {
        await iamApi.createMenu(input);
        success('菜单已创建。');
      }
      setDialogOpen(false);
      await loadMenus();
    } catch (err) {
      error(err instanceof Error ? err.message : '保存菜单失败。');
    } finally {
      setSaving(false);
    }
  };

  const deleteMenu = async () => {
    if (!deleting) return;
    try {
      await iamApi.deleteMenu(deleting.id);
      success(`已删除「${deleting.menu_name}」。`);
      setDeleting(null);
      await loadMenus();
    } catch (err) {
      error(err instanceof Error ? err.message : '删除菜单失败。子菜单或角色绑定需要先处理。');
    }
  };

  const menuTypeLabel = (type: IAMMenu['menu_type']) => ({ directory: '目录', page: '页面', external: '外链', button: '按钮权限' })[type];
  const parentOptions = draft.menu_type === 'button'
    ? menus.filter((menu) => menu.menu_type === 'page' && menu.status === 'active')
    : directories;

  return (
    <div className="space-y-5">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-slate-900">菜单管理</h1>
          <p className="mt-1 text-xs text-slate-500">维护控制台导航树、路由标识和菜单显示状态；菜单数据保存至 Management IAM。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => void loadMenus()} isLoading={loading} aria-label="刷新菜单"><RefreshCw className="h-4 w-4" /></Button>
          {canWrite && <Button onClick={openCreate}><Plus className="h-4 w-4" />新建菜单</Button>}
        </div>
      </div>

      {!canWrite && <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800">当前账号只有读取权限；创建、编辑和删除菜单需要 iam.role.write。</div>}

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Card className="flex items-center gap-3 p-4"><span className="rounded-lg bg-blue-50 p-2 text-blue-700"><FolderTree className="h-4 w-4" /></span><div><div className="text-xl font-semibold text-slate-900">{directories.length}</div><div className="text-xs text-slate-500">导航目录</div></div></Card>
        <Card className="flex items-center gap-3 p-4"><span className="rounded-lg bg-indigo-50 p-2 text-indigo-700"><ListTree className="h-4 w-4" /></span><div><div className="text-xl font-semibold text-slate-900">{pageCount}</div><div className="text-xs text-slate-500">页面与外链</div></div></Card>
        <Card className="flex items-center gap-3 p-4"><span className="rounded-lg bg-violet-50 p-2 text-violet-700"><MousePointerClick className="h-4 w-4" /></span><div><div className="text-xl font-semibold text-slate-900">{actionCount}</div><div className="text-xs text-slate-500">按钮级权限项</div></div></Card>
        <Card className="flex items-center gap-3 p-4"><span className="rounded-lg bg-slate-100 p-2 text-slate-600"><EyeOff className="h-4 w-4" /></span><div><div className="text-xl font-semibold text-slate-900">{hiddenCount}</div><div className="text-xs text-slate-500">隐藏菜单</div></div></Card>
      </div>

      <Card className="overflow-hidden">
        <div className="flex flex-col gap-3 border-b border-slate-200 bg-slate-50 p-4 sm:flex-row sm:items-center sm:justify-between">
          <div><h2 className="text-sm font-semibold text-slate-800">导航结构</h2><p className="mt-0.5 text-[11px] text-slate-500">目录作为根节点，页面和外链放在目录下；排序值越小越靠前。</p></div>
          <div className="flex flex-col gap-2 sm:flex-row">
            <label className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" /><Input className="pl-9 sm:w-64" aria-label="搜索菜单" placeholder="搜索名称、编码或路由" value={search} onChange={(event) => setSearch(event.target.value)} /></label>
            <select className="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700" value={filter} onChange={(event) => setFilter(event.target.value as MenuFilter)} aria-label="菜单类型筛选">
              <option value="all">全部类型</option><option value="directory">目录</option><option value="page">页面</option><option value="external">外链</option><option value="button">按钮权限</option>
            </select>
          </div>
        </div>

        <div className="overflow-x-auto">
          <div className="min-w-[980px]">
            <div className="grid grid-cols-[minmax(220px,1.4fr)_minmax(140px,1fr)_minmax(170px,1.1fr)_80px_70px_80px_100px] gap-4 border-b border-slate-200 px-5 py-3 text-[10px] font-semibold uppercase tracking-wide text-slate-500">
              <span>菜单名称</span><span>菜单编码</span><span>路由 / 组件</span><span>类型</span><span>排序</span><span>状态</span><span className="text-right">操作</span>
            </div>
            {loading ? <div className="p-10 text-center text-sm text-slate-500">正在从 Management 加载菜单…</div> : rows.length === 0 ? (
              <div className="p-8"><EmptyState icon={ListTree} title={menus.length ? '没有匹配的菜单' : '菜单目录为空'} description={menus.length ? '调整搜索关键词或类型筛选。' : '运行菜单 seed migration 后，原型中的导航结构会显示在这里。'} /></div>
            ) : rows.map(({ menu, depth }) => (
              <div key={menu.id} className="grid grid-cols-[minmax(220px,1.4fr)_minmax(140px,1fr)_minmax(170px,1.1fr)_80px_70px_80px_100px] items-center gap-4 border-b border-slate-100 px-5 py-3.5 last:border-0 hover:bg-slate-50/70">
                <div className="flex min-w-0 items-center gap-2" style={{ paddingLeft: depth * 22 }}>
                  <MenuIcon name={menu.icon} className={`h-4 w-4 shrink-0 ${menu.menu_type === 'button' ? 'text-violet-600' : 'text-blue-600'}`} />
                  <div className="flex min-w-0 items-center gap-2 truncate text-sm font-medium text-slate-800">{menu.menu_name}{!menu.visible && <EyeOff className="h-3.5 w-3.5 shrink-0 text-slate-400" />}</div>
                </div>
                <span className="truncate font-mono text-[11px] text-slate-600">{menu.menu_code}</span>
                <div className="min-w-0"><div className="truncate font-mono text-[11px] text-slate-700">{menu.path || '—'}</div>{menu.component && <div className="truncate text-[10px] text-slate-400">{menu.component}</div>}</div>
                <Badge variant={menu.menu_type === 'directory' ? 'primary' : 'outline'}>{menuTypeLabel(menu.menu_type)}</Badge>
                <span className="font-mono text-xs tabular-nums text-slate-600">{menu.sort_order}</span>
                <Badge variant={menu.status === 'active' ? 'success' : 'slate'}>{menu.status === 'active' ? '启用' : '停用'}</Badge>
                <div className="flex justify-end gap-1">
                  {canWrite && <>
                    <Button size="sm" variant="ghost" className="px-2" onClick={() => openEdit(menu)} aria-label={`编辑${menu.menu_name}`}><Pencil className="h-3.5 w-3.5" /></Button>
                    <Button size="sm" variant="ghost" className="px-2 text-rose-600 hover:text-rose-700" onClick={() => setDeleting(menu)} aria-label={`删除${menu.menu_name}`}><Trash2 className="h-3.5 w-3.5" /></Button>
                  </>}
                </div>
              </div>
            ))}
          </div>
        </div>
        <div className="flex items-center justify-between border-t border-slate-100 bg-white px-5 py-3 text-[11px] text-slate-500"><span>共 {menus.length} 个菜单项</span><span>最近变更在保存时写入 IAM 审计日志</span></div>
      </Card>

      <Dialog isOpen={dialogOpen} onClose={() => setDialogOpen(false)} title={editing ? '编辑菜单' : '新建菜单'} description="菜单结构与路由元数据保存至 Management IAM。" maxWidth="2xl">
        <form className="space-y-4" onSubmit={saveMenu}>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="block space-y-1.5 text-xs font-medium text-slate-700">菜单名称<Input required maxLength={128} value={draft.menu_name} onChange={(event) => setDraft({ ...draft, menu_name: event.target.value })} placeholder="例如：菜单管理" /></label>
            <label className="block space-y-1.5 text-xs font-medium text-slate-700">菜单编码<Input required maxLength={64} pattern="[A-Za-z0-9_.:-]+" value={draft.menu_code} onChange={(event) => setDraft({ ...draft, menu_code: event.target.value })} placeholder="例如：menu:menus" /></label>
            <label className="block space-y-1.5 text-xs font-medium text-slate-700">节点类型<select className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" value={draft.menu_type} onChange={(event) => { const menuType = event.target.value as IAMMenu['menu_type']; setDraft({ ...draft, menu_type: menuType, parent_id: menuType === 'directory' ? '' : '', path: menuType === 'button' ? '' : draft.path, component: menuType === 'button' || menuType === 'external' ? '' : draft.component }); }}><option value="directory">导航目录</option><option value="page">页面菜单</option><option value="external">外部链接</option><option value="button">按钮级权限</option></select></label>
            <label className="block space-y-1.5 text-xs font-medium text-slate-700">{draft.menu_type === 'directory' ? '父级' : draft.menu_type === 'button' ? '所属页面' : '上级目录'}<select required={draft.menu_type !== 'directory'} disabled={draft.menu_type === 'directory'} className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm disabled:bg-slate-100 disabled:text-slate-400" value={draft.menu_type === 'directory' ? '' : draft.parent_id} onChange={(event) => setDraft({ ...draft, parent_id: event.target.value })}><option value="">{draft.menu_type === 'directory' ? '根目录' : draft.menu_type === 'button' ? '选择所属页面' : '选择导航目录'}</option>{parentOptions.filter((menu) => menu.id !== editing?.id).map((menu) => <option key={menu.id} value={menu.id}>{menu.menu_name}</option>)}</select></label>
            <label className="block space-y-1.5 text-xs font-medium text-slate-700">{draft.menu_type === 'external' ? '外部链接' : '路由路径'}<Input maxLength={255} disabled={draft.menu_type === 'button'} required={draft.menu_type === 'page' || draft.menu_type === 'external'} value={draft.path} onChange={(event) => setDraft({ ...draft, path: event.target.value })} placeholder={draft.menu_type === 'external' ? 'https://example.com' : draft.menu_type === 'button' ? '按钮项不配置路由' : '/menus'} /></label>
            <label className="block space-y-1.5 text-xs font-medium text-slate-700">组件路径<Input maxLength={255} disabled={draft.menu_type === 'external' || draft.menu_type === 'button'} value={draft.component} onChange={(event) => setDraft({ ...draft, component: event.target.value })} placeholder="pages/Menus/index.tsx" /></label>
            <label className="block space-y-1.5 text-xs font-medium text-slate-700">图标名称<div className="flex items-center gap-2"><Input className="min-w-0 flex-1" maxLength={128} value={draft.icon} onChange={(event) => setDraft({ ...draft, icon: event.target.value })} placeholder="例如：ListTree" /><span className="flex h-9 min-w-28 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-3 text-xs text-slate-600"><MenuIcon name={draft.icon} className="h-4 w-4 shrink-0 text-blue-600" /><span className="truncate">{draft.icon || '默认图标'}</span></span></div></label>
            <label className="block space-y-1.5 text-xs font-medium text-slate-700">排序值<Input type="number" min={0} max={1000000} required value={draft.sort_order} onChange={(event) => setDraft({ ...draft, sort_order: event.target.value })} /></label>
            <label className="block space-y-1.5 text-xs font-medium text-slate-700">运行状态<select className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value as IAMMenu['status'] })}><option value="active">启用</option><option value="disabled">停用</option></select></label>
            <label className="flex items-center gap-2 self-end pb-2 text-xs font-medium text-slate-700"><input type="checkbox" className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" checked={draft.visible} onChange={(event) => setDraft({ ...draft, visible: event.target.checked })} /><Eye className="h-4 w-4 text-slate-500" />在导航中显示</label>
          </div>
          <div className="rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-[11px] leading-5 text-blue-800">目录作为根节点；页面和外链隶属于目录，按钮权限隶属于页面。按钮节点仅用于权限目录展示，不会单独授权操作。删除操作只允许没有子菜单及角色绑定的节点。</div>
          <div className="flex justify-end gap-2 pt-1"><Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>取消</Button><Button type="submit" isLoading={saving}>{editing ? '保存修改' : '创建菜单'}</Button></div>
        </form>
      </Dialog>

      <ConfirmDialog isOpen={!!deleting} onClose={() => setDeleting(null)} onConfirm={deleteMenu} title="删除菜单" description={deleting ? `确定删除「${deleting.menu_name}」？存在子菜单或角色绑定时，服务端会拒绝删除。` : ''} confirmText="删除" />
    </div>
  );
};
