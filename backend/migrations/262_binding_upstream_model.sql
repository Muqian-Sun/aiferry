-- 262_binding_upstream_model.sql
--
-- 模型改名并进承接关系（方案页 X8eQjqzjAEx3hF4xzCzKZr 第三版 D4；muqian 2026-10-01 定）：
-- 用户只能请求目录里的模型标识，转发时只做一次「目录标识 → 这个渠道的上游模型名」的转换。
--
-- 1. 承接关系加 upstream_model：这个渠道给这个模型用的上游模型名，空串 = 与目录标识同名。
--    按目录模型精确对应，不支持通配。
-- 2. 渠道上管理员配的「模型改名」（credentials.model_mapping 与 model_mapping_rename_only）作废，
--    对所有渠道类型删掉。spark 影子号（parent_account_id 非空）的 model_mapping 是系统维护的
--    「只接 spark」模型列表，不是改名，保留。

ALTER TABLE model_catalog_bindings
    ADD COLUMN upstream_model VARCHAR(255) NOT NULL DEFAULT '';

UPDATE accounts
SET credentials = credentials - 'model_mapping' - 'model_mapping_rename_only'
WHERE parent_account_id IS NULL
  AND (credentials ? 'model_mapping' OR credentials ? 'model_mapping_rename_only');
