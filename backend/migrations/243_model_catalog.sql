-- 模型目录：平台上「有哪些模型」的权威表。
-- 本次只建表；价格解析在 model_pricing_resolver 中改用这些表，渠道侧的
-- channel_model_pricing / channel_pricing_intervals 不动（仍供渠道展示页使用）。

CREATE TABLE IF NOT EXISTS model_catalog_entries (
    id BIGSERIAL PRIMARY KEY,
    model_id VARCHAR(200) NOT NULL,
    display_name VARCHAR(200) NOT NULL DEFAULT '',
    vendor VARCHAR(50) NOT NULL DEFAULT '',
    protocols JSONB NOT NULL DEFAULT '[]'::jsonb,
    billing_mode VARCHAR(20) NOT NULL DEFAULT 'token',
    status VARCHAR(20) NOT NULL DEFAULT 'listed',
    managed_by VARCHAR(20) NOT NULL DEFAULT 'seed',

    input_price NUMERIC(20,12),
    output_price NUMERIC(20,12),
    cache_write_price NUMERIC(20,12),
    cache_write_1h_price NUMERIC(20,12),
    cache_read_price NUMERIC(20,12),
    image_input_price NUMERIC(20,12),
    image_output_price NUMERIC(20,12),
    image_cache_read_price NUMERIC(20,12),

    input_price_priority NUMERIC(20,12),
    output_price_priority NUMERIC(20,12),
    cache_write_price_priority NUMERIC(20,12),
    cache_read_price_priority NUMERIC(20,12),

    per_request_price NUMERIC(20,12),

    long_context_input_threshold INTEGER,
    long_context_threshold_inclusive BOOLEAN NOT NULL DEFAULT FALSE,
    long_context_input_multiplier NUMERIC(10,4),
    long_context_output_multiplier NUMERIC(10,4),

    fast_multiplier NUMERIC(10,4),
    flex_multiplier NUMERIC(10,4),
    max_reasoning_effort_multiplier NUMERIC(10,4),

    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT model_catalog_entries_billing_mode_check
        CHECK (billing_mode IN ('token', 'per_request', 'image', 'video')),
    CONSTRAINT model_catalog_entries_status_check
        CHECK (status IN ('listed', 'unlisted')),
    CONSTRAINT model_catalog_entries_managed_by_check
        CHECK (managed_by IN ('seed', 'admin'))
);

-- 模型标识大小写不敏感唯一：计费查表前一律 lower()，两条只差大小写的记录会让
-- 「命中哪条」取决于行序。
CREATE UNIQUE INDEX IF NOT EXISTS idx_model_catalog_entries_model_id_lower
    ON model_catalog_entries (lower(model_id));

CREATE INDEX IF NOT EXISTS idx_model_catalog_entries_vendor
    ON model_catalog_entries (vendor);
CREATE INDEX IF NOT EXISTS idx_model_catalog_entries_status
    ON model_catalog_entries (status);
CREATE INDEX IF NOT EXISTS idx_model_catalog_entries_managed_by
    ON model_catalog_entries (managed_by);

CREATE TABLE IF NOT EXISTS model_catalog_aliases (
    id BIGSERIAL PRIMARY KEY,
    alias VARCHAR(200) NOT NULL,
    entry_id BIGINT NOT NULL REFERENCES model_catalog_entries(id) ON DELETE CASCADE,
    source VARCHAR(30) NOT NULL DEFAULT 'manual',
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT model_catalog_aliases_source_check
        CHECK (source IN ('manual', 'seed'))
);

-- 别名全局唯一（大小写不敏感）：同一个别名指向两个模型在库层就不可能发生。
CREATE UNIQUE INDEX IF NOT EXISTS idx_model_catalog_aliases_alias_lower
    ON model_catalog_aliases (lower(alias));

CREATE INDEX IF NOT EXISTS idx_model_catalog_aliases_entry
    ON model_catalog_aliases (entry_id);
CREATE INDEX IF NOT EXISTS idx_model_catalog_aliases_source
    ON model_catalog_aliases (source);

CREATE TABLE IF NOT EXISTS model_catalog_price_intervals (
    id BIGSERIAL PRIMARY KEY,
    entry_id BIGINT NOT NULL REFERENCES model_catalog_entries(id) ON DELETE CASCADE,
    min_tokens INTEGER NOT NULL DEFAULT 0,
    max_tokens INTEGER,
    tier_label VARCHAR(50) NOT NULL DEFAULT '',

    input_price NUMERIC(20,12),
    output_price NUMERIC(20,12),
    cache_write_price NUMERIC(20,12),
    cache_write_1h_price NUMERIC(20,12),
    cache_read_price NUMERIC(20,12),
    per_request_price NUMERIC(20,12),

    input_multiplier NUMERIC(10,4),
    output_multiplier NUMERIC(10,4),
    cache_write_multiplier NUMERIC(10,4),
    cache_read_multiplier NUMERIC(10,4),

    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT model_catalog_price_intervals_min_check CHECK (min_tokens >= 0),
    CONSTRAINT model_catalog_price_intervals_max_check
        CHECK (max_tokens IS NULL OR max_tokens > min_tokens)
);

CREATE INDEX IF NOT EXISTS idx_model_catalog_price_intervals_entry
    ON model_catalog_price_intervals (entry_id, sort_order, id);

CREATE TABLE IF NOT EXISTS model_catalog_time_pricing (
    id BIGSERIAL PRIMARY KEY,
    entry_id BIGINT NOT NULL REFERENCES model_catalog_entries(id) ON DELETE CASCADE,
    timezone VARCHAR(64) NOT NULL DEFAULT '',
    weekdays_only BOOLEAN NOT NULL DEFAULT FALSE,
    periods JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 一个条目至多一份分时配置。
CREATE UNIQUE INDEX IF NOT EXISTS idx_model_catalog_time_pricing_entry
    ON model_catalog_time_pricing (entry_id);
