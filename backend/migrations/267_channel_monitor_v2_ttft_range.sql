-- 267_channel_monitor_v2_ttft_range.sql
--
-- 服务状态 / 渠道状态的首字延迟分位数原来直接取延迟分布的档位上沿（2026-10-04 走查：只有一个请求、
-- 实际 146 秒，页面显示「300 s」）。读数改成档内线性插值，并用实测的最小 / 最大首字延迟收紧首末两档：
-- 只有一个样本时结果就是它本身。汇总表加这两列（没有首字延迟样本时为空）。
--
-- 与 259 一样，这些表全是从 usage_logs / ops_error_logs 汇总出来的派生数据：清空并删掉聚合游标，
-- 聚合任务按首次启用的流程从原始日志重新补历史，不在迁移里回填。

TRUNCATE channel_monitor_v2_metrics_1m, channel_monitor_v2_metrics_rollup,
         channel_monitor_v2_latency_histograms_1m, channel_monitor_v2_latency_histograms_rollup,
         channel_monitor_v2_error_metrics_1m, channel_monitor_v2_error_metrics_rollup;
DELETE FROM channel_monitor_v2_watermarks;

ALTER TABLE channel_monitor_v2_metrics_1m
  ADD COLUMN IF NOT EXISTS ttft_min_ms INTEGER,
  ADD COLUMN IF NOT EXISTS ttft_max_ms INTEGER;
ALTER TABLE channel_monitor_v2_metrics_rollup
  ADD COLUMN IF NOT EXISTS ttft_min_ms INTEGER,
  ADD COLUMN IF NOT EXISTS ttft_max_ms INTEGER;
