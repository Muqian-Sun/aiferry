-- 去分组收尾：删分组表与各表的分组列（7c）。
--
-- 代码侧从 7a 到 7b-4 已经把分组读写全部删掉，这里只清库。分组行不做汇总合并——
-- 开发阶段只有演示数据，按「禁止数据回填迁移」直接删；生产上线前库是空的。
--
-- 顺序要求：
--   1. 先删分组行，否则第 3 步收窄主键 / 唯一索引时会撞重复（实测 ops_metrics_hourly
--      17 组、ops_metrics_daily 3 组、4 张 channel_monitor_v2 rollup 表 2–10 组，
--      每组恰好是「1 个平台行 + N 个分组行」）。
--   2. 先拆 usage_logs 上三个分组汇总触发器，再 DROP COLUMN。
--   3. 先 DROP COLUMN 再 DROP TABLE groups：api_keys / usage_logs /
--      content_moderation_logs / prompt_audit_events / prompt_audit_jobs 五张表都有
--      外键指向 groups，删列时外键会一起消失，groups 才删得掉（反过来会报
--      "cannot drop table groups because other objects depend on it"）。
--   4. DROP COLUMN 会连带删掉依赖该列的索引与主键约束，最后一步把非分组部分仍有用的
--      那些重建回来。
--   5. enqueue_api_key_auth_cache_invalidation 的函数体引用 OLD.group_id，必须改写——
--      plpgsql 里是运行时解析，DROP COLUMN 检测不到这个依赖。

-- ── 1) 删分组维度的行 ────────────────────────────────────────────────
DELETE FROM ops_metrics_hourly WHERE group_id IS NOT NULL;
DELETE FROM ops_metrics_daily  WHERE group_id IS NOT NULL;

DELETE FROM channel_monitor_v2_metrics_1m                WHERE group_id <> 0;
DELETE FROM channel_monitor_v2_metrics_rollup            WHERE group_id <> 0;
DELETE FROM channel_monitor_v2_error_metrics_1m          WHERE group_id <> 0;
DELETE FROM channel_monitor_v2_error_metrics_rollup      WHERE group_id <> 0;
DELETE FROM channel_monitor_v2_user_metrics_1m           WHERE group_id <> 0;
DELETE FROM channel_monitor_v2_user_metrics_rollup       WHERE group_id <> 0;
DELETE FROM channel_monitor_v2_latency_histograms_1m     WHERE group_id <> 0;
DELETE FROM channel_monitor_v2_latency_histograms_rollup WHERE group_id <> 0;

-- ── 2) 拆掉依赖分组列的触发器与函数 ──────────────────────────────────
-- usage_logs 上三个分组用量汇总失效触发器：汇总表本身在第 4 步删，触发器必须先拆，
-- 否则 DROP COLUMN group_id 会报 "other objects depend on it"。
DROP TRIGGER IF EXISTS usage_logs_group_rollup_invalidate_insert ON usage_logs;
DROP TRIGGER IF EXISTS usage_logs_group_rollup_invalidate_update ON usage_logs;
DROP TRIGGER IF EXISTS usage_logs_group_rollup_invalidate_delete ON usage_logs;
DROP FUNCTION IF EXISTS invalidate_group_usage_rollup_state();
DROP FUNCTION IF EXISTS invalidate_group_usage_rollup_state_after_insert();

-- groups / user_allowed_groups 上的认证缓存失效触发器随表删（第 4 步），
-- 但它们的函数要显式删——函数不随表消失。
DROP FUNCTION IF EXISTS enqueue_group_auth_cache_invalidation() CASCADE;
DROP FUNCTION IF EXISTS enqueue_allowed_group_auth_cache_invalidation() CASCADE;

-- api_keys 的认证缓存失效触发器要保留（key / status / user_id / IP 名单 / 过期都还要
-- 失效缓存），但函数体里的 OLD.group_id 必须去掉。plpgsql 的字段引用是运行时解析的，
-- DROP COLUMN 检测不到，不改的话删列后每一次 api_keys UPDATE 都会在触发器里报错。
CREATE OR REPLACE FUNCTION enqueue_api_key_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM enqueue_auth_cache_invalidation(OLD.key);
        RETURN OLD;
    END IF;

    IF OLD.key IS DISTINCT FROM NEW.key
       OR OLD.status IS DISTINCT FROM NEW.status
       OR OLD.deleted_at IS DISTINCT FROM NEW.deleted_at
       OR OLD.user_id IS DISTINCT FROM NEW.user_id
       OR OLD.ip_whitelist IS DISTINCT FROM NEW.ip_whitelist
       OR OLD.ip_blacklist IS DISTINCT FROM NEW.ip_blacklist
       OR OLD.expires_at IS DISTINCT FROM NEW.expires_at THEN
        PERFORM enqueue_auth_cache_invalidation(OLD.key);
        IF NEW.deleted_at IS NULL AND NEW.key IS DISTINCT FROM OLD.key THEN
            PERFORM enqueue_auth_cache_invalidation(NEW.key);
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

-- ── 3) 删各表的分组列（同时自动清掉指向 groups 的外键与分组索引）───────────────────────────────────────────────
ALTER TABLE api_keys   DROP COLUMN IF EXISTS group_id;
ALTER TABLE users      DROP COLUMN IF EXISTS restrict_public_groups;
ALTER TABLE scheduler_outbox DROP COLUMN IF EXISTS group_id;

-- usage_logs：分组列 + 三条随渠道代码下线的死列（Go 生产引用各 0 处）
ALTER TABLE usage_logs
  DROP COLUMN IF EXISTS group_id,
  DROP COLUMN IF EXISTS channel_id,
  DROP COLUMN IF EXISTS model_mapping_chain,
  DROP COLUMN IF EXISTS billing_tier;

