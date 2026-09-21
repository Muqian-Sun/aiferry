-- 目录条目：模型内置搜索每次调用价（alpha search 用；未配则用内置单价 0.01）。
ALTER TABLE model_catalog_entries ADD COLUMN search_price_per_call NUMERIC(20,12);

-- 媒体单价与媒体倍率从分组消失：图片 / 视频价在目录条目，音频 / 搜索是内置常量，倍率只剩用户倍率。
ALTER TABLE groups
    DROP COLUMN image_price_1k,
    DROP COLUMN image_price_2k,
    DROP COLUMN image_price_4k,
    DROP COLUMN video_price_480p,
    DROP COLUMN video_price_720p,
    DROP COLUMN video_price_1080p,
    DROP COLUMN video_model_prices,
    DROP COLUMN web_search_price_per_call,
    DROP COLUMN search_price_per_1k,
    DROP COLUMN audio_realtime_price_per_min,
    DROP COLUMN audio_tts_price_per_million_chars,
    DROP COLUMN audio_stt_price_per_hour,
    DROP COLUMN image_rate_independent,
    DROP COLUMN image_rate_multiplier,
    DROP COLUMN video_rate_independent,
    DROP COLUMN video_rate_multiplier;
DROP TABLE IF EXISTS groups_video_price_backup_220;
