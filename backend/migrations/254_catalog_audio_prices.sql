-- 254_catalog_audio_prices.sql
--
-- 目录条目：音频输入 / 输出 token 价（USD per token，与其它价格列同精度）。
-- 上游把音频 token 计在输入 / 输出总数里（OpenAI prompt/completion_tokens_details.audio_tokens，
-- Gemini usageMetadata 的 AUDIO 模态），计费时从文本 token 中剥出来按音频价计；
-- 空值表示未配置，音频 token 回退到文本输入 / 输出价（与迁移前的计费口径一致）。
-- 播种器从价格文件的 input_cost_per_audio_token / output_cost_per_audio_token 填充。

ALTER TABLE model_catalog_entries
    ADD COLUMN IF NOT EXISTS audio_input_price NUMERIC(20,12),
    ADD COLUMN IF NOT EXISTS audio_output_price NUMERIC(20,12);
