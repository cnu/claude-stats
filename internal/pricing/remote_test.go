package pricing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "litellm_prices.json"))
	require.NoError(t, err)
	return raw
}

func TestParse(t *testing.T) {
	table, err := Parse(fixture(t))
	require.NoError(t, err)

	assert.Len(t, table, 2, "only first-party anthropic entries")
	assert.InDelta(t, 5.0, table["claude-opus-5"].InputPerMTok, 1e-9)
	assert.InDelta(t, 0.5, table["claude-opus-5"].CacheReadPerMTok, 1e-9)

	// Missing cache fields use the standard multipliers.
	haiku := table["claude-haiku-4-5"]
	assert.InDelta(t, 1.25, haiku.CacheWritePerMTok, 1e-9)
	assert.InDelta(t, 0.1, haiku.CacheReadPerMTok, 1e-9)
}

func TestLoad_FetchesAndCaches(t *testing.T) {
	t.Cleanup(func() { SetOverrides(nil, SourceEmbedded) })
	raw := fixture(t)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	cache := filepath.Join(t.TempDir(), "pricing.json")
	opts := LoadOptions{URL: srv.URL, CachePath: cache}

	assert.Equal(t, SourceRemote, Load(context.Background(), opts))
	assert.FileExists(t, cache)

	// A fresh cache is used without a network call.
	assert.Equal(t, SourceCache, Load(context.Background(), opts))
	assert.Equal(t, 1, hits)
}

func TestLoad_FallsBackWhenFetchFails(t *testing.T) {
	t.Cleanup(func() { SetOverrides(nil, SourceEmbedded) })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	opts := LoadOptions{URL: srv.URL, CachePath: filepath.Join(dir, "pricing.json")}
	assert.Equal(t, SourceEmbedded, Load(context.Background(), opts))

	// A stale cache beats the embedded table.
	require.NoError(t, os.WriteFile(opts.CachePath, fixture(t), 0o644))
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(opts.CachePath, old, old))
	assert.Equal(t, SourceCache, Load(context.Background(), opts))
}

func TestLoad_Offline(t *testing.T) {
	t.Cleanup(func() { SetOverrides(nil, SourceEmbedded) })
	opts := LoadOptions{URL: "http://127.0.0.1:1", CachePath: filepath.Join(t.TempDir(), "pricing.json"), Offline: true}
	assert.Equal(t, SourceEmbedded, Load(context.Background(), opts))
}
