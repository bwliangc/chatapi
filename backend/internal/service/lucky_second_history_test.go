package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type luckyHistoryRedeemRepo struct {
	RedeemCodeRepository
	codes []RedeemCode
}

func (r *luckyHistoryRedeemRepo) ListByUserPaginated(_ context.Context, _ int64, p pagination.PaginationParams, _ string) ([]RedeemCode, *pagination.PaginationResult, error) {
	return mergeBalanceHistoryCodes(r.codes, nil, nil, p), &pagination.PaginationResult{Total: int64(len(r.codes))}, nil
}

func (r *luckyHistoryRedeemRepo) ListByUser(_ context.Context, _ int64, limit int) ([]RedeemCode, error) {
	return mergeBalanceHistoryCodes(r.codes, nil, nil, pagination.PaginationParams{Page: 1, PageSize: limit}), nil
}

func (r *luckyHistoryRedeemRepo) SumPositiveBalanceByUser(context.Context, int64) (float64, error) {
	return 25, nil
}

func luckyHistoryClient(t *testing.T) (*dbent.Client, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { require.NoError(t, mock.ExpectationsWereMet()); _ = client.Close() })
	return client, mock
}

func expectLuckyHistory(mock sqlmock.Sqlmock, offset, limit int, at time.Time) {
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM lucky_second_slots WHERE user_id=\$1 AND state='awarded'`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT s.id,s.amount,s.awarded_at,c.name FROM lucky_second_slots s JOIN lucky_second_campaigns c ON c.id=s.campaign_id WHERE s.user_id=\$1 AND s.state='awarded' ORDER BY s.awarded_at DESC,s.id DESC OFFSET \$2 LIMIT \$3`).
		WithArgs(int64(7), offset, limit).
		WillReturnRows(sqlmock.NewRows([]string{"id", "amount", "awarded_at", "name"}).AddRow(9, "0.000001", at, "国庆幸运秒"))
}

func TestLuckySecondHistoryAdminFilter(t *testing.T) {
	client, mock := luckyHistoryClient(t)
	at := time.Now().UTC()
	expectLuckyHistory(mock, 0, 15, at)
	s := &adminServiceImpl{entClient: client, redeemCodeRepo: &luckyHistoryRedeemRepo{}}
	codes, total, recharged, err := s.GetUserBalanceHistory(context.Background(), 7, 1, 15, RedeemTypeLuckySecondReward)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, 25.0, recharged, "awards must not inflate recharges")
	require.Len(t, codes, 1)
	require.Equal(t, RedeemTypeLuckySecondReward, codes[0].Type)
	require.Equal(t, -(luckySecondHistoryIDOffset + int64(9)), codes[0].ID)
	require.Equal(t, "国庆幸运秒", codes[0].Notes)
	require.Equal(t, 0.000001, codes[0].Value)
	require.Equal(t, at, *codes[0].UsedAt)
	require.EqualValues(t, 7, *codes[0].UsedBy)
}

func TestLuckySecondHistoryUserPaginationAcrossSources(t *testing.T) {
	client, mock := luckyHistoryClient(t)
	at := time.Now().UTC()
	repo := &luckyHistoryRedeemRepo{codes: []RedeemCode{
		{ID: 1, UsedAt: timePtrForLuckyHistory(at.Add(time.Second))},
		{ID: 2, UsedAt: timePtrForLuckyHistory(at.Add(-time.Second))},
	}}
	s := &RedeemService{entClient: client, redeemRepo: repo}
	for page, ids := range [][]int64{{1, -(luckySecondHistoryIDOffset + 9)}, {2}, {}} {
		expectLuckyHistory(mock, 0, (page+1)*2, at)
		codes, result, err := s.GetUserHistoryPaginated(context.Background(), 7, pagination.PaginationParams{Page: page + 1, PageSize: 2})
		require.NoError(t, err)
		require.EqualValues(t, 3, result.Total)
		require.Equal(t, 2, result.Pages)
		require.Len(t, codes, len(ids))
		for i, id := range ids {
			require.Equal(t, id, codes[i].ID)
		}
	}
}

