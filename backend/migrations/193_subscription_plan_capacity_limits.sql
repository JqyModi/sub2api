ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS max_sales INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS per_user_limit INT NOT NULL DEFAULT 0;

UPDATE subscription_plans
SET max_sales = GREATEST(max_sales, 0),
    per_user_limit = GREATEST(per_user_limit, 0);
