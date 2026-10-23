-- Store the employee-group to dynamic Client address-pool association directly
-- on the pool. Gateway access remains an access-policy decision.
BEGIN;

ALTER TABLE public.t_ztna_address_pool
    ADD COLUMN IF NOT EXISTS client_group_id BIGINT;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM public.t_ztna_client_group_assignment assignment
        JOIN public.t_ztna_address_pool pool ON pool.id=assignment.pool_id
        WHERE assignment.status='active'
          AND (pool.pool_type <> 'User-Device' OR pool.allocation_mode <> 'dynamic')
    ) THEN
        RAISE EXCEPTION 'cannot migrate active Client group assignments: every target pool must be a dynamic User-Device pool';
    END IF;

    IF EXISTS (
        SELECT assignment.pool_id
        FROM public.t_ztna_client_group_assignment assignment
        WHERE assignment.status='active'
        GROUP BY assignment.pool_id
        HAVING COUNT(DISTINCT assignment.group_id) > 1
    ) THEN
        RAISE EXCEPTION 'cannot migrate active Client group assignments: one address pool is mapped to multiple employee groups';
    END IF;
END $$;

UPDATE public.t_ztna_address_pool pool
SET client_group_id=assignment.group_id
FROM public.t_ztna_client_group_assignment assignment
WHERE assignment.pool_id=pool.id
  AND assignment.status='active'
  AND pool.client_group_id IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname='fk_t_ztna_address_pool_client_group'
          AND conrelid='public.t_ztna_address_pool'::regclass
    ) THEN
        ALTER TABLE public.t_ztna_address_pool
            ADD CONSTRAINT fk_t_ztna_address_pool_client_group
            FOREIGN KEY (client_group_id) REFERENCES public.t_iam_group (id) ON DELETE RESTRICT;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname='ck_t_ztna_address_pool_client_group'
          AND conrelid='public.t_ztna_address_pool'::regclass
    ) THEN
        ALTER TABLE public.t_ztna_address_pool
            ADD CONSTRAINT ck_t_ztna_address_pool_client_group
            CHECK (client_group_id IS NULL OR (pool_type='User-Device' AND allocation_mode='dynamic'));
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_t_ztna_address_pool_client_group
    ON public.t_ztna_address_pool (client_group_id) WHERE client_group_id IS NOT NULL;

DROP TABLE public.t_ztna_client_group_assignment;

COMMIT;
