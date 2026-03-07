package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testdataPath(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

func TestParseLine_UserMessage(t *testing.T) {
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-1","timestamp":"2026-03-01T10:00:00.000Z","type":"user","cwd":"/Users/test/Projects/myapp","message":{"role":"user","content":[{"type":"text","text":"hello world"}]}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)

	assert.Equal(t, "sess-1", msg.SessionID)
	assert.Equal(t, "msg-1", msg.UUID)
	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "hello world", msg.ContentPreview)
	assert.Equal(t, "/Users/test/Projects/myapp", msg.CWD)
	assert.Empty(t, msg.ToolUses)
	assert.Equal(t, time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC), msg.Timestamp)
}

func TestParseLine_AssistantMessage(t *testing.T) {
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-2","parentUuid":"msg-1","timestamp":"2026-03-01T10:00:05.000Z","type":"assistant","message":{"model":"claude-sonnet-4-6-20250925","role":"assistant","content":[{"type":"text","text":"I can help with that."},{"type":"tool_use","id":"toolu_01","name":"Read","input":{"file_path":"/tmp/test.go"}}],"usage":{"input_tokens":1500,"output_tokens":200,"cache_creation_input_tokens":5000,"cache_read_input_tokens":0}}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)

	assert.Equal(t, "assistant", msg.Role)
	assert.Equal(t, "claude-sonnet-4-6-20250925", msg.Model)
	assert.Equal(t, "msg-1", msg.ParentUUID)
	assert.Equal(t, "I can help with that.", msg.ContentPreview)
	assert.Equal(t, 1500, msg.Usage.InputTokens)
	assert.Equal(t, 200, msg.Usage.OutputTokens)
	assert.Equal(t, 5000, msg.Usage.CacheCreationInputTokens)
	assert.Equal(t, 0, msg.Usage.CacheReadInputTokens)

	require.Len(t, msg.ToolUses, 1)
	assert.Equal(t, "Read", msg.ToolUses[0].Name)
	assert.Equal(t, "toolu_01", msg.ToolUses[0].ID)
	assert.Contains(t, msg.ToolUses[0].InputPreview, "/tmp/test.go")
}

func TestParseLine_EmptyLine(t *testing.T) {
	msg, err := ParseLine([]byte(""))
	assert.NoError(t, err)
	assert.Nil(t, msg)

	msg, err = ParseLine([]byte("   "))
	assert.NoError(t, err)
	assert.Nil(t, msg)
}

func TestParseLine_NonMessageTypes(t *testing.T) {
	types := []string{
		`{"type":"queue-operation","operation":"enqueue","timestamp":"2026-03-01T10:00:00.000Z","sessionId":"s1"}`,
		`{"type":"progress","data":{"type":"hook_progress"},"timestamp":"2026-03-01T10:00:00.000Z","sessionId":"s1","uuid":"u1"}`,
		`{"type":"file-history-snapshot","messageId":"m1","snapshot":{},"timestamp":"2026-03-01T10:00:00.000Z"}`,
	}

	for _, line := range types {
		msg, err := ParseLine([]byte(line))
		assert.NoError(t, err, "line: %s", line)
		assert.Nil(t, msg, "should skip non-message type: %s", line)
	}
}

func TestParseLine_MalformedJSON(t *testing.T) {
	msg, err := ParseLine([]byte(`{this is not valid json}`))
	assert.Error(t, err)
	assert.Nil(t, msg)
}

func TestParseLine_UTF8BOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	data := append(bom, []byte(`{"sessionId":"sess-bom","uuid":"msg-bom","timestamp":"2026-03-01T10:00:00.000Z","type":"user","message":{"role":"user","content":[{"type":"text","text":"bom test"}]}}`)...)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "sess-bom", msg.SessionID)
	assert.Equal(t, "bom test", msg.ContentPreview)
}

