package service

type CostCalculatorUsageSummary struct {
	StartDate         string                       `json:"start_date,omitempty"`
	EndDate           string                       `json:"end_date,omitempty"`
	TotalRequests     int64                        `json:"total_requests"`
	TotalTokens       int64                        `json:"total_tokens"`
	TotalStandardCost float64                      `json:"total_standard_cost"`
	TotalActualCost   float64                      `json:"total_actual_cost"`
	TotalUsageCost    float64                      `json:"total_usage_cost"`
	Groups            []CostCalculatorGroupUsage   `json:"groups"`
	Models            []CostCalculatorModelUsage   `json:"models"`
	Accounts          []CostCalculatorAccountUsage `json:"accounts"`
}

type CostCalculatorGroupUsage struct {
	GroupID      int64   `json:"group_id"`
	GroupName    string  `json:"group_name"`
	Requests     int64   `json:"requests"`
	TotalTokens  int64   `json:"total_tokens"`
	StandardCost float64 `json:"standard_cost"`
	ActualCost   float64 `json:"actual_cost"`
	UsageCost    float64 `json:"usage_cost"`
}

type CostCalculatorModelUsage struct {
	Model        string  `json:"model"`
	Requests     int64   `json:"requests"`
	TotalTokens  int64   `json:"total_tokens"`
	StandardCost float64 `json:"standard_cost"`
	ActualCost   float64 `json:"actual_cost"`
	UsageCost    float64 `json:"usage_cost"`
}

type CostCalculatorAccountUsage struct {
	AccountID    int64   `json:"account_id"`
	AccountName  string  `json:"account_name"`
	Platform     string  `json:"platform"`
	Requests     int64   `json:"requests"`
	TotalTokens  int64   `json:"total_tokens"`
	StandardCost float64 `json:"standard_cost"`
	ActualCost   float64 `json:"actual_cost"`
	UsageCost    float64 `json:"usage_cost"`
}
