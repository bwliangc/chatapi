package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type leaderboardDisplaySettingRepo struct {
	values  map[string]string
	updates map[string]string
}

func (r *leaderboardDisplaySettingRepo) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (r *leaderboardDisplaySettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *leaderboardDisplaySettingRepo) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (r *leaderboardDisplaySettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (r *leaderboardDisplaySettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	r.updates = make(map[string]string, len(values))
	for key, value := range values {
		r.updates[key] = value
	}
	return nil
}

func (r *leaderboardDisplaySettingRepo) GetAll(context.Context) (map[string]string, error) {
	values := make(map[string]string, len(r.values))
	for key, value := range r.values {
		values[key] = value
	}
	return values, nil
}

func (r *leaderboardDisplaySettingRepo) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestSettingServiceLeaderboardDisplayTopN(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
	}{
		{name: "missing defaults to ten", want: LeaderboardDisplayTopNDefault},
		{name: "zero defaults to ten", value: "0", want: LeaderboardDisplayTopNDefault},
		{name: "configured value", value: "50", want: 50},
		{name: "value is capped", value: "5000", want: LeaderboardDisplayTopNMax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := map[string]string{}
			if tt.value != "" {
				values[SettingKeyLeaderboardDisplayTopN] = tt.value
			}
			repo := &leaderboardDisplaySettingRepo{values: values}
			svc := NewSettingService(repo, &config.Config{})
			require.Equal(t, tt.want, svc.GetLeaderboardDisplayTopN(context.Background()))
		})
	}
}

func TestSettingServicePersistsLeaderboardDisplayTopN(t *testing.T) {
	repo := &leaderboardDisplaySettingRepo{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	require.NoError(t, svc.UpdateSettings(context.Background(), &SystemSettings{
		LeaderboardDisplayTopN: 50,
	}))
	require.Equal(t, "50", repo.updates[SettingKeyLeaderboardDisplayTopN])
}
