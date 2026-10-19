-- Normalize user-facing site labels while keeping resource and menu codes
-- stable for API compatibility.

UPDATE public.t_iam_menu
SET menu_name = '网络站点',
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'migration',
    version_no = version_no + 1
WHERE menu_code = 'menu:sites'
  AND menu_name IS DISTINCT FROM '网络站点';

UPDATE public.t_iam_resource
SET resource_name = '网络站点',
    description = '管理网络站点、云网络及其归属网关',
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'migration',
    version_no = version_no + 1
WHERE resource_code = 'ztna.site'
  AND (resource_name, description) IS DISTINCT FROM
      ('网络站点'::VARCHAR, '管理网络站点、云网络及其归属网关'::VARCHAR);

UPDATE public.t_iam_permission AS permission
SET permission_name = labels.permission_name,
    description = labels.description,
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'migration',
    version_no = permission.version_no + 1
FROM (VALUES
    ('ztna.site.read', '查看网络站点', '查看网络站点与其网络信息'),
    ('ztna.site.write', '管理网络站点', '新增、编辑、停用和删除网络站点')
) AS labels(permission_code, permission_name, description)
WHERE permission.permission_code = labels.permission_code
  AND (permission.permission_name, permission.description) IS DISTINCT FROM
      (labels.permission_name, labels.description);
