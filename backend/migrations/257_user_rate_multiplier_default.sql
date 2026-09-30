-- 257_user_rate_multiplier_default.sql
--
-- 用户倍率直接相对官方价：实付 = 目录官方价 × 用户倍率（muqian 2026-09-30：「绝大多数用户和未登录用户都是一个倍率，
-- 只有极少数用户可能有不同的倍率」）。users.rate_multiplier 改为可空：NULL = 跟全站默认倍率（代码常量
-- NewUserRateMultiplier，官方价的 1/15），只有单独设过的少数用户存值。
--
-- 开发阶段没有需要保留的数据，不回填：已有用户的倍率仍是旧口径的值，需要时在后台改回默认。

ALTER TABLE users
    ALTER COLUMN rate_multiplier DROP NOT NULL,
    ALTER COLUMN rate_multiplier DROP DEFAULT;
