package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type detectionHistoryReader struct {
	AccountRepository
	exists     bool
	err        error
	id         int64
	page, size int
}

func (r *detectionHistoryReader) ExistsByID(context.Context, int64) (bool, error) {
	return r.exists, r.err
}
func (r *detectionHistoryReader) SaveModelDetectionResult(context.Context, int64, string, ModelDetectionSnapshot) error {
	return nil
}
func (r *detectionHistoryReader) ListModelDetectionHistory(_ context.Context, id int64, page, size int) ([]ModelDetectionHistoryRecord, int64, error) {
	r.id = id
	r.page = page
	r.size = size
	return []ModelDetectionHistoryRecord{{ID: 1}}, 1, r.err
}
func TestModelDetectionHistoryReadWithoutFeatureEnabled(t *testing.T) {
	repo := &detectionHistoryReader{exists: true}
	s := &AccountTestService{accountRepo: repo}
	items, total, err := s.GetModelDetectionHistory(context.Background(), 42, 2, 20)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.EqualValues(t, 1, total)
	require.EqualValues(t, 42, repo.id)
	require.Equal(t, 2, repo.page)
	require.Equal(t, 20, repo.size)
	_, _, err = s.GetModelDetectionHistory(context.Background(), 42, 0, 200)
	require.NoError(t, err)
	require.Equal(t, 1, repo.page)
	require.Equal(t, 20, repo.size)
	repo.exists = false
	_, _, err = s.GetModelDetectionHistory(context.Background(), 42, 1, 20)
	require.ErrorIs(t, err, ErrAccountNotFound)
	repo.err = errors.New("database unavailable")
	_, _, err = s.GetModelDetectionHistory(context.Background(), 42, 1, 20)
	require.ErrorIs(t, err, repo.err)
}
