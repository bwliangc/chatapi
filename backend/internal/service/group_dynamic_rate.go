package service

import (
	"math"
	"net/http"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type GroupDynamicRate = domain.GroupDynamicRate

func NormalizeGroupDynamicRate(cfg GroupDynamicRate) (GroupDynamicRate, error) {
	if !cfg.Enabled {
		return GroupDynamicRate{}, nil
	}
	if math.IsNaN(cfg.Min) || math.IsInf(cfg.Min, 0) || math.IsNaN(cfg.Max) || math.IsInf(cfg.Max, 0) ||
		cfg.Min < 0 || cfg.Max < cfg.Min || cfg.Max > 999999.9999 ||
		math.Abs(cfg.Min*10000-math.Round(cfg.Min*10000)) > 1e-6 ||
		math.Abs(cfg.Max*10000-math.Round(cfg.Max*10000)) > 1e-6 {
		return cfg, infraerrors.New(http.StatusBadRequest, "INVALID_DYNAMIC_RATE", "dynamic rate requires 0 <= min <= max <= 999999.9999 with at most 4 decimal places")
	}
	return cfg, nil
}

// CalculateGroupDynamicRate maps current hourly usage between the historical
// P20 and P90. The caller supplies complete hourly buckets, including zeroes.
// Each five-minute publication moves 25% towards the target to avoid jumps.
// Insufficient history retains the current rate, clamped to the configured range.
func CalculateGroupDynamicRate(cfg GroupDynamicRate, previous, current float64, history []float64) float64 {
	if !cfg.Enabled {
		return previous
	}
	clamp := func(v float64) float64 { return math.Max(cfg.Min, math.Min(cfg.Max, v)) }
	previous = clamp(previous)
	if len(history) < 24 {
		return previous
	}
	values := append([]float64(nil), history...)
	sort.Float64s(values)
	quantile := func(p float64) float64 {
		pos := p * float64(len(values)-1)
		lo := int(pos)
		hi := int(math.Ceil(pos))
		return values[lo] + (values[hi]-values[lo])*(pos-float64(lo))
	}
	low, high := quantile(.2), quantile(.9)
	// Sparse or constant history still needs a positive scale. Use historical
	// maximum for sparse traffic, and zero-to-2x for constant nonzero traffic.
	if high <= low {
		low, high = 0, math.Max(values[len(values)-1], 2*high)
	}
	weight := 0.0
	if high > low {
		weight = math.Max(0, math.Min(1, (current-low)/(high-low)))
	} else if current > 0 {
		weight = 1
	}
	target := cfg.Min + (cfg.Max-cfg.Min)*weight
	return clamp(math.Round((previous+.25*(target-previous))*10000) / 10000)
}

func clampGroupDynamicRate(cfg GroupDynamicRate, v float64) float64 {
	return math.Max(cfg.Min, math.Min(cfg.Max, v))
}
