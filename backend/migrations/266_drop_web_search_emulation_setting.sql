-- 266_drop_web_search_emulation_setting.sql
--
-- 联网搜索模拟（Brave / Tavily 代搜）删了（方案页第三版「联网搜索」，2026-10-04）：
-- 服务商与 Key 的设置一起删。渠道 extra 里残留的 web_search_emulation 开关早就不读了，不动。

DELETE FROM settings WHERE key = 'web_search_emulation_config';
