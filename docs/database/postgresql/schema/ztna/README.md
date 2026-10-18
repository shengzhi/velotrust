# ZTNA business schema

The executable migration is
`management/migrations/postgresql/011_ztna_business_core.sql`. Management applies
it through the versioned PostgreSQL migration runner. The SQL in this directory
is a review copy; keep it byte-for-byte aligned with the executable migration.

The migration follows sections 6–10 of
`docs/VeloTrust_数据模型设计文档_最终版.md` and creates 21 business tables:

- Device identity and posture: `t_ztna_device`,
  `t_ztna_device_credential`, `t_ztna_device_posture_check`.
- Managed nodes and protected resources: `t_ztna_node`,
  `t_ztna_node_network`, `t_ztna_node_endpoint`,
  `t_ztna_protected_resource`.
- Policy and authorization: `t_ztna_access_policy`,
  `t_ztna_access_policy_revision`, `t_ztna_policy_subject`,
  `t_ztna_policy_resource`, `t_ztna_authorization_decision`,
  `t_ztna_authorization_grant`.
- Tunnels and configuration delivery: `t_ztna_tunnel`,
  `t_ztna_tunnel_peer`, `t_ztna_config_revision`,
  `t_ztna_node_config_task`, `t_ztna_node_config_task_event`,
  `t_ztna_node_status_report`, `t_ztna_relay`.
- Security operations: `t_ztna_security_event`.

Some of these entities are marked recommended or optional in the model. Their
tables establish the persistence contract; they do not imply that the related
business services or UI are implemented. Nodes use `t_ztna_node_network` as the
source of truth for network membership so a node can attach to multiple
networks without duplicating a single `network_id` field on the node row.

The migration depends on IAM tables from migration 001 and the enterprise
network catalog `t_ztna_network` from migration 008. Existing Gateway inventory,
Overlay/IPAM, and IAM tables are not recreated here. Use the Management
migration runner rather than manually applying only this file so dependencies
are created in order.

The constraints preserve credential and task history, validate lifecycle
states, and keep policy revisions immutable by application contract. CIDR
overlap and `allowed_ips` semantic conflict checks remain application-level
validation because they depend on the compiled network configuration.
