-- 251_drop_subscription_plan_currency.sql
--
-- 套餐一律按美元定价（muqian 2026-09-24「统一成美元」）：原 currency 列只是展示标签、不参与扣款，
-- 卡片按它显示币种、结算页按支付通道币种显示，两处对不上。代码已同分支移除，本迁移删列。
-- 177 加列的历史迁移保持不动：全新部署按原顺序重放，到这里再删除。

ALTER TABLE subscription_plans DROP COLUMN IF EXISTS currency;
