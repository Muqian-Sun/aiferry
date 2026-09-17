-- 242_merge_legacy_upstream_accounts_into_apikey.sql
--
-- 历史账号类型 upstream 已从代码中移除：没有任何转发、取令牌或建号路径再认领它。
--
-- 052 只转换了 platform = 'antigravity' 且未软删除的行；而在本次移除之前，管理端的
-- 建号 / 改号 / 批量建号接口与数据导入仍接受 type = 'upstream'，所以 052 之后库里
-- 仍可能出现这类行。这里按 052 的口径把它们全部并入 apikey，不再限定平台与软删除
-- 状态，保证表里不再存在 upstream 类型。
--
-- source_kind 一并写成 api_key：239 回填时 upstream 本就归为第三方 key，这里只是让它
-- 与 DeriveAccountSourceKind('apikey') 的结果显式对齐。
--
-- 只改 type / source_kind 两列，accounts 上的触发器（UPDATE OF platform, extra, ...）
-- 不会被触发。可重复执行：第二次执行时 WHERE 不再命中任何行。

UPDATE accounts
SET type = 'apikey',
    source_kind = 'api_key'
WHERE type = 'upstream';
