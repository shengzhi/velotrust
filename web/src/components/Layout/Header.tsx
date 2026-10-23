import React, { useState, useEffect, useRef } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../../contexts/AuthContext';
import { User, LogOut, ChevronDown } from 'lucide-react';
import { useToast } from '../ui/toast';

export const Header: React.FC = () => {
  const { user, logout, isAuthenticated } = useAuth();
  const { error } = useToast();
  const location = useLocation();
  const navigate = useNavigate();
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const [timeStr, setTimeStr] = useState('');

  // Page title mapping
  const getPageTitle = (pathname: string) => {
    if (pathname === '/') return '控制台概览';
    if (pathname.startsWith('/nodes/')) return '节点详情';
    if (pathname.startsWith('/nodes')) return '虚拟节点';
    if (pathname.startsWith('/enrollment-tokens')) return '注册凭证管理';
    if (pathname.startsWith('/topology')) return '网络拓扑图';
    if (pathname.startsWith('/resources')) return '受控资源中心';
    if (pathname.startsWith('/roles')) return '角色管理';
    if (pathname.startsWith('/menus')) return '菜单管理';
    if (pathname.startsWith('/permissions')) return '权限与资源中心';
    if (pathname.startsWith('/api-mappings')) return 'API 映射';
    if (pathname.startsWith('/users')) return '系统用户';
    if (pathname.startsWith('/employee-users')) return '员工用户';
    if (pathname.startsWith('/groups')) return '安全用户组';
    if (pathname.startsWith('/devices')) return '终端设备';
    if (pathname.startsWith('/sites')) return '网络站点';
    if (pathname.startsWith('/ipam')) return 'IPAM 网段管理';
    if (pathname.startsWith('/dns')) return '内网 DNS';
    if (pathname.startsWith('/idp')) return '身份源集成';
    if (pathname.startsWith('/policies')) return '访问策略 (ACL)';
    if (pathname.startsWith('/access-matrix')) return '访问权限透视';
    if (pathname.startsWith('/audit')) return '安全审计日志';
    if (pathname.startsWith('/login-logs')) return '登录日志';
    if (pathname.startsWith('/operation-logs')) return '操作日志';
    if (pathname.startsWith('/profile')) return '个人信息与安全中心';
    if (pathname.startsWith('/settings')) return '系统设置';
    return '控制平面';
  };

  useEffect(() => {
    const updateTime = () => {
      const now = new Date();
      setTimeStr(
        now.toLocaleTimeString('zh-CN', {
          hour12: false,
          hour: '2-digit',
          minute: '2-digit',
        })
      );
    };
    updateTime();
    const timer = setInterval(updateTime, 1000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleLogout = async () => {
    try {
      await logout();
    } catch (err) {
      error(err instanceof Error ? `服务端退出失败，本地会话已清除：${err.message}` : '服务端退出失败，本地会话已清除。');
    } finally {
      navigate('/login');
    }
  };

  return (
    <header className="h-14 bg-white border-b border-slate-200/80 px-6 flex items-center justify-between shrink-0 select-none">
      {/* Page Title */}
      <div className="flex items-center gap-3">
        <h2 className="text-sm font-semibold text-slate-800">
          {getPageTitle(location.pathname)}
        </h2>
        <span className="hidden sm:inline-flex items-center gap-1.5 text-xs text-slate-500">
          <span className={`w-1.5 h-1.5 rounded-full ${isAuthenticated ? 'bg-emerald-500' : 'bg-rose-500'}`} />
          <span>{isAuthenticated ? 'IAM 会话已认证' : 'IAM 会话未认证'}</span>
        </span>
      </div>

      {/* Right Controls */}
      <div className="flex items-center gap-3">
        {/* Time */}
        <div className="hidden xl:flex items-center text-xs font-mono text-slate-400">
          <span>{timeStr}</span>
        </div>

        {/* User Dropdown */}
        <div className="relative" ref={dropdownRef}>
          <button
            onClick={() => setDropdownOpen(!dropdownOpen)}
            className="flex items-center gap-2.5 p-1.5 rounded-lg hover:bg-slate-100 transition-colors focus:outline-none"
          >
            <div className="h-8 w-8 rounded-full bg-blue-100 border border-blue-200 text-blue-700 flex items-center justify-center font-bold text-xs">
              {user?.name ? user.name.slice(0, 1) : <User className="w-4 h-4" />}
            </div>
            <div className="hidden md:flex flex-col text-left">
              <span className="text-xs font-medium text-slate-800 leading-tight">
                {user?.name || user?.username || '管理员'}
              </span>
              <span className="text-[10px] text-slate-400">
                Management IAM
              </span>
            </div>
            <ChevronDown className="w-3.5 h-3.5 text-slate-400" />
          </button>

          {/* Menu popup */}
          {dropdownOpen && (
            <div className="absolute right-0 mt-2 w-64 rounded-xl bg-white p-2 shadow-xl border border-slate-200 z-50 animate-in fade-in slide-in-from-top-1 duration-150">
              <div className="px-3 py-2 border-b border-slate-100 mb-1">
                <p className="text-xs font-semibold text-slate-800">{user?.name || user?.username}</p>
                <p className="text-[11px] text-slate-400 truncate">{user?.email}</p>
              </div>

              <div className="px-3 py-2 text-[11px] text-slate-500 bg-slate-50 rounded-lg my-1 border border-slate-100">
                <div className="font-medium text-slate-700 mb-0.5">权限状态</div>
                <div className="text-[10px] text-slate-500 leading-relaxed">每项管理操作由 Management 根据当前账号权限进行校验。</div>
              </div>

              <div className="border-t border-slate-100 my-1 pt-1 space-y-1">
                <button
                  onClick={() => {
                    setDropdownOpen(false);
                    navigate('/profile');
                  }}
                  className="w-full flex items-center justify-between px-3 py-2 text-xs font-medium text-slate-700 hover:bg-blue-50/70 hover:text-blue-700 rounded-lg transition-colors group"
                >
                  <div className="flex items-center gap-2">
                    <User className="w-3.5 h-3.5 text-slate-400 group-hover:text-blue-600 transition-colors" />
                    <span>个人信息与安全中心</span>
                  </div>
                  <span className="text-[10px] text-blue-600 font-medium">配置</span>
                </button>
              </div>

              <div className="border-t border-slate-100 mt-1 pt-1">
                <button
                  onClick={() => void handleLogout()}
                  className="w-full flex items-center gap-2 px-3 py-2 text-xs font-medium text-rose-600 hover:bg-rose-50 rounded-lg transition-colors"
                >
                  <LogOut className="w-3.5 h-3.5" />
                  <span>退出登录</span>
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </header>
  );
};
