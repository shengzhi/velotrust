import { jrpcCall } from './jrpc';

export interface IAMUser {
  id: number;
  username: string;
  display_name: string;
  email?: string | null;
  user_status: 'active' | 'disabled' | 'locked' | 'pending';
  must_change_password: boolean;
  permissions?: string[];
  created_at: string;
  last_login_at?: string | null;
  totp_enabled?: boolean;
  passkeys?: IAMPasskey[];
}

export interface IAMPasskey {
  id: number;
  name: string;
  created_at: string;
  last_used_at?: string | null;
}

export interface IAMSessionGrant {
  access_token: string;
  token_type: string;
  expires_at: string;
  user: IAMUser;
}

export interface IAMRole {
  id: number;
  role_name: string;
  role_code: string;
  description?: string | null;
  is_system: boolean;
  status: string;
  created_at: string;
}

export interface IAMPermission {
  id: number;
  permission_name: string;
  permission_code: string;
  resource_id: number;
  resource_name?: string;
  resource_code?: string;
  resource_type?: string;
  action_code: string;
  description?: string | null;
  status: string;
}

export interface IAMResource {
  id: number;
  resource_name: string;
  resource_code: string;
  resource_type: string;
  description?: string | null;
  status: string;
}

export interface IAMMethodCatalogEntry {
  method_code: string;
  service_name: string;
  method_name: string;
  policy_type: 'public' | 'authenticated' | 'permission';
  permission_code?: string;
}

export interface IAMMenu {
  id: number;
  parent_id: number | null;
  menu_name: string;
  menu_code: string;
  path: string | null;
  component: string | null;
  icon: string | null;
  sort_order: number;
  menu_type: 'directory' | 'page' | 'external' | 'button';
  visible: boolean;
  status: 'active' | 'disabled';
  created_at: string;
  updated_at: string;
  version_no: number;
}

export type IAMMenuInput = Omit<IAMMenu, 'id' | 'created_at' | 'updated_at' | 'version_no'> & { version_no?: number };

export interface WebAuthnCeremony<TOptions> {
  challenge_id: string;
  options: TOptions;
}

