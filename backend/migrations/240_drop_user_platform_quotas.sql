-- 240_drop_user_platform_quotas.sql
--
-- 「用户 × 平台配额」功能已整体删除：用户侧不再有平台这个维度，按平台限制
-- 单个用户消费失去落点。代码在同一分支上先行移除，本迁移清理数据。
--
-- 142 建表、157/224/237/238 改表的历史迁移保持不动：全新部署仍按原顺序重放，
-- 到这里再删除，历史链条保持完整。

DROP TABLE IF EXISTS user_platform_quotas;

-- 配套的系统设置：全局默认限额，以及按登录来源覆盖的七个键。
DELETE FROM settings
WHERE key = 'default_platform_quotas'
   OR key LIKE 'auth_source_default_%_platform_quotas';
