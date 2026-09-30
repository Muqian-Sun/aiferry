-- 261_upstream_price_account_cost.sql
--
-- 删掉「渠道倍率」，付给上游的钱 = 用量 × 这个渠道给这个模型的上游价（muqian 2026-09-30：
-- 「付给上游的钱就是上游的标价就行了，不要再引入渠道倍率这个概念」；每个渠道 × 模型各填一份、
-- 必须填，没填不能承接；和官方价一样可分段）。方案页 X8eQjqzjAEx3hF4xzCzKZr 第三版。
--
-- 1. 承接关系（model_catalog_bindings）带上游价：输入 / 输出必填，缓存三项可空（官方价有的项由服务层要求必填）；
--    按 Token 分段存在 price_intervals（与官方分段同一套 JSON 形状）。绑定上的优先级删掉，只留渠道自己的（D2）。
--    现有承接关系都是测试数据、没有上游价，直接清掉，由价格页重新建。
-- 2. 用量逐行落「渠道成本」usage_logs.account_cost；删掉 usage_logs / batch_image_jobs 上的渠道倍率快照
--    与 accounts.rate_multiplier。历史用量的渠道成本不回填（全是测试数据），记 0。

DELETE FROM model_catalog_bindings;

ALTER TABLE model_catalog_bindings
    DROP COLUMN IF EXISTS priority,
    ADD COLUMN input_price          DECIMAL(20, 12) NOT NULL,
    ADD COLUMN output_price         DECIMAL(20, 12) NOT NULL,
    ADD COLUMN cache_write_price    DECIMAL(20, 12),
    ADD COLUMN cache_write_1h_price DECIMAL(20, 12),
    ADD COLUMN cache_read_price     DECIMAL(20, 12),
    ADD COLUMN price_intervals      JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD CONSTRAINT model_catalog_bindings_prices_non_negative CHECK (
        input_price >= 0 AND output_price >= 0
        AND (cache_write_price IS NULL OR cache_write_price >= 0)
        AND (cache_write_1h_price IS NULL OR cache_write_1h_price >= 0)
        AND (cache_read_price IS NULL OR cache_read_price >= 0)
    );

ALTER TABLE usage_logs
    DROP COLUMN IF EXISTS account_rate_multiplier,
    ADD COLUMN account_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;

ALTER TABLE batch_image_jobs DROP COLUMN IF EXISTS account_rate_multiplier;

ALTER TABLE accounts DROP COLUMN IF EXISTS rate_multiplier;
