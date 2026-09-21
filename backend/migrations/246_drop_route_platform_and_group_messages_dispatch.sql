-- 目录条目不再有「网关族」：调度按协议，扩展端点分发与厂商特有处理按条目 vendor。
ALTER TABLE model_catalog_entries DROP COLUMN route_platform;
-- 分组「Messages 调度」配置只喂 count_tokens 里两个在目录路由下恒短路的函数，整条链删。
ALTER TABLE groups DROP COLUMN allow_messages_dispatch;
ALTER TABLE groups DROP COLUMN default_mapped_model;
ALTER TABLE groups DROP COLUMN messages_dispatch_model_config;
