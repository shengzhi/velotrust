# IPAM network catalog schema

The executable migrations are `management/migrations/postgresql/008_ipam_network_catalog.sql`,
`management/migrations/postgresql/009_ipam_overlay_pools_leases_nat.sql`, and
`management/migrations/postgresql/010_ipam_overlay_network.sql`. Management applies
them with the normal versioned migration runner.

`t_ztna_network` stores routed network prefixes. Gateway registration selects
only active rows with `network_type='site'`, which represent physical-site LAN
prefixes. This catalog is separate from the overlay address pools used to
allocate WireGuard tunnel addresses. New gateway registrations store a CIDR
snapshot and a foreign-keyed association to every selected network in
`t_gateway_node_network`.

The migration also adds the `ipam.network.read` and `ipam.network.write`
permissions and grants them to active `platform_admin` roles.

Migration 009 creates `t_ztna_address_pool` and `t_ztna_address_lease` for IPv4
WireGuard Overlay address allocation, and `t_ztna_nat_mapping` for persistent
enterprise NAT configuration records. Active or reserved leases occupy their
address; released leases remain as history. Pool and route ranges are checked
again inside serialized write transactions. NAT records are currently an IPAM
configuration inventory; Management does not yet synchronize them to the
Gateway Agent dataplane.

Migration 010 adds the singleton `t_ztna_overlay_network` configuration. An
administrator initializes it with the **Initialize global Overlay network**
action in the IPAM page header before creating address pools. The configured CIDR must be an IPv4
prefix inside the private Class A `10.0.0.0/8` range, with a prefix length from
`/8` through `/16`. Every address pool must fit inside it. Changes are rejected
if the new range overlaps an active enterprise network or enabled NAT shadow
network, or no longer contains an existing pool. `Overview` returns an empty
`global_cidr` until this configuration has been initialized; it is not inferred
from existing pools.
