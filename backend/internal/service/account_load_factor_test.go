//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 调度负载一律按并发数算（2026-09-28 P5 删了渠道级负载因子），并发数不大于 0 时按 1。

func TestEffectiveLoadFactor_NilAccount(t *testing.T) {
	var a *Account
	require.Equal(t, 1, a.EffectiveLoadFactor())
}

func TestEffectiveLoadFactor_PositiveConcurrency(t *testing.T) {
	a := &Account{Concurrency: 5}
	require.Equal(t, 5, a.EffectiveLoadFactor())
}

func TestEffectiveLoadFactor_ZeroConcurrency(t *testing.T) {
	a := &Account{Concurrency: 0}
	require.Equal(t, 1, a.EffectiveLoadFactor())
}

func TestEffectiveLoadFactor_NegativeConcurrency(t *testing.T) {
	a := &Account{Concurrency: -2}
	require.Equal(t, 1, a.EffectiveLoadFactor())
}
