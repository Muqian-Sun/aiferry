-- 274_model_catalog_binding_per_request_price.sql
--
-- 生图 / 生视频 / 按次的模型上价格页（PR-A）：承接关系加上游按次价（USD / 张、/ 秒、/ 次，与目录条目的
-- per_request_price 同单位）。渠道成本 = 媒体用量 × 它，分档按官方价各档同比例折算（见 upstreamMediaEntry）。
-- NULL = 没填（按 Token 计费的模型不用它）；0 允许：成品号的上游价今天就记 0。

ALTER TABLE model_catalog_bindings
    ADD COLUMN IF NOT EXISTS per_request_price DECIMAL(20, 12);

ALTER TABLE model_catalog_bindings
    DROP CONSTRAINT IF EXISTS chk_model_catalog_bindings_per_request_price_non_negative;
ALTER TABLE model_catalog_bindings
    ADD CONSTRAINT chk_model_catalog_bindings_per_request_price_non_negative
    CHECK (per_request_price IS NULL OR per_request_price >= 0);
