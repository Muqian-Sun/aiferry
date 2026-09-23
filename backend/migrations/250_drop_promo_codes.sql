-- 250_drop_promo_codes.sql
--
-- 注册优惠码功能已整体删除（muqian 2026-09-23「删掉」）：注册页输入框、管理端页面、
-- 公开校验接口、注册 / OAuth 发放逻辑都已在同一分支移除，本迁移删表。
-- 配套的 settings 键 promo_code_enabled 不在这里删：这个仓库的编号 SQL 只建表 / 删表，
-- 不做数据操作；读取路径已删除，残留键无人读取。已发放到余额里的赠送金额不受影响。
--
-- 033 建表的历史迁移保持不动：全新部署仍按原顺序重放，到这里再删除。

DROP TABLE IF EXISTS promo_code_usages;
DROP TABLE IF EXISTS promo_codes;
