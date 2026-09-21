-- 账号成本改按 total_cost × 账号倍率：渠道统计价卡随渠道代码下线，供应商成本价目表落地时再加真正的成本列。
ALTER TABLE usage_logs DROP COLUMN account_stats_cost;
