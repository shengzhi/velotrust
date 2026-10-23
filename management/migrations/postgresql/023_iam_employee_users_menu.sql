-- Add a dedicated employee-user page under the Zero Trust Security Policy menu.
-- Keep this additive so installations that already applied the menu catalog
-- receive the new entry without rewriting the historical seed migration.
BEGIN;

INSERT INTO public.t_iam_menu
    (parent_id, menu_name, menu_code, path, component, icon, sort_order, menu_type, visible, status, created_by, updated_by)
SELECT parent.id, '员工用户', 'menu:employee-users', '/employee-users',
       'pages/Users/EmployeeList.tsx', 'UsersRound', 44, 'page', TRUE, 'active', 'migration', 'migration'
FROM public.t_iam_menu parent
WHERE parent.menu_code = 'menu:policy:dir'
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
    updated_by = 'migration',
    version_no = public.t_iam_menu.version_no + 1
WHERE (public.t_iam_menu.parent_id, public.t_iam_menu.menu_name, public.t_iam_menu.path,
       public.t_iam_menu.component, public.t_iam_menu.icon, public.t_iam_menu.sort_order,
       public.t_iam_menu.menu_type, public.t_iam_menu.visible, public.t_iam_menu.status)
  IS DISTINCT FROM
      (EXCLUDED.parent_id, EXCLUDED.menu_name, EXCLUDED.path, EXCLUDED.component,
       EXCLUDED.icon, EXCLUDED.sort_order, EXCLUDED.menu_type, EXCLUDED.visible, EXCLUDED.status);

INSERT INTO public.t_iam_role_menu (role_id, menu_id, created_at, created_by)
SELECT role.id, menu.id, CURRENT_TIMESTAMP, 'migration'
FROM public.t_iam_role role
CROSS JOIN public.t_iam_menu menu
WHERE role.role_code = 'platform_admin'
  AND role.status = 'active'
  AND menu.menu_code = 'menu:employee-users'
  AND menu.status = 'active'
ON CONFLICT (role_id, menu_id) DO NOTHING;

COMMIT;
