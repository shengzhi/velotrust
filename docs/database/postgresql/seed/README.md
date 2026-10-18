# PostgreSQL Seed Data

`004_iam_menu_seed.sql` initializes the Management console navigation from the
prototype menu catalog. It contains the seven root directories, page entries,
and hidden button-level permission entries. The script is idempotent and
updates an existing seeded menu only when its metadata differs.

The executable migration is embedded at
`management/migrations/postgresql/004_iam_menu_seed.sql` and is applied by the
normal Management migration runner. This documentation copy is kept byte-for-
byte identical so it can also be reviewed or applied manually after the IAM
schema and `003_iam_menu_button_type.sql` migration exist:

```sh
go run ./cmd/management migrate --config management/configs/management.example.toml
```

Migration `003_iam_menu_button_type.sql` extends the menu type constraint to
accept the prototype's `button` entries. Button rows remain metadata only;
their operation authorization is enforced separately and is never granted by
navigation visibility.

`005_iam_role_menu_seed.sql` assigns the active menu catalog to the existing
`platform_admin` role. Other roles receive only explicitly assigned menus. A
fresh bootstrap also assigns all currently active menus to `platform_admin` in
the same transaction, regardless of whether bootstrap runs before or after
migration `004`.

`006_resource_permissions.sql` initializes the system resource and permission
catalog for IAM, gateway registration, and IPAM, then grants every active
catalog permission to the active system `platform_admin` role. It upserts catalog
metadata, does not bump versions when nothing changed, preserves deliberately
disabled resource/permission status, and adds missing role grants without
changing other roles. Run it after Management migrations and administrator
bootstrap; it fails clearly if the platform administrator role is missing. To
apply it manually from the repository root:

```sh
psql "$DATABASE_DSN" -v ON_ERROR_STOP=1 -f docs/database/postgresql/seed/006_resource_permissions.sql
```

Migration `management/migrations/postgresql/008_ipam_network_catalog.sql`
creates the persistent enterprise network catalog, gateway-to-network
associations, and the IPAM network read/write permissions. These networks
represent routed business/VPC ranges and remain separate from WireGuard overlay
address pools.
