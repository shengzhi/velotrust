-- Initialize VeloTrust system resources and their permission catalog.
-- This seed is safe to rerun: it repairs catalog metadata without incrementing
-- versions when unchanged, preserves deliberate disabled status, and adds only
-- missing platform-admin grants. Every active permission in the catalog is
-- granted to platform_admin, including permissions added by later modules.
BEGIN;

INSERT INTO public.t_iam_resource AS existing
    (resource_name, resource_code, resource_type, description, status, created_by, updated_by)
VALUES
    ('用户管理', 'iam.user', 'management', '管理 VeloTrust 管理员及用户账号', 'active', 'seed', 'seed'),
    ('角色与权限', 'iam.role', 'management', '管理角色、权限与菜单授权', 'active', 'seed', 'seed'),
    ('审计日志', 'iam.audit_log', 'management', '查询 IAM 管理审计日志', 'active', 'seed', 'seed'),
    ('网关注册凭证', 'gateway.registration', 'management', '管理逻辑网关和一次性注册凭证', 'active', 'seed', 'seed'),
    ('IPAM 网络目录', 'ipam.network', 'management', '管理 Overlay 地址池和地址租约', 'active', 'seed', 'seed'),
    ('网络站点', 'ztna.site', 'management', '管理网络站点、云网络及其归属网关', 'active', 'seed', 'seed'),
    ('虚拟节点', 'ztna.node', 'management', '查询运行节点并管理节点生命周期状态', 'active', 'seed', 'seed')
ON CONFLICT (resource_code) DO UPDATE SET
    resource_name = EXCLUDED.resource_name,
    resource_type = EXCLUDED.resource_type,
    description = EXCLUDED.description,
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'seed',
    version_no = existing.version_no + 1
WHERE (existing.resource_name, existing.resource_type, existing.description)
      IS DISTINCT FROM (EXCLUDED.resource_name, EXCLUDED.resource_type, EXCLUDED.description);

-- Permissions are keyed by permission_code; disabled status is retained on
-- existing records.
INSERT INTO public.t_iam_permission AS existing
    (permission_name, permission_code, resource_id, action_code, description, status, created_by, updated_by)
SELECT seed.permission_name, seed.permission_code, resource.id, seed.action_code,
       seed.description, 'active', 'seed', 'seed'
FROM public.t_iam_resource AS resource
CROSS JOIN (VALUES
    ('查看用户', 'iam.user.read', 'read', '查看用户列表、详情及角色关联', 'iam.user'),
    ('管理用户', 'iam.user.write', 'write', '创建、停用用户并重置用户密码', 'iam.user'),
    ('查看角色与权限', 'iam.role.read', 'read', '查看角色、权限目录和菜单目录', 'iam.role'),
    ('管理角色与权限', 'iam.role.write', 'write', '创建角色并管理角色权限和菜单', 'iam.role'),
    ('授予角色', 'iam.role.grant', 'grant', '向用户授予或撤销角色', 'iam.role'),
    ('查看审计日志', 'iam.audit_log.read', 'read', '查看 IAM 管理操作审计日志', 'iam.audit_log'),
    ('查看网关注册凭证', 'gateway.registration.read', 'read', '查看网关和注册凭证元数据', 'gateway.registration'),
    ('管理网关注册凭证', 'gateway.registration.write', 'write', '预建网关、签发、延期和撤销注册凭证', 'gateway.registration'),
    ('查看 IPAM 网络', 'ipam.network.read', 'read', '查看企业网络、Overlay 地址池、租约和 NAT 台账', 'ipam.network'),
    ('管理 IPAM 网络', 'ipam.network.write', 'write', '管理 Overlay 地址池和地址租约', 'ipam.network'),
    ('查看网络站点', 'ztna.site.read', 'read', '查看网络站点与其网络信息', 'ztna.site'),
    ('管理网络站点', 'ztna.site.write', 'write', '创建、编辑、停用和删除网络站点', 'ztna.site'),
    ('查看虚拟节点', 'ztna.node.read', 'read', '查看虚拟节点身份、端点、网络关联和运行状态', 'ztna.node'),
    ('管理虚拟节点', 'ztna.node.write', 'write', '启用、禁用、隔离或撤销虚拟节点', 'ztna.node')
) AS seed(permission_name, permission_code, action_code, description, resource_code)
WHERE resource.resource_code = seed.resource_code
ON CONFLICT (permission_code) DO UPDATE SET
    permission_name = EXCLUDED.permission_name,
    resource_id = EXCLUDED.resource_id,
    action_code = EXCLUDED.action_code,
    description = EXCLUDED.description,
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'seed',
    version_no = existing.version_no + 1
WHERE (existing.permission_name, existing.resource_id, existing.action_code, existing.description)
      IS DISTINCT FROM (EXCLUDED.permission_name, EXCLUDED.resource_id, EXCLUDED.action_code, EXCLUDED.description);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM public.t_iam_role
        WHERE role_code = 'platform_admin'
          AND is_system = TRUE
          AND status = 'active'
    ) THEN
        RAISE EXCEPTION 'active system role platform_admin is missing; run Management bootstrap first';
    END IF;
END;
$$;

INSERT INTO public.t_iam_role_permission (role_id, permission_id, created_at, created_by)
SELECT role.id, permission.id, CURRENT_TIMESTAMP, 'seed'
FROM public.t_iam_role AS role
JOIN public.t_iam_permission AS permission ON permission.status = 'active'
JOIN public.t_iam_resource AS resource ON resource.id = permission.resource_id AND resource.status = 'active'
WHERE role.role_code = 'platform_admin'
  AND role.is_system = TRUE
  AND role.status = 'active'
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;
