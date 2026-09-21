//go:build unit

package service

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func timeConfig(periods ...TimePricingPeriod) *TimePricing {
	return &TimePricing{Timezone: "Asia/Shanghai", Periods: periods}
}

func onePeriod() []TimePricingPeriod {
	return []TimePricingPeriod{{StartTime: "09:00", EndTime: "12:00", Multiplier: 2}}
}

func TestValidateChannelTimePricing(t *testing.T) {
	tests := []struct {
		name    string
		config  *TimePricing
		wantErr string
	}{
		{name: "nil disabled", config: nil},
		{name: "empty disabled", config: &TimePricing{Timezone: "Asia/Shanghai"}},
		{name: "adjacent", config: timeConfig(
			TimePricingPeriod{StartTime: "09:00", EndTime: "12:00", Multiplier: 2},
			TimePricingPeriod{StartTime: "12:00", EndTime: "14:00", Multiplier: 1.5})},
		{name: "midnight split", config: timeConfig(
			TimePricingPeriod{StartTime: "22:00", EndTime: "00:00", Multiplier: 2},
			TimePricingPeriod{StartTime: "00:00", EndTime: "02:00", Multiplier: 2})},
		{name: "second precision", config: timeConfig(
			TimePricingPeriod{StartTime: "09:00:00", EndTime: "12:00:00", Multiplier: 2},
			TimePricingPeriod{StartTime: "14:00:00", EndTime: "18:00:00", Multiplier: 2})},
		{name: "second precision overlap", config: timeConfig(
			TimePricingPeriod{StartTime: "09:00:00", EndTime: "12:00:00", Multiplier: 2},
			TimePricingPeriod{StartTime: "11:59:59", EndTime: "14:00:00", Multiplier: 2}), wantErr: "overlap"},
		{name: "empty timezone", config: &TimePricing{Periods: onePeriod()}, wantErr: "timezone"},
		{name: "whitespace timezone", config: &TimePricing{Timezone: "  ", Periods: onePeriod()}, wantErr: "timezone"},
		{name: "timezone", config: &TimePricing{Timezone: "UTC+8", Periods: onePeriod()}, wantErr: "timezone"},
		{name: "format", config: timeConfig(TimePricingPeriod{StartTime: "9:00", EndTime: "12:00", Multiplier: 2}), wantErr: "HH:mm"},
		{name: "equal midnight", config: timeConfig(TimePricingPeriod{StartTime: "00:00", EndTime: "00:00", Multiplier: 2}), wantErr: "before"},
		{name: "cross midnight", config: timeConfig(TimePricingPeriod{StartTime: "22:00", EndTime: "02:00", Multiplier: 2}), wantErr: "before"},
		{name: "overlap", config: timeConfig(
			TimePricingPeriod{StartTime: "09:00", EndTime: "12:00", Multiplier: 2},
			TimePricingPeriod{StartTime: "11:59", EndTime: "14:00", Multiplier: 2}), wantErr: "overlap"},
		{name: "zero", config: timeConfig(TimePricingPeriod{StartTime: "09:00", EndTime: "12:00", Multiplier: 0}), wantErr: "greater than 0"},
		{name: "minimum positive", config: timeConfig(TimePricingPeriod{StartTime: "09:00", EndTime: "12:00", Multiplier: 0.01})},
		{name: "tiny positive", config: timeConfig(TimePricingPeriod{StartTime: "09:00", EndTime: "12:00", Multiplier: 1e-12}), wantErr: "at least 0.01"},
		{name: "below minimum", config: timeConfig(TimePricingPeriod{StartTime: "09:00", EndTime: "12:00", Multiplier: 0.001}), wantErr: "at least 0.01"},
		{name: "three decimals", config: timeConfig(TimePricingPeriod{StartTime: "09:00", EndTime: "12:00", Multiplier: 1.001}), wantErr: "decimal"},
		{name: "scaled overflow", config: timeConfig(TimePricingPeriod{StartTime: "09:00", EndTime: "12:00", Multiplier: math.MaxFloat64}), wantErr: "finite"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTimePricing(tt.config)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.True(t, strings.Contains(err.Error(), tt.wantErr), "error %q does not contain %q", err, tt.wantErr)
		})
	}
}

