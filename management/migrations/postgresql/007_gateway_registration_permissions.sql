-- Register least-privilege IAM permissions for gateway enrollment management.
INSERT INTO public.t_iam_resource
    (resource_name, resource_code, resource_type, description, status, created_by, updated_by)
VALUES
    ('网关注册凭证', 'gateway.registration', 'management', '管理逻辑网关和一次性注册凭证', 'active', 'migration', 'migration')
ON CONFLICT (resource_code) DO UPDATE SET
    resource_name = EXCLUDED.resource_name,
    resource_type = EXCLUDED.resource_type,
    description = EXCLUDED.description,
    status = 'active',
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'migration',
    version_no = public.t_iam_resource.version_no + 1;

INSERT INTO public.t_iam_permission
    (permission_name, permission_code, resource_id, action_code, description, status, created_by, updated_by)
SELECT seed.permission_name, seed.permission_code, resource.id, seed.action_code, seed.description,
       'active', 'migration', 'migration'
FROM public.t_iam_resource resource
CROSS JOIN (VALUES
    ('查看网关注册凭证', 'gateway.registration.read', 'read', '查看网关和注册凭证元数据'),
    ('管理网关注册凭证', 'gateway.registration.write', 'write', '预建网关、签发、延期和撤销注册凭证')
) AS seed(permission_name, permission_code, action_code, description)
WHERE resource.resource_code = 'gateway.registration'
ON CONFLICT (permission_code) DO UPDATE SET
    permission_name = EXCLUDED.permission_name,
    resource_id = EXCLUDED.resource_id,
    action_code = EXCLUDED.action_code,
    description = EXCLUDED.description,
    status = 'active',
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'migration',
    version_no = public.t_iam_permission.version_no + 1;

INSERT INTO public.t_iam_role_permission (role_id, permission_id, created_by)
SELECT role.id, permission.id, 'migration'
FROM public.t_iam_role role
CROSS JOIN public.t_iam_permission permission
WHERE role.role_code = 'platform_admin'
  AND role.status = 'active'
  AND permission.permission_code IN ('gateway.registration.read', 'gateway.registration.write')
ON CONFLICT (role_id, permission_id) DO NOTHING;
