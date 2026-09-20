-- 目录条目绑定资源（账号）；条目的路由平台可显式指定。
ALTER TABLE model_catalog_entries
    ADD COLUMN IF NOT EXISTS route_platform VARCHAR(20) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS model_catalog_bindings (
    entry_id BIGINT NOT NULL REFERENCES model_catalog_entries(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    priority INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (entry_id, account_id)
);
CREATE INDEX IF NOT EXISTS idx_model_catalog_bindings_account ON model_catalog_bindings (account_id);
