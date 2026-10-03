-- 263_usage_web_search.sql
--
-- 联网搜索按次计费（方案页 X8eQjqzjAEx3hF4xzCzKZr 第三版「联网搜索」；muqian 2026-09-30 定）：
-- 官方云端搜索工具（Anthropic web_search、OpenAI web_search、xAI web_search / x_search）的搜索费
-- 一律按上游返回内容里的次数、以官方原价收，不乘用户倍率。用量逐行记次数和搜索费；
-- 搜索费已含在 total_cost / actual_cost 里，这两列用来显示和对账。历史用量记 0（全是测试数据）。

ALTER TABLE usage_logs
    ADD COLUMN web_search_count INT NOT NULL DEFAULT 0,
    ADD COLUMN web_search_cost  DECIMAL(20, 10) NOT NULL DEFAULT 0;
