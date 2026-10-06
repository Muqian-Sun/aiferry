-- 268_drop_model_catalog_aliases.sql
--
-- 模型目录只存官网模型 ID、不存别名（muqian 2026-10-06）：上游名字与目录不同的，在渠道承接行的
-- 上游模型名里配。目录别名表连同它的索引一起删；表里只有播种写的 xAI Imagine 别名，没有管理员手建的。

DROP TABLE IF EXISTS model_catalog_aliases;
