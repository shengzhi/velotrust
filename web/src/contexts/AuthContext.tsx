import React, { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { iamApi, IAMMenu, IAMUser } from '../services/iam';
import { IAM_TOKEN_KEY } from '../services/jrpc';
import { getPasskey } from '../services/webauthn';

export type AuthUser = IAMUser & {
  name: string;
  email: string;
};

interface AuthContextType {
  user: AuthUser | null;
  menus: IAMMenu[];
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (username: string, password: string, totpCode?: string) => Promise<void>;
  loginWithPasskey: () => Promise<void>;
  logout: () => Promise<void>;
  refreshCurrentUser: () => Promise<void>;
  hasPermission: (permissionCode: string) => boolean;
  canAccessRoute: (pathname: string) => boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);
const LEGACY_USER_KEY = 'wg_mesh_auth_user';
const SESSION_EXPIRY_KEY = `${IAM_TOKEN_KEY}.expires_at`;

function toAuthUser(user: IAMUser): AuthUser {
  return {
    ...user,
    name: user.display_name || user.username,
    email: user.email || '',
    permissions: user.permissions ?? [],
    passkeys: user.passkeys ?? [],
  };
}

function saveSession(token: string, expiresAt: string): void {
  sessionStorage.setItem(IAM_TOKEN_KEY, token);
  sessionStorage.setItem(SESSION_EXPIRY_KEY, expiresAt);
  localStorage.removeItem(LEGACY_USER_KEY);
  localStorage.removeItem('wg_mesh_active_role_code');
}

function clearSession(): void {
  sessionStorage.removeItem(IAM_TOKEN_KEY);
  sessionStorage.removeItem(SESSION_EXPIRY_KEY);
  localStorage.removeItem(LEGACY_USER_KEY);
  localStorage.removeItem('wg_mesh_active_role_code');
}

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [menus, setMenus] = useState<IAMMenu[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let mounted = true;
    const handleUnauthorized = () => {
      clearSession();
      if (mounted) {
        setUser(null);
        setMenus([]);
      }
    };
    window.addEventListener('velotrust:iam-unauthorized', handleUnauthorized);
    const token = sessionStorage.getItem(IAM_TOKEN_KEY);
    const expiry = sessionStorage.getItem(SESSION_EXPIRY_KEY);
    localStorage.removeItem(LEGACY_USER_KEY);
    localStorage.removeItem('wg_mesh_active_role_code');

    if (!token || !expiry || Date.parse(expiry) <= Date.now()) {
      clearSession();
      setIsLoading(false);
      return () => {
        mounted = false;
        window.removeEventListener('velotrust:iam-unauthorized', handleUnauthorized);
      };
    }

    void (async () => {
      try {
        const currentUser = await iamApi.getCurrentUser(token);
        let currentMenus: IAMMenu[] = [];
        try {
          currentMenus = await iamApi.listCurrentUserMenus();
        } catch {
          // Menu lookup failure must not expand access; the empty menu set is fail-closed.
        }
        if (mounted) {
          setUser(toAuthUser(currentUser));
          setMenus(currentMenus);
        }
      } catch {
        clearSession();
        if (mounted) {
          setUser(null);
          setMenus([]);
        }
      } finally {
        if (mounted) setIsLoading(false);
      }
    })();

    return () => {
      mounted = false;
      window.removeEventListener('velotrust:iam-unauthorized', handleUnauthorized);
    };
  }, []);

  const acceptGrant = async (grant: { access_token: string; expires_at: string; user: IAMUser }) => {
    if (!grant.access_token || !grant.expires_at) throw new Error('Management 未返回有效会话。');
    saveSession(grant.access_token, grant.expires_at);
    try {
      const currentUser = await iamApi.getCurrentUser(grant.access_token);
      let currentMenus: IAMMenu[] = [];
      try {
        currentMenus = await iamApi.listCurrentUserMenus();
      } catch {
        // Menu lookup failure must not expand access; the empty menu set is fail-closed.
      }
      setUser(toAuthUser(currentUser));
      setMenus(currentMenus);
    } catch (err) {
      clearSession();
      setUser(null);
      setMenus([]);
      throw err;
    }
  };

  const login = async (username: string, password: string, totpCode = '') => {
    // Public login calls must not inherit a stale bearer token left in this tab.
    // The RPC identity resolver validates Authorization headers before dispatch,
    // even for public methods, so an expired token can otherwise block login.
    clearSession();
    setUser(null);
    setMenus([]);
    const grant = await iamApi.login(username, password, totpCode);
    await acceptGrant(grant);
  };

  const loginWithPasskey = async () => {
    clearSession();
    setUser(null);
    setMenus([]);
    const { ceremony } = await iamApi.beginPasskeyLogin();
    if (!ceremony) throw new Error('Management 未返回通行密钥登录挑战。');
    const response = await getPasskey(ceremony.options);
    const grant = await iamApi.finishPasskeyLogin(ceremony.challenge_id, response);
    await acceptGrant(grant);
  };

  const logout = async () => {
    try {
      if (sessionStorage.getItem(IAM_TOKEN_KEY)) await iamApi.logout();
    } finally {
      clearSession();
      setUser(null);
      setMenus([]);
    }
  };

  const refreshCurrentUser = async () => {
    const currentUser = await iamApi.getCurrentUser();
    setUser(toAuthUser(currentUser));
    try {
      setMenus(await iamApi.listCurrentUserMenus());
    } catch {
      setMenus([]);
    }
  };

  const hasPermission = (permissionCode: string) => !!user?.permissions?.includes(permissionCode);

  const canAccessRoute = (pathname: string): boolean => {
    const target = pathname.split('?')[0].replace(/\/$/, '') || '/';
    if (target === '/profile') return true;
    if ((target === '/users' || target.startsWith('/users/') || target === '/employee-users' || target === '/groups') && !hasPermission('iam.user.read')) return false;
    if (target === '/roles' || target === '/permissions' || target === '/api-mappings' || target === '/menus') {
      if (!hasPermission('iam.role.read')) return false;
    }
    const menuPath = target === '/api-mappings' ? '/permissions' : target;
    return menus.some((menu) => {
      if (!menu.path) return false;
      if (menu.menu_type === 'external') return menu.path === pathname;
      if (menu.menu_type !== 'page') return false;
      const path = menu.path.replace(/\/$/, '') || '/';
      return menuPath === path || (path !== '/' && menuPath.startsWith(`${path}/`));
    });
  };

  const value = useMemo<AuthContextType>(
    () => ({
      user,
      menus,
      isAuthenticated: !!user,
      isLoading,
      login,
      loginWithPasskey,
      logout,
      refreshCurrentUser,
      hasPermission,
      canAccessRoute,
    }),
    [user, menus, isLoading]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used within an AuthProvider');
  return context;
};

export const ProtectedRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { isAuthenticated, isLoading, user, canAccessRoute } = useAuth();
  const location = useLocation();

  if (isLoading) {
    return <div className="flex min-h-screen items-center justify-center text-sm text-slate-500">正在验证 Management 会话…</div>;
  }
  if (!isAuthenticated) return <Navigate to="/login" state={{ from: location }} replace />;
  if (user?.must_change_password && location.pathname !== '/profile') {
    return <Navigate to="/profile?tab=password" replace />;
  }
  if (!canAccessRoute(location.pathname)) return <Navigate to="/profile" replace />;
  return <>{children}</>;
};
