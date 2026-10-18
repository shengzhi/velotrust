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
