package domain

// GroupDynamicRate configures the group-wide demand-based base multiplier.
type GroupDynamicRate struct {
	Enabled bool    `json:"enabled"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
}
