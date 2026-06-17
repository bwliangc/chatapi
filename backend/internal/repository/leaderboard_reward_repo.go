package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const leaderboardRewardDateLayout = "2006-01-02"

type leaderboardRewardRepository struct {
	client *dbent.Client
}

// NewLeaderboardRewardRepository 创建排行榜激励发放仓储。
func NewLeaderboardRewardRepository(client *dbent.Client) service.LeaderboardRewardRepository {
	return &leaderboardRewardRepository{client: client}
}

func (r *leaderboardRewardRepository) HasSettled(ctx context.Context, rewardDate time.Time) (bool, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM leaderboard_reward_logs WHERE reward_date = $1)`,
		rewardDate.Format(leaderboardRewardDateLayout),
	)
	if err != nil {
		return false, fmt.Errorf("check leaderboard reward settled: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var exists bool
	if rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			return false, fmt.Errorf("scan leaderboard reward settled: %w", err)
		}
	}
	return exists, rows.Err()
}

func (r *leaderboardRewardRepository) GrantRewards(ctx context.Context, grant service.LeaderboardRewardGrant) (int, error) {
	dateStr := grant.RewardDate.Format(leaderboardRewardDateLayout)
	granted := 0
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		granted = 0
		for _, item := range grant.Items {
			if item.Amount <= 0 || item.UserID <= 0 {
				continue
			}
			// (reward_date, user_id) 唯一约束保证幂等：若该日已发放，INSERT 冲突会使整个事务回滚，不会重复加余额。
			if _, err := txClient.ExecContext(txCtx,
				`INSERT INTO leaderboard_reward_logs
				    (reward_date, user_id, rank, reward_amount, pool_amount, total_cost, distribution_mode)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				dateStr, item.UserID, item.Rank, item.Amount, grant.PoolAmount, grant.TotalCost, grant.DistributionMode,
			); err != nil {
				return fmt.Errorf("insert leaderboard reward log (user %d): %w", item.UserID, err)
			}

			affected, err := txClient.User.Update().
				Where(user.IDEQ(item.UserID)).
				AddBalance(item.Amount).
				Save(txCtx)
			if err != nil {
				return fmt.Errorf("credit leaderboard reward to user %d: %w", item.UserID, err)
			}
			if affected == 0 {
				return fmt.Errorf("credit leaderboard reward: user %d not found", item.UserID)
			}
			granted++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return granted, nil
}

func (r *leaderboardRewardRepository) withTx(ctx context.Context, fn func(txCtx context.Context, txClient *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin leaderboard reward transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit leaderboard reward transaction: %w", err)
	}
	return nil
}
