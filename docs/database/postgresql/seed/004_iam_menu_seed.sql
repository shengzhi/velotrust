-- Seed the console navigation from the prototype catalog.
-- Button rows are metadata only; API authorization remains independent.
BEGIN;

INSERT INTO public.t_iam_menu
    (parent_id, menu_name, menu_code, path, component, icon, sort_order, menu_type, visible, status, created_by, updated_by)
VALUES
    (NULL, '概览与监控', 'menu:overview:dir', '/overview', NULL, 'LayoutDashboard', 1, 'directory', TRUE, 'active', 'seed', 'seed'),
    (NULL, '业务资源与服务', 'menu:resource:dir', '/resources-center', NULL, 'ShieldCheck', 2, 'directory', TRUE, 'active', 'seed', 'seed'),
    (NULL, '网络与路由', 'menu:network:dir', '/network', NULL, 'Layers', 3, 'directory', TRUE, 'active', 'seed', 'seed'),
    (NULL, '零信任安全策略', 'menu:policy:dir', '/policies-dir', NULL, 'Shield', 4, 'directory', TRUE, 'active', 'seed', 'seed'),
    (NULL, '身份与权限体系', 'menu:identity:dir', '/identity', NULL, 'KeyRound', 5, 'directory', TRUE, 'active', 'seed', 'seed'),
    (NULL, '合规与审计', 'menu:audit:dir', '/audit-dir', NULL, 'FileText', 6, 'directory', TRUE, 'active', 'seed', 'seed'),
    (NULL, '系统与容灾', 'menu:system:dir', '/system', NULL, 'Settings2', 7, 'directory', TRUE, 'active', 'seed', 'seed')
ON CONFLICT (menu_code) DO UPDATE SET
    parent_id = EXCLUDED.parent_id,
    menu_name = EXCLUDED.menu_name,
    path = EXCLUDED.path,
    component = EXCLUDED.component,
    icon = EXCLUDED.icon,
    sort_order = EXCLUDED.sort_order,
    menu_type = EXCLUDED.menu_type,
    visible = EXCLUDED.visible,
    status = EXCLUDED.status,
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'seed',
    version_no = public.t_iam_menu.version_no + 1
WHERE (public.t_iam_menu.parent_id, public.t_iam_menu.menu_name, public.t_iam_menu.path,
       public.t_iam_menu.component, public.t_iam_menu.icon, public.t_iam_menu.sort_order,
       public.t_iam_menu.menu_type, public.t_iam_menu.visible, public.t_iam_menu.status)
  IS DISTINCT FROM
      (EXCLUDED.parent_id, EXCLUDED.menu_name, EXCLUDED.path, EXCLUDED.component,
       EXCLUDED.icon, EXCLUDED.sort_order, EXCLUDED.menu_type, EXCLUDED.visible, EXCLUDED.status);

INSERT INTO public.t_iam_menu
    (parent_id, menu_name, menu_code, path, component, icon, sort_order, menu_type, visible, status, created_by, updated_by)
SELECT parent.id, seed.menu_name, seed.menu_code, seed.path, seed.component, seed.icon,
       seed.sort_order, seed.menu_type, TRUE, 'active', 'seed', 'seed'
