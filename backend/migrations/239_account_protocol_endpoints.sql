-- 239_account_protocol_endpoints.sql
--
-- 资源池拆分：给账号加「协议 → 上游地址」映射，
-- 形如 {"anthropic_messages": "https://relay.example.com"}。
-- 第三方 key 的上游地址只从这里取；成品号只走厂商官方地址，不用这一列。
--
-- 账号来源（成品号 / 第三方 key）由 type 决定（apikey 即第三方 key），不另设列。

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS protocol_endpoints JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN accounts.protocol_endpoints IS '协议到上游地址的映射，键为协议标识，值为该协议的 base URL';
