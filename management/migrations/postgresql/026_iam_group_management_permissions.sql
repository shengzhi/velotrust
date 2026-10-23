-- Give platform administrators explicit permissions to manage employee groups
-- and the group memberships used by Client automatic tunnel assignment.
BEGIN;

INSERT INTO public.t_iam_resource AS existing
    (resource_name, resource_code, resource_type, description, status, created_by, updated_by)
VALUES
    ('安全用户组', 'iam.group', 'management', '管理员工用户组及成员关系', 'active', 'migration', 'migration')
ON CONFLICT (resource_code) DO UPDATE SET
    resource_name=EXCLUDED.resource_name,
    resource_type=EXCLUDED.resource_type,
    description=EXCLUDED.description,
    updated_at=CURRENT_TIMESTAMP,
    updated_by='migration',
    version_no=existing.version_no+1
WHERE (existing.resource_name, existing.resource_type, existing.description)
    IS DISTINCT FROM (EXCLUDED.resource_name, EXCLUDED.resource_type, EXCLUDED.description);

INSERT INTO public.t_iam_permission AS existing
    (permission_name, permission_code, resource_id, action_code, description, status, created_by, updated_by)
SELECT seed.permission_name, seed.permission_code, resource.id, seed.action_code,
       seed.description, 'active', 'migration', 'migration'
FROM public.t_iam_resource resource
CROSS JOIN (VALUES
    ('查看安全用户组', 'iam.group.read', 'read', '查看员工用户组与成员', 'iam.group'),
    ('管理安全用户组', 'iam.group.write', 'write', '创建用户组并维护员工成员关系', 'iam.group')
) AS seed(permission_name, permission_code, action_code, description, resource_code)
WHERE resource.resource_code=seed.resource_code
ON CONFLICT (permission_code) DO UPDATE SET
    permission_name=EXCLUDED.permission_name,
    resource_id=EXCLUDED.resource_id,
    action_code=EXCLUDED.action_code,
    description=EXCLUDED.description,
    updated_at=CURRENT_TIMESTAMP,
    updated_by='migration',
    version_no=existing.version_no+1
WHERE (existing.permission_name, existing.resource_id, existing.action_code, existing.description)
    IS DISTINCT FROM (EXCLUDED.permission_name, EXCLUDED.resource_id, EXCLUDED.action_code, EXCLUDED.description);

INSERT INTO public.t_iam_role_permission (role_id, permission_id, created_at, created_by)
SELECT role.id, permission.id, CURRENT_TIMESTAMP, 'migration'
FROM public.t_iam_role role
JOIN public.t_iam_permission permission ON permission.permission_code IN ('iam.group.read', 'iam.group.write')
WHERE role.role_code='platform_admin' AND role.status='active'
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;