export const iamApi = {
  login: (username: string, password: string, totpCode = '') =>
    jrpcCall<IAMSessionGrant>('IAM_Login', {
      username,
      password,
      ...(totpCode ? { totp_code: totpCode } : {}),
    }),

  getCurrentUser: (token?: string) =>
    jrpcCall<{ user: IAMUser }>('IAM_GetCurrentUser', {}, token).then((result) => result.user),

  logout: () => jrpcCall<{ success: boolean }>('IAM_Logout'),

  changePassword: (currentPassword: string, newPassword: string) =>
    jrpcCall<{ success: boolean }>('IAM_ChangePassword', {
      current_password: currentPassword,
      new_password: newPassword,
    }),

  setupTOTP: (password: string) =>
    jrpcCall<{ secret: string; otpauth_uri: string }>('IAM_SetupTOTP', { password }),

  confirmTOTP: (code: string) => jrpcCall<{ success: boolean }>('IAM_ConfirmTOTP', { code }),

  disableTOTP: (password: string, code: string) =>
    jrpcCall<{ success: boolean }>('IAM_DisableTOTP', { password, code }),

  beginPasskeyLogin: () =>
    jrpcCall<{ ceremony: WebAuthnCeremony<Record<string, unknown>> }>('IAM_BeginPasskeyLogin', {}),

  finishPasskeyLogin: (challengeId: string, response: object) =>
    jrpcCall<IAMSessionGrant>('IAM_FinishPasskeyLogin', {
      challenge_id: challengeId,
      response,
    }),

  beginPasskeyRegistration: () =>
    jrpcCall<{ ceremony: WebAuthnCeremony<Record<string, unknown>> }>('IAM_BeginPasskeyRegistration', {}),

  finishPasskeyRegistration: (challengeId: string, credentialName: string, response: object) =>
    jrpcCall<{ success: boolean }>('IAM_FinishPasskeyRegistration', {
      challenge_id: challengeId,
      credential_name: credentialName,
      response,
    }),

  revokePasskey: (credentialId: number) =>
    jrpcCall<{ success: boolean }>('IAM_RevokePasskey', { credential_id: credentialId }),

  listUsers: (page = 1, pageSize = 100) =>
    jrpcCall<{ items: IAMUser[]; total: number }>('IAM_ListUsers', { page, page_size: pageSize }),

  createUser: (input: { username: string; display_name: string; email: string; password: string }) =>
    jrpcCall<{ user: IAMUser }>('IAM_CreateUser', input).then((result) => result.user),

  setUserStatus: (userId: number, status: 'active' | 'disabled') =>
    jrpcCall<{ success: boolean }>('IAM_SetUserStatus', { user_id: userId, status }),

  resetUserPassword: (userId: number, newPassword: string) =>
    jrpcCall<{ success: boolean }>('IAM_ResetUserPassword', { user_id: userId, new_password: newPassword }),

  listRoles: () => jrpcCall<{ items: IAMRole[] }>('IAM_ListRoles', {}).then((result) => result.items ?? []),

  listMenus: () => jrpcCall<{ items: IAMMenu[] }>('IAM_ListMenus', {}).then((result) => result.items ?? []),

  listCurrentUserMenus: () =>
    jrpcCall<{ items: IAMMenu[] }>('IAM_ListCurrentUserMenus', {}).then((result) => result.items ?? []),

  listRoleMenus: (roleId: number) =>
    jrpcCall<{ items: IAMMenu[] }>('IAM_ListRoleMenus', { role_id: roleId }).then((result) => result.items ?? []),

  setRoleMenus: (roleId: number, menuIds: number[]) =>
    jrpcCall<{ success: boolean }>('IAM_SetRoleMenus', { role_id: roleId, menu_ids: menuIds }),

  createMenu: (input: IAMMenuInput) =>
    jrpcCall<{ menu: IAMMenu }>('IAM_CreateMenu', input).then((result) => result.menu),

  updateMenu: (input: IAMMenu & { version_no: number }) =>
    jrpcCall<{ menu: IAMMenu }>('IAM_UpdateMenu', input).then((result) => result.menu),

  deleteMenu: (menuId: number) => jrpcCall<{ success: boolean }>('IAM_DeleteMenu', { menu_id: menuId }),

  listUserRoles: (userId: number) =>
    jrpcCall<{ items: IAMRole[] }>('IAM_ListUserRoles', { user_id: userId }).then((result) => result.items ?? []),

  createRole: (input: { role_name: string; role_code: string; description: string }) =>
    jrpcCall<{ role: IAMRole }>('IAM_CreateRole', input).then((result) => result.role),

  listPermissionCatalog: () =>
    jrpcCall<{ items: IAMPermission[] }>('IAM_ListPermissionCatalog', {}).then((result) => result.items ?? []),

  listResourceCatalog: () =>
    jrpcCall<{ items: IAMResource[] }>('IAM_ListResourceCatalog', {}).then((result) => result.items ?? []),

  listMethodCatalog: () =>
    jrpcCall<{ items: IAMMethodCatalogEntry[] }>('IAM_ListMethodCatalog', {}).then((result) => result.items ?? []),

  listRolePermissions: (roleId: number) =>
    jrpcCall<{ items: IAMPermission[] }>('IAM_ListRolePermissions', { role_id: roleId }).then((result) => result.items ?? []),

  grantRole: (userId: number, roleId: number) =>
    jrpcCall<{ success: boolean }>('IAM_GrantRole', { user_id: userId, role_id: roleId }),

  revokeRole: (userId: number, roleId: number) =>
    jrpcCall<{ success: boolean }>('IAM_RevokeRole', { user_id: userId, role_id: roleId }),

  grantPermission: (roleId: number, permissionId: number) =>
    jrpcCall<{ success: boolean }>('IAM_GrantPermission', { role_id: roleId, permission_id: permissionId }),

  revokePermission: (roleId: number, permissionId: number) =>
    jrpcCall<{ success: boolean }>('IAM_RevokePermission', { role_id: roleId, permission_id: permissionId }),
};
