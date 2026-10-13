package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RemoteURL is the LiteLLM model price list. It is community-maintained and
// tracks Anthropic's published API prices, including new models.
const RemoteURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

// DefaultCacheTTL is how long a downloaded price list is used before a refresh.
const DefaultCacheTTL = 24 * time.Hour

// LoadOptions controls how remote pricing is loaded.
type LoadOptions struct {
	URL       string        // defaults to RemoteURL
	CachePath string        // file to store the downloaded price list
	TTL       time.Duration // defaults to DefaultCacheTTL
	Offline   bool          // never fetch; use the cache file if present
	Client    *http.Client  // defaults to a client with a 60s timeout
}

// litellmEntry holds the fields we read from one LiteLLM model entry (USD per token).
type litellmEntry struct {
	Provider        string   `json:"litellm_provider"`
	InputPerToken   *float64 `json:"input_cost_per_token"`
	OutputPerToken  *float64 `json:"output_cost_per_token"`
	CacheWritePerTk *float64 `json:"cache_creation_input_token_cost"`
	CacheReadPerTk  *float64 `json:"cache_read_input_token_cost"`
}

// Load fetches the latest pricing (or reads a fresh cached copy) and installs it
// with SetOverrides. On any failure it falls back to a stale cache, then to the
// embedded table. It returns the source that is now active.
func Load(ctx context.Context, opts LoadOptions) Source {
	if opts.URL == "" {
		opts.URL = RemoteURL
	}
	if opts.TTL == 0 {
		opts.TTL = DefaultCacheTTL
	}
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 60 * time.Second}
	}

	cached, cacheAge, cacheErr := readCache(opts.CachePath)
	if cacheErr == nil && (opts.Offline || cacheAge < opts.TTL) {
		SetOverrides(cached, SourceCache)
		slog.Debug("using cached pricing", "path", opts.CachePath, "age", cacheAge.Round(time.Second), "models", len(cached))
		return SourceCache
	}

	if !opts.Offline {
		table, raw, err := fetch(ctx, opts.Client, opts.URL)
		if err == nil {
			SetOverrides(table, SourceRemote)
			if err := writeCache(opts.CachePath, raw); err != nil {
				slog.Debug("failed to write pricing cache", "path", opts.CachePath, "error", err)
			}
			slog.Debug("fetched remote pricing", "url", opts.URL, "models", len(table))
			return SourceRemote
		}
		slog.Warn("failed to fetch latest pricing, using built-in prices", "error", err)
	}

	if cacheErr == nil {
		SetOverrides(cached, SourceCache)
		return SourceCache
	}
	SetOverrides(nil, SourceEmbedded)
	return SourceEmbedded
}

func fetch(ctx context.Context, client *http.Client, url string) (map[string]ModelPricing, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("get %s: status %s", url, resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read body: %w", err)
	}
	table, err := Parse(raw)
	if err != nil {
		return nil, nil, err
	}
	return table, raw, nil
}

// Parse converts a LiteLLM price list into a pricing table of Anthropic models,
// keyed by normalized model ID.
func Parse(raw []byte) (map[string]ModelPricing, error) {
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("decode pricing json: %w", err)
	}

	table := make(map[string]ModelPricing)
	for key, msg := range entries {
		if !strings.Contains(key, "claude") {
			continue
		}
		var e litellmEntry
		if err := json.Unmarshal(msg, &e); err != nil {
			continue
		}
		// Bedrock/Vertex entries can have different prices; use first-party ones only.
		if e.Provider != "anthropic" || e.InputPerToken == nil || e.OutputPerToken == nil {
			continue
		}
		p := ModelPricing{
			InputPerMTok:  *e.InputPerToken * 1_000_000,
			OutputPerMTok: *e.OutputPerToken * 1_000_000,
		}
		if e.CacheWritePerTk != nil {
			p.CacheWritePerMTok = *e.CacheWritePerTk * 1_000_000
		} else {
			p.CacheWritePerMTok = p.InputPerMTok * 1.25
		}
		if e.CacheReadPerTk != nil {
			p.CacheReadPerMTok = *e.CacheReadPerTk * 1_000_000
		} else {
			p.CacheReadPerMTok = p.InputPerMTok * 0.1
		}
		table[NormalizeModel(key)] = p
	}

	if len(table) == 0 {
		return nil, fmt.Errorf("decode pricing json: no anthropic models found")
	}
	return table, nil
}

func readCache(path string) (map[string]ModelPricing, time.Duration, error) {
	if path == "" {
		return nil, 0, fmt.Errorf("no cache path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("stat pricing cache: %w", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read pricing cache: %w", err)
	}
	table, err := Parse(raw)
	if err != nil {
		return nil, 0, err
	}
	return table, time.Since(info.ModTime()), nil
}

func writeCache(path string, raw []byte) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write pricing cache: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename pricing cache: %w", err)
	}
	return nil
}
