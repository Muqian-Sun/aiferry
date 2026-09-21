-- 订阅脱离分组：套餐自带限额与模型集，订阅 / 兑换码按套餐关联，订阅 key 绑到订阅行。
-- 无历史数据：只改结构，不回填。

-- 1. 套餐：去分组，加三档限额
ALTER TABLE subscription_plans DROP COLUMN IF EXISTS group_id;
DROP INDEX IF EXISTS idx_subscription_plans_group_id;
ALTER TABLE subscription_plans ADD COLUMN IF NOT EXISTS daily_limit_usd   NUMERIC(20,8);
ALTER TABLE subscription_plans ADD COLUMN IF NOT EXISTS weekly_limit_usd  NUMERIC(20,8);
ALTER TABLE subscription_plans ADD COLUMN IF NOT EXISTS monthly_limit_usd NUMERIC(20,8);

-- 2. 套餐模型集
CREATE TABLE IF NOT EXISTS subscription_plan_models (
    plan_id  BIGINT NOT NULL REFERENCES subscription_plans(id) ON DELETE CASCADE,
    entry_id BIGINT NOT NULL REFERENCES model_catalog_entries(id) ON DELETE CASCADE,
    PRIMARY KEY (plan_id, entry_id)
);
CREATE INDEX IF NOT EXISTS idx_subscription_plan_models_entry ON subscription_plan_models (entry_id);

-- 3. 用户订阅：group_id → plan_id
ALTER TABLE user_subscriptions DROP COLUMN IF EXISTS group_id;
DROP INDEX IF EXISTS user_subscriptions_user_group_unique_active;
DROP INDEX IF EXISTS idx_user_subscriptions_group_id;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS plan_id BIGINT NOT NULL REFERENCES subscription_plans(id) ON DELETE RESTRICT;
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_plan_id ON user_subscriptions(plan_id);
CREATE UNIQUE INDEX IF NOT EXISTS user_subscriptions_user_plan_unique_active
    ON user_subscriptions(user_id, plan_id) WHERE deleted_at IS NULL;

-- 4. 订阅 key
-- 不写 ON DELETE：默认 NO ACTION 在语句末检查，users 级联能过；单独硬删订阅行会被拦，订阅 key 不会悄悄变成余额 key。
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS subscription_id BIGINT REFERENCES user_subscriptions(id);
CREATE INDEX IF NOT EXISTS idx_api_keys_subscription_id ON api_keys(subscription_id);

-- 5. 兑换码：group_id → plan_id
ALTER TABLE redeem_codes DROP COLUMN IF EXISTS group_id;
DROP INDEX IF EXISTS idx_redeem_codes_group_id;
ALTER TABLE redeem_codes ADD COLUMN IF NOT EXISTS plan_id BIGINT REFERENCES subscription_plans(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_redeem_codes_plan_id ON redeem_codes(plan_id);

-- 6. 订单：删分组快照列（plan_id 已有，无外键照旧）
ALTER TABLE payment_orders DROP COLUMN IF EXISTS subscription_group_id;

-- 7. 分组订阅列
ALTER TABLE groups DROP COLUMN IF EXISTS subscription_type;
ALTER TABLE groups DROP COLUMN IF EXISTS daily_limit_usd;
ALTER TABLE groups DROP COLUMN IF EXISTS weekly_limit_usd;
ALTER TABLE groups DROP COLUMN IF EXISTS monthly_limit_usd;
ALTER TABLE groups DROP COLUMN IF EXISTS default_validity_days;
DROP INDEX IF EXISTS idx_groups_subscription_type;