ALTER TABLE ops_error_logs     DROP COLUMN IF EXISTS group_id;
ALTER TABLE ops_system_metrics DROP COLUMN IF EXISTS group_id;
ALTER TABLE ops_metrics_hourly DROP COLUMN IF EXISTS group_id;
ALTER TABLE ops_metrics_daily  DROP COLUMN IF EXISTS group_id;

ALTER TABLE content_moderation_logs DROP COLUMN IF EXISTS group_id, DROP COLUMN IF EXISTS group_name;
ALTER TABLE prompt_audit_events     DROP COLUMN IF EXISTS group_id, DROP COLUMN IF EXISTS group_name;
ALTER TABLE prompt_audit_jobs       DROP COLUMN IF EXISTS group_id, DROP COLUMN IF EXISTS group_name;

ALTER TABLE channel_monitor_v2_config DROP COLUMN IF EXISTS group_ids;
ALTER TABLE channel_account_stats_pricing_rules DROP COLUMN IF EXISTS group_ids;

ALTER TABLE channel_monitor_v2_metrics_1m                DROP COLUMN IF EXISTS group_id;
ALTER TABLE channel_monitor_v2_metrics_rollup            DROP COLUMN IF EXISTS group_id;
ALTER TABLE channel_monitor_v2_error_metrics_1m          DROP COLUMN IF EXISTS group_id;
ALTER TABLE channel_monitor_v2_error_metrics_rollup      DROP COLUMN IF EXISTS group_id;
ALTER TABLE channel_monitor_v2_user_metrics_1m           DROP COLUMN IF EXISTS group_id;
ALTER TABLE channel_monitor_v2_user_metrics_rollup       DROP COLUMN IF EXISTS group_id;
ALTER TABLE channel_monitor_v2_latency_histograms_1m     DROP COLUMN IF EXISTS group_id;
ALTER TABLE channel_monitor_v2_latency_histograms_rollup DROP COLUMN IF EXISTS group_id;

-- ── 4) 删分组表 ───────────────────────────────────────────
-- 到这一步 api_keys / usage_logs / content_moderation_logs / prompt_audit_events /
-- prompt_audit_jobs 指向 groups 的外键已随第 3 步的 DROP COLUMN 一起消失。
-- channels 那套其余 5 张表不是分组概念，留着。
DROP TABLE IF EXISTS channel_groups;
DROP TABLE IF EXISTS account_groups;
DROP TABLE IF EXISTS user_allowed_groups;
DROP TABLE IF EXISTS user_group_rate_multipliers;
DROP TABLE IF EXISTS orphan_allowed_groups_audit;
DROP TABLE IF EXISTS composite_model_routes;
DROP TABLE IF EXISTS usage_group_daily_rollups;
DROP TABLE IF EXISTS usage_group_rollup_state;
DROP TABLE IF EXISTS groups;

-- ── 5) 重建被连带删掉、非分组部分仍有用的索引与主键 ──────────────────
-- ops preagg 的唯一维度收窄成两列（写入侧的 ON CONFLICT 同步改）
CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_metrics_hourly_unique_dim
  ON ops_metrics_hourly (bucket_start, COALESCE(platform, ''));
CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_metrics_daily_unique_dim
  ON ops_metrics_daily (bucket_date, COALESCE(platform, ''));

-- 平台维度的部分索引：去掉 group_id IS NULL 条件
CREATE INDEX IF NOT EXISTS idx_ops_metrics_hourly_platform_bucket
  ON ops_metrics_hourly (platform, bucket_start DESC)
  WHERE platform IS NOT NULL AND platform <> '';
CREATE INDEX IF NOT EXISTS idx_ops_metrics_daily_platform_bucket
  ON ops_metrics_daily (platform, bucket_date DESC)
  WHERE platform IS NOT NULL AND platform <> '';
CREATE INDEX IF NOT EXISTS idx_ops_system_metrics_platform_time
  ON ops_system_metrics (platform, created_at DESC)
  WHERE platform IS NOT NULL AND platform <> '';

-- channel_monitor_v2 的 8 个主键去掉 group_id 后重建（UPSERT 按主键冲突）
ALTER TABLE channel_monitor_v2_metrics_1m
  ADD PRIMARY KEY (bucket_start, platform, model);
ALTER TABLE channel_monitor_v2_metrics_rollup
  ADD PRIMARY KEY (bucket_seconds, bucket_start, platform, model);
ALTER TABLE channel_monitor_v2_error_metrics_1m
  ADD PRIMARY KEY (bucket_start, platform, model, error_category, taxonomy_version);
ALTER TABLE channel_monitor_v2_error_metrics_rollup
  ADD PRIMARY KEY (bucket_seconds, bucket_start, platform, model, error_category, taxonomy_version);
ALTER TABLE channel_monitor_v2_user_metrics_1m
  ADD PRIMARY KEY (bucket_start, platform, model, user_id);
ALTER TABLE channel_monitor_v2_user_metrics_rollup
  ADD PRIMARY KEY (bucket_seconds, bucket_start, platform, model, user_id);
ALTER TABLE channel_monitor_v2_latency_histograms_1m
  ADD PRIMARY KEY (bucket_start, platform, model, user_id, metric, upper_bound_ms);
ALTER TABLE channel_monitor_v2_latency_histograms_rollup
  ADD PRIMARY KEY (bucket_seconds, bucket_start, platform, model, user_id, metric, upper_bound_ms);
