-- A physical site is a routed-network record whose network_type is 'site'.
-- Keep its management attributes on the same row to avoid a one-to-one parent
-- table and duplicate site/network identifiers.

ALTER TABLE public.t_ztna_network
    ADD COLUMN gateway_row_id BIGINT,
    ADD COLUMN is_del BOOLEAN NOT NULL DEFAULT FALSE,
    ADD CONSTRAINT fk_t_ztna_network_gateway
        FOREIGN KEY (gateway_row_id) REFERENCES public.t_gateway_node (id) ON DELETE RESTRICT;

CREATE INDEX ix_t_ztna_network_sites
    ON public.t_ztna_network (network_name, id)
    WHERE network_type='site' AND is_del=FALSE;

INSERT INTO public.t_iam_resource
    (resource_name, resource_code, resource_type, description, status, created_by, updated_by)
VALUES
    ('物理站点', 'ztna.site', 'management', '管理物理站点、云网络及其归属网关', 'active', 'migration', 'migration')
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
    ('查看物理站点', 'ztna.site.read', 'read', '查看物理站点与其网络信息'),
    ('管理物理站点', 'ztna.site.write', 'write', '新增、编辑、停用和删除物理站点')
) AS seed(permission_name, permission_code, action_code, description)
WHERE resource.resource_code = 'ztna.site'
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
  AND permission.permission_code IN ('ztna.site.read', 'ztna.site.write')
ON CONFLICT (role_id, permission_id) DO NOTHING;
