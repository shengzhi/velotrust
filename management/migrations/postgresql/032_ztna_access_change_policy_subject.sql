-- Keep the durable authorization-change outbox compatible with the current
-- access-policy compiler. Earlier database snapshots may still carry the
-- pre-policy constraint from migration 020.

ALTER TABLE public.t_ztna_access_change_event
    DROP CONSTRAINT IF EXISTS ck_t_ztna_access_change_subject,
    ADD CONSTRAINT ck_t_ztna_access_change_subject
        CHECK (subject_type IN ('user', 'device', 'node', 'policy'));
