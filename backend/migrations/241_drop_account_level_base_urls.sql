-- 241_drop_account_level_base_urls.sql
--
-- 账号级上游地址已从代码中整体移除：
--   * 成品号（OAuth / Setup Token / Bedrock / 服务账号）只走厂商官方地址，
--     credentials.base_url（Grok、Gemini）与 extra.custom_base_url（Anthropic 中继）
--     不再被读取；要走中转一律按第三方 key 建号。
--   * 第三方 key 的地址只在 protocol_endpoints 里，credentials.base_url /
--     api_base_urls 不再被读取。
-- 本迁移清理这些残留字段，避免界面或排查时把它们误当成生效配置。
--
-- 第三方 key 只在已配置协议映射时清理：映射为空的行本就无法转发，保留旧字段
-- 便于排查是哪条数据没补映射。成品号与第三方 key 的划分与 239 回填口径一致。

UPDATE accounts
SET credentials = credentials - 'base_url' - 'api_base_urls'
WHERE (credentials ? 'base_url' OR credentials ? 'api_base_urls')
  AND (
    type NOT IN ('apikey', 'upstream')
    OR protocol_endpoints <> '{}'::jsonb
  );

UPDATE accounts
SET extra = extra - 'custom_base_url' - 'custom_base_url_enabled'
WHERE extra ? 'custom_base_url' OR extra ? 'custom_base_url_enabled';
