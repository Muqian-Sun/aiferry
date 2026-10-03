-- 265_usage_web_search_delegated.sql
--
-- Claude Code 配第三方模型时，那次单独的搜索请求交给 claude-haiku-4-5 执行（方案页 X8eQjqzjAEx3hF4xzCzKZr
-- 第三版；muqian 2026-10-03 定）。用量行标上这一笔是代执行的搜索：用户站只显示客户端请求的模型和「联网搜索」，
-- 管理站照常看发往上游的 Haiku。历史用量记 false（全是测试数据）。

ALTER TABLE usage_logs
    ADD COLUMN web_search_delegated BOOLEAN NOT NULL DEFAULT false;
