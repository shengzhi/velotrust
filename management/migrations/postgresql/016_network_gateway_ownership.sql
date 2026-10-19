-- A routed network belongs to at most one gateway. Preserve unambiguous
-- gateway assignments before removing the redundant node/network relation.
-- The migration runner owns the transaction; any conflict rolls everything back.

LOCK TABLE public.t_ztna_network, public.t_ztna_node_network IN ACCESS EXCLUSIVE MODE;

DO $$
DECLARE
    conflict_id BIGINT;
BEGIN
    SELECT network.id INTO conflict_id
    FROM public.t_ztna_network AS network
    JOIN public.t_ztna_node AS node ON node.id = network.gateway_row_id
    WHERE node.node_type <> 'gateway'
    ORDER BY network.id LIMIT 1;
    IF FOUND THEN
        RAISE EXCEPTION 'network % points to a non-gateway node; assign a gateway or clear gateway_row_id before retrying migration 016', conflict_id;
    END IF;

    SELECT link.id INTO conflict_id
    FROM public.t_ztna_node_network AS link
    JOIN public.t_ztna_node AS node ON node.id = link.node_id
    WHERE node.node_type <> 'gateway' OR link.role <> 'route_gateway'
    ORDER BY link.id LIMIT 1;
    IF FOUND THEN
        RAISE EXCEPTION 'node/network relation % is not a gateway route; explicitly reconcile this relation before retrying migration 016', conflict_id;
    END IF;

    SELECT network_id INTO conflict_id
    FROM public.t_ztna_node_network
    GROUP BY network_id HAVING COUNT(DISTINCT node_id) > 1
    ORDER BY network_id LIMIT 1;
    IF FOUND THEN
        RAISE EXCEPTION 'network % is linked to multiple gateways; retain exactly one gateway assignment before retrying migration 016', conflict_id;
    END IF;

    SELECT network.id INTO conflict_id
    FROM public.t_ztna_network AS network
    JOIN public.t_ztna_node_network AS link ON link.network_id = network.id
    WHERE network.gateway_row_id IS NOT NULL AND network.gateway_row_id <> link.node_id
    ORDER BY network.id LIMIT 1;
    IF FOUND THEN
        RAISE EXCEPTION 'network % has conflicting gateway_row_id and node/network assignments; reconcile them before retrying migration 016', conflict_id;
    END IF;

    SELECT network.id INTO conflict_id
    FROM public.t_ztna_network AS network
    JOIN public.t_ztna_node_network AS link ON link.network_id = network.id
    WHERE link.allowed_cidr IS NOT NULL AND link.allowed_cidr <> network.cidr
    ORDER BY network.id LIMIT 1;
    IF FOUND THEN
        RAISE EXCEPTION 'network % has a route snapshot different from its CIDR; reconcile the route before retrying migration 016', conflict_id;
    END IF;
END;
$$;

UPDATE public.t_ztna_network AS network
SET gateway_row_id = link.node_id,
    updated_at = CURRENT_TIMESTAMP,
    updated_by = 'migration',
    version_no = network.version_no + 1
FROM public.t_ztna_node_network AS link
WHERE link.network_id = network.id AND network.gateway_row_id IS NULL;

ALTER TABLE public.t_ztna_network
    RENAME COLUMN gateway_row_id TO gateway_node_id;

CREATE INDEX ix_t_ztna_network_gateway_node
    ON public.t_ztna_network (gateway_node_id)
    WHERE gateway_node_id IS NOT NULL;

DROP TABLE public.t_ztna_node_network;

COMMENT ON COLUMN public.t_ztna_network.gateway_node_id IS
    'The single owning gateway in t_ztna_node. A gateway may own multiple routed networks; network CIDR is the route source of truth.';
