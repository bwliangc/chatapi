package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *usageLogRepository) GetGroupRateUsage(ctx context.Context, ids []int64, at time.Time) ([]service.GroupRateUsageBucket, error) {
	if len(ids) == 0 {
		return []service.GroupRateUsageBucket{}, nil
	}
	// The WHERE bounds use the existing (group_id, created_at) index. The last
	// hour and token breakdown share this one bounded scan with the hourly trend.
	rows, err := r.sql.QueryContext(ctx, `SELECT group_id,
		date_trunc('hour', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS bucket,
		COUNT(*), COALESCE(SUM(input_tokens::bigint), 0), COALESCE(SUM(output_tokens::bigint), 0),
		COALESCE(SUM(cache_creation_tokens::bigint), 0), COALESCE(SUM(cache_read_tokens::bigint), 0),
		COUNT(*) FILTER (WHERE created_at >= $4),
		COALESCE(SUM(input_tokens::bigint) FILTER (WHERE created_at >= $4), 0),
		COALESCE(SUM(output_tokens::bigint) FILTER (WHERE created_at >= $4), 0),
		COALESCE(SUM(cache_creation_tokens::bigint) FILTER (WHERE created_at >= $4), 0),
		COALESCE(SUM(cache_read_tokens::bigint) FILTER (WHERE created_at >= $4), 0)
		FROM usage_logs WHERE group_id = ANY($1) AND created_at >= $2 AND created_at < $3
		GROUP BY group_id, bucket ORDER BY group_id, bucket`, pq.Array(ids), at.Add(-24*time.Hour), at, at.Add(-time.Hour))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.GroupRateUsageBucket, 0)
	for rows.Next() {
		var b service.GroupRateUsageBucket
		u, h := &b.Usage, &b.LastHour
		if err := rows.Scan(&b.GroupID, &b.At, &u.Requests, &u.InputTokens, &u.OutputTokens, &u.CacheCreationTokens, &u.CacheReadTokens,
			&h.Requests, &h.InputTokens, &h.OutputTokens, &h.CacheCreationTokens, &h.CacheReadTokens); err != nil {
			return nil, err
		}
		u.TotalTokens = u.InputTokens + u.OutputTokens + u.CacheCreationTokens + u.CacheReadTokens
		h.TotalTokens = h.InputTokens + h.OutputTokens + h.CacheCreationTokens + h.CacheReadTokens
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *usageLogRepository) GetGroupRateHistory(ctx context.Context, ids []int64, at time.Time) ([]service.GroupRateHistoryPoint, error) {
	if len(ids) == 0 {
		return []service.GroupRateHistoryPoint{}, nil
	}
	start := at.Add(-24 * time.Hour)
	rows, err := r.sql.QueryContext(ctx, `WITH baseline AS (
		SELECT DISTINCT ON (group_id) group_id, recorded_at, rate_multiplier
		FROM group_dynamic_rate_history
		WHERE group_id = ANY($1) AND recorded_at < $2
		ORDER BY group_id, recorded_at DESC
	), visible AS (
		SELECT group_id, recorded_at, rate_multiplier
		FROM group_dynamic_rate_history
		WHERE group_id = ANY($1) AND recorded_at >= $2 AND recorded_at <= $3
	)
	SELECT group_id, recorded_at, rate_multiplier FROM baseline
	UNION ALL
	SELECT group_id, recorded_at, rate_multiplier FROM visible
	ORDER BY group_id, recorded_at`, pq.Array(ids), start, at)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.GroupRateHistoryPoint, 0)
	for rows.Next() {
		var point service.GroupRateHistoryPoint
		if err := rows.Scan(&point.GroupID, &point.At, &point.RateMultiplier); err != nil {
			return nil, err
		}
		if point.At.Before(start) {
			point.At = start
		}
		out = append(out, point)
	}
	return out, rows.Err()
}