func TestLuckySecondHistoryAdminAllTypes(t *testing.T) {
	client, mock := luckyHistoryClient(t)
	at := time.Now().UTC()
	mock.ExpectQuery(`SELECT id, amount::double precision, created_at FROM user_affiliate_ledger`).WithArgs(int64(7), 0, 1000).
		WillReturnRows(sqlmock.NewRows([]string{"id", "amount", "created_at"}))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_affiliate_ledger`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT id, reward_amount::double precision, created_at FROM leaderboard_reward_logs`).WithArgs(int64(7), 0, 1000).
		WillReturnRows(sqlmock.NewRows([]string{"id", "amount", "created_at"}))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM leaderboard_reward_logs`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	expectLuckyHistory(mock, 0, 15, at)
	s := &adminServiceImpl{entClient: client, redeemCodeRepo: &luckyHistoryRedeemRepo{codes: []RedeemCode{{ID: 1, UsedAt: timePtrForLuckyHistory(at.Add(-time.Second))}}}}
	codes, total, recharged, err := s.GetUserBalanceHistory(context.Background(), 7, 1, 15, "")
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Equal(t, 25.0, recharged)
	require.Len(t, codes, 2)
	require.Equal(t, RedeemTypeLuckySecondReward, codes[0].Type)
	require.EqualValues(t, 1, codes[1].ID)
}

func timePtrForLuckyHistory(at time.Time) *time.Time { return &at }

func TestLuckySecondHistoryDeepPagination(t *testing.T) {
	client, mock := luckyHistoryClient(t)
	at := time.Now().UTC()
	repo := &luckyHistoryRedeemRepo{}
	for i := 0; i < 1005; i++ {
		repo.codes = append(repo.codes, RedeemCode{ID: int64(i + 1), UsedAt: timePtrForLuckyHistory(at.Add(-time.Duration(i+1) * time.Second))})
	}
	expectLuckyHistory(mock, 0, 1005, at)
	s := &RedeemService{entClient: client, redeemRepo: repo}
	codes, result, err := s.GetUserHistoryPaginated(context.Background(), 7, pagination.PaginationParams{Page: 201, PageSize: 5})
	require.NoError(t, err)
	require.EqualValues(t, 1006, result.Total)
	require.Len(t, codes, 5)
	require.EqualValues(t, 1000, codes[0].ID)
	require.EqualValues(t, 1004, codes[4].ID)
}

func TestLuckySecondHistoryLegacyIncludesAwards(t *testing.T) {
	client, mock := luckyHistoryClient(t)
	at := time.Now().UTC()
	mock.ExpectQuery(`SELECT id, reward_amount, reward_date, created_at FROM leaderboard_reward_logs`).WithArgs(int64(7), 25).
		WillReturnRows(sqlmock.NewRows([]string{"id", "amount", "date", "created_at"}))
	expectLuckyHistory(mock, 0, 25, at)
	s := &RedeemService{entClient: client, redeemRepo: &luckyHistoryRedeemRepo{}}
	codes, err := s.GetUserHistory(context.Background(), 7, 25)
	require.NoError(t, err)
	require.Len(t, codes, 1)
	require.Equal(t, RedeemTypeLuckySecondReward, codes[0].Type)
}

func TestLuckySecondHistoryQueryFailureIsNotEmptyHistory(t *testing.T) {
	client, mock := luckyHistoryClient(t)
	mock.ExpectQuery(`SELECT COUNT`).WithArgs(int64(7)).WillReturnError(errors.New("database unavailable"))
	s := &RedeemService{entClient: client}
	_, _, err := s.GetUserHistoryPaginated(context.Background(), 7, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.ErrorContains(t, err, "database unavailable")
}
