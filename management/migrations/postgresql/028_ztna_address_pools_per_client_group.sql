-- Allow an employee IAM group to use multiple dynamic User-Device pools.
BEGIN;

DROP INDEX IF EXISTS public.uq_t_ztna_address_pool_client_group;

COMMIT;
