-- Migration: 152_channel_monitor_active_window
-- 渠道监控新增「检测时间窗口」配置（按服务器本地时区判断）：
-- 仅在窗口内才触发检测，窗口外跳过本轮（不取消任务）。
-- 结构：{"enabled":bool,"start":"HH:MM","end":"HH:MM","weekdays":[0..6]}
--   - enabled=false（默认）表示 7×24 全天检测，与历史行为完全一致。
--   - start==end 表示当天整天；start>end 表示跨午夜；weekdays 空表示每天。

ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS active_window JSONB NOT NULL DEFAULT '{"enabled":false}'::jsonb;
