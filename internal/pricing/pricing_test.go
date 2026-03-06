package pricing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookupPricing_ExactMatch(t *testing.T) {
	tests := []struct {
		model string
		input float64
	}{
		{"claude-opus-4-20250514", 15.0},
		{"claude-sonnet-4-20250514", 3.0},
		{"claude-sonnet-4-6-20250925", 3.0},
		{"claude-opus-4-6-20250918", 15.0},
		{"claude-haiku-4-5-20251001", 0.80},
	}

	for _, tt := range tests {
		p := LookupPricing(tt.model)
		assert.Equal(t, tt.input, p.InputPerMTok, "model: %s", tt.model)
	}
}

func TestLookupPricing_ShortName(t *testing.T) {
	tests := []struct {
		short string
		input float64
	}{
		{"claude-opus-4-6", 15.0},
		{"claude-sonnet-4-6", 3.0},
		{"claude-haiku-4-5", 0.80},
		{"claude-opus-4", 15.0},
		{"claude-sonnet-4", 3.0},
	}

	for _, tt := range tests {
		p := LookupPricing(tt.short)
		assert.Equal(t, tt.input, p.InputPerMTok, "short name: %s", tt.short)
	}
}

func TestLookupPricing_UnknownModel(t *testing.T) {
	p := LookupPricing("claude-unknown-99")
	// Should fall back to Sonnet pricing
	assert.Equal(t, 3.0, p.InputPerMTok)
	assert.Equal(t, 15.0, p.OutputPerMTok)
}

func TestCalculateCost_Sonnet(t *testing.T) {
	// 1000 input tokens at $3/MTok = $0.003
	// 500 output tokens at $15/MTok = $0.0075
	// Total = $0.0105
	cost := CalculateCost("claude-sonnet-4-6-20250925", 1000, 500, 0, 0)
	assert.InDelta(t, 0.0105, cost, 0.0001)
}

func TestCalculateCost_Opus(t *testing.T) {
	// 1000 input at $15/MTok = $0.015
	// 500 output at $75/MTok = $0.0375
	// Total = $0.0525
	cost := CalculateCost("claude-opus-4-6-20250918", 1000, 500, 0, 0)
	assert.InDelta(t, 0.0525, cost, 0.0001)
}

func TestCalculateCost_WithCache(t *testing.T) {
	// Sonnet: 1000 input, 500 output, 2000 cache create, 3000 cache read
	// input: 1000 * 3.0/1M = 0.003
	// output: 500 * 15.0/1M = 0.0075
	// cache_create: 2000 * 3.75/1M = 0.0075
	// cache_read: 3000 * 0.30/1M = 0.0009
	// Total = 0.0189
	cost := CalculateCost("claude-sonnet-4-6-20250925", 1000, 500, 2000, 3000)
	assert.InDelta(t, 0.0189, cost, 0.0001)
}

func TestCalculateCost_Haiku(t *testing.T) {
	// 10000 input at $0.80/MTok = $0.008
	// 5000 output at $4.0/MTok = $0.02
	// Total = $0.028
	cost := CalculateCost("claude-haiku-4-5-20251001", 10000, 5000, 0, 0)
	assert.InDelta(t, 0.028, cost, 0.0001)
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	cost := CalculateCost("claude-sonnet-4-6-20250925", 0, 0, 0, 0)
	assert.Equal(t, 0.0, cost)
}

func TestCalculateCost_OpusWithFullCache(t *testing.T) {
	// Opus: 5000 input, 800 output, 10000 cache create, 2000 cache read
	// input: 5000 * 15.0/1M = 0.075
	// output: 800 * 75.0/1M = 0.06
	// cache_create: 10000 * 18.75/1M = 0.1875
	// cache_read: 2000 * 1.5/1M = 0.003
	// Total = 0.3255
	cost := CalculateCost("claude-opus-4-6-20250918", 5000, 800, 10000, 2000)
	assert.InDelta(t, 0.3255, cost, 0.0001)
}
