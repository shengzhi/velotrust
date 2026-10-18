ALTER TABLE public.t_iam_user_authenticator
    ADD COLUMN IF NOT EXISTS last_used_step BIGINT NOT NULL DEFAULT 0;
