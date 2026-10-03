package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