func TestParseLine_ThinkingContent(t *testing.T) {
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-think","timestamp":"2026-03-01T10:00:00.000Z","type":"assistant","message":{"model":"claude-opus-4-6","role":"assistant","content":[{"type":"thinking","thinking":"Let me think..."},{"type":"text","text":"Here is my answer."}],"usage":{"input_tokens":100,"output_tokens":50}}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)
	// Content preview should skip thinking and get the text block
	assert.Equal(t, "Here is my answer.", msg.ContentPreview)
}

func TestParseLine_LongContentPreview(t *testing.T) {
	longText := ""
	for i := 0; i < 300; i++ {
		longText += "x"
	}
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-long","timestamp":"2026-03-01T10:00:00.000Z","type":"user","message":{"role":"user","content":[{"type":"text","text":"` + longText + `"}]}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Len(t, msg.ContentPreview, 200)
}

func TestParseLine_NoCostUSD(t *testing.T) {
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-1","timestamp":"2026-03-01T10:00:00.000Z","type":"assistant","message":{"model":"claude-sonnet-4-6-20250925","role":"assistant","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":100,"output_tokens":50}}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	assert.Nil(t, msg.CostUSD)
}

func TestParseLine_WithCostUSD(t *testing.T) {
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-1","timestamp":"2026-03-01T10:00:00.000Z","type":"assistant","costUSD":0.0042,"message":{"model":"claude-sonnet-4-6-20250925","role":"assistant","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":100,"output_tokens":50}}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg.CostUSD)
	assert.InDelta(t, 0.0042, *msg.CostUSD, 0.0001)
}

func TestParseFile_ValidSession(t *testing.T) {
	messages, err := ParseFile(testdataPath("valid_session.jsonl"))
	require.NoError(t, err)
	assert.Len(t, messages, 6)

	// First message is user
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "a1b2c3d4-e5f6-7890-abcd-ef1234567890", messages[0].SessionID)

	// Second message is assistant with tool use
	assert.Equal(t, "assistant", messages[1].Role)
	assert.Equal(t, "claude-sonnet-4-6-20250925", messages[1].Model)
	assert.Len(t, messages[1].ToolUses, 1)
	assert.Equal(t, "Read", messages[1].ToolUses[0].Name)
}

func TestParseFile_MinimalSession(t *testing.T) {
	messages, err := ParseFile(testdataPath("minimal_session.jsonl"))
	require.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, "claude-haiku-4-5-20251001", messages[1].Model)
}

func TestParseFile_CorruptLine(t *testing.T) {
	messages, err := ParseFile(testdataPath("corrupt_line.jsonl"))
	require.NoError(t, err)
	// Should parse 2 valid messages, skip the corrupt line
	assert.Len(t, messages, 2)
}

func TestParseFile_Empty(t *testing.T) {
	messages, err := ParseFile(testdataPath("empty.jsonl"))
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestParseFile_MixedEvents(t *testing.T) {
	messages, err := ParseFile(testdataPath("mixed_events.jsonl"))
	require.NoError(t, err)
	// Should only get user + assistant messages, not progress/queue/snapshot
	assert.Len(t, messages, 2)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "assistant", messages[1].Role)
	assert.Equal(t, "claude-opus-4-6-20250918", messages[1].Model)
}

func TestParseFile_NonExistent(t *testing.T) {
	_, err := ParseFile("/nonexistent/path.jsonl")
	assert.Error(t, err)
}

func TestScanDirectory(t *testing.T) {
	dir := testdataPath("")
	files, err := ScanDirectory(dir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 6)

	// All files should be .jsonl
	for _, f := range files {
		assert.True(t, filepath.Ext(f.Path) == ".jsonl", "unexpected file: %s", f.Path)
		assert.NotEmpty(t, f.SessionID)
	}
}

func TestParseLine_StringContent(t *testing.T) {
	// Older format where content is a plain string instead of array
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-str","timestamp":"2026-03-01T10:00:00.000Z","type":"user","message":{"role":"user","content":"just a plain string"}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "just a plain string", msg.ContentPreview)
}

