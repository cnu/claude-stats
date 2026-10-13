package pricing

import (
	"log/slog"
	"regexp"
	"strings"
	"sync"
)

// ModelPricing holds per-model token pricing in USD per million tokens.
type ModelPricing struct {
	InputPerMTok      float64
	OutputPerMTok     float64
	CacheWritePerMTok float64
	CacheReadPerMTok  float64
}

// Source describes where the active pricing table came from.
type Source string

// Pricing table sources.
const (
	SourceEmbedded Source = "embedded"
	SourceRemote   Source = "remote"
	SourceCache    Source = "cache"
)

// mtok builds a ModelPricing with the standard 5-minute cache write rate (1.25x input).
func mtok(input, output, cacheRead float64) ModelPricing {
	return ModelPricing{
		InputPerMTok:      input,
		OutputPerMTok:     output,
		CacheWritePerMTok: input * 1.25,
		CacheReadPerMTok:  cacheRead,
	}
}

// defaultPricing is the embedded pricing table, keyed by model ID without date suffix.
// It is used when remote pricing is unavailable, and for models missing from remote data.
var defaultPricing = map[string]ModelPricing{
	// Fable / Mythos
	"claude-fable-5-1":  mtok(10.0, 50.0, 0.25),
	"claude-fable-5":    mtok(10.0, 50.0, 1.0),
	"claude-mythos-5-1": mtok(10.0, 50.0, 0.25),
	"claude-mythos-5":   mtok(10.0, 50.0, 1.0),

	// Opus
	"claude-opus-5-5": mtok(4.0, 20.0, 0.20),
	"claude-opus-5":   mtok(5.0, 25.0, 0.50),
	"claude-opus-4-8": mtok(5.0, 25.0, 0.50),
	"claude-opus-4-7": mtok(5.0, 25.0, 0.50),
	"claude-opus-4-6": mtok(5.0, 25.0, 0.50),
	"claude-opus-4-5": mtok(5.0, 25.0, 0.50),
	"claude-opus-4-1": mtok(15.0, 75.0, 1.5),
	"claude-opus-4":   mtok(15.0, 75.0, 1.5),

	// Sonnet
	"claude-sonnet-5-5": mtok(2.0, 10.0, 0.20),
	"claude-sonnet-5":   mtok(2.0, 10.0, 0.20),
	"claude-sonnet-4-6": mtok(3.0, 15.0, 0.30),
	"claude-sonnet-4-5": mtok(3.0, 15.0, 0.30),
	"claude-sonnet-4":   mtok(3.0, 15.0, 0.30),
	"claude-3-7-sonnet": mtok(3.0, 15.0, 0.30),
	"claude-3-5-sonnet": mtok(3.0, 15.0, 0.30),

	// Haiku (Haiku 5.5 prices are for prompts up to 100K tokens)
	"claude-haiku-5-5": mtok(0.10, 0.50, 0.01),
	"claude-haiku-4-5": mtok(1.0, 5.0, 0.10),
	"claude-3-5-haiku": mtok(0.80, 4.0, 0.08),
}

// familyFallback maps a model family to the model whose pricing is used
// for unknown models of that family.
var familyFallback = map[string]string{
	"fable":  "claude-fable-5-1",
	"mythos": "claude-mythos-5-1",
	"opus":   "claude-opus-5-5",
	"sonnet": "claude-sonnet-5-5",
	"haiku":  "claude-haiku-5-5",
}

const fallbackModel = "claude-sonnet-5-5"

// zeroCostModels are synthetic or internal model names that have no real API cost.
var zeroCostModels = map[string]bool{
	"<synthetic>": true,
}

var (
	mu          sync.RWMutex
	overrides   map[string]ModelPricing
	activeSrc   = SourceEmbedded
	warnedMu    sync.Mutex
	warnedModel = map[string]bool{}
)

// SetOverrides installs a pricing table (for example, fetched remotely) that
// takes precedence over the embedded table. Keys must be normalized model IDs.
func SetOverrides(table map[string]ModelPricing, src Source) {
	mu.Lock()
	defer mu.Unlock()
	overrides = table
	activeSrc = src
}

// ActiveSource returns where the current pricing table came from.
func ActiveSource() Source {
	mu.RLock()
	defer mu.RUnlock()
	return activeSrc
}

var (
	dateSuffix    = regexp.MustCompile(`-\d{8}$`)
	versionSuffix = regexp.MustCompile(`-v\d+(:\d+)?$`)
	bracketSuffix = regexp.MustCompile(`\[[^\]]*\]$`)
)

// NormalizeModel strips provider prefixes, context-window tags, version and
// date suffixes so that e.g. "us.anthropic.claude-opus-4-5-20251101-v1:0" and
// "claude-opus-4-5[1m]" both become "claude-opus-4-5".
func NormalizeModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	m = bracketSuffix.ReplaceAllString(m, "")
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	if i := strings.Index(m, "anthropic."); i >= 0 {
		m = m[i+len("anthropic."):]
	}
	if i := strings.Index(m, "@"); i >= 0 {
		m = m[:i]
	}
	m = versionSuffix.ReplaceAllString(m, "")
	m = dateSuffix.ReplaceAllString(m, "")
	return m
}

func lookup(name string) (ModelPricing, bool) {
	mu.RLock()
	p, ok := overrides[name]
	mu.RUnlock()
	if ok {
		return p, true
	}
	p, ok = defaultPricing[name]
	return p, ok
}

// LookupPricing returns the pricing for a given model name.
// Unknown models fall back to the newest model of the same family
// (opus, sonnet, haiku, ...), or to Sonnet pricing when no family matches.
// The fallback warning is logged once per model name.
func LookupPricing(model string) ModelPricing {
	// Zero-cost synthetic models
	if zeroCostModels[model] {
		return ModelPricing{}
	}

	if p, ok := lookup(model); ok {
		return p
	}

	name := NormalizeModel(model)
	if p, ok := lookup(name); ok {
		return p
	}

	fallback := fallbackModel
	for family, m := range familyFallback {
		if strings.Contains(name, family) {
			fallback = m
			break
		}
	}

	warnOnce(model, fallback)
	p, _ := lookup(fallback)
	return p
}

func warnOnce(model, fallback string) {
	warnedMu.Lock()
	defer warnedMu.Unlock()
	if warnedModel[model] {
		return
	}
	warnedModel[model] = true
	slog.Warn("unknown model, using fallback pricing", "model", model, "fallback", fallback)
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
