ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS name_en VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS description_en TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS features_en TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN subscription_plans.name_en IS 'English plan name; empty falls back to name';
COMMENT ON COLUMN subscription_plans.description_en IS 'English plan description; empty falls back to description';
COMMENT ON COLUMN subscription_plans.features_en IS 'Newline-separated English features; empty falls back to features';
