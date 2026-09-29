package service

import (
	"context"
	"errors"
)

// ModelDetectionHistoryRepository atomically appends a result and updates the
// account's latest snapshot, keeping the list and history views consistent.
type ModelDetectionHistoryRepository interface {
	SaveModelDetectionResult(context.Context, int64, string, ModelDetectionSnapshot) error
	ListModelDetectionHistory(context.Context, int64, int, int) ([]ModelDetectionHistoryRecord, int64, error)
}

type ModelDetectionHistoryRecord struct {
	ID     int64  `json:"id,string"`
	Source string `json:"source"`
	ModelDetectionSnapshot
}

type modelDetectionSourceKey struct{}

func (s *AccountTestService) GetModelDetectionHistory(ctx context.Context, id int64, page, pageSize int) ([]ModelDetectionHistoryRecord, int64, error) {
	exists, err := s.accountRepo.ExistsByID(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	if !exists {
		return nil, 0, ErrAccountNotFound
	}
	repo, ok := s.accountRepo.(ModelDetectionHistoryRepository)
	if !ok {
		return nil, 0, errors.New("检测历史存储不可用")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return repo.ListModelDetectionHistory(ctx, id, page, pageSize)
}
