-- 273_scheduling_traces.sql
--
-- 请求级调度轨迹（管理站「调度 · 请求轨迹」，2026-10-18）：每个走到调度器的对话请求一行，
-- 记录每次选渠道考虑过哪些候选、各自为什么被跳过、选中了谁，以及每次上游尝试的结果。
-- 不存请求内容。保留 7 天，由写入服务每小时清理。

CREATE TABLE IF NOT EXISTS scheduling_traces (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    request_id VARCHAR(128) NOT NULL DEFAULT '',
    user_id BIGINT,
    api_key_id BIGINT,
    -- 客户端写的模型名；catalog_entry_id 是它命中的目录条目（同调度 scope）
    model VARCHAR(255) NOT NULL DEFAULT '',
    catalog_entry_id BIGINT,
    inbound_protocol VARCHAR(32) NOT NULL DEFAULT '',
    -- success / failed / no_channel / canceled
    outcome VARCHAR(16) NOT NULL,
    status_code INTEGER NOT NULL DEFAULT 0,
    -- 最后一次选中的渠道（成功时就是接下请求的渠道）
    final_account_id BIGINT,
    -- 选渠道的次数减一 = 换渠道次数
    switch_count INTEGER NOT NULL DEFAULT 0,
    sticky_hit BOOLEAN NOT NULL DEFAULT FALSE,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    picks JSONB NOT NULL DEFAULT '[]'::jsonb,
    attempts JSONB NOT NULL DEFAULT '[]'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_scheduling_traces_created_at
    ON scheduling_traces (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_scheduling_traces_entry_created_at
    ON scheduling_traces (catalog_entry_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_scheduling_traces_account_created_at
    ON scheduling_traces (final_account_id, created_at DESC);
