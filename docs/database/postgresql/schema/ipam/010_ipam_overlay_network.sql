-- Persist the private IPv4 address space from which Overlay pools are carved.

CREATE TABLE public.t_ztna_overlay_network (
    id           SMALLINT PRIMARY KEY DEFAULT 1,
    network_cidr CIDR NOT NULL,
    created_by   BIGINT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version_no   BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_t_ztna_overlay_network_singleton CHECK (id = 1),
    CONSTRAINT ck_t_ztna_overlay_network_private_a CHECK (
        family(network_cidr) = 4
        AND network_cidr <<= '10.0.0.0/8'::cidr
        AND masklen(network_cidr) BETWEEN 8 AND 16
    ),
    CONSTRAINT ck_t_ztna_overlay_network_version CHECK (version_no > 0),
    CONSTRAINT fk_t_ztna_overlay_network_creator FOREIGN KEY (created_by)
        REFERENCES public.t_iam_user (id) ON DELETE RESTRICT
);

COMMENT ON TABLE public.t_ztna_overlay_network IS
    'Singleton VeloTrust IPv4 Overlay supernet configuration. Address pools must be contained within this private 10.0.0.0/8 space.';