func TestParseLine_NoTextContent(t *testing.T) {
	// Message with only tool_use blocks, no text
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-notxt","timestamp":"2026-03-01T10:00:00.000Z","type":"assistant","message":{"model":"claude-sonnet-4-6-20250925","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"ls"}}],"usage":{"input_tokens":100,"output_tokens":50}}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Empty(t, msg.ContentPreview)
	require.Len(t, msg.ToolUses, 1)
	assert.Equal(t, "Bash", msg.ToolUses[0].Name)
}

func TestParseLine_NoMessage(t *testing.T) {
	// User type but missing message field entirely
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-nomsg","timestamp":"2026-03-01T10:00:00.000Z","type":"user"}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "user", msg.Role)
	assert.Empty(t, msg.ContentPreview)
}

func TestParseLine_DurationField(t *testing.T) {
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-dur","timestamp":"2026-03-01T10:00:00.000Z","type":"assistant","duration":1500,"message":{"model":"claude-sonnet-4-6-20250925","role":"assistant","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":100,"output_tokens":50}}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, int64(1500), msg.DurationMs)
}

func TestParseLine_MultipleToolUses(t *testing.T) {
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-multi","timestamp":"2026-03-01T10:00:00.000Z","type":"assistant","message":{"model":"claude-sonnet-4-6-20250925","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"a.go"}},{"type":"tool_use","id":"t2","name":"Grep","input":{"pattern":"foo"}},{"type":"text","text":"Found it"}],"usage":{"input_tokens":100,"output_tokens":50}}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.Len(t, msg.ToolUses, 2)
	assert.Equal(t, "Read", msg.ToolUses[0].Name)
	assert.Equal(t, "Grep", msg.ToolUses[1].Name)
	assert.Equal(t, "Found it", msg.ContentPreview)
}

func TestParseLine_ToolUseNilInput(t *testing.T) {
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-nil","timestamp":"2026-03-01T10:00:00.000Z","type":"assistant","message":{"model":"claude-sonnet-4-6-20250925","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Bash"}],"usage":{"input_tokens":100,"output_tokens":50}}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.Len(t, msg.ToolUses, 1)
	assert.Equal(t, "", msg.ToolUses[0].InputPreview)
}

func TestScanDirectory_NonExistent(t *testing.T) {
	files, err := ScanDirectory("/nonexistent/path")
	assert.NoError(t, err)
	assert.Empty(t, files)
}

func TestScanDirectory_SkipsNonJsonl(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claude-stats-ext-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "session.jsonl"), []byte(`{}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("hello"), 0644))

	files, err := ScanDirectory(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestParseLine_AlternateTimestamp(t *testing.T) {
	// Timestamp without fractional seconds
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-alt","timestamp":"2026-03-01T10:00:00Z","type":"user","message":{"role":"user","content":[{"type":"text","text":"alt ts"}]}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "alt ts", msg.ContentPreview)
}

func TestParseLine_InvalidTimestamp(t *testing.T) {
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-badts","timestamp":"not-a-date","type":"user","message":{"role":"user","content":[{"type":"text","text":"hi"}]}}`)

	msg, err := ParseLine(data)
	assert.Error(t, err)
	assert.Nil(t, msg)
	assert.Contains(t, err.Error(), "parse timestamp")
}

func TestParseLine_LongToolInputPreview(t *testing.T) {
	// Tool input that exceeds 200 chars when serialized
	longValue := ""
	for i := 0; i < 250; i++ {
		longValue += "a"
	}
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-longtool","timestamp":"2026-03-01T10:00:00.000Z","type":"assistant","message":{"model":"claude-sonnet-4-6-20250925","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Write","input":{"content":"` + longValue + `"}}],"usage":{"input_tokens":100,"output_tokens":50}}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.Len(t, msg.ToolUses, 1)
	assert.Len(t, msg.ToolUses[0].InputPreview, 200)
}

