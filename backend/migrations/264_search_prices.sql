-- 264_search_prices.sql
--
-- 价格页加联网搜索价（方案页 X8eQjqzjAEx3hF4xzCzKZr 第三版「联网搜索」；muqian 2026-10-03 定按模型目录收）：
-- 1. 官方价：每次 web 搜索价已有（search_price_per_call），加 xAI X 搜索按取回条目收的每条帖子价、每个主页价。
-- 2. 上游价：承接关系加同样三项。官方价显式设了的项上游价必须填（服务层校验）；没填的按官方搜索价记成本。

ALTER TABLE model_catalog_entries
    ADD COLUMN x_post_price NUMERIC(20, 12),
    ADD COLUMN x_user_price NUMERIC(20, 12);

ALTER TABLE model_catalog_bindings
    ADD COLUMN search_price_per_call DECIMAL(20, 12),
    ADD COLUMN x_post_price          DECIMAL(20, 12),
    ADD COLUMN x_user_price          DECIMAL(20, 12),
    ADD CONSTRAINT model_catalog_bindings_search_prices_non_negative CHECK (
        (search_price_per_call IS NULL OR search_price_per_call >= 0)
        AND (x_post_price IS NULL OR x_post_price >= 0)
        AND (x_user_price IS NULL OR x_user_price >= 0)
    );
