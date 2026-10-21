-- The current Gateway dataplane state is compiled by HUB from authoritative
-- ZTNA, tunnel, and IPAM records. Keep only delivery tasks and runtime reports.
-- The migration runner owns the transaction.

DELETE FROM public.t_iam_role_permission AS role_permission
USING public.t_iam_permission AS permission
WHERE role_permission.permission_id = permission.id
  AND permission.permission_code IN ('ztna.config.read', 'ztna.config.write');

DELETE FROM public.t_iam_rpc_method AS rpc_method
USING public.t_iam_permission AS permission
WHERE rpc_method.permission_id = permission.id
  AND permission.permission_code IN ('ztna.config.read', 'ztna.config.write');

DELETE FROM public.t_iam_permission
WHERE permission_code IN ('ztna.config.read', 'ztna.config.write');

DELETE FROM public.t_iam_resource
WHERE resource_code = 'ztna.config';

DROP TABLE IF EXISTS public.t_ztna_config;
