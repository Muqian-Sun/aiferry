-- 275_video_tasks.sql
--
-- 视频生成任务（2026-10-18）：网关对外的任务 ID（vtask_ 前缀）→ 承接账号与上游任务 ID 的映射，
-- 以及规范化请求、最近一次查询到的状态与计费标记。状态 / 内容查询按 (id, api_key_id) 找回任务，
-- 首次看到成功时用 billed_at 抢占计费（只扣一次）。不存视频文件。保留 7 天，由服务每小时清理。

CREATE TABLE IF NOT EXISTS video_tasks (
    id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    catalog_entry_id BIGINT NOT NULL DEFAULT 0,
    -- 目录模型标识（计费用）；upstream_model 是发给上游的名字
    model VARCHAR(255) NOT NULL DEFAULT '',
    upstream_model VARCHAR(255) NOT NULL DEFAULT '',
    -- grok_oauth / xai_relay / newapi_relay …
    upstream_kind VARCHAR(32) NOT NULL,
    upstream_task_id VARCHAR(255) NOT NULL,
    -- 创建时的客户端形态：openai / xai / newapi
    inbound_shape VARCHAR(16) NOT NULL,
    request JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- queued / running / succeeded / failed / canceled
    state VARCHAR(16) NOT NULL DEFAULT 'queued',
    progress INTEGER,
    video_url TEXT NOT NULL DEFAULT '',
    error_code VARCHAR(128) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    duration_seconds DOUBLE PRECISION NOT NULL DEFAULT 0,
    resolution VARCHAR(16) NOT NULL DEFAULT '',
    billed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_video_tasks_api_key_created_at
    ON video_tasks (api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_video_tasks_state_updated_at
    ON video_tasks (state, updated_at);
CREATE INDEX IF NOT EXISTS idx_video_tasks_created_at
    ON video_tasks (created_at);
