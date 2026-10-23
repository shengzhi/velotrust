-- Connect enterprise resources, security groups, policies, and Gateway ACLs.
-- Legacy active policies authored with direct source/destination CIDRs cannot
-- be safely interpreted as resource authorization rules; publish refresh events
-- before returning them to draft for an administrator to rebuild.

BEGIN;

INSERT INTO public.t_ztna_access_change_event
	(subject_type, subject_id, change_type, status, targets_initialized, created_by, updated_by)
SELECT 'policy', id, 'authorization_changed', 'pending', FALSE, 'migration', 'migration'
FROM public.t_ztna_access_policy
WHERE status='active';

SELECT pg_notify('velotrust_access_change', 'policy-resource-migration');

UPDATE public.t_ztna_access_policy
SET status='draft', updated_at=CURRENT_TIMESTAMP, updated_by='migration', version_no=version_no+1
WHERE status='active';

ALTER TABLE public.t_ztna_access_change_event
	DROP CONSTRAINT ck_t_ztna_access_change_subject,
	ADD CONSTRAINT ck_t_ztna_access_change_subject
		CHECK (subject_type IN ('user', 'device', 'node', 'policy'));

INSERT INTO public.t_iam_resource AS existing
	(resource_name, resource_code, resource_type, description, status, created_by, updated_by)
VALUES
	('企业资源管理中心', 'ztna.resource', 'management', '维护可授权给安全用户组的受保护网络服务资源', 'active', 'migration', 'migration')
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
	('查看企业资源', 'ztna.resource.read', 'read', '查看策略可选择的受保护网络资源'),
	('管理企业资源', 'ztna.resource.write', 'write', '创建、修改和删除受保护网络资源')
) AS seed(permission_name, permission_code, action_code, description)
WHERE resource.resource_code='ztna.resource'
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
SELECT method.method_code, 'jrpc', method.service_name, method.method_name, permission.id,
	method.description, 'active', 'migration', 'migration'
FROM (VALUES
	('POLICY_ListGroups', 'POLICY', 'ListGroups', '读取访问策略可选择的安全用户组', 'ztna.policy.read'),
	('POLICY_ListResources', 'POLICY', 'ListResources', '读取访问策略可选择的企业资源', 'ztna.policy.read'),
	('RESOURCE_ListResources', 'RESOURCE', 'ListResources', '查询企业受保护资源', 'ztna.resource.read'),
	('RESOURCE_SaveResource', 'RESOURCE', 'SaveResource', '新增或更新企业受保护资源', 'ztna.resource.write'),
	('RESOURCE_DeleteResource', 'RESOURCE', 'DeleteResource', '删除未被策略引用的企业受保护资源', 'ztna.resource.write')
) AS method(method_code, service_name, method_name, description, permission_code)
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
JOIN public.t_iam_permission AS permission
	ON permission.permission_code IN ('ztna.resource.read', 'ztna.resource.write')
WHERE role.role_code='platform_admin' AND role.is_system=TRUE AND role.status='active'
ON CONFLICT (role_id, permission_id) DO NOTHING;

COMMIT;
