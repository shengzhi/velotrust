-- Management ACL policy catalog and durable Gateway refresh fanout.
-- The policy tables already hold one current database state; this migration
-- adds the Management permissions and allows policy changes in the outbox.

ALTER TABLE public.t_ztna_access_change_event
    DROP CONSTRAINT ck_t_ztna_access_change_subject,
    ADD CONSTRAINT ck_t_ztna_access_change_subject
        CHECK (subject_type IN ('user', 'device', 'node', 'policy'));

INSERT INTO public.t_iam_resource AS existing
    (resource_name, resource_code, resource_type, description, status, created_by, updated_by)
VALUES
    ('访问策略与 ACL', 'ztna.policy', 'management', '配置访问规则并下发至 Gateway 防火墙', 'active', 'migration', 'migration')
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
FROM public.t_iam_resource AS resource
CROSS JOIN (VALUES
    ('查看访问策略', 'ztna.policy.read', 'read', '查看访问策略、ACL 规则和 Gateway 选项'),
    ('管理访问策略', 'ztna.policy.write', 'write', '创建、修改、启停和删除访问策略并触发 Gateway ACL 同步')
) AS seed(permission_name, permission_code, action_code, description)
WHERE resource.resource_code='ztna.policy'
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

INSERT INTO public.t_iam_rpc_method AS existing
    (method_code, protocol, service_name, method_name, permission_id, description, status, created_by, updated_by)
SELECT method.method_code, 'jrpc', 'POLICY', method.method_name, permission.id,
       method.description, 'active', 'migration', 'migration'
FROM (VALUES
    ('POLICY_ListPolicies', 'ListPolicies', '查看当前访问策略与 ACL 规则', 'ztna.policy.read'),
    ('POLICY_ListGateways', 'ListGateways', '选择策略作用的 Gateway', 'ztna.policy.read'),
    ('POLICY_SavePolicy', 'SavePolicy', '创建或更新访问策略', 'ztna.policy.write'),
    ('POLICY_SetPolicyStatus', 'SetPolicyStatus', '启用或停用访问策略', 'ztna.policy.write'),
    ('POLICY_DeletePolicy', 'DeletePolicy', '删除访问策略', 'ztna.policy.write')
) AS method(method_code, method_name, description, permission_code)
JOIN public.t_iam_permission AS permission ON permission.permission_code=method.permission_code
ON CONFLICT (method_code) DO UPDATE SET
    protocol=EXCLUDED.protocol,
    service_name=EXCLUDED.service_name,
    method_name=EXCLUDED.method_name,
    permission_id=EXCLUDED.permission_id,
    description=EXCLUDED.description,
    status='active',
    updated_at=CURRENT_TIMESTAMP,
    updated_by='migration',
    version_no=existing.version_no+1
WHERE (existing.protocol, existing.service_name, existing.method_name, existing.permission_id, existing.description, existing.status)
    IS DISTINCT FROM (EXCLUDED.protocol, EXCLUDED.service_name, EXCLUDED.method_name, EXCLUDED.permission_id, EXCLUDED.description, 'active');

INSERT INTO public.t_iam_role_permission (role_id, permission_id, created_at, created_by)
SELECT role.id, permission.id, CURRENT_TIMESTAMP, 'migration'
FROM public.t_iam_role AS role
JOIN public.t_iam_permission AS permission ON permission.permission_code IN ('ztna.policy.read', 'ztna.policy.write')
WHERE role.role_code='platform_admin' AND role.is_system=TRUE AND role.status='active'
ON CONFLICT (role_id, permission_id) DO NOTHING;
