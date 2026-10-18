-- The prototype menu tree supports action-level button entries below pages.
-- Their authorization remains enforced by the IAM permission layer.
BEGIN;

ALTER TABLE public.t_iam_menu
    DROP CONSTRAINT IF EXISTS ck_t_iam_menu_type;

ALTER TABLE public.t_iam_menu
    ADD CONSTRAINT ck_t_iam_menu_type
    CHECK (menu_type IN ('directory', 'page', 'external', 'button'));

COMMIT;
