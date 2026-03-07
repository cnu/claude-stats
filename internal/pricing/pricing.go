package pricing

import "log/slog"

// ModelPricing holds per-model token pricing in USD per million tokens.
type ModelPricing struct {
	InputPerMTok      float64
	OutputPerMTok     float64
	CacheWritePerMTok float64
	CacheReadPerMTok  float64
}

// defaultPricing is the embedded pricing table for known models.
var defaultPricing = map[string]ModelPricing{
	"claude-opus-4-20250514": {
		InputPerMTok:      15.0,
		OutputPerMTok:     75.0,
		CacheWritePerMTok: 18.75,
		CacheReadPerMTok:  1.5,
	},
	"claude-sonnet-4-20250514": {
		InputPerMTok:      3.0,
		OutputPerMTok:     15.0,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.30,
	},
	"claude-sonnet-4-6-20250925": {
		InputPerMTok:      3.0,
		OutputPerMTok:     15.0,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.30,
	},
	"claude-opus-4-6-20250918": {
		InputPerMTok:      15.0,
		OutputPerMTok:     75.0,
		CacheWritePerMTok: 18.75,
		CacheReadPerMTok:  1.5,
	},
	"claude-sonnet-4-5-20250929": {
		InputPerMTok:      3.0,
		OutputPerMTok:     15.0,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.30,
	},
	"claude-haiku-4-5-20251001": {
		InputPerMTok:      0.80,
		OutputPerMTok:     4.0,
		CacheWritePerMTok: 1.0,
		CacheReadPerMTok:  0.08,
	},
}

// shortNameMap maps short model names (without date suffix) to full names.
var shortNameMap = map[string]string{
	"claude-opus-4-6":   "claude-opus-4-6-20250918",
	"claude-sonnet-4-6": "claude-sonnet-4-6-20250925",
	"claude-haiku-4-5":  "claude-haiku-4-5-20251001",
	"claude-sonnet-4-5": "claude-sonnet-4-5-20250929",
	"claude-opus-4":     "claude-opus-4-20250514",
	"claude-sonnet-4":   "claude-sonnet-4-20250514",
}

const fallbackModel = "claude-sonnet-4-20250514"

// zeroCostModels are synthetic or internal model names that have no real API cost.
var zeroCostModels = map[string]bool{
	"<synthetic>": true,
}

// LookupPricing returns the pricing for a given model name.
// Falls back to Sonnet pricing for unknown models.
func LookupPricing(model string) ModelPricing {
	// Zero-cost synthetic models
	if zeroCostModels[model] {
		return ModelPricing{}
	}

	// Direct match
	if p, ok := defaultPricing[model]; ok {
		return p
	}

	// Try short name mapping
	if fullName, ok := shortNameMap[model]; ok {
		if p, ok := defaultPricing[fullName]; ok {
			return p
		}
	}

	slog.Warn("unknown model, using fallback pricing", "model", model, "fallback", fallbackModel)
	return defaultPricing[fallbackModel]
}

// CalculateCost computes the cost in USD for a message given model and token counts.
func CalculateCost(model string, inputTokens, outputTokens, cacheCreateTokens, cacheReadTokens int) float64 {
	p := LookupPricing(model)

	cost := float64(inputTokens)*p.InputPerMTok/1_000_000 +
		float64(outputTokens)*p.OutputPerMTok/1_000_000 +
		float64(cacheCreateTokens)*p.CacheWritePerMTok/1_000_000 +
		float64(cacheReadTokens)*p.CacheReadPerMTok/1_000_000

	return cost
}
