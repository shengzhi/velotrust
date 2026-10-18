# Gateway Registration Schema

`001_gateway_enrollment.sql` is the reviewable schema snapshot for the
Management-owned gateway inventory and single-use registration credentials.
The executable, versioned migrations are kept under
`management/migrations/postgresql/006_gateway_enrollment.sql` and
`007_gateway_registration_permissions.sql` and run through the Management
migration command:

```sh
go run ./cmd/management migrate --config management/configs/management.example.toml
```

The credential table stores only a SHA-256 digest. The plaintext credential is
returned once by the issuance RPC and is not included in listing responses or
audit details. This schema does not yet implement Gateway Agent credential
consumption, registration approval, or registered-node authentication.
