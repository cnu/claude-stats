package search

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSession writes a JSONL fixture and ingests it into the database so it
// has a sessions row pointing at the file.
func writeSession(t *testing.T, database *db.DB, dir, sessionID string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, sessionID+".jsonl")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644))

	msgs, err := parser.ParseFile(path)
	require.NoError(t, err)
	require.NoError(t, database.IngestSession(
		parser.SessionFile{Path: path, SessionID: sessionID, Size: 1, ModTime: time.Now()},
		msgs,
	))
	return path
}

func userLine(sessionID, uuid, ts, cwd, text string) string {
	return fmt.Sprintf(`{"sessionId":%q,"uuid":%q,"timestamp":%q,"type":"user","cwd":%q,"message":{"role":"user","content":[{"type":"text","text":%q}]}}`,
		sessionID, uuid, ts, cwd, text)
}

func assistantLine(sessionID, uuid, ts, text string) string {
	return fmt.Sprintf(`{"sessionId":%q,"uuid":%q,"timestamp":%q,"type":"assistant","message":{"role":"assistant","model":"claude-sonnet-4-6-20250925","content":[{"type":"text","text":%q}]}}`,
		sessionID, uuid, ts, text)
}

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.OpenMemory()
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestRun_MatchBeyondPreview(t *testing.T) {
	database := newTestDB(t)
	dir := t.TempDir()

	// Keyword only appears after 200 bytes of padding, in the third message,
	// so it is absent from every stored content_preview.
	longText := strings.Repeat("padding ", 40) + "the secret flamingo appears here"
	writeSession(t, database, dir, "sess-deep", []string{
		userLine("sess-deep", "m1", "2026-07-01T10:00:00.000Z", "/home/u/Projects/webapp", "start"),
		assistantLine("sess-deep", "m2", "2026-07-01T10:00:01.000Z", "ok"),
		assistantLine("sess-deep", "m3", "2026-07-01T10:00:02.000Z", longText),
	})

	results, err := Run(database, Options{Keyword: "flamingo"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "sess-deep", results[0].SessionID)
	assert.Equal(t, 1, results[0].MatchCount)
	assert.Equal(t, "assistant", results[0].SnippetRole)
	assert.Contains(t, results[0].Snippet, "flamingo")
}

func TestRun_MatchInSubagentFile(t *testing.T) {
	database := newTestDB(t)
	dir := t.TempDir()

	writeSession(t, database, dir, "sess-sub", []string{
		userLine("sess-sub", "m1", "2026-07-01T10:00:00.000Z", "/home/u/Projects/webapp", "main transcript"),
	})
	subLine := assistantLine("sess-sub", "sa1", "2026-07-01T10:01:00.000Z", "subagent found the walrus")
	subDir := filepath.Join(dir, "sess-sub", "subagents")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "agent-1.jsonl"), []byte(subLine+"\n"), 0o644))

	results, err := Run(database, Options{Keyword: "walrus"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "sess-sub", results[0].SessionID)
	assert.Contains(t, results[0].Snippet, "walrus")
}

