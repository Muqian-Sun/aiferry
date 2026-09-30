-- 256_drop_long_context_pricing.sql
--
-- 阶梯定价只留按 token 分段一套（muqian 2026-09-29：「我们现在只做按token分段计价」，长上下文「合并」进分段）。
-- 长上下文（阈值 × 输入 / 输出倍数）是只有两段、只能按倍数加价的分段；目录改为只认 model_catalog_price_intervals，
-- 价格文件里的长上下文阶梯在播种 / 导入时换算成分段写进去。
--
-- - model_catalog_entries 的四个长上下文列删掉；
-- - usage_logs.long_context_billing_applied（「按长上下文价计费」标记）随之删掉；
-- - 175 装在 accounts 上的 openai_long_context_billing_enabled 维护触发器早已没有代码读这个键，一并删掉。
--
-- 开发阶段没有需要保留的数据，不做回填：已有条目的长上下文阶梯需要时在模型页按分段重新填。

ALTER TABLE model_catalog_entries
    DROP COLUMN IF EXISTS long_context_input_threshold,
    DROP COLUMN IF EXISTS long_context_threshold_inclusive,
    DROP COLUMN IF EXISTS long_context_input_multiplier,
    DROP COLUMN IF EXISTS long_context_output_multiplier;

ALTER TABLE usage_logs
    DROP COLUMN IF EXISTS long_context_billing_applied;

DROP TRIGGER IF EXISTS accounts_propagate_openai_long_context_billing_extra ON accounts;
DROP TRIGGER IF EXISTS accounts_enforce_openai_long_context_billing_extra ON accounts;
DROP FUNCTION IF EXISTS public.propagate_openai_long_context_billing_extra_to_shadows();
DROP FUNCTION IF EXISTS public.enforce_openai_long_context_billing_extra();
