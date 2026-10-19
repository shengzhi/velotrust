-- Consolidate the original gateway inventory into the canonical ZTNA node
-- catalog and normalize model audit actors to IAM user names.

ALTER TABLE public.t_ztna_node
    ADD COLUMN gateway_type VARCHAR(64),
    ADD COLUMN gateway_network_name VARCHAR(128),
    ADD COLUMN require_approval BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN registered_at TIMESTAMPTZ;

ALTER TABLE public.t_ztna_node
    ADD CONSTRAINT ck_t_ztna_node_gateway_type_scope
        CHECK (gateway_type IS NULL OR node_type = 'gateway'),
    ADD CONSTRAINT ck_t_ztna_node_gateway_network_scope
        CHECK (gateway_network_name IS NULL OR node_type = 'gateway'),
    ADD CONSTRAINT ck_t_ztna_node_registered_at_scope
        CHECK (registered_at IS NULL OR node_type = 'gateway');

ALTER TABLE public.t_ztna_node_network
    ADD COLUMN allowed_cidr CIDR;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM public.t_gateway_node AS old_gateway
        JOIN public.t_ztna_node AS existing_node
          ON existing_node.node_code = old_gateway.gateway_id
         AND existing_node.node_type <> 'gateway'
    ) THEN
        RAISE EXCEPTION 'gateway migration found a node_code collision with a non-gateway t_ztna_node row';
    END IF;
END;
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM public.t_gateway_node AS old_gateway
        WHERE EXISTS (
            SELECT 1
            FROM jsonb_array_elements_text(old_gateway.allowed_subnets) AS subnet(value)
            WHERE NOT EXISTS (
                SELECT 1
                FROM public.t_gateway_node_network AS old_link
                WHERE old_link.gateway_row_id = old_gateway.id
                  AND old_link.allowed_cidr = subnet.value::CIDR
            )
        ) OR EXISTS (
            SELECT 1
            FROM public.t_gateway_node_network AS old_link
            WHERE old_link.gateway_row_id = old_gateway.id
              AND NOT EXISTS (
                  SELECT 1
                  FROM jsonb_array_elements_text(old_gateway.allowed_subnets) AS subnet(value)
                  WHERE subnet.value::CIDR = old_link.allowed_cidr
              )
        )
    ) THEN
        RAISE EXCEPTION 'gateway migration found allowed_subnets that do not match t_gateway_node_network; reconcile the selected IPAM networks before retrying';
    END IF;
END;
$$;

CREATE TEMP TABLE velotrust_gateway_node_id_map (
    legacy_id BIGINT PRIMARY KEY,
    node_id BIGINT NOT NULL
) ON COMMIT DROP;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM public.t_gateway_node_network AS old_link
        LEFT JOIN public.t_iam_user AS creator ON creator.id = old_link.created_by
        LEFT JOIN public.t_iam_user AS updater ON updater.id = old_link.updated_by
        WHERE (old_link.created_by <> 0 AND creator.id IS NULL)
           OR (old_link.updated_by <> 0 AND updater.id IS NULL)
    ) THEN
        RAISE EXCEPTION 'gateway network migration found an audit actor ID without a matching IAM username';
    END IF;
END;
$$;

INSERT INTO public.t_ztna_node (
    node_name, node_code, node_type, node_status, health_status,
    created_at, created_by, updated_at, updated_by, version_no,
    gateway_type, gateway_network_name, require_approval, registered_at
)
SELECT old_gateway.gateway_name,
       old_gateway.gateway_id,
       'gateway',
       CASE old_gateway.status
           WHEN 'pending_enrollment' THEN 'pending'
           WHEN 'registered' THEN 'active'
           WHEN 'suspended' THEN 'disabled'
           WHEN 'revoked' THEN 'revoked'
       END,
       'unknown',
       old_gateway.created_at,
       COALESCE(creator.username, ''),
       old_gateway.updated_at,
       COALESCE(creator.username, ''),
       GREATEST(old_gateway.version_no, 1),
       old_gateway.gateway_type,
       old_gateway.network_name,
       old_gateway.require_approval,
       old_gateway.registered_at
FROM public.t_gateway_node AS old_gateway
LEFT JOIN public.t_iam_user AS creator ON creator.id = old_gateway.created_by
WHERE NOT EXISTS (
    SELECT 1 FROM public.t_ztna_node AS existing_node
    WHERE existing_node.node_code = old_gateway.gateway_id
      AND existing_node.node_type = 'gateway'
);

