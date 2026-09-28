package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// Keep synthesized history IDs separate from redeem, affiliate and leaderboard IDs.
const luckySecondHistoryIDOffset = 2_000_000_000_000

// Read the award ledger directly so existing awards appear without issuing money
// again. Only committed awards are visible, regardless of the feature switch.
func listLuckySecondBalanceHistory(ctx context.Context, client *dbent.Client, userID int64, offset, limit int) ([]RedeemCode, int64, error) {
	if client == nil || userID <= 0 || limit <= 0 {
		return nil, 0, nil
	}
	counts, err := client.QueryContext(ctx, `SELECT COUNT(*) FROM lucky_second_slots WHERE user_id=$1 AND state='awarded'`, userID)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if counts.Next() {
		err = counts.Scan(&total)
	}
	if err == nil {
		err = counts.Err()
	}
	counts.Close()
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []RedeemCode{}, 0, nil
	}
	rows, err := client.QueryContext(ctx, `SELECT s.id,s.amount,s.awarded_at,c.name
FROM lucky_second_slots s JOIN lucky_second_campaigns c ON c.id=s.campaign_id
WHERE s.user_id=$1 AND s.state='awarded'
ORDER BY s.awarded_at DESC,s.id DESC OFFSET $2 LIMIT $3`, userID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	codes := make([]RedeemCode, 0)
	for rows.Next() {
		var id int64
		var amount float64
		var awardedAt time.Time
		var name string
		if err := rows.Scan(&id, &amount, &awardedAt, &name); err != nil {
			return nil, 0, err
		}
		codes = append(codes, RedeemCode{
			ID: -(luckySecondHistoryIDOffset + id), Code: fmt.Sprintf("LSR-%d", id),
			Type: RedeemTypeLuckySecondReward, Value: amount, Status: StatusUsed,
			UsedBy: &userID, UsedAt: &awardedAt, CreatedAt: awardedAt, Notes: name,
		})
	}
	return codes, total, rows.Err()
}
