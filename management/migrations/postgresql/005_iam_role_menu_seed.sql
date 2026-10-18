-- The platform administrator role starts with every active menu. Other roles
-- receive only the menus explicitly assigned by an administrator.
BEGIN;

INSERT INTO public.t_iam_role_menu (role_id, menu_id, created_at, created_by)
SELECT role.id, menu.id, CURRENT_TIMESTAMP, 'seed'
FROM public.t_iam_role role
CROSS JOIN public.t_iam_menu menu
WHERE role.status = 'active'
  AND role.role_code = 'platform_admin'
  AND menu.status = 'active'
ON CONFLICT (role_id, menu_id) DO NOTHING;

COMMIT;