UPDATE public.t_ztna_node AS node
SET node_name = old_gateway.gateway_name,
    node_status = CASE old_gateway.status
        WHEN 'pending_enrollment' THEN 'pending'
        WHEN 'registered' THEN 'active'
        WHEN 'suspended' THEN 'disabled'
        WHEN 'revoked' THEN 'revoked'
    END,
    gateway_type = old_gateway.gateway_type,
    gateway_network_name = old_gateway.network_name,
    require_approval = old_gateway.require_approval,
    registered_at = old_gateway.registered_at,
    created_at = old_gateway.created_at,
    created_by = COALESCE(creator.username, ''),
    updated_at = old_gateway.updated_at,
    updated_by = COALESCE(creator.username, ''),
    version_no = GREATEST(old_gateway.version_no, 1)
FROM public.t_gateway_node AS old_gateway
LEFT JOIN public.t_iam_user AS creator ON creator.id = old_gateway.created_by
WHERE node.node_code = old_gateway.gateway_id
  AND node.node_type = 'gateway';

INSERT INTO velotrust_gateway_node_id_map (legacy_id, node_id)
SELECT old_gateway.id, node.id
FROM public.t_gateway_node AS old_gateway
JOIN public.t_ztna_node AS node
  ON node.node_code = old_gateway.gateway_id
 AND node.node_type = 'gateway';

INSERT INTO public.t_ztna_node_network (
    node_id, network_id, role, allowed_cidr,
    created_at, created_by, updated_at, updated_by, version_no
)
SELECT id_map.node_id,
       old_link.network_id,
       'route_gateway',
       old_link.allowed_cidr,
       old_link.created_at,
       COALESCE(link_creator.username, gateway_creator.username, ''),
       old_link.created_at,
       COALESCE(link_updater.username, gateway_creator.username, ''),
       1
FROM public.t_gateway_node_network AS old_link
JOIN velotrust_gateway_node_id_map AS id_map ON id_map.legacy_id = old_link.gateway_row_id
JOIN public.t_gateway_node AS old_gateway ON old_gateway.id = old_link.gateway_row_id
LEFT JOIN public.t_iam_user AS gateway_creator ON gateway_creator.id = old_gateway.created_by
LEFT JOIN public.t_iam_user AS link_creator ON link_creator.id = old_link.created_by
LEFT JOIN public.t_iam_user AS link_updater ON link_updater.id = old_link.updated_by
ON CONFLICT (node_id, network_id) DO UPDATE
SET role = 'route_gateway',
    allowed_cidr = EXCLUDED.allowed_cidr;

ALTER TABLE public.t_ztna_network
    DROP CONSTRAINT fk_t_ztna_network_gateway;

UPDATE public.t_ztna_network AS network
SET gateway_row_id = id_map.node_id
FROM velotrust_gateway_node_id_map AS id_map
WHERE network.gateway_row_id = id_map.legacy_id;

ALTER TABLE public.t_ztna_network
    ADD CONSTRAINT fk_t_ztna_network_gateway
    FOREIGN KEY (gateway_row_id) REFERENCES public.t_ztna_node (id) ON DELETE RESTRICT;

ALTER TABLE public.t_gateway_registration_token
    DROP CONSTRAINT fk_t_gateway_registration_gateway;

ALTER TABLE public.t_gateway_registration_token
    RENAME COLUMN gateway_row_id TO node_id;

ALTER INDEX public.uq_t_gateway_registration_pending_gateway
    RENAME TO uq_t_gateway_registration_pending_node;
ALTER INDEX public.ix_t_gateway_registration_gateway_created
    RENAME TO ix_t_gateway_registration_node_created;

UPDATE public.t_gateway_registration_token AS credential
SET node_id = id_map.node_id
FROM velotrust_gateway_node_id_map AS id_map
WHERE credential.node_id = id_map.legacy_id;

ALTER TABLE public.t_gateway_registration_token
    ADD CONSTRAINT fk_t_gateway_registration_node
    FOREIGN KEY (node_id) REFERENCES public.t_ztna_node (id) ON DELETE RESTRICT;

DROP TABLE public.t_gateway_node_network;
DROP TABLE public.t_gateway_node;

