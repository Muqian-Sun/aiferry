-- 用户级计费倍率：用户价 = 目录价 × rate_multiplier。0 表示免费。
ALTER TABLE users ADD COLUMN IF NOT EXISTS rate_multiplier NUMERIC(10,4) NOT NULL DEFAULT 1;