FROM (VALUES
    ('menu:overview:dir', '仪表盘', 'menu:dashboard', '/', 'pages/Dashboard.tsx', 'LayoutDashboard', 11, 'page'),
    ('menu:overview:dir', '全景拓扑图', 'menu:topology', '/topology', 'pages/Topology.tsx', 'Network', 12, 'page'),
    ('menu:resource:dir', '资源管理中心', 'menu:resources', '/resources', 'pages/Resources/index.tsx', 'ShieldCheck', 21, 'page'),
    ('menu:network:dir', '虚拟网段 (IPAM)', 'menu:ipam', '/ipam', 'pages/Ipam.tsx', 'Layers', 31, 'page'),
    ('menu:network:dir', '虚拟节点', 'menu:nodes', '/nodes', 'pages/Nodes/NodeList.tsx', 'Server', 32, 'page'),
    ('menu:network:dir', '物理站点', 'menu:sites', '/sites', 'pages/Sites.tsx', 'Globe', 33, 'page'),
    ('menu:network:dir', '内网 DNS 管理', 'menu:dns', '/dns', 'pages/Dns.tsx', 'Compass', 34, 'page'),
    ('menu:network:dir', '注册凭证', 'menu:enrollment-tokens', '/enrollment-tokens', 'pages/EnrollmentTokens.tsx', 'Key', 35, 'page'),
    ('menu:policy:dir', '访问策略与 ACL', 'menu:policies', '/policies', 'pages/Policies.tsx', 'Shield', 41, 'page'),
    ('menu:policy:dir', '权限访问透视', 'menu:access-matrix', '/access-matrix', 'pages/AccessMatrix.tsx', 'Compass', 42, 'page'),
    ('menu:policy:dir', '终端设备', 'menu:devices', '/devices', 'pages/Devices.tsx', 'Monitor', 43, 'page'),
    ('menu:identity:dir', '系统用户', 'menu:users', '/users', 'pages/Users/UserList.tsx', 'Users', 51, 'page'),
    ('menu:identity:dir', '安全用户组', 'menu:groups', '/groups', 'pages/Users/GroupList.tsx', 'UsersRound', 52, 'page'),
    ('menu:identity:dir', '角色管理', 'menu:roles', '/roles', 'pages/Roles/index.tsx', 'KeyRound', 53, 'page'),
    ('menu:identity:dir', '菜单管理', 'menu:menus', '/menus', 'pages/Menus/index.tsx', 'SlidersHorizontal', 54, 'page'),
    ('menu:identity:dir', '权限管理', 'menu:permissions', '/permissions', 'pages/Permissions/index.tsx', 'ShieldCheck', 55, 'page'),
    ('menu:identity:dir', '身份源集成 (IdP)', 'menu:idp', '/idp', 'pages/Idp.tsx', 'Fingerprint', 56, 'page'),
    ('menu:audit:dir', '安全审计日志', 'menu:audit', '/audit', 'pages/Audit.tsx', 'FileText', 61, 'page'),
    ('menu:audit:dir', '操作审计日志', 'menu:operation-logs', '/operation-logs', 'pages/OperationLogs/index.tsx', 'FileSpreadsheet', 62, 'page'),
    ('menu:audit:dir', '登录审计日志', 'menu:login-logs', '/login-logs', 'pages/LoginLogs/index.tsx', 'History', 63, 'page'),
    ('menu:system:dir', '系统与灾备设置', 'menu:settings', '/settings', 'pages/Settings.tsx', 'Settings2', 71, 'page')
) AS seed(parent_code, menu_name, menu_code, path, component, icon, sort_order, menu_type)
JOIN public.t_iam_menu parent ON parent.menu_code = seed.parent_code
ON CONFLICT (menu_code) DO UPDATE SET
    parent_id = EXCLUDED.parent_id,
    menu_name = EXCLUDED.menu_name,
    path = EXCLUDED.path,
    component = EXCLUDED.component,
    icon = EXCLUDED.icon,
    sort_order = EXCLUDED.sort_order,
    menu_type = EXCLUDED.menu_type,
    visible = EXCLUDED.visible,
    status = EXCLUDED.status,
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'seed',
    version_no = public.t_iam_menu.version_no + 1
WHERE (public.t_iam_menu.parent_id, public.t_iam_menu.menu_name, public.t_iam_menu.path,
       public.t_iam_menu.component, public.t_iam_menu.icon, public.t_iam_menu.sort_order,
       public.t_iam_menu.menu_type, public.t_iam_menu.visible, public.t_iam_menu.status)
  IS DISTINCT FROM
      (EXCLUDED.parent_id, EXCLUDED.menu_name, EXCLUDED.path, EXCLUDED.component,
       EXCLUDED.icon, EXCLUDED.sort_order, EXCLUDED.menu_type, EXCLUDED.visible, EXCLUDED.status);

INSERT INTO public.t_iam_menu
    (parent_id, menu_name, menu_code, path, component, icon, sort_order, menu_type, visible, status, created_by, updated_by)
SELECT parent.id, seed.menu_name, seed.menu_code, NULL, NULL, seed.icon,
       seed.sort_order, 'button', FALSE, 'active', 'seed', 'seed'
FROM (VALUES
    ('menu:resources', '健康连通探测', 'action:resource:probe', 'Activity', 211),
    ('menu:resources', '注册业务资产', 'action:resource:create', 'Plus', 212),
    ('menu:operation-logs', '导出操作日志', 'action:operation:export', 'Download', 621)
) AS seed(parent_code, menu_name, menu_code, icon, sort_order)
JOIN public.t_iam_menu parent ON parent.menu_code = seed.parent_code
ON CONFLICT (menu_code) DO UPDATE SET
    parent_id = EXCLUDED.parent_id,
    menu_name = EXCLUDED.menu_name,
    path = EXCLUDED.path,
    component = EXCLUDED.component,
    icon = EXCLUDED.icon,
    sort_order = EXCLUDED.sort_order,
    menu_type = EXCLUDED.menu_type,
    visible = EXCLUDED.visible,
    status = EXCLUDED.status,
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'seed',
    version_no = public.t_iam_menu.version_no + 1
WHERE (public.t_iam_menu.parent_id, public.t_iam_menu.menu_name, public.t_iam_menu.path,
       public.t_iam_menu.component, public.t_iam_menu.icon, public.t_iam_menu.sort_order,
       public.t_iam_menu.menu_type, public.t_iam_menu.visible, public.t_iam_menu.status)
  IS DISTINCT FROM
      (EXCLUDED.parent_id, EXCLUDED.menu_name, EXCLUDED.path, EXCLUDED.component,
       EXCLUDED.icon, EXCLUDED.sort_order, EXCLUDED.menu_type, EXCLUDED.visible, EXCLUDED.status);

COMMIT;
