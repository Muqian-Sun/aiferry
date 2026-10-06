-- 271_model_catalog_time_pricing_exclude_dates.sql
-- 目录条目的忙闲时加「节假日」：这些日期（YYYY-MM-DD，按条目时区的本地日期）全天按平时。
-- DeepSeek 官方高峰不含中国法定节假日（muqian 2026-10-06）。NULL = 没有节假日。
ALTER TABLE model_catalog_time_pricing
    ADD COLUMN IF NOT EXISTS exclude_dates JSONB;
