-- Keep one authoritative policy/configuration state in the database. Policy
-- rules move onto the policy row. Configuration revisions collapse to the
-- latest snapshot per target, identified by content hash rather than a
-- monotonically increasing business version. BaseModel.version_no remains an
-- optimistic concurrency token and is not a policy/configuration history.
-- The migration runner owns the transaction.

DO $$
DECLARE
    unresolved_task VARCHAR(64);
BEGIN
    SELECT task.task_id INTO unresolved_task
    FROM public.t_ztna_node_config_task AS task
    JOIN public.t_ztna_node AS node ON node.id = task.node_id
    LEFT JOIN LATERAL (
        SELECT COUNT(DISTINCT current_config.config_hash) AS hash_count
        FROM (
            SELECT DISTINCT ON (config.target_type) config.config_hash
            FROM public.t_ztna_config_revision AS config
            WHERE config.target_id = node.id
              AND config.target_type IN ('node', 'gateway')
            ORDER BY config.target_type,
                     config.config_version DESC,
                     config.created_at DESC,
                     config.id DESC
        ) AS current_config
    ) AS config_match ON TRUE
    WHERE task.task_type = 'apply_config'
      AND task.status IN ('pending', 'queued', 'sent', 'received', 'applying', 'failed')
      AND config_match.hash_count <> 1
    ORDER BY task.id
    LIMIT 1;

    IF FOUND THEN
        RAISE EXCEPTION 'active apply_config task % has no unique current node/gateway snapshot; reconcile or cancel it before retrying migration 018', unresolved_task;
    END IF;
END;
$$;

ALTER TABLE public.t_ztna_access_policy
    ADD COLUMN rules_json JSONB NOT NULL DEFAULT '[]'::JSONB,
    ADD COLUMN compiled_hash VARCHAR(128);

WITH ranked_policy_rules AS (
    SELECT revision.policy_id,
           revision.rules_json,
           revision.compiled_hash,
           ROW_NUMBER() OVER (
               PARTITION BY revision.policy_id
               ORDER BY (revision.id = policy.current_revision_id) DESC,
                        revision.revision_no DESC,
                        revision.created_at DESC,
                        revision.id DESC
           ) AS row_no
    FROM public.t_ztna_access_policy_revision AS revision
    JOIN public.t_ztna_access_policy AS policy ON policy.id = revision.policy_id
)
UPDATE public.t_ztna_access_policy AS policy
SET rules_json = selected.rules_json,
    compiled_hash = selected.compiled_hash
FROM ranked_policy_rules AS selected
WHERE selected.policy_id = policy.id AND selected.row_no = 1;

ALTER TABLE public.t_ztna_access_policy
    DROP CONSTRAINT fk_t_ztna_access_policy_current_revision,
    DROP CONSTRAINT ck_t_ztna_access_policy_active_revision,
    DROP COLUMN current_revision_id,
    ADD CONSTRAINT ck_t_ztna_access_policy_rules
        CHECK (jsonb_typeof(rules_json) IN ('array', 'object'));
ALTER TABLE public.t_ztna_access_policy
    RENAME CONSTRAINT ck_t_ztna_access_policy_version TO ck_t_ztna_access_policy_version_no;
DROP INDEX public.ix_t_ztna_access_policy_current_revision;

ALTER TABLE public.t_ztna_authorization_decision
    DROP CONSTRAINT fk_t_ztna_authorization_decision_revision,
    DROP CONSTRAINT ck_t_ztna_authorization_decision_policy_revision,
    DROP COLUMN policy_revision_id;

DROP TABLE public.t_ztna_access_policy_revision;

ALTER TABLE public.t_ztna_node
    ADD COLUMN last_config_hash VARCHAR(128),
    ADD COLUMN actual_config_hash VARCHAR(128);

UPDATE public.t_ztna_node AS node
SET last_config_hash = (
        SELECT CASE WHEN COUNT(DISTINCT config.config_hash) = 1 THEN MIN(config.config_hash) END
        FROM public.t_ztna_config_revision AS config
        WHERE config.target_type IN ('node', 'gateway')
          AND config.target_id = node.id
          AND config.config_version = node.last_config_version
    ),
    actual_config_hash = (
        SELECT CASE WHEN COUNT(DISTINCT config.config_hash) = 1 THEN MIN(config.config_hash) END
        FROM public.t_ztna_config_revision AS config
        WHERE config.target_type IN ('node', 'gateway')
          AND config.target_id = node.id
          AND config.config_version = node.actual_config_version
    );

