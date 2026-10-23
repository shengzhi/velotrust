-- Separate Management login accounts from employee identities used by Client devices.
-- Existing users remain platform accounts so current Management access is preserved.

ALTER TABLE public.t_iam_user
    ADD COLUMN account_type VARCHAR(24) NOT NULL DEFAULT 'platform';

ALTER TABLE public.t_iam_user
    ADD CONSTRAINT ck_t_iam_user_account_type
        CHECK (account_type IN ('platform', 'employee'));

CREATE INDEX ix_t_iam_user_account_type_status
    ON public.t_iam_user (account_type, user_status);

COMMENT ON COLUMN public.t_iam_user.account_type IS
    'platform accounts may authenticate to Management and receive IAM roles; employee accounts may authenticate to Client Agent through HUB but cannot authenticate to Management.';