DO $$
BEGIN
    IF EXISTS (
        SELECT node_name
        FROM public.t_ztna_node
        WHERE node_type = 'gateway'
        GROUP BY node_name
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'gateway migration found duplicate gateway names in t_ztna_node; rename duplicates before retrying';
    END IF;
END;
$$;

CREATE UNIQUE INDEX uq_t_ztna_gateway_node_name
    ON public.t_ztna_node (node_name)
    WHERE node_type = 'gateway';

ALTER TABLE public.t_gateway_registration_token
    DROP CONSTRAINT fk_t_gateway_registration_creator;
ALTER TABLE public.t_ztna_network
    DROP CONSTRAINT fk_t_ztna_network_creator;
ALTER TABLE public.t_ztna_address_pool
    DROP CONSTRAINT fk_t_ztna_address_pool_creator;
ALTER TABLE public.t_ztna_address_lease
    DROP CONSTRAINT fk_t_ztna_address_lease_creator;
ALTER TABLE public.t_ztna_nat_mapping
    DROP CONSTRAINT fk_t_ztna_nat_mapping_creator;
ALTER TABLE public.t_ztna_overlay_network
    DROP CONSTRAINT fk_t_ztna_overlay_network_creator;

DO $$
DECLARE
    item RECORD;
    column_type TEXT;
    legacy_column TEXT;
    has_unmapped_actor BOOLEAN;
BEGIN
    FOR item IN
        SELECT * FROM (VALUES
            ('t_gateway_registration_token', 'created_by'),
            ('t_gateway_registration_token', 'updated_by'),
            ('t_ztna_network', 'created_by'),
            ('t_ztna_network', 'updated_by'),
            ('t_ztna_address_pool', 'created_by'),
            ('t_ztna_address_pool', 'updated_by'),
            ('t_ztna_address_lease', 'created_by'),
            ('t_ztna_address_lease', 'updated_by'),
            ('t_ztna_nat_mapping', 'created_by'),
            ('t_ztna_nat_mapping', 'updated_by'),
            ('t_ztna_overlay_network', 'created_by'),
            ('t_ztna_overlay_network', 'updated_by')
        ) AS columns_to_convert(table_name, column_name)
    LOOP
        SELECT data_type INTO column_type
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = item.table_name
          AND column_name = item.column_name;

        IF column_type IN ('smallint', 'integer', 'bigint') THEN
            legacy_column := item.column_name || '_legacy_actor_id';
            EXECUTE format('ALTER TABLE public.%I RENAME COLUMN %I TO %I',
                           item.table_name, item.column_name, legacy_column);
            EXECUTE format('ALTER TABLE public.%I ADD COLUMN %I VARCHAR(64) NOT NULL DEFAULT %L',
                           item.table_name, item.column_name, '');
            EXECUTE format(
                'SELECT EXISTS (
                    SELECT 1 FROM public.%I AS row
                    LEFT JOIN public.t_iam_user AS actor ON actor.id = row.%I
                    WHERE row.%I <> 0 AND actor.id IS NULL
                )',
                item.table_name, legacy_column, legacy_column
            ) INTO has_unmapped_actor;
            IF has_unmapped_actor THEN
                RAISE EXCEPTION 'cannot convert %.%: a nonzero IAM user ID has no username',
                    item.table_name, item.column_name;
            END IF;
            EXECUTE format(
                'UPDATE public.%I AS row SET %I = actor.username
                 FROM public.t_iam_user AS actor WHERE actor.id = row.%I',
                item.table_name, item.column_name, legacy_column
            );
            EXECUTE format('ALTER TABLE public.%I DROP COLUMN %I', item.table_name, legacy_column);
            EXECUTE format('ALTER TABLE public.%I ALTER COLUMN %I SET DEFAULT %L',
                           item.table_name, item.column_name, '');
        END IF;
    END LOOP;
END;
$$;

COMMENT ON COLUMN public.t_ztna_node.gateway_type IS
    'Gateway subtype; populated only when node_type is gateway.';
COMMENT ON COLUMN public.t_ztna_node.gateway_network_name IS
    'Gateway registration network label retained for the registration API.';
COMMENT ON COLUMN public.t_ztna_node_network.allowed_cidr IS
    'Gateway route snapshot for route_gateway relations; the associated network remains the source of truth.';
COMMENT ON TABLE public.t_gateway_registration_token IS
    'Single-use gateway enrollment credentials linked to the canonical t_ztna_node inventory.';
