-- 255_drop_plugins.sql
--
-- 插件系统（管理员上传的本地进程插件，229 / 230 建表）已整块下线，删掉两张插件表。
-- sub2api_plugin_bindings.plugin_id 引用 sub2api_plugin_installations(id)，先删引用方。
-- 开发阶段没有需要保留的插件数据，不做备份或回填。

DROP TABLE IF EXISTS sub2api_plugin_bindings;
DROP TABLE IF EXISTS sub2api_plugin_installations;