func TestFlexContent_InvalidType(t *testing.T) {
	// Content that is neither string nor array (e.g., a number)
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-badcontent","timestamp":"2026-03-01T10:00:00.000Z","type":"user","message":{"role":"user","content":12345}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Empty(t, msg.ContentPreview)
}

func TestFlexContent_NullContent(t *testing.T) {
	data := []byte(`{"sessionId":"sess-1","uuid":"msg-null","timestamp":"2026-03-01T10:00:00.000Z","type":"user","message":{"role":"user","content":null}}`)

	msg, err := ParseLine(data)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Empty(t, msg.ContentPreview)
}

func TestScanDirectory_InaccessiblePath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	tmpDir := t.TempDir()

	// Create an accessible file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "good.jsonl"), []byte(`{}`), 0644))

	// Create a subdirectory with a jsonl file, then make it inaccessible
	subDir := filepath.Join(tmpDir, "blocked")
	require.NoError(t, os.Mkdir(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "hidden.jsonl"), []byte(`{}`), 0644))
	require.NoError(t, os.Chmod(subDir, 0000))
	t.Cleanup(func() { _ = os.Chmod(subDir, 0755) })

	files, err := ScanDirectory(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 1) // Should skip inaccessible dir without error
	assert.Equal(t, "good", files[0].SessionID)
}

func TestParseFile_ScannerError(t *testing.T) {
	// Create a file with a single line exceeding the 10MB scanner buffer
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "huge.jsonl")

	f, err := os.Create(fpath)
	require.NoError(t, err)

	// Write a valid short line first
	_, _ = fmt.Fprintln(f, `{"sessionId":"sess-huge","uuid":"msg-h1","timestamp":"2026-03-01T10:00:00.000Z","type":"user","message":{"role":"user","content":[{"type":"text","text":"ok"}]}}`)

	// Write a line >10MB (no newline until after 11MB)
	_, _ = f.WriteString(`{"sessionId":"sess-huge","uuid":"msg-h2","timestamp":"2026-03-01T10:00:00.000Z","type":"user","message":{"role":"user","content":[{"type":"text","text":"`)
	buf := make([]byte, 11*1024*1024)
	for i := range buf {
		buf[i] = 'x'
	}
	_, _ = f.Write(buf)
	_, _ = f.WriteString(`"}]}}` + "\n")
	_ = f.Close()

	messages, err := ParseFile(fpath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scan file")
	// Should still return the first valid message parsed before the error
	assert.Len(t, messages, 1)
}

func TestScanDirectory_IncludesSubagents(t *testing.T) {
	// Create a temp directory structure: project/session1.jsonl + project/session1/subagents/agent1.jsonl
	tmpDir, err := os.MkdirTemp("", "claude-stats-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	// Create project dir
	projDir := filepath.Join(tmpDir, "myproject")
	require.NoError(t, os.Mkdir(projDir, 0755))

	// Create a regular session file
	err = os.WriteFile(filepath.Join(projDir, "session1.jsonl"), []byte(`{}`), 0644)
	require.NoError(t, err)

	// Create subagents directory under a session dir
	sessDir := filepath.Join(projDir, "session1", "subagents")
	require.NoError(t, os.MkdirAll(sessDir, 0755))
	err = os.WriteFile(filepath.Join(sessDir, "agent1.jsonl"), []byte(`{}`), 0644)
	require.NoError(t, err)

	files, err := ScanDirectory(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 2)

	// Find main and subagent files
	var mainFile, subagentFile *SessionFile
	for i := range files {
		if files[i].IsSubagent {
			subagentFile = &files[i]
		} else {
			mainFile = &files[i]
		}
	}

	require.NotNil(t, mainFile)
	assert.Equal(t, "session1", mainFile.SessionID)
	assert.False(t, mainFile.IsSubagent)

	require.NotNil(t, subagentFile)
	assert.Equal(t, "session1", subagentFile.SessionID) // Gets parent session ID
	assert.True(t, subagentFile.IsSubagent)
}