func TestChannelTimePricingMultiplierAt(t *testing.T) {
	config := timeConfig(TimePricingPeriod{StartTime: "09:00", EndTime: "12:00", Multiplier: 2})
	tests := []struct {
		name string
		at   time.Time
		want float64
	}{
		{name: "Shanghai 08:59", at: time.Date(2026, 6, 29, 0, 59, 0, 0, time.UTC), want: 1},
		{name: "Shanghai 09:00", at: time.Date(2026, 6, 29, 1, 0, 0, 0, time.UTC), want: 2},
		{name: "Shanghai 11:59", at: time.Date(2026, 6, 29, 3, 59, 0, 0, time.UTC), want: 2},
		{name: "Shanghai 12:00", at: time.Date(2026, 6, 29, 4, 0, 0, 0, time.UTC), want: 1},
		{name: "Shanghai 14:00", at: time.Date(2026, 6, 29, 6, 0, 0, 0, time.UTC), want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, config.MultiplierAt(tt.at))
		})
	}

	newYork := &TimePricing{Timezone: "America/New_York", Periods: onePeriod()}
	at := time.Date(2026, 6, 29, 14, 0, 0, 0, time.UTC)
	require.Equal(t, 1.0, config.MultiplierAt(at))
	require.Equal(t, 2.0, newYork.MultiplierAt(at))
}

func TestChannelTimePricingMultiplierAtWeekdaysOnly(t *testing.T) {
	config := &TimePricing{
		Timezone:     "Asia/Shanghai",
		WeekdaysOnly: true,
		Periods:      onePeriod(),
	}

	tests := []struct {
		name string
		at   time.Time
		want float64
	}{
		{name: "Monday in configured timezone", at: time.Date(2026, 6, 29, 1, 0, 0, 0, time.UTC), want: 2},
		{name: "Saturday in configured timezone", at: time.Date(2026, 7, 4, 1, 0, 0, 0, time.UTC), want: 1},
		{name: "Sunday in configured timezone", at: time.Date(2026, 7, 5, 1, 0, 0, 0, time.UTC), want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, config.MultiplierAt(tt.at))
		})
	}
}

func TestChannelTimePricingMultiplierAtSecondPrecision(t *testing.T) {
	config := timeConfig(TimePricingPeriod{StartTime: "09:00:30", EndTime: "09:00:45", Multiplier: 2})
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	tests := []struct {
		name string
		at   time.Time
		want float64
	}{
		{name: "before", at: time.Date(2026, 6, 29, 9, 0, 29, 0, shanghai), want: 1},
		{name: "start", at: time.Date(2026, 6, 29, 9, 0, 30, 0, shanghai), want: 2},
		{name: "last matching second", at: time.Date(2026, 6, 29, 9, 0, 44, 999_999_999, shanghai), want: 2},
		{name: "end", at: time.Date(2026, 6, 29, 9, 0, 45, 0, shanghai), want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, config.MultiplierAt(tt.at))
		})
	}
}

func TestChannelTimePricingMultiplierAtMidnightSplit(t *testing.T) {
	config := timeConfig(
		TimePricingPeriod{StartTime: "22:00", EndTime: "00:00", Multiplier: 2},
		TimePricingPeriod{StartTime: "00:00", EndTime: "02:00", Multiplier: 3},
	)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	tests := []struct {
		name string
		at   time.Time
		want float64
	}{
		{name: "23:59", at: time.Date(2026, 6, 29, 23, 59, 0, 0, shanghai), want: 2},
		{name: "next day 00:00", at: time.Date(2026, 6, 30, 0, 0, 0, 0, shanghai), want: 3},
		{name: "02:00", at: time.Date(2026, 6, 30, 2, 0, 0, 0, shanghai), want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, config.MultiplierAt(tt.at))
		})
	}
}

func TestChannelTimePricingMultiplierAtDegradesForInvalidConfigurations(t *testing.T) {
	var nilConfig *TimePricing
	zeroTime := time.Time{}
	validAt := time.Date(2026, 6, 29, 1, 0, 0, 0, time.UTC)

	require.Equal(t, 1.0, nilConfig.MultiplierAt(validAt))
	require.Equal(t, 1.0, timeConfig().MultiplierAt(validAt))
	require.Equal(t, 1.0, timeConfig(TimePricingPeriod{StartTime: "09:00", EndTime: "12:00", Multiplier: 2}).MultiplierAt(zeroTime))
	require.Equal(t, 1.0, (&TimePricing{Periods: onePeriod()}).MultiplierAt(validAt))
	require.Equal(t, 1.0, (&TimePricing{Timezone: "  ", Periods: onePeriod()}).MultiplierAt(validAt))
	require.Equal(t, 1.0, (&TimePricing{Timezone: "UTC+8", Periods: onePeriod()}).MultiplierAt(validAt))
	require.Equal(t, 1.0, timeConfig(TimePricingPeriod{StartTime: "22:00", EndTime: "02:00", Multiplier: 2}).MultiplierAt(validAt))
}

func TestChannelTimePricingRejectsLocalTimezone(t *testing.T) {
	config := &TimePricing{Timezone: "Local", Periods: onePeriod()}

	err := validateTimePricing(config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "timezone")
	require.Equal(t, 1.0, config.MultiplierAt(time.Date(2026, 6, 29, 1, 0, 0, 0, time.UTC)))
}
