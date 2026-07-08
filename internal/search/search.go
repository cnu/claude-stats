// Package search implements full-transcript keyword search across ingested
// Claude Code sessions. Session metadata and source paths come from the
// SQLite database, but matching happens against the original JSONL files so
// content beyond the stored previews is searchable.
package search

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/cnu/claude-stats/internal/db"
	"github.com/cnu/claude-stats/internal/parser"
)

// Options controls a transcript search.
type Options struct {
	Keyword       string
	Project       string // exact project_name filter; empty means all projects
	CaseSensitive bool
	Limit         int // max sessions returned; <=0 means all
	Workers       int // concurrent session scanners; <=0 picks a default
}

// Result describes one session that matched the keyword.
type Result struct {
	SessionID   string `json:"session_id"`
	ProjectName string `json:"project"`
	LastMsgAt   int64  `json:"last_msg_at"` // unix milliseconds
	MatchCount  int    `json:"matches"`     // matching messages, not occurrences
	Snippet     string `json:"snippet"`     // context around the first match
	SnippetRole string `json:"role"`        // role of the first matching message
}

// Run searches all ingested sessions' transcript files (including subagent
// transcripts) for the keyword and returns matching sessions, newest first.
func Run(database *db.DB, opts Options) ([]Result, error) {
	if opts.Keyword == "" {
		return nil, fmt.Errorf("search keyword must not be empty")
	}

	sources, err := database.GetSessionSources(opts.Project)
	if err != nil {
		return nil, err
	}

	m := newMatcher(opts.Keyword, opts.CaseSensitive)

	workers := opts.Workers
	if workers <= 0 {
		workers = min(8, runtime.NumCPU())
	}

	jobs := make(chan db.SessionSource)
	var (
		mu      sync.Mutex
		results []Result
		wg      sync.WaitGroup
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for src := range jobs {
				if r, ok := searchSession(src, m); ok {
					mu.Lock()
					results = append(results, r)
					mu.Unlock()
				}
			}
		}()
	}
	for _, src := range sources {
		jobs <- src
	}
	close(jobs)
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if results[i].LastMsgAt == results[j].LastMsgAt {
			return results[i].SessionID < results[j].SessionID
		}
		return results[i].LastMsgAt > results[j].LastMsgAt
	})
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results, nil
}

// searchSession scans all transcript files for one session. Missing or
// unreadable files are logged and skipped so one stale session cannot fail
// the whole search.
func searchSession(src db.SessionSource, m *matcher) (Result, bool) {
	paths, err := parser.CollectSessionSourcePaths(src.FilePath)
	if err != nil {
		// Transcript files routinely age out of ~/.claude/projects while
		// their sessions remain in the database, so a missing source is
		// expected noise, not a warning.
		slog.Debug("skipping session with unavailable source", "session", src.SessionID, "error", err)
		return Result{}, false
	}

	result := Result{
		SessionID:   src.SessionID,
		ProjectName: src.ProjectName,
		LastMsgAt:   src.LastMsgAt,
	}
	for _, path := range paths {
		count, snippet, role, err := scanSessionFile(path, m)
		if err != nil {
			slog.Warn("error scanning transcript", "file", path, "error", err)
			continue
		}
		result.MatchCount += count
		if result.Snippet == "" && snippet != "" {
			result.Snippet = snippet
			result.SnippetRole = role
		}
	}
	return result, result.MatchCount > 0
}

// scanSessionFile counts matching messages in one JSONL file and returns the
// snippet and role of the first match. Malformed lines are skipped leniently.
func scanSessionFile(path string, m *matcher) (int, string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", "", fmt.Errorf("open source file %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	var (
		count   int
		snippet string
		role    string
	)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		line = bytes.TrimPrefix(line, []byte{0xEF, 0xBB, 0xBF})

		// Cheap raw-byte pre-filter: skip JSON parsing for lines that cannot
		// contain the keyword. Only valid for keywords JSON escaping cannot
		// alter.
		if m.preFilterable && !m.matchBytes(line) {
			continue
		}

		var raw parser.RawJSONLLine
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}
		if (raw.Type != "user" && raw.Type != "assistant") || raw.Message == nil {
			continue
		}

		matched, matchedText, idx := matchMessage(raw.Message.Content, m)
		if !matched {
			continue
		}
		count++
		if snippet == "" {
			snippet = makeSnippet(matchedText, idx, len(m.needle))
			role = raw.Type
			if raw.Message.Role != "" {
				role = raw.Message.Role
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return count, snippet, role, fmt.Errorf("scan source file %s: %w", path, err)
	}
	return count, snippet, role, nil
}

