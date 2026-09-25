package service

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func luckySecondInput() LuckySecondCreate {
	return LuckySecondCreate{Name: "October", StartsAt: time.Date(2027, 10, 1, 0, 0, 0, 0, time.UTC), EndsAt: time.Date(2027, 10, 8, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Shanghai", TotalAmount: "100", RewardCount: 50}
}
func TestLuckySecondScheduleConservesPoolAndUsesDistinctSeconds(t *testing.T) {
	now := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		amount  string
		count   int
		seconds int64
	}{{"100", 50, 604800}, {"0.000050", 50, 50}, {"1.000001", 1, 1}, {"99999999.999999", 10000, 10000}} {
		in := luckySecondInput()
		in.TotalAmount = test.amount
		in.RewardCount = test.count
		in.EndsAt = in.StartsAt.Add(time.Duration(test.seconds) * time.Second)
		slots, err := GenerateLuckySecondSchedule(in, now)
		require.NoError(t, err)
		require.Len(t, slots, test.count)
		total := decimal.Zero
		seen := map[int64]bool{}
		for _, s := range slots {
			n, err := decimal.NewFromString(s.Amount)
			require.NoError(t, err)
			require.True(t, n.IsPositive())
			total = total.Add(n)
			require.False(t, seen[s.SecondAt.Unix()])
			seen[s.SecondAt.Unix()] = true
			require.False(t, s.SecondAt.Before(in.StartsAt))
			require.True(t, s.SecondAt.Before(in.EndsAt))
		}
		expected, _ := decimal.NewFromString(test.amount)
		require.True(t, total.Equal(expected), "got %s expected %s", total, expected)
	}
}
func TestLuckySecondScheduleRejectsInvalidBudgetsAndDates(t *testing.T) {
	now := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, amount := range []string{"0", "-1", "NaN", "0.0000001", "0.000049", "100000001"} {
		in := luckySecondInput()
		in.TotalAmount = amount
		_, err := GenerateLuckySecondSchedule(in, now)
		require.Error(t, err, amount)
	}
	in := luckySecondInput()
	in.EndsAt = in.StartsAt.Add(time.Second)
	_, err := GenerateLuckySecondSchedule(in, now)
	require.Error(t, err)
	in = luckySecondInput()
	in.StartsAt = now
	_, err = GenerateLuckySecondSchedule(in, now)
	require.Error(t, err)
	in = luckySecondInput()
	in.Timezone = "not/a/timezone"
	_, err = GenerateLuckySecondSchedule(in, now)
	require.Error(t, err)
}

type luckyFinishRepo struct {
	LuckySecondRepository
	finished int
}

func (r *luckyFinishRepo) Finish(context.Context, string) error { r.finished++; return nil }
func TestLuckySecondAsyncParticipationWaitsForAllTasks(t *testing.T) {
	repo := &luckyFinishRepo{}
	svc := &LuckySecondService{Repo: repo}
	p := &luckySecondParticipation{attempt: "attempt", service: svc}
	p.refs.Store(1)
	ctx := context.WithValue(context.Background(), luckySecondContextKey{}, p)
	RetainLuckySecond(ctx)
	RetainLuckySecond(ctx)
	snapshot := CopyLuckySecondContext(ctx, context.Background())
	require.Equal(t, "attempt", LuckySecondAttempt(snapshot))
	ReleaseLuckySecond(ctx)
	require.Zero(t, repo.finished)
	ReleaseLuckySecond(snapshot)
	require.Zero(t, repo.finished)
	ReleaseLuckySecond(snapshot)
	require.Equal(t, 1, repo.finished)
}

func TestLuckySecondEditPreservesScheduleForRenameAndEquivalentAmount(t *testing.T) {
	in := luckySecondInput()
	current := LuckySecondCampaign{Name: in.Name, StartsAt: in.StartsAt, EndsAt: in.EndsAt, Timezone: in.Timezone, TotalAmount: in.TotalAmount, RewardCount: in.RewardCount}
	name, amount := " Updated ", "100.00000000"
	merged, slots, err := PlanLuckySecondUpdate(current, LuckySecondUpdate{Name: &name, TotalAmount: &amount}, in.EndsAt.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, "Updated", merged.Name)
	require.Equal(t, in.Timezone, merged.Timezone)
	require.Nil(t, slots)
	amount = "200"
	_, _, err = PlanLuckySecondUpdate(current, LuckySecondUpdate{TotalAmount: &amount}, in.StartsAt)
	require.ErrorIs(t, err, ErrLuckySecondEditLocked)
	_, slots, err = PlanLuckySecondUpdate(current, LuckySecondUpdate{TotalAmount: &amount}, in.StartsAt.Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, slots, in.RewardCount)
	current.CancelledAt = &in.StartsAt
	_, _, err = PlanLuckySecondUpdate(current, LuckySecondUpdate{TotalAmount: &amount}, in.StartsAt.Add(-time.Hour))
	require.ErrorIs(t, err, ErrLuckySecondEditLocked)
	name = " "
	_, _, err = PlanLuckySecondUpdate(current, LuckySecondUpdate{Name: &name}, in.StartsAt.Add(-time.Hour))
	require.ErrorIs(t, err, ErrLuckySecondInvalidUpdate)
}
