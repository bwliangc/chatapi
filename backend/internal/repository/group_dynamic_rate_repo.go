package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// SyncGroupDynamicRates publishes at most once per five-minute interval. The
// compare-and-swap prevents a slow computation from overwriting an admin edit
// or applying smoothing twice when multiple instances execute the same tick.
func (r *dashboardAggregationRepository) SyncGroupDynamicRates(ctx context.Context, now time.Time) error {
	at := now.UTC().Truncate(5 * time.Minute)
	rows, err := r.sql.QueryContext(ctx, `SELECT id, dynamic_rate, rate_multiplier, created_at, updated_at
		FROM groups WHERE deleted_at IS NULL AND status = 'active'
		AND dynamic_rate ->> 'enabled' = 'true'
		AND (dynamic_rate_updated_at IS NULL OR dynamic_rate_updated_at < $1)
		ORDER BY id`, at)
	if err != nil {
		return err
	}
	type candidate struct {
		id               int64
		raw              []byte
		rate             float64
		created, updated time.Time
	}
	var groups []candidate
	for rows.Next() {
		var g candidate
		if err := rows.Scan(&g.id, &g.raw, &g.rate, &g.created, &g.updated); err != nil {
			rows.Close()
			return err
		}
		groups = append(groups, g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		return nil
	}
	// Never treat history removed by retention (or a fresh installation with
	// no usage at all) as a full week of idle traffic.
	retainedRows, err := r.sql.QueryContext(ctx, `SELECT MIN(created_at) FROM usage_logs`)
	if err != nil {
		return err
	}
	var retainedFrom sql.NullTime
	if retainedRows.Next() {
		err = retainedRows.Scan(&retainedFrom)
	}
	if err == nil {
		err = retainedRows.Err()
	}
	retainedRows.Close()
	if err != nil {
		return err
	}
	for _, g := range groups {
		var cfg service.GroupDynamicRate
		if err := json.Unmarshal(g.raw, &cfg); err != nil {
			return fmt.Errorf("group %d dynamic rate: %w", g.id, err)
		}
		cfg, err = service.NormalizeGroupDynamicRate(cfg)
		if err != nil {
			return fmt.Errorf("group %d dynamic rate: %w", g.id, err)
		}
		currentStart := at.Add(-time.Hour)
		if !retainedFrom.Valid {
			g.created = at
		} else if retainedFrom.Time.After(g.created) {
			g.created = retainedFrom.Time
		}
		hours := int(currentStart.Sub(g.created) / time.Hour)
		if hours < 0 {
			hours = 0
		}
		if hours > 168 {
			hours = 168
		}
		start := currentStart.Add(-time.Duration(hours) * time.Hour)
		history, current, err := r.groupDynamicRateUsage(ctx, g.id, start, at, hours)
		if err != nil {
			return fmt.Errorf("group %d usage: %w", g.id, err)
		}
		rate := service.CalculateGroupDynamicRate(cfg, g.rate, current, history)
		// Both outbox records and publication commit in the same SQL statement:
		// scheduler outbox here, auth invalidation via the groups update trigger.
		_, err = r.sql.ExecContext(ctx, `WITH published AS (
			UPDATE groups SET rate_multiplier = $2, dynamic_rate_updated_at = $3, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL AND status = 'active'
			AND updated_at = $4 AND dynamic_rate = $5::jsonb AND rate_multiplier = $6
			AND (dynamic_rate_updated_at IS NULL OR dynamic_rate_updated_at < $3)
			RETURNING id
		) INSERT INTO scheduler_outbox (event_type, group_id)
		SELECT $7, id FROM published`, g.id, rate, at, g.updated, string(g.raw), g.rate, service.SchedulerOutboxEventGroupChanged)
		if err != nil {
			return fmt.Errorf("publish group %d dynamic rate: %w", g.id, err)
		}
	}
	return nil
}

func (r *dashboardAggregationRepository) groupDynamicRateUsage(ctx context.Context, groupID int64, start, end time.Time, hours int) ([]float64, float64, error) {
	rows, err := r.sql.QueryContext(ctx, `SELECT
		FLOOR(EXTRACT(EPOCH FROM (created_at - $2::timestamptz)) / 3600)::int AS bucket,
		COALESCE(SUM(input_tokens::bigint + output_tokens::bigint + cache_creation_tokens::bigint + cache_read_tokens::bigint), 0)
		FROM usage_logs WHERE group_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY bucket`, groupID, start, end)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	history := make([]float64, hours)
	var current float64
	for rows.Next() {
		var bucket int
		var tokens float64
		if err := rows.Scan(&bucket, &tokens); err != nil {
			return nil, 0, err
		}
		if bucket >= 0 && bucket < hours {
			history[bucket] = tokens
		}
		if bucket == hours {
			current = tokens
		}
	}
	return history, current, rows.Err()
}
