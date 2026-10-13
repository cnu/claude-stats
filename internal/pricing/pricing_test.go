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
		{"claude-opus-4-6-20250918", 5.0},
		{"claude-haiku-4-5-20251001", 1.0},
		{"claude-opus-5", 5.0},
		{"claude-opus-5-5", 4.0},
		{"claude-sonnet-5-5", 2.0},
		{"claude-haiku-5-5", 0.10},
		{"claude-fable-5-1", 10.0},
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
		{"claude-opus-4-6", 5.0},
		{"claude-sonnet-4-6", 3.0},
		{"claude-haiku-4-5", 1.0},
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
	// Should fall back to the newest Sonnet pricing
	assert.Equal(t, 2.0, p.InputPerMTok)
	assert.Equal(t, 10.0, p.OutputPerMTok)
}

func TestLookupPricing_UnknownModelUsesFamily(t *testing.T) {
	assert.Equal(t, 4.0, LookupPricing("claude-opus-9").InputPerMTok)
	assert.Equal(t, 0.10, LookupPricing("claude-haiku-9").InputPerMTok)
}

func TestNormalizeModel(t *testing.T) {
	tests := map[string]string{
		"claude-opus-4-5-20251101":                   "claude-opus-4-5",
		"claude-opus-4-6[1m]":                        "claude-opus-4-6",
		"us.anthropic.claude-opus-4-5-20251101-v1:0": "claude-opus-4-5",
		"anthropic/claude-sonnet-4-5":                "claude-sonnet-4-5",
		"claude-sonnet-4-5@20250929":                 "claude-sonnet-4-5",
		"claude-opus-5":                              "claude-opus-5",
	}
	for in, want := range tests {
		assert.Equal(t, want, NormalizeModel(in), "input: %s", in)
	}
}

func TestSetOverrides(t *testing.T) {
	t.Cleanup(func() { SetOverrides(nil, SourceEmbedded) })
	SetOverrides(map[string]ModelPricing{"claude-opus-5": {InputPerMTok: 7}}, SourceRemote)
	assert.Equal(t, 7.0, LookupPricing("claude-opus-5").InputPerMTok)
	assert.Equal(t, 3.0, LookupPricing("claude-sonnet-4-6").InputPerMTok, "embedded table still used for missing models")
	assert.Equal(t, SourceRemote, ActiveSource())
}

func TestCalculateCost_Sonnet(t *testing.T) {
	// 1000 input tokens at $3/MTok = $0.003
	// 500 output tokens at $15/MTok = $0.0075
	// Total = $0.0105
	cost := CalculateCost("claude-sonnet-4-6-20250925", 1000, 500, 0, 0)
	assert.InDelta(t, 0.0105, cost, 0.0001)
}

func TestCalculateCost_Opus(t *testing.T) {
	// 1000 input at $5/MTok = $0.005
	// 500 output at $25/MTok = $0.0125
	// Total = $0.0175
	cost := CalculateCost("claude-opus-4-6-20250918", 1000, 500, 0, 0)
	assert.InDelta(t, 0.0175, cost, 0.0001)
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
	// 10000 input at $1.0/MTok = $0.01
	// 5000 output at $5.0/MTok = $0.025
	// Total = $0.035
	cost := CalculateCost("claude-haiku-4-5-20251001", 10000, 5000, 0, 0)
	assert.InDelta(t, 0.035, cost, 0.0001)
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	cost := CalculateCost("claude-sonnet-4-6-20250925", 0, 0, 0, 0)
	assert.Equal(t, 0.0, cost)
}

func TestCalculateCost_OpusWithFullCache(t *testing.T) {
	// Opus: 5000 input, 800 output, 10000 cache create, 2000 cache read
	// input: 5000 * 5.0/1M = 0.025
	// output: 800 * 25.0/1M = 0.02
	// cache_create: 10000 * 6.25/1M = 0.0625
	// cache_read: 2000 * 0.5/1M = 0.001
	// Total = 0.1085
	cost := CalculateCost("claude-opus-4-6-20250918", 5000, 800, 10000, 2000)
	assert.InDelta(t, 0.1085, cost, 0.0001)
}
