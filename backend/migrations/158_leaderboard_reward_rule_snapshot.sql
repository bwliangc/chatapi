-- 保存排行榜奖励结算时的规则快照，避免昨日榜被后续配置变更影响。

ALTER TABLE leaderboard_reward_logs
    ADD COLUMN IF NOT EXISTS pool_rate DECIMAL(10,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS top_n INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS min_spend DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS distribution_weights TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN leaderboard_reward_logs.pool_rate IS '结算时的奖池比例快照（百分比）';
COMMENT ON COLUMN leaderboard_reward_logs.top_n IS '结算时的奖励名额快照';
COMMENT ON COLUMN leaderboard_reward_logs.min_spend IS '结算时的参与门槛快照';
COMMENT ON COLUMN leaderboard_reward_logs.distribution_weights IS '结算时的分配权重快照';
