-- 270_model_catalog_binding_time_pricing.sql
--
-- 上游忙闲时（muqian 2026-10-06：算成本按我们填的渠道价，上游有没有忙闲时在填承接时定）：
-- 每条承接加一列分时倍率（时区、仅工作日、时段 × 倍率，形状见 domain.TimePricingSpec），渠道成本按请求时刻整单 × 倍率。
-- NULL = 上游不分忙闲时。

ALTER TABLE model_catalog_bindings
    ADD COLUMN IF NOT EXISTS time_pricing JSONB;
