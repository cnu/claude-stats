package parser

import (
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

func TestScanDirectory_SkipsSubagents(t *testing.T) {
	// Create a temp directory with a subagents/ subdir
	tmpDir, err := os.MkdirTemp("", "claude-stats-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a regular session file
	err = os.WriteFile(filepath.Join(tmpDir, "session1.jsonl"), []byte(`{}`), 0644)
	require.NoError(t, err)

	// Create subagents directory with a file
	subDir := filepath.Join(tmpDir, "subagents")
	require.NoError(t, os.Mkdir(subDir, 0755))
	err = os.WriteFile(filepath.Join(subDir, "agent1.jsonl"), []byte(`{}`), 0644)
	require.NoError(t, err)

	files, err := ScanDirectory(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "session1", files[0].SessionID)
}
