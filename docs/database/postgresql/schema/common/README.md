# Shared model metadata migration

The executable migration is `management/migrations/postgresql/012_model_save_metadata.sql`.
The matching SQL copy in this directory is for schema review; Management's
versioned migration runner remains the source of truth.

This migration adds the common `created_at`, `updated_at`, `version_no`,
`created_by`, and `updated_by` fields needed by persisted models that embed
`database.BaseModel`. Existing columns are left in place, and newly added
columns receive defaults so existing rows remain valid.
