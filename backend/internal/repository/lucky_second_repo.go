package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type luckySecondRepository struct{ db *sql.DB }

func NewLuckySecondRepository(db *sql.DB) service.LuckySecondRepository {
	return &luckySecondRepository{db: db}
}

func (r *luckySecondRepository) Create(ctx context.Context, in service.LuckySecondCreate, slots []service.LuckySecondSlot) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO lucky_second_campaigns(name,starts_at,ends_at,timezone,total_amount,reward_count) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, in.Name, in.StartsAt, in.EndsAt, in.Timezone, in.TotalAmount, in.RewardCount).Scan(&id)
	if err != nil {
		return 0, err
	}
	for _, s := range slots {
		if _, err = tx.ExecContext(ctx, `INSERT INTO lucky_second_slots(campaign_id,second_at,amount) VALUES($1,$2,$3)`, id, s.SecondAt, s.Amount); err != nil {
			return 0, err
		}
	}
	// Generation/large inserts must not publish a schedule which has already begun.
	var future bool
	if err = tx.QueryRowContext(ctx, `SELECT $1::timestamptz > clock_timestamp()`, in.StartsAt).Scan(&future); err != nil {
		return 0, err
	}
	if !future {
		return 0, fmt.Errorf("活动开始时间已过，请重新选择")
	}
	return id, tx.Commit()
}
func (r *luckySecondRepository) List(ctx context.Context, page, size int, viewerID int64) ([]service.LuckySecondCampaign, int64, error) {
	items := make([]service.LuckySecondCampaign, 0)
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM lucky_second_campaigns`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT c.id,c.name,c.starts_at,c.ends_at,c.timezone,c.total_amount::text,c.reward_count,c.cancelled_at,c.created_at,
 (SELECT count(*) FROM lucky_second_slots s WHERE s.campaign_id=c.id AND s.state='awarded'),
 COALESCE((SELECT sum(amount) FROM lucky_second_slots s WHERE s.campaign_id=c.id AND s.state='awarded'),0)::text,
 (SELECT count(*) FROM lucky_second_slots s WHERE s.campaign_id=c.id AND s.state='expired'),
 (SELECT count(*) FROM lucky_second_slots s WHERE s.campaign_id=c.id AND s.state='waiting' AND s.second_at < now()),
 COALESCE((SELECT sum(amount) FROM lucky_second_slots s WHERE s.campaign_id=c.id AND s.state='awarded' AND s.user_id=$3),0)::text
 FROM lucky_second_campaigns c ORDER BY c.starts_at DESC,c.id DESC LIMIT $1 OFFSET $2`, size, (page-1)*size, viewerID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var c service.LuckySecondCampaign
		var myAmount string
		if err = rows.Scan(&c.ID, &c.Name, &c.StartsAt, &c.EndsAt, &c.Timezone, &c.TotalAmount, &c.RewardCount, &c.CancelledAt, &c.CreatedAt, &c.AwardedCount, &c.AwardedAmount, &c.ExpiredCount, &c.PendingCount, &myAmount); err != nil {
			return nil, 0, err
		}
		if viewerID > 0 {
			c.MyAwardedAmount = &myAmount
		}
		items = append(items, c)
	}
	return items, total, rows.Err()
}
func (r *luckySecondRepository) Slots(ctx context.Context, id int64, admin bool, page, size int) ([]service.LuckySecondSlot, int64, error) {
	items := make([]service.LuckySecondSlot, 0)
	var total int64
	// Public queries only return awarded slots, never the secret future schedule.
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM lucky_second_slots WHERE campaign_id=$1 AND ($2 OR state='awarded')`, id, admin).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT s.id,s.second_at,s.amount::text,s.state,s.user_id,COALESCE(u.email,''),s.request_id,s.awarded_at FROM lucky_second_slots s LEFT JOIN users u ON u.id=s.user_id WHERE s.campaign_id=$1 AND ($2 OR s.state='awarded') ORDER BY s.second_at DESC,s.id DESC LIMIT $3 OFFSET $4`, id, admin, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var s service.LuckySecondSlot
		if err = rows.Scan(&s.ID, &s.SecondAt, &s.Amount, &s.State, &s.UserID, &s.Name, &s.RequestID, &s.AwardedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, s)
	}
	return items, total, rows.Err()
}
func (r *luckySecondRepository) Cancel(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE lucky_second_campaigns SET cancelled_at=COALESCE(cancelled_at,clock_timestamp()) WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	// Already admitted seconds remain settleable. Cancellation affects future admission only.
	_, err = tx.ExecContext(ctx, `UPDATE lucky_second_slots s SET state='cancelled' WHERE campaign_id=$1 AND state='waiting' AND NOT EXISTS(SELECT 1 FROM lucky_second_candidates c WHERE c.slot_id=s.id)`, id)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *luckySecondRepository) Begin(ctx context.Context, worker, attempt string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	// The common DB clock and ordered slot locks define arrival/admission order
	// across gateway instances. Register before auth/forwarding/async billing.
	rows, err := tx.QueryContext(ctx, `SELECT s.id FROM lucky_second_slots s JOIN lucky_second_campaigns c ON c.id=s.campaign_id
 WHERE s.second_at=date_trunc('second',statement_timestamp()) AND s.state='waiting' AND c.cancelled_at IS NULL
 AND EXISTS(SELECT 1 FROM settings WHERE key='lucky_second_enabled' AND value='true')
 ORDER BY s.id FOR UPDATE OF s`)
	if err != nil {
		return false, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return false, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if _, err = tx.ExecContext(ctx, `INSERT INTO lucky_second_candidates(slot_id,attempt_id,worker_id) VALUES($1,$2,$3)`, id, attempt, worker); err != nil {
			return false, err
		}
	}
	if len(ids) == 0 {
		return false, nil
	}
	return true, tx.Commit()
}
func (r *luckySecondRepository) Finish(ctx context.Context, attempt string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE lucky_second_candidates SET state='invalid' WHERE attempt_id=$1 AND state='pending'`, attempt)
	return err
}

// Invoked in the billing transaction. Rolled-back and duplicate charges cannot
// qualify, and a crash between billing and the worker cannot lose a valid claim.
func qualifyLuckySecond(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) error {
	if cmd.LuckySecondAttempt == "" || (cmd.BalanceCost <= 0 && cmd.SubscriptionCost <= 0) {
		return nil
	}
	_, err := tx.ExecContext(ctx, `UPDATE lucky_second_candidates SET state='valid',user_id=$2,api_key_id=$3,request_id=$4 WHERE attempt_id=$1 AND state='pending'`, cmd.LuckySecondAttempt, cmd.UserID, cmd.APIKeyID, cmd.RequestID)
	return err
}

func (r *luckySecondRepository) Tick(ctx context.Context, worker string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO lucky_second_workers(id) VALUES($1) ON CONFLICT(id) DO UPDATE SET heartbeat_at=clock_timestamp()`, worker)
	if err != nil {
		return err
	}
	// Reclaim work from dead processes; a committed valid candidate is never expired.
	if _, err = r.db.ExecContext(ctx, `UPDATE lucky_second_candidates c SET state='invalid' FROM lucky_second_workers w WHERE c.worker_id=w.id AND c.state='pending' AND w.heartbeat_at < now()-interval '5 minutes'`); err != nil {
		return err
	}
	// Bound each pass; SKIP LOCKED supports multiple instances settling together.
	for i := 0; i < 200; i++ {
		done, err := r.settleOne(ctx)
		if err != nil {
			return err
		}
		if !done {
			break
		}
	}
	return nil
}
func (r *luckySecondRepository) settleOne(ctx context.Context) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var id int64
	var amount string
	// A later completed candidate never overtakes an earlier pending candidate.
	err = tx.QueryRowContext(ctx, `SELECT s.id,s.amount::text FROM lucky_second_slots s WHERE s.state='waiting' AND s.second_at+interval '1 second' <= now()
 AND NOT EXISTS(SELECT 1 FROM lucky_second_candidates p WHERE p.slot_id=s.id AND p.state='pending' AND p.id < COALESCE((SELECT min(v.id) FROM lucky_second_candidates v WHERE v.slot_id=s.id AND v.state='valid'),9223372036854775807))
 ORDER BY s.second_at,s.id LIMIT 1 FOR UPDATE OF s SKIP LOCKED`).Scan(&id, &amount)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var user int64
	var request string
	err = tx.QueryRowContext(ctx, `SELECT user_id,request_id FROM lucky_second_candidates WHERE slot_id=$1 AND state='valid' ORDER BY id LIMIT 1`, id).Scan(&user, &request)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `UPDATE lucky_second_slots SET state='expired' WHERE id=$1`, id)
	} else if err == nil {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$2::numeric,updated_at=now() WHERE id=$1`, user, amount)
		if err == nil {
			n, _ := result.RowsAffected()
			if n != 1 {
				return false, fmt.Errorf("lucky second winner %d missing", user)
			}
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE lucky_second_slots SET state='awarded',user_id=$2,request_id=$3,awarded_at=clock_timestamp() WHERE id=$1`, id, user, request)
		}
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
func (r *luckySecondRepository) PendingCacheInvalidations(ctx context.Context) (map[int64][]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,user_id FROM lucky_second_slots WHERE state='awarded' AND NOT cache_invalidated ORDER BY id LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int64][]int64{}
	for rows.Next() {
		var id, user int64
		if err = rows.Scan(&id, &user); err != nil {
			return nil, err
		}
		result[user] = append(result[user], id)
	}
	return result, rows.Err()
}
func (r *luckySecondRepository) MarkCacheInvalidated(ctx context.Context, ids []int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE lucky_second_slots SET cache_invalidated=true WHERE id=ANY($1)`, pq.Array(ids))
	return err
}
