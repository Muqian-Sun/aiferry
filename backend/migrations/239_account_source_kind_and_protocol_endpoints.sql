-- 239_account_source_kind_and_protocol_endpoints.sql
--
-- 资源池拆分：给账号加两个新维度。
--   source_kind        账号来源：成品号（subscription）还是第三方 key（api_key）
--   protocol_endpoints 协议 → 上游地址 的映射，形如 {"anthropic_messages": "https://relay.example.com"}
--
-- 本次只加字段并回填存量数据，读取路径仍走 platform，切换在后续提交里做。
--
-- source_kind 刻意可空且不给默认值：若给默认值，本次之后、切换读取之前新建的
-- 账号会被静默标成成品号，属于没有任何报错的错误分类。留空表示「尚未分类」，
-- 在数据里看得见，切换读取时再回填并收紧为 NOT NULL。

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS source_kind VARCHAR(20),
    ADD COLUMN IF NOT EXISTS protocol_endpoints JSONB NOT NULL DEFAULT '{}'::jsonb;

-- 回填存量：oauth / setup-token / bedrock / service_account 是成品号；
-- apikey / upstream 是第三方 key（upstream 为历史类型，迁移 052 起不再新建）。
UPDATE accounts
SET source_kind = CASE
        WHEN type IN ('apikey', 'upstream') THEN 'api_key'
        ELSE 'subscription'
    END
WHERE source_kind IS NULL;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_accounts_source_kind') THEN
    ALTER TABLE accounts ADD CONSTRAINT chk_accounts_source_kind
      CHECK (source_kind IS NULL OR source_kind IN ('subscription', 'api_key')) NOT VALID;
  END IF;
END $$;

COMMENT ON COLUMN accounts.source_kind IS '账号来源：subscription=成品号，api_key=第三方 key，NULL=尚未分类';
COMMENT ON COLUMN accounts.protocol_endpoints IS '协议到上游地址的映射，键为协议标识，值为该协议的 base URL';
