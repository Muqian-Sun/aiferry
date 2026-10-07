-- 272_model_catalog_binding_max_reasoning.sql
--
-- 最高推理倍率分官方 / 售价 / 成本三套（muqian 2026-10-07：与忙闲时一样，售价与成本默认跟官方，可单独设）：
-- 官方 = model_catalog_entries.max_reasoning_effort_multiplier（已有），售价存在 sale_prices JSONB 里，
-- 成本 = 每条承接加这一列：上游在最高推理档（effort = max）整单乘的倍数。NULL = 跟官方。

ALTER TABLE model_catalog_bindings
    ADD COLUMN IF NOT EXISTS max_reasoning_effort_multiplier NUMERIC(10,4);

ALTER TABLE model_catalog_bindings
    DROP CONSTRAINT IF EXISTS chk_model_catalog_bindings_max_reasoning_positive;
ALTER TABLE model_catalog_bindings
    ADD CONSTRAINT chk_model_catalog_bindings_max_reasoning_positive
    CHECK (max_reasoning_effort_multiplier IS NULL OR max_reasoning_effort_multiplier > 0);
