package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateSearchCost(t *testing.T) {
	t.Parallel()
	s := &BillingService{}
	require.Equal(t, 0.0, s.CalculateSearchCost(0, 1).ActualCost)
	// 内置单价 $5/1k：5 次 = 0.025
	require.InDelta(t, 0.025, s.CalculateSearchCost(5, 1).ActualCost, 1e-9)
	cost := s.CalculateSearchCost(100, 1.5)
	// 5 / 1000 * 100 = 0.5；× 1.5 = 0.75
	require.InDelta(t, 0.5, cost.TotalCost, 1e-9)
	require.InDelta(t, 0.75, cost.ActualCost, 1e-9)
	require.Equal(t, string(BillingModePerRequest), cost.BillingMode)
}

func TestCalculateAudioCost(t *testing.T) {
	t.Parallel()
	s := &BillingService{}
	// 内置单价：realtime $0.05/min、TTS $15/M chars、STT $0.10/hr
	require.InDelta(t, 0.10, s.CalculateAudioCost("realtime", 2, 1).ActualCost, 1e-9)
	require.InDelta(t, 1.5, s.CalculateAudioCost("tts", 0.1, 1).ActualCost, 1e-9)
	require.InDelta(t, 0.05, s.CalculateAudioCost("stt", 0.5, 1).ActualCost, 1e-9)
	require.Equal(t, 0.0, s.CalculateAudioCost("unknown", 1, 1).ActualCost)
	require.Equal(t, 0.0, s.CalculateAudioCost("tts", 0, 1).ActualCost)
	cost := s.CalculateAudioCost("tts", 1, 2)
	require.InDelta(t, 15.0, cost.TotalCost, 1e-9)
	require.InDelta(t, 30.0, cost.ActualCost, 1e-9)
}

func TestCalculateWebSearchCost(t *testing.T) {
	t.Parallel()
	s := &BillingService{}
	require.Equal(t, 0.0, s.CalculateWebSearchCost(0, nil, 1).ActualCost)
	// 条目未配 search_price_per_call → 内置单价 $0.01/次
	require.InDelta(t, 0.03, s.CalculateWebSearchCost(3, nil, 1).ActualCost, 1e-9)
	// 条目价覆盖内置单价
	price := 0.02
	cost := s.CalculateWebSearchCost(3, &price, 1.5)
	require.InDelta(t, 0.06, cost.TotalCost, 1e-9)
	require.InDelta(t, 0.09, cost.ActualCost, 1e-9)
	// 条目价显式 0 → 免费
	zero := 0.0
	require.Equal(t, 0.0, s.CalculateWebSearchCost(3, &zero, 1).ActualCost)
}
