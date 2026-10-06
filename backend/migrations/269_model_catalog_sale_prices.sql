-- 269_model_catalog_sale_prices.sql
--
-- 售价每项单独填（muqian 2026-10-06）：目录条目加一列售价，与官方价并列。整份 JSONB（基础价五项 + 各段售价，
-- 形状见 domain.CatalogSalePrices）；没填的项按官方价 × 默认售价比例收，所以新列默认 '{}' 即「一项都没单独定」。

ALTER TABLE model_catalog_entries
    ADD COLUMN IF NOT EXISTS sale_prices JSONB NOT NULL DEFAULT '{}'::jsonb;