ALTER TABLE public.t_ztna_node
    DROP CONSTRAINT ck_t_ztna_node_config_versions,
    DROP COLUMN last_config_version,
    DROP COLUMN actual_config_version;

ALTER TABLE public.t_ztna_node_status_report
    ADD COLUMN actual_config_hash VARCHAR(128);

UPDATE public.t_ztna_node_status_report AS report
SET actual_config_hash = (
    SELECT CASE WHEN COUNT(DISTINCT config.config_hash) = 1 THEN MIN(config.config_hash) END
    FROM public.t_ztna_config_revision AS config
    WHERE config.target_type IN ('node', 'gateway')
      AND config.target_id = report.node_id
      AND config.config_version = report.actual_config_version
);

ALTER TABLE public.t_ztna_node_status_report
    DROP CONSTRAINT ck_t_ztna_node_status_report_config,
    DROP COLUMN actual_config_version;

ALTER TABLE public.t_ztna_node_config_task
    ADD COLUMN target_config_hash VARCHAR(128);

UPDATE public.t_ztna_node_config_task AS task
SET target_config_hash = (
    SELECT CASE WHEN COUNT(DISTINCT config.config_hash) = 1 THEN MIN(config.config_hash) END
    FROM public.t_ztna_node AS node
    JOIN public.t_ztna_config_revision AS config
      ON config.target_id = node.id
     AND config.target_type IN ('node', 'gateway')
     AND config.config_version = task.target_config_version
    WHERE node.id = task.node_id
);

-- Pending apply work always reconciles to the database's current desired
-- snapshot. Do not preserve an older queued config version as an apply target.
UPDATE public.t_ztna_node_config_task AS task
SET target_config_hash = (
    SELECT MIN(current_config.config_hash)
    FROM (
        SELECT DISTINCT ON (config.target_type) config.config_hash
        FROM public.t_ztna_config_revision AS config
        WHERE config.target_id = task.node_id
          AND config.target_type IN ('node', 'gateway')
        ORDER BY config.target_type,
                 config.config_version DESC,
                 config.created_at DESC,
                 config.id DESC
    ) AS current_config
)
WHERE task.task_type = 'apply_config'
  AND task.status IN ('pending', 'queued', 'sent', 'received', 'applying', 'failed');

ALTER TABLE public.t_ztna_node_config_task
    DROP CONSTRAINT ck_t_ztna_node_config_task_version,
    DROP COLUMN target_config_version,
    ADD CONSTRAINT ck_t_ztna_node_config_task_hash
        CHECK (target_config_hash IS NULL OR length(btrim(target_config_hash)) > 0);

WITH ranked_configs AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY target_type, target_id
               ORDER BY config_version DESC, created_at DESC, id DESC
           ) AS row_no
    FROM public.t_ztna_config_revision
)
DELETE FROM public.t_ztna_config_revision AS config
USING ranked_configs AS ranked
WHERE ranked.id = config.id AND ranked.row_no > 1;

UPDATE public.t_ztna_config_revision
SET updated_at = created_at,
    updated_by = created_by,
    version_no = GREATEST(version_no, 1);

DROP INDEX public.ix_t_ztna_config_revision_created;
ALTER TABLE public.t_ztna_config_revision
    DROP CONSTRAINT uq_t_ztna_config_revision_target_version,
    DROP CONSTRAINT ck_t_ztna_config_revision_version,
    DROP COLUMN config_version;
ALTER TABLE public.t_ztna_config_revision
    RENAME CONSTRAINT ck_t_ztna_config_revision_target TO ck_t_ztna_config_target;
ALTER TABLE public.t_ztna_config_revision
    RENAME CONSTRAINT ck_t_ztna_config_revision_payload TO ck_t_ztna_config_payload;
ALTER TABLE public.t_ztna_config_revision
    ADD CONSTRAINT uq_t_ztna_config_target UNIQUE (target_type, target_id);
ALTER TABLE public.t_ztna_config_revision
    RENAME TO t_ztna_config;

COMMENT ON TABLE public.t_ztna_config IS
    'Authoritative current desired configuration, one mutable row per target; config_hash identifies content and no revision history is retained.';
COMMENT ON COLUMN public.t_ztna_config.config_hash IS
    'Content fingerprint used to identify and apply the current desired configuration; it is not a revision number.';
COMMENT ON TABLE public.t_ztna_access_policy IS
    'Authoritative current access policy. Rules are stored on this row and updates replace the current database state.';
