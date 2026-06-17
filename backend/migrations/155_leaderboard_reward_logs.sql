-- 排行榜激励发放记录：后台定时任务每日按昨日消费排行榜发放余额奖励的明细。
-- (reward_date, user_id) 唯一：既是发放明细，也用于幂等防重复发放（同一天重跑不会重复发放）。

CREATE TABLE IF NOT EXISTS leaderboard_reward_logs (
    id                BIGSERIAL PRIMARY KEY,
    reward_date       DATE NOT NULL,                                  -- 结算的消费日期（即“昨天”）
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rank              INT NOT NULL DEFAULT 0,                         -- 当日排行榜名次（1 开始）
    reward_amount     DECIMAL(20,8) NOT NULL DEFAULT 0,              -- 实际发放到余额的金额
    pool_amount       DECIMAL(20,8) NOT NULL DEFAULT 0,              -- 当日奖池总额（昨日总消费 × 比例）
    total_cost        DECIMAL(20,8) NOT NULL DEFAULT 0,              -- 当日全站总消费（actual_cost）
    distribution_mode VARCHAR(20) NOT NULL DEFAULT 'average',        -- 分配模式 average/proportional/weighted
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_leaderboard_reward_logs_date_user UNIQUE (reward_date, user_id)
);

CREATE INDEX IF NOT EXISTS idx_leaderboard_reward_logs_reward_date ON leaderboard_reward_logs(reward_date);
CREATE INDEX IF NOT EXISTS idx_leaderboard_reward_logs_user_id ON leaderboard_reward_logs(user_id);

COMMENT ON TABLE leaderboard_reward_logs IS '排行榜激励每日发放记录（兼幂等去重）';
