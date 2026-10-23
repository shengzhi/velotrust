-- Normalize ZTNA audit actors for employee-enrolled Client nodes.
-- New writes use employee usernames for employee-triggered enrollment and
-- typed node principals for authenticated node activity.
BEGIN;

WITH client_owners AS (
    SELECT device.node_id, MIN(owner.username) AS username
    FROM public.t_ztna_device AS device
    JOIN public.t_iam_user AS owner ON owner.id=device.owner_user_id
    WHERE device.node_id IS NOT NULL
    GROUP BY device.node_id
    HAVING COUNT(DISTINCT owner.id)=1
)
UPDATE public.t_ztna_node AS node
SET created_by=owners.username,
    updated_by=CASE
        WHEN node.version_no=1 AND node.created_at=node.updated_at AND node.updated_by=node.created_by
            THEN owners.username
        ELSE node.updated_by
    END
FROM client_owners AS owners
WHERE node.id=owners.node_id
  AND node.node_type='client_agent'
  AND (node.created_by='hub' OR node.created_by LIKE 'hub:%');

UPDATE public.t_ztna_device AS device
SET created_by=owner.username,
    updated_by=CASE
        WHEN device.version_no=1 AND device.created_at=device.updated_at AND device.updated_by=device.created_by
            THEN owner.username
        ELSE device.updated_by
    END
FROM public.t_iam_user AS owner
WHERE owner.id=device.owner_user_id
  AND device.node_id IS NOT NULL
  AND (device.created_by='hub' OR device.created_by LIKE 'hub:%');

UPDATE public.t_ztna_device_credential AS credential
SET created_by=owner.username,
    updated_by=CASE
        WHEN credential.version_no=1 AND credential.created_at=credential.updated_at AND credential.updated_by=credential.created_by
            THEN owner.username
        ELSE credential.updated_by
    END
FROM public.t_ztna_device AS device
JOIN public.t_iam_user AS owner ON owner.id=device.owner_user_id
WHERE credential.device_id=device.id
  AND device.node_id IS NOT NULL
  AND (credential.created_by='hub' OR credential.created_by LIKE 'hub:%');

COMMIT;
