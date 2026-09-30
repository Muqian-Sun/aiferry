-- 260_drop_fast_flex_pricing.sql
--
-- 删掉 Fast / Flex 档（OpenAI service_tier = priority / flex、Anthropic speed = fast）的计价
-- （muqian 2026-09-30「fast 可以删掉了，我看上游都没有这个东西」，Flex 同一套机制一起删）：
-- 模型目录上的 Fast 档价与 Fast / Flex 倍率列一并删除，带不带档位一律按标准价计费。
-- 实测 fenno、qiniu 收到 service_tier=priority 都按 default 处理。

ALTER TABLE model_catalog_entries
    DROP COLUMN IF EXISTS input_price_priority,
    DROP COLUMN IF EXISTS output_price_priority,
    DROP COLUMN IF EXISTS cache_write_price_priority,
    DROP COLUMN IF EXISTS cache_read_price_priority,
    DROP COLUMN IF EXISTS fast_multiplier,
    DROP COLUMN IF EXISTS flex_multiplier;
