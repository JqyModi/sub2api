BEGIN;

-- Launch activity: price floor uses the stable upstream ceiling of CNY 0.15 per standard USD.
UPDATE groups SET daily_limit_usd = 10, weekly_limit_usd = 20, monthly_limit_usd = 20, rpm_limit = 5, updated_at = now()
WHERE id = 2;

UPDATE groups SET daily_limit_usd = 30, weekly_limit_usd = 100, monthly_limit_usd = 300, rpm_limit = 10, updated_at = now()
WHERE id = 3;

UPDATE groups SET daily_limit_usd = 60, weekly_limit_usd = 220, monthly_limit_usd = 570, rpm_limit = 15, updated_at = now()
WHERE id = 4;

UPDATE groups SET daily_limit_usd = 100, weekly_limit_usd = 350, monthly_limit_usd = 950, rpm_limit = 20, updated_at = now()
WHERE id = 5;

UPDATE groups SET daily_limit_usd = 180, weekly_limit_usd = 650, monthly_limit_usd = 1700, rpm_limit = 25, updated_at = now()
WHERE id = 6;

UPDATE subscription_plans SET
  description = $$适合邮箱验证后的短期体验，无需支付即可领取 3 天试用额度。$$,
  features = $$3 天有效期
$20 标准用量额度
每日最高 $10
RPM 5
邮箱验证后自动领取$$,
  description_en = $$A short trial for email-verified users. No payment is required to receive 3 days of access. $$,
  features_en = $$3-day validity
$20 standard usage quota
Up to $10 per day
RPM 5
Granted after email verification$$,
  for_sale = false,
  updated_at = now()
WHERE id = 1;

UPDATE subscription_plans SET
  description = $$适合轻度使用和初次体验，30 天内可使用 $300 标准用量额度。活动期间同价加量。$$,
  features = $$ $300 标准用量额度
每日最高 $30
每周最高 $100
RPM 10
支持桌面一键接入$$,
  description_en = $$For light usage and first-time users. Includes $300 of standard usage quota over 30 days during the launch activity. $$,
  features_en = $$ $300 standard usage quota
Up to $30 per day
Up to $100 per week
RPM 10
One-click desktop connection$$,
  max_sales = 20,
  updated_at = now()
WHERE id = 2;

UPDATE subscription_plans SET
  description = $$适合持续开发和日常项目使用，30 天内可使用 $570 标准用量额度。活动期间同价加量。$$,
  features = $$ $570 标准用量额度
每日最高 $60
每周最高 $220
RPM 15
支持桌面一键接入$$,
  description_en = $$For continuous development and everyday projects. Includes $570 of standard usage quota over 30 days during the launch activity. $$,
  features_en = $$ $570 standard usage quota
Up to $60 per day
Up to $220 per week
RPM 15
One-click desktop connection$$,
  max_sales = 20,
  updated_at = now()
WHERE id = 3;

UPDATE subscription_plans SET
  description = $$适合高频开发和多个项目并行使用，30 天内可使用 $950 标准用量额度。活动期间同价加量。$$,
  features = $$ $950 标准用量额度
每日最高 $100
每周最高 $350
RPM 20
适合多个 Profile 并行$$,
  description_en = $$For frequent development and multiple concurrent projects. Includes $950 of standard usage quota over 30 days during the launch activity. $$,
  features_en = $$ $950 standard usage quota
Up to $100 per day
Up to $350 per week
RPM 20
Suitable for multiple Profiles$$,
  max_sales = 8,
  updated_at = now()
WHERE id = 4;

UPDATE subscription_plans SET
  description = $$适合重度开发和多 Profile 高频并行，30 天内可使用 $1,700 标准用量额度。活动期间同价加量。$$,
  features = $$ $1,700 标准用量额度
每日最高 $180
每周最高 $650
RPM 25
适合重度和多 Profile 使用$$,
  description_en = $$For intensive development and multiple concurrent Profiles. Includes $1,700 of standard usage quota over 30 days during the launch activity. $$,
  features_en = $$ $1,700 standard usage quota
Up to $180 per day
Up to $650 per week
RPM 25
Designed for intensive use and multiple Profiles$$,
  max_sales = 3,
  updated_at = now()
WHERE id = 5;

-- Email signup receives the trial subscription after verification.
UPDATE settings SET value = 'true', updated_at = now()
WHERE key = 'auth_source_default_email_grant_on_signup';

UPDATE settings SET value = '[{"group_id":2,"validity_days":3}]', updated_at = now()
WHERE key = 'auth_source_default_email_subscriptions';

COMMIT;
