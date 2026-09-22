package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type GroupDynamicRateRepository interface {
	SyncGroupDynamicRates(context.Context, time.Time) error
}

func (s *DashboardAggregationService) runScheduledGroupDynamicRates() {
	repo, ok := s.repo.(GroupDynamicRateRepository)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	release, acquired := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, "groups:dynamic-rate", s.instanceID, 3*time.Minute)
	if !acquired {
		return
	}
	defer release()
	if err := repo.SyncGroupDynamicRates(ctx, time.Now()); err != nil {
		logger.LegacyPrintf("service.group_dynamic_rate", "dynamic rate refresh failed; retaining published rates: %v", err)
	}
}