func TestRun_MatchInThinkingBlock(t *testing.T) {
	database := newTestDB(t)
	dir := t.TempDir()

	line := `{"sessionId":"sess-think","uuid":"t1","timestamp":"2026-07-01T10:00:00.000Z","type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"pondering the axolotl deeply"},{"type":"text","text":"done"}]}}`
	writeSession(t, database, dir, "sess-think", []string{line})

	results, err := Run(database, Options{Keyword: "axolotl"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Snippet, "axolotl")
}

func TestRun_MatchInToolInput(t *testing.T) {
	database := newTestDB(t)
	dir := t.TempDir()

	line := `{"sessionId":"sess-tool","uuid":"tl1","timestamp":"2026-07-01T10:00:00.000Z","type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu1","name":"Read","input":{"file_path":"/repo/internal/pangolin/handler.go"}}]}}`
	writeSession(t, database, dir, "sess-tool", []string{line})

	results, err := Run(database, Options{Keyword: "pangolin"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Snippet, "pangolin")
}

func TestRun_CaseSensitivity(t *testing.T) {
	database := newTestDB(t)
	dir := t.TempDir()

	writeSession(t, database, dir, "sess-case", []string{
		userLine("sess-case", "m1", "2026-07-01T10:00:00.000Z", "/home/u/Projects/webapp", "Deploy the Kubernetes cluster"),
	})

	// Case-insensitive by default.
	results, err := Run(database, Options{Keyword: "kubernetes"})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Case-sensitive misses differing case.
	results, err = Run(database, Options{Keyword: "kubernetes", CaseSensitive: true})
	require.NoError(t, err)
	assert.Empty(t, results)

	results, err = Run(database, Options{Keyword: "Kubernetes", CaseSensitive: true})
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestRun_KeywordWithQuoteBypassesPreFilter(t *testing.T) {
	database := newTestDB(t)
	dir := t.TempDir()

	// The stored JSON escapes the quotes, so a raw-byte scan for `say "hi"`
	// would never match; the matcher must fall back to full parsing.
	writeSession(t, database, dir, "sess-quote", []string{
		userLine("sess-quote", "m1", "2026-07-01T10:00:00.000Z", "/home/u/Projects/webapp", `please say "hi" to the team`),
	})

	results, err := Run(database, Options{Keyword: `say "hi"`})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Snippet, `say "hi"`)
}

func TestRun_NoMatch(t *testing.T) {
	database := newTestDB(t)
	dir := t.TempDir()

	writeSession(t, database, dir, "sess-a", []string{
		userLine("sess-a", "m1", "2026-07-01T10:00:00.000Z", "/home/u/Projects/webapp", "nothing interesting"),
	})

	results, err := Run(database, Options{Keyword: "quokka"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestRun_ProjectFilter(t *testing.T) {
	database := newTestDB(t)
	dir := t.TempDir()

	writeSession(t, database, dir, "sess-web", []string{
		userLine("sess-web", "m1", "2026-07-01T10:00:00.000Z", "/home/u/Projects/webapp", "shared keyword ocelot"),
	})
	writeSession(t, database, dir, "sess-cli", []string{
		userLine("sess-cli", "m1", "2026-07-01T11:00:00.000Z", "/home/u/Projects/cli-tool", "shared keyword ocelot"),
	})

	results, err := Run(database, Options{Keyword: "ocelot"})
	require.NoError(t, err)
	assert.Len(t, results, 2)

	results, err = Run(database, Options{Keyword: "ocelot", Project: "Projects/webapp"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "sess-web", results[0].SessionID)
}

func TestRun_MissingSourceFileSkipped(t *testing.T) {
	database := newTestDB(t)
	dir := t.TempDir()

	path := writeSession(t, database, dir, "sess-gone", []string{
		userLine("sess-gone", "m1", "2026-07-01T10:00:00.000Z", "/home/u/Projects/webapp", "vanishing ibex"),
	})
	writeSession(t, database, dir, "sess-here", []string{
		userLine("sess-here", "m1", "2026-07-01T11:00:00.000Z", "/home/u/Projects/webapp", "living ibex"),
	})
	require.NoError(t, os.Remove(path))

	results, err := Run(database, Options{Keyword: "ibex"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "sess-here", results[0].SessionID)
}

func TestRun_MalformedLinesSkipped(t *testing.T) {
	database := newTestDB(t)
	dir := t.TempDir()

	writeSession(t, database, dir, "sess-mal", []string{
		userLine("sess-mal", "m1", "2026-07-01T10:00:00.000Z", "/home/u/Projects/webapp", "first message"),
		`{"this is not valid json`,
		assistantLine("sess-mal", "m2", "2026-07-01T10:00:02.000Z", "the tapir survives malformed neighbors"),
	})

	results, err := Run(database, Options{Keyword: "tapir"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Snippet, "tapir")
}

func TestRun_MatchCountSortAndLimit(t *testing.T) {
	database := newTestDB(t)
	dir := t.TempDir()

	// Older session: keyword in 2 messages (and twice within one message —
	// counts once per message).
	writeSession(t, database, dir, "sess-old", []string{
		userLine("sess-old", "m1", "2026-07-01T10:00:00.000Z", "/home/u/Projects/webapp", "lemur lemur in one message"),
		assistantLine("sess-old", "m2", "2026-07-01T10:00:01.000Z", "another lemur here"),
	})
	// Newer session: keyword in 1 message.
	writeSession(t, database, dir, "sess-new", []string{
		userLine("sess-new", "m1", "2026-07-02T10:00:00.000Z", "/home/u/Projects/webapp", "a single lemur"),
	})

	results, err := Run(database, Options{Keyword: "lemur"})
	require.NoError(t, err)
	require.Len(t, results, 2)
	// Recency sort: newest first despite fewer matches.
	assert.Equal(t, "sess-new", results[0].SessionID)
	assert.Equal(t, 1, results[0].MatchCount)
	assert.Equal(t, "sess-old", results[1].SessionID)
	assert.Equal(t, 2, results[1].MatchCount)

	results, err = Run(database, Options{Keyword: "lemur", Limit: 1})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "sess-new", results[0].SessionID)
}

func TestRun_EmptyKeyword(t *testing.T) {
	database := newTestDB(t)

	_, err := Run(database, Options{Keyword: ""})
	require.Error(t, err)
}

func TestMakeSnippet(t *testing.T) {
	t.Run("short string untouched", func(t *testing.T) {
		s := "hello flamingo world"
		idx := strings.Index(s, "flamingo")
		snippet := makeSnippet(s, idx, len("flamingo"))
		assert.Equal(t, "hello flamingo world", snippet)
	})

	t.Run("keyword stays near the front with ellipses", func(t *testing.T) {
		s := strings.Repeat("x", 100) + " flamingo " + strings.Repeat("y", 100)
		idx := strings.Index(s, "flamingo")
		snippet := makeSnippet(s, idx, len("flamingo"))
		assert.True(t, strings.HasPrefix(snippet, "…"))
		assert.True(t, strings.HasSuffix(snippet, "…"))
		pos := strings.Index(snippet, "flamingo")
		require.GreaterOrEqual(t, pos, 0)
		assert.LessOrEqual(t, pos, 20, "keyword must appear early so table truncation keeps it visible")
	})

	t.Run("whitespace collapsed", func(t *testing.T) {
		s := "line one\n\n\tflamingo\t \nline two"
		idx := strings.Index(s, "flamingo")
		snippet := makeSnippet(s, idx, len("flamingo"))
		assert.Equal(t, "line one flamingo line two", snippet)
	})

	t.Run("multibyte text does not panic", func(t *testing.T) {
		s := "日本語のテキスト flamingo 日本語のテキスト"
		idx := strings.Index(s, "flamingo")
		snippet := makeSnippet(s, idx, len("flamingo"))
		assert.Contains(t, snippet, "flamingo")
	})
}
