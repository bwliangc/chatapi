//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func luckyFixture(t *testing.T) (*luckySecondRepository, int64, int64, int64, string) {
	t.Helper()
	ctx := context.Background()
	r := &luckySecondRepository{db: integrationDB}
	user := mustCreateUser(t, testEntClient(t), &service.User{Email: uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 10})
	start := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	in := service.LuckySecondCreate{Name: "Test", StartsAt: start, EndsAt: start.Add(time.Hour), Timezone: "UTC", TotalAmount: "1", RewardCount: 1}
	id, err := r.Create(ctx, in, []service.LuckySecondSlot{{SecondAt: start, Amount: "1"}})
	require.NoError(t, err)
	worker := uuid.NewString()
	_, err = integrationDB.Exec(`INSERT INTO lucky_second_workers(id) VALUES($1)`, worker)
	require.NoError(t, err)
	var slot int64
	require.NoError(t, integrationDB.QueryRow(`UPDATE lucky_second_slots SET second_at=now()-interval '10 seconds' WHERE campaign_id=$1 RETURNING id`, id).Scan(&slot))
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM lucky_second_candidates WHERE slot_id=$1`, slot)
		_, _ = integrationDB.Exec(`DELETE FROM lucky_second_slots WHERE campaign_id=$1`, id)
		_, _ = integrationDB.Exec(`DELETE FROM lucky_second_campaigns WHERE id=$1`, id)
		_, _ = integrationDB.Exec(`DELETE FROM lucky_second_workers WHERE id=$1`, worker)
	})
	return r, id, slot, user.ID, worker
}
func addLuckyCandidate(t *testing.T, slot, user int64, worker, state, attempt string) {
	t.Helper()
	_, err := integrationDB.Exec(`INSERT INTO lucky_second_candidates(slot_id,user_id,worker_id,state,attempt_id,request_id) VALUES($1,$2,$3,$4,$5,$5)`, slot, user, worker, state, attempt)
	require.NoError(t, err)
}
func TestLuckySecondArrivalOrderAndConcurrentSettlement(t *testing.T) {
	ctx := context.Background()
	r, _, slot, user, worker := luckyFixture(t)
	first, second := uuid.NewString(), uuid.NewString()
	addLuckyCandidate(t, slot, user, worker, "pending", first)
	addLuckyCandidate(t, slot, user, worker, "valid", second)
	done, err := r.settleOne(ctx)
	require.NoError(t, err)
	require.False(t, done, "later completion must wait for earlier arrival")
	_, err = integrationDB.Exec(`UPDATE lucky_second_candidates SET state='valid' WHERE attempt_id=$1`, first)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := r.settleOne(ctx); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var winner, state string
	require.NoError(t, integrationDB.QueryRow(`SELECT request_id,state FROM lucky_second_slots WHERE id=$1`, slot).Scan(&winner, &state))
	require.Equal(t, first, winner)
	require.Equal(t, "awarded", state)
	var balance string
	require.NoError(t, integrationDB.QueryRow(`SELECT balance::text FROM users WHERE id=$1`, user).Scan(&balance))
	require.Equal(t, "11.00000000", balance)
}
func TestLuckySecondFailedFirstAndUnclaimedRewards(t *testing.T) {
	ctx := context.Background()
	r, _, slot, user, worker := luckyFixture(t)
	first, second := uuid.NewString(), uuid.NewString()
	addLuckyCandidate(t, slot, user, worker, "pending", first)
	addLuckyCandidate(t, slot, user, worker, "valid", second)
	require.NoError(t, r.Finish(ctx, first))
	done, err := r.settleOne(ctx)
	require.NoError(t, err)
	require.True(t, done)
	var winner string
	require.NoError(t, integrationDB.QueryRow(`SELECT request_id FROM lucky_second_slots WHERE id=$1`, slot).Scan(&winner))
	require.Equal(t, second, winner)
	r, _, empty, _, _ := luckyFixture(t)
	done, err = r.settleOne(ctx)
	require.NoError(t, err)
	require.True(t, done)
	var state string
	require.NoError(t, integrationDB.QueryRow(`SELECT state FROM lucky_second_slots WHERE id=$1`, empty).Scan(&state))
	require.Equal(t, "expired", state)
}
func TestLuckySecondPublicScheduleAndCancellation(t *testing.T) {
	ctx := context.Background()
	r, id, slot, user, worker := luckyFixture(t)
	slots, total, err := r.Slots(ctx, id, false, 1, 20)
	require.NoError(t, err)
	require.Empty(t, slots)
	require.Zero(t, total)
	_, total, err = r.Slots(ctx, id, true, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	addLuckyCandidate(t, slot, user, worker, "valid", uuid.NewString())
	require.NoError(t, r.Cancel(ctx, id))
	_, err = r.settleOne(ctx)
	require.NoError(t, err)
	slots, total, err = r.Slots(ctx, id, false, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "awarded", slots[0].State)
}
func TestLuckySecondQualificationSharesBillingTransaction(t *testing.T) {
	ctx := context.Background()
	_, _, slot, user, worker := luckyFixture(t)
	attempt := uuid.NewString()
	addLuckyCandidate(t, slot, user, worker, "pending", attempt)
	cmd := &service.UsageBillingCommand{LuckySecondAttempt: attempt, UserID: user, APIKeyID: 7, RequestID: "bill", BalanceCost: 1}
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, qualifyLuckySecond(ctx, tx, cmd))
	require.NoError(t, tx.Rollback())
	var state string
	require.NoError(t, integrationDB.QueryRow(`SELECT state FROM lucky_second_candidates WHERE attempt_id=$1`, attempt).Scan(&state))
	require.Equal(t, "pending", state)
	tx, err = integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, qualifyLuckySecond(ctx, tx, cmd))
	require.NoError(t, tx.Commit())
	require.NoError(t, integrationDB.QueryRow(`SELECT state FROM lucky_second_candidates WHERE attempt_id=$1`, attempt).Scan(&state))
	require.Equal(t, "valid", state)
}
func TestLuckySecondAdmissionRespectsFeatureSwitch(t *testing.T) {
	ctx := context.Background()
	r, id, slot, _, worker := luckyFixture(t)
	_, err := integrationDB.Exec(`INSERT INTO settings(key,value) VALUES('lucky_second_enabled','false') ON CONFLICT(key) DO UPDATE SET value='false'`)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = integrationDB.Exec(`DELETE FROM settings WHERE key='lucky_second_enabled'`) })
	_, err = integrationDB.Exec(`UPDATE lucky_second_slots SET second_at=date_trunc('second',now()) WHERE id=$1`, slot)
	require.NoError(t, err)
	ok, err := r.Begin(ctx, worker, uuid.NewString())
	require.NoError(t, err)
	require.False(t, ok)
	_, err = integrationDB.Exec(`UPDATE settings SET value='true' WHERE key='lucky_second_enabled'`)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err = integrationDB.Exec(`UPDATE lucky_second_slots SET second_at=date_trunc('second',now()) WHERE campaign_id=$1`, id)
		require.NoError(t, err)
		ok, err = r.Begin(ctx, worker, fmt.Sprintf("%s-%d", worker, i))
		require.NoError(t, err)
		if ok {
			break
		}
	}
	require.True(t, ok)
}

func TestLuckySecondPersonalTotalIncludesAllAwardsOnlyForThisUserAndCampaign(t *testing.T) {
	ctx := context.Background()
	r, campaignID, slot, user, _ := luckyFixture(t)
	other := mustCreateUser(t, testEntClient(t), &service.User{Email: uuid.NewString() + "@example.com", PasswordHash: "hash"})
	_, err := integrationDB.Exec(`UPDATE lucky_second_slots SET state='awarded',user_id=$2 WHERE id=$1`, slot, other.ID)
	require.NoError(t, err)
	// More than one page of personal awards, plus an unissued amount that must not count.
	_, err = integrationDB.Exec(`INSERT INTO lucky_second_slots(campaign_id,second_at,amount,state,user_id)
	 SELECT $1, c.starts_at+n*interval '1 second', '0.100001', 'awarded', $2
	 FROM lucky_second_campaigns c CROSS JOIN generate_series(1,25) n WHERE c.id=$1`, campaignID, user)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO lucky_second_slots(campaign_id,second_at,amount,state,user_id)
	 SELECT id,starts_at+interval '26 seconds',3,'waiting',$2 FROM lucky_second_campaigns WHERE id=$1`, campaignID, user)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE lucky_second_campaigns SET reward_count=27,total_amount='6.500025' WHERE id=$1`, campaignID)
	require.NoError(t, err)
	_, _, anotherSlot, _, _ := luckyFixture(t)
	_, err = integrationDB.Exec(`UPDATE lucky_second_slots SET state='awarded',user_id=$2 WHERE id=$1`, anotherSlot, user)
	require.NoError(t, err)
	firstPage, total, err := r.Slots(ctx, campaignID, false, 1, 20)
	require.NoError(t, err)
	require.Len(t, firstPage, 20)
	require.EqualValues(t, 26, total)
	for _, test := range []struct {
		viewer int64
		amount string
	}{{user, "2.50002500"}, {other.ID, "1.00000000"}, {9223372036854775807, "0"}, {0, ""}} {
		campaigns, _, err := r.List(ctx, 1, 100, test.viewer)
		require.NoError(t, err)
		found := false
		for _, c := range campaigns {
			if c.ID != campaignID {
				continue
			}
			found = true
			if test.viewer == 0 {
				require.Nil(t, c.MyAwardedAmount)
			} else {
				require.NotNil(t, c.MyAwardedAmount)
				require.Equal(t, test.amount, *c.MyAwardedAmount)
			}
		}
		require.True(t, found)
	}
}

func TestLuckySecondEditRegeneratesFutureScheduleAndPreservesRenames(t *testing.T) {
	ctx := context.Background()
	r, id, _, _, _ := luckyFixture(t)
	name, amount, count := "Edited", "12.345678", 4
	start := time.Now().UTC().Truncate(time.Second).Add(2 * time.Hour)
	end := start.Add(time.Hour)
	require.NoError(t, r.Update(ctx, id, service.LuckySecondUpdate{Name: &name, TotalAmount: &amount, RewardCount: &count, StartsAt: &start, EndsAt: &end}))
	slots, total, err := r.Slots(ctx, id, true, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	for _, slot := range slots {
		require.False(t, slot.SecondAt.Before(start))
		require.True(t, slot.SecondAt.Before(end))
		require.Equal(t, "waiting", slot.State)
	}
	var pool, sum, storedName string
	require.NoError(t, integrationDB.QueryRow(`SELECT name,total_amount::text,(SELECT sum(amount)::text FROM lucky_second_slots WHERE campaign_id=$1) FROM lucky_second_campaigns WHERE id=$1`, id).Scan(&storedName, &pool, &sum))
	require.Equal(t, name, storedName)
	require.Equal(t, "12.34567800", pool)
	require.Equal(t, pool, sum)
	name = "Renamed"
	amount = "12.34567800"
	require.NoError(t, r.Update(ctx, id, service.LuckySecondUpdate{Name: &name, TotalAmount: &amount}))
	after, _, err := r.Slots(ctx, id, true, 1, 20)
	require.NoError(t, err)
	require.Equal(t, slots, after)
	count = 0
	require.ErrorIs(t, r.Update(ctx, id, service.LuckySecondUpdate{RewardCount: &count}), service.ErrLuckySecondInvalidUpdate)
	after, _, err = r.Slots(ctx, id, true, 1, 20)
	require.NoError(t, err)
	require.Equal(t, slots, after)
}

func TestLuckySecondEditProtectsStartedCancelledAndAdmittedCampaigns(t *testing.T) {
	for _, state := range []string{"started", "cancelled", "admitted"} {
		t.Run(state, func(t *testing.T) {
			ctx := context.Background()
			r, id, slot, user, worker := luckyFixture(t)
			switch state {
			case "started":
				_, err := integrationDB.Exec(`UPDATE lucky_second_campaigns SET starts_at=now()-interval '1 second' WHERE id=$1`, id)
				require.NoError(t, err)
			case "cancelled":
				require.NoError(t, r.Cancel(ctx, id))
			case "admitted":
				addLuckyCandidate(t, slot, user, worker, "pending", uuid.NewString())
			}
			before, _, err := r.Slots(ctx, id, true, 1, 20)
			require.NoError(t, err)
			amount := "2"
			require.ErrorIs(t, r.Update(ctx, id, service.LuckySecondUpdate{TotalAmount: &amount}), service.ErrLuckySecondEditLocked)
			name := "New name"
			require.NoError(t, r.Update(ctx, id, service.LuckySecondUpdate{Name: &name}))
			after, _, err := r.Slots(ctx, id, true, 1, 20)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

func TestLuckySecondEditRechecksStartAfterWaitingForLock(t *testing.T) {
	ctx := context.Background()
	r, id, _, _, _ := luckyFixture(t)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE lucky_second_campaigns SET starts_at=now()-interval '1 second' WHERE id=$1`, id)
	require.NoError(t, err)
	result := make(chan error, 1)
	amount := "2"
	go func() { result <- r.Update(ctx, id, service.LuckySecondUpdate{TotalAmount: &amount}) }()
	require.NoError(t, tx.Commit())
	require.ErrorIs(t, <-result, service.ErrLuckySecondEditLocked)
}
