# IAM PostgreSQL Schema

`001_iam.sql` creates the IAM tables using the `public` schema. It covers the tables in the data model document plus the `password_hash_version`, `webauthn_user_handle`, `t_iam_webauthn_credential`, and `t_iam_auth_throttle` additions from the IAM design.

The script is a one-time initial schema snapshot for a fresh database. It is wrapped in a PostgreSQL transaction and deliberately does not use `IF NOT EXISTS`, so an existing conflicting schema fails instead of being silently accepted. Subsequent schema changes must be written as versioned migrations under `migrations/`.

The following tables are included for the complete IAM model. TOTP MFA and Passkey flows are implemented in the IAM service; external identity federation remains staged:

- MFA: `t_iam_user_authenticator` stores encrypted TOTP secrets and accepted time steps; `t_iam_auth_challenge` supports one-time ceremonies
- Passkeys: `t_iam_webauthn_credential`; WebAuthn challenges also use `t_iam_auth_challenge`
- External sign-in and event deduplication: `t_iam_identity_provider`, `t_iam_external_identity`, `t_iam_integration_event`
- Method metadata: `t_iam_rpc_method`. The first IAM implementation uses code-declared JRPC permissions as its runtime authority; this table must not make an unregistered method public.

External organization tables (`t_iam_external_org` and `t_iam_external_identity_org`) and the directory synchronization task table (`t_iam_integration_sync_task`) are intentionally omitted for now. The WebAuthn RP ID and allowed origins must come from trusted deployment configuration; do not derive them from an untrusted request host.

The `device_id` columns in IAM session and login event tables intentionally have no foreign key here because `t_ztna_device` belongs to the separately managed ZTNA schema. User and audit/session history is retained; application code should disable users instead of hard-deleting them. The application is responsible for setting `updated_at` and incrementing `version_no` on updates.

## Apply through the migration runner

For a new database, use the versioned migration runner, which applies schema
changes transactionally and records checksums:

```sh
go run ./cmd/management migrate --config management/configs/management.example.toml
```

If this schema was already applied manually, verify that it matches the snapshot
and then record the initial migration baseline:

```sh
go run ./cmd/management migrate --config management/configs/management.example.toml --baseline-existing-iam
```

The baseline flag skips only the initial schema creation. Subsequent numbered
migrations still run normally.

The initial snapshot constrains menu rows to directories, pages, and external
links. Migration `003_iam_menu_button_type.sql` extends that constraint for the
prototype's hidden button-level permission entries; seed data is applied by
`004_iam_menu_seed.sql` and documented under `docs/database/postgresql/seed/`.

This SQL was generated from `docs/VeloTrust_数据模型设计文档_最终版.md`, with IAM-specific additions documented in `docs/VeloTrust_IAM身份与访问管理模块设计.md`.
