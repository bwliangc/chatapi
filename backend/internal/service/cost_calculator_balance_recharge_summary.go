package service

type CostCalculatorBalanceRechargePackageStat struct {
	BalanceAmount float64 `json:"balance_amount"`
	ActualAmount  float64 `json:"actual_amount"`
	Count         int64   `json:"count"`
	BalanceTotal  float64 `json:"balance_total"`
	ActualTotal   float64 `json:"actual_total"`
}

type CostCalculatorBalanceRechargeSummary struct {
	TotalAmount             float64                                    `json:"total_amount"`
	RedeemAmount            float64                                    `json:"redeem_amount"`
	AdminNetAmount          float64                                    `json:"admin_net_amount"`
	AdminAddedAmount        float64                                    `json:"admin_added_amount"`
	AdminDeductedAmount     float64                                    `json:"admin_deducted_amount"`
	ActualRevenue           float64                                    `json:"actual_revenue"`
	RedeemActualRevenue     float64                                    `json:"redeem_actual_revenue"`
	AdminActualRevenue      float64                                    `json:"admin_actual_revenue"`
	MatchedBalanceAmount    float64                                    `json:"matched_balance_amount"`
	UnmatchedBalanceAmount  float64                                    `json:"unmatched_balance_amount"`
	MatchedRecordCount      int64                                      `json:"matched_record_count"`
	UnmatchedRecordCount    int64                                      `json:"unmatched_record_count"`
	LeaderboardRewardAmount float64                                    `json:"leaderboard_reward_amount"`
	LeaderboardRewardCount  int64                                      `json:"leaderboard_reward_count"`
	RecordCount             int64                                      `json:"record_count"`
	RedeemCount             int64                                      `json:"redeem_count"`
	AdminCount              int64                                      `json:"admin_count"`
	PackageStats            []CostCalculatorBalanceRechargePackageStat `json:"package_stats"`
}

type CostCalculatorBalanceLiabilitySummary struct {
	TotalBalance             float64 `json:"total_balance"`
	PositiveUserCount        int64   `json:"positive_user_count"`
	EstimatedActualLiability float64 `json:"estimated_actual_liability"`
	EstimatedUnitCost        float64 `json:"estimated_unit_cost"`
	ValuationSource          string  `json:"valuation_source"`
	MatchedBalanceAmount     float64 `json:"matched_balance_amount"`
	MatchedActualRevenue     float64 `json:"matched_actual_revenue"`
	UnmatchedBalanceAmount   float64 `json:"unmatched_balance_amount"`
	LeaderboardRewardAmount  float64 `json:"leaderboard_reward_amount"`
	LeaderboardRewardCount   int64   `json:"leaderboard_reward_count"`
}
