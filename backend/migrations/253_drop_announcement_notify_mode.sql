-- 253_drop_announcement_notify_mode.sql
--
-- 公告「通知方式」（silent 仅铃铛 / popup 逐条弹窗）作废（muqian 2026-09-24「通知方式删掉」）：
-- 用户站改为登录后弹一个公告窗、列出全部公告，用户可选「今日不再弹出」，不再按条区分弹不弹。
-- 代码已同分支移除，本迁移删列。068 加列的历史迁移保持不动：全新部署按原顺序重放，到这里再删除。

ALTER TABLE announcements DROP COLUMN IF EXISTS notify_mode;
