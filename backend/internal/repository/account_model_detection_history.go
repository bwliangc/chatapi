package repository

import (
	"context"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.ModelDetectionHistoryRepository = (*accountRepository)(nil)

func (r *accountRepository) SaveModelDetectionResult(ctx context.Context, accountID int64, source string, snapshot service.ModelDetectionSnapshot) error {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	// One statement: failure to insert history also rolls back the snapshot update.
	result, err := r.client.ExecContext(ctx, `
  WITH updated AS (
   UPDATE accounts
   SET extra = COALESCE(extra, '{}'::jsonb) || jsonb_build_object('model_detection_snapshot', $2::jsonb), updated_at = NOW()
   WHERE id = $1 AND deleted_at IS NULL
   RETURNING id
  )
  INSERT INTO model_detection_history (account_id, source, snapshot)
  SELECT id, $3, $2::jsonb FROM updated
 `, accountID, string(raw), source)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return service.ErrAccountNotFound
	}
	return nil
}

func (r *accountRepository) ListModelDetectionHistory(ctx context.Context, accountID int64, page, pageSize int) ([]service.ModelDetectionHistoryRecord, int64, error) {
	items := make([]service.ModelDetectionHistoryRecord, 0)
	var total int64
	rows, err := r.client.QueryContext(ctx, `SELECT COUNT(*) FROM model_detection_history WHERE account_id = $1`, accountID)
	if err != nil {
		return nil, 0, err
	}
	if rows.Next() {
		err = rows.Scan(&total)
	}
	rowsErr := rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, 0, err
	}
	if rowsErr != nil {
		return nil, 0, rowsErr
	}
	rows, err = r.client.QueryContext(ctx, `
  SELECT id, source, snapshot FROM model_detection_history
  WHERE account_id = $1 ORDER BY id DESC LIMIT $2 OFFSET $3
 `, accountID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var item service.ModelDetectionHistoryRecord
		var raw []byte
		if err := rows.Scan(&item.ID, &item.Source, &raw); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(raw, &item.ModelDetectionSnapshot); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}
