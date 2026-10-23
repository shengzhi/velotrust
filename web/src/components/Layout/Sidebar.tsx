import React from 'react';
import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  Server,
  Network,
  Users,
  UsersRound,
  Monitor,
  Globe,
  Shield,
  FileText,
  Layers,
  Compass,
  Fingerprint,
  Settings2,
  ShieldCheck,
  KeyRound,
  History,
  ClipboardList,
  Code2,
  Key,
  ListTree,
  SlidersHorizontal,
  FileSpreadsheet,
} from 'lucide-react';

import { cn } from '../../lib/utils';
import { useAuth } from '../../contexts/AuthContext';
import { IAMMenu } from '../../services/iam';

interface MenuItem {
  label: string;
  icon: any;
  to: string;
  badge?: string;
  external?: boolean;
}

interface MenuSection {
  title: string;
  items: MenuItem[];
}

const menuIcons: Record<string, any> = {
  LayoutDashboard, Server, Network, Users, UsersRound, Monitor, Globe, Shield,
  FileText, Layers, Compass, Fingerprint, Settings2, ShieldCheck, KeyRound,
  History, Key, ListTree, SlidersHorizontal, FileSpreadsheet,
};

const applicationRoutes = new Set([
  '/', '/topology', '/resources', '/ipam', '/nodes', '/enrollment-tokens', '/sites', '/dns',
  '/policies', '/access-matrix', '/devices', '/employee-users', '/users', '/groups', '/roles', '/menus',
  '/permissions', '/idp', '/audit', '/operation-logs', '/login-logs', '/settings',
]);

function sectionsFromMenus(menus: IAMMenu[]): MenuSection[] {
  const directories = menus
    .filter((menu) => menu.menu_type === 'directory' && menu.status === 'active' && menu.visible)
    .sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);

  return directories.map((directory) => ({
    title: directory.menu_name,
    items: menus
      .filter((menu) => menu.parent_id === directory.id && menu.status === 'active' && menu.visible && (menu.menu_type === 'page' || menu.menu_type === 'external'))
      .sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
      .filter((menu) => menu.menu_type === 'external' || (!!menu.path && applicationRoutes.has(menu.path)))
      .map((menu) => ({
        label: menu.menu_name,
        icon: menuIcons[menu.icon ?? ''] ?? ListTree,
        to: menu.path ?? '',
        external: menu.menu_type === 'external',
      })),
  })).filter((section) => section.items.length > 0);
}

export const Sidebar: React.FC = () => {
  const { canAccessRoute, menus, user } = useAuth();

  // Dynamic RBAC filtering based on current active role's permitted routes
  const accountSection: MenuSection = {
    title: '账户与安全',
    items: [{ label: '个人信息与安全', icon: Fingerprint, to: '/profile' }],
  };
  const sourceSections = [...sectionsFromMenus(menus), accountSection];
  const filteredSections = sourceSections
    .map((section) => ({
      ...section,
      items: section.items.filter((item) => canAccessRoute(item.to)),
    }))
    .filter((section) => section.items.length > 0);

  return (
    <aside className="w-64 bg-slate-900 text-slate-300 flex flex-col h-screen shrink-0 border-r border-slate-800 select-none">
      {/* Brand Header */}
      <div className="h-16 flex items-center gap-3 px-5 border-b border-slate-800/80 bg-slate-950/40">
        <div className="h-9 w-9 rounded-xl bg-gradient-to-tr from-blue-700 to-indigo-500 flex items-center justify-center text-white shadow-md shadow-blue-900/40">
          <Layers className="h-5 w-5" />
        </div>
        <div>
          <div className="flex items-center gap-1.5">
            <h1 className="text-sm font-bold text-white tracking-wide">VeloTrust</h1>
            <span className="px-1.5 py-0.2 rounded text-[9px] font-bold bg-blue-500/20 text-blue-400 border border-blue-500/30">
              ZTNA
            </span>
          </div>
          <p className="text-[10px] text-slate-400 font-medium">企业级零信任控制平面</p>
        </div>
      </div>

      {/* Nav Menu */}
      <nav className="flex-1 py-3 px-3 space-y-4 overflow-y-auto">
        {filteredSections.map((section, sIdx) => (
          <div key={sIdx} className="space-y-1">
            <div className="px-3 py-1 text-[10px] font-semibold uppercase tracking-wider text-slate-500">
              {section.title}
            </div>
            {section.items.map((item) => {
              const Icon = item.icon;
              const renderContents = (isActive: boolean) => (
                <>
                  <div className="flex items-center gap-2.5 truncate">
                    <Icon className={cn('h-4 w-4 shrink-0 transition-colors', isActive ? 'text-white' : 'text-slate-400 group-hover:text-slate-200')} />
                    <span className="truncate">{item.label}</span>
                  </div>
                  {item.badge && (
                    <span className={cn('text-[9px] px-1.5 py-0.5 rounded font-mono', isActive ? 'bg-white/20 text-white' : 'bg-slate-800 text-blue-400 border border-blue-500/30')}>
                      {item.badge}
                    </span>
                  )}
                </>
              );
              const linkClassName = ({ isActive = false }: { isActive?: boolean } = {}) => cn(
                'group flex items-center justify-between px-3 py-2 rounded-lg text-xs font-medium transition-all duration-150',
                isActive ? 'bg-blue-600 text-white shadow-sm shadow-blue-600/20 font-semibold' : 'text-slate-300 hover:bg-slate-800/70 hover:text-white'
              );
              if (item.external) {
                return <a key={item.to} href={item.to} target="_blank" rel="noreferrer" className={linkClassName()}>{renderContents(false)}</a>;
              }
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.to === '/'}
                  className={linkClassName}
                >
                  {({ isActive }) => renderContents(isActive)}
                </NavLink>
              );
            })}
          </div>
        ))}
      </nav>

      {/* Current Management identity */}
      <div className="px-3 py-2 bg-slate-950/60 border-t border-slate-800/60">
        <div className="flex items-center gap-2 px-2 py-1.5 rounded-md bg-slate-800/50 border border-slate-700/50">
          <Fingerprint className="w-3.5 h-3.5 text-emerald-400 shrink-0" />
          <div className="min-w-0 flex-1">
            <div className="text-[10px] text-slate-400 font-medium">Management IAM 账号</div>
            <div className="text-xs font-semibold text-slate-200 truncate">{user?.name || user?.username}</div>
          </div>
        </div>
      </div>

      {/* Footer Info */}
      <div className="p-3 border-t border-slate-800/80 bg-slate-950/40">
        <div className="flex items-center justify-between p-2 rounded-lg bg-slate-800/60 border border-slate-800 text-xs">
          <div className="flex items-center gap-2">
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
              <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
            </span>
            <span className="text-[11px] text-slate-300 font-medium">ZTNA Mesh Core</span>
          </div>
          <span className="text-[10px] text-slate-400 font-mono">v2.5.0</span>
        </div>
      </div>
    </aside>
  );
};