// matchMessage tests every content block's text, thinking, and tool input
// against the matcher. It returns the first matching string and the byte
// index of the match within it.
func matchMessage(blocks []parser.ContentBlock, m *matcher) (bool, string, int) {
	for _, block := range blocks {
		for _, s := range []string{block.Text, block.Thinking} {
			if idx := m.matchString(s); idx >= 0 {
				return true, s, idx
			}
		}
		if block.Type == "tool_use" && block.Input != nil {
			if input, err := json.Marshal(block.Input); err == nil {
				if idx := m.matchString(string(input)); idx >= 0 {
					return true, string(input), idx
				}
			}
		}
	}
	return false, "", 0
}

// matcher holds the precomputed needle for repeated matching.
type matcher struct {
	needle        string // lowercased unless caseSensitive
	needleBytes   []byte
	caseSensitive bool
	preFilterable bool
}

func newMatcher(keyword string, caseSensitive bool) *matcher {
	needle := keyword
	if !caseSensitive {
		needle = strings.ToLower(keyword)
	}
	return &matcher{
		needle:        needle,
		needleBytes:   []byte(needle),
		caseSensitive: caseSensitive,
		preFilterable: preFilterSafe(keyword),
	}
}

// preFilterSafe reports whether a keyword's byte representation is guaranteed
// to survive JSON string escaping unchanged, making a raw-line scan safe as a
// pre-filter. Quotes, backslashes, and control characters can be escaped in
// the raw JSON, so keywords containing them must skip the pre-filter.
func preFilterSafe(keyword string) bool {
	for _, r := range keyword {
		if r == '"' || r == '\\' || r < 0x20 {
			return false
		}
	}
	return true
}

// matchBytes tests a raw line for the needle without allocating for the
// common case-sensitive path.
func (m *matcher) matchBytes(line []byte) bool {
	if m.caseSensitive {
		return bytes.Contains(line, m.needleBytes)
	}
	return bytes.Contains(bytes.ToLower(line), m.needleBytes)
}

// matchString returns the byte index of the needle in s, or -1. The index is
// computed on the case-folded string when matching case-insensitively; for
// ASCII keywords it aligns with the original string.
func (m *matcher) matchString(s string) int {
	if s == "" {
		return -1
	}
	if m.caseSensitive {
		return strings.Index(s, m.needle)
	}
	return strings.Index(strings.ToLower(s), m.needle)
}

// makeSnippet extracts a compact, single-line context window around a match:
// up to 15 runes before it and about 80 runes total, with ellipses marking
// truncation and whitespace runs collapsed to single spaces.
func makeSnippet(s string, idx, matchLen int) string {
	const (
		runesBefore = 15
		runesTotal  = 80
	)
	if idx < 0 || idx > len(s) {
		idx = 0
	}
	// Clamp to a rune boundary in case a case-folded index misaligns.
	for idx > 0 && idx < len(s) && !utf8.RuneStart(s[idx]) {
		idx--
	}

	start := idx
	for i := 0; i < runesBefore && start > 0; i++ {
		_, size := utf8.DecodeLastRuneInString(s[:start])
		start -= size
	}

	end := min(idx+matchLen, len(s))
	for !utf8.RuneStart(s[end-1]) && end < len(s) {
		end++
	}
	taken := utf8.RuneCountInString(s[start:end])
	for end < len(s) && taken < runesTotal {
		_, size := utf8.DecodeRuneInString(s[end:])
		end += size
		taken++
	}

	snippet := strings.Join(strings.Fields(s[start:end]), " ")
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(s) {
		snippet += "…"
	}
	return snippet
}
