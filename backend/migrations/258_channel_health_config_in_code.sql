-- 258_channel_health_config_in_code.sql
--
-- 渠道健康（用户站「服务状态」）的配置写进代码（muqian 2026-09-30），管理站的配置页与读数一起删：
--   1. channel_monitor_v2_config：汇总频率 / 健康阈值 / 忽略的错误类别改成代码常量
--      （service.channelMonitorV2Config），模型名单改按上架目录，平台名单不再有。
--   2. 按用户的汇总（channel_monitor_v2_user_metrics_*）只给已删的「用户排行」用，整表删。
--   3. 延迟分布每个用户各存一份（user_id > 0）也只给排行下钻用，只留全站那份（user_id = 0），
--      再去掉 user_id 列、主键收窄（同迁移 252 删 group_id 的做法：先删行再删列，否则收窄主键撞重复）。
--   4. 「隐藏吞吐 / 隐藏排行」两个设置随读数一起删。
--
-- 开发阶段没有需要保留的数据，不回填。

DROP TABLE IF EXISTS channel_monitor_v2_config;

DROP TABLE IF EXISTS channel_monitor_v2_user_metrics_1m;
DROP TABLE IF EXISTS channel_monitor_v2_user_metrics_rollup;

DELETE FROM channel_monitor_v2_latency_histograms_1m     WHERE user_id <> 0;
DELETE FROM channel_monitor_v2_latency_histograms_rollup WHERE user_id <> 0;
-- DROP COLUMN 连带删掉含该列的主键
ALTER TABLE channel_monitor_v2_latency_histograms_1m     DROP COLUMN IF EXISTS user_id;
ALTER TABLE channel_monitor_v2_latency_histograms_rollup DROP COLUMN IF EXISTS user_id;
ALTER TABLE channel_monitor_v2_latency_histograms_1m
  ADD PRIMARY KEY (bucket_start, platform, model, metric, upper_bound_ms);
ALTER TABLE channel_monitor_v2_latency_histograms_rollup
  ADD PRIMARY KEY (bucket_seconds, bucket_start, platform, model, metric, upper_bound_ms);

DELETE FROM settings WHERE key IN ('channel_monitor_hide_throughput', 'channel_monitor_hide_user_ranking');
