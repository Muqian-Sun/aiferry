-- 259_channel_monitor_v2_account_dimension.sql
--
-- 管理站「渠道状态」页（muqian 2026-09-30）要按渠道看可用率 / 首字延迟 / 缓存命中率：
-- 渠道健康的 6 张汇总表加渠道维度 account_id（0 = 没选到渠道就失败的请求），主键带上它。
-- 用户站服务状态按模型读，读数照样把各渠道加总，不受影响。
--
-- 这些表全是从 usage_logs / ops_error_logs 汇总出来的派生数据：清空并删掉聚合游标，
-- 聚合任务会按首次启用的流程从原始日志重新补历史（最近 30 天），不在迁移里回填。

TRUNCATE channel_monitor_v2_metrics_1m, channel_monitor_v2_metrics_rollup,
         channel_monitor_v2_latency_histograms_1m, channel_monitor_v2_latency_histograms_rollup,
         channel_monitor_v2_error_metrics_1m, channel_monitor_v2_error_metrics_rollup;
DELETE FROM channel_monitor_v2_watermarks;

ALTER TABLE channel_monitor_v2_metrics_1m                ADD COLUMN account_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE channel_monitor_v2_metrics_rollup            ADD COLUMN account_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE channel_monitor_v2_latency_histograms_1m     ADD COLUMN account_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE channel_monitor_v2_latency_histograms_rollup ADD COLUMN account_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE channel_monitor_v2_error_metrics_1m          ADD COLUMN account_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE channel_monitor_v2_error_metrics_rollup      ADD COLUMN account_id BIGINT NOT NULL DEFAULT 0;

ALTER TABLE channel_monitor_v2_metrics_1m DROP CONSTRAINT channel_monitor_v2_metrics_1m_pkey,
  ADD PRIMARY KEY (bucket_start, platform, model, account_id);
ALTER TABLE channel_monitor_v2_metrics_rollup DROP CONSTRAINT channel_monitor_v2_metrics_rollup_pkey,
  ADD PRIMARY KEY (bucket_seconds, bucket_start, platform, model, account_id);
ALTER TABLE channel_monitor_v2_latency_histograms_1m DROP CONSTRAINT channel_monitor_v2_latency_histograms_1m_pkey,
  ADD PRIMARY KEY (bucket_start, platform, model, account_id, metric, upper_bound_ms);
ALTER TABLE channel_monitor_v2_latency_histograms_rollup DROP CONSTRAINT channel_monitor_v2_latency_histograms_rollup_pkey,
  ADD PRIMARY KEY (bucket_seconds, bucket_start, platform, model, account_id, metric, upper_bound_ms);
ALTER TABLE channel_monitor_v2_error_metrics_1m DROP CONSTRAINT channel_monitor_v2_error_metrics_1m_pkey,
  ADD PRIMARY KEY (bucket_start, platform, model, account_id, error_category, taxonomy_version);
ALTER TABLE channel_monitor_v2_error_metrics_rollup DROP CONSTRAINT channel_monitor_v2_error_metrics_rollup_pkey,
  ADD PRIMARY KEY (bucket_seconds, bucket_start, platform, model, account_id, error_category, taxonomy_version);
