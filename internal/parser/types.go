package parser

import (
	"encoding/json"
	"time"
)

// RawJSONLLine represents a single line from a Claude Code JSONL file.
// Fields are lenient — missing fields get zero values.
type RawJSONLLine struct {
	ParentUUID     *string         `json:"parentUuid"`
	IsSidechain    bool            `json:"isSidechain"`
	UserType       string          `json:"userType"`
	CWD            string          `json:"cwd"`
	SessionID      string          `json:"sessionId"`
	Version        string          `json:"version"`
	GitBranch      string          `json:"gitBranch"`
	Slug           string          `json:"slug"`
	Type           string          `json:"type"`
	Message        *MessagePayload `json:"message"`
	UUID           string          `json:"uuid"`
	Timestamp      string          `json:"timestamp"`
	RequestID      string          `json:"requestId"`
	PermissionMode string          `json:"permissionMode"`
	CostUSD        *float64        `json:"costUSD"`
	Duration       *int64          `json:"duration"`
}

// MessagePayload is the nested message object in a JSONL line.
type MessagePayload struct {
	Model        string         `json:"model"`
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      FlexContent    `json:"content"`
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        *UsageStats    `json:"usage"`
}

// FlexContent handles message.content being either a JSON array of ContentBlocks
// or a plain string (older format).
type FlexContent []ContentBlock

// UnmarshalJSON implements custom unmarshaling for content that can be string or array.
func (fc *FlexContent) UnmarshalJSON(data []byte) error {
	// Try array first (most common)
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err == nil {
		*fc = blocks
		return nil
	}

	// Try string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*fc = []ContentBlock{{Type: "text", Text: s}}
		return nil
	}

	// If neither works, return empty (lenient)
	*fc = nil
	return nil
}

// ContentBlock represents one block in message.content[].
type ContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Thinking  string `json:"thinking"`
	Name      string `json:"name"`
	ID        string `json:"id"`
	ToolUseID string `json:"tool_use_id"`
	// Input is kept as raw JSON to avoid type assertion issues.
	Input interface{} `json:"input"`
}

// UsageStats holds token usage data from assistant messages.
type UsageStats struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// ParsedMessage is the normalized output of parsing a single JSONL line.
// Only "user" and "assistant" type lines produce a ParsedMessage.
type ParsedMessage struct {
	SessionID  string
	UUID       string
	ParentUUID string
	Timestamp  time.Time
	Role       string // "user" or "assistant"
	Model      string
	Usage      UsageStats
	CostUSD    *float64 // Pre-calculated cost from JSONL, if present
	DurationMs int64

	// Extracted content
	ContentPreview string     // First 200 chars of first text block
	ToolUses       []ToolUse  // Extracted tool_use blocks

	// Session metadata (from first message seen)
	CWD        string
	Version    string
	GitBranch  string
}

// ToolUse represents a single tool invocation extracted from content blocks.
type ToolUse struct {
	ID               string
	Name             string
	InputPreview     string // First 200 chars of serialized input
}

// SessionFile represents a discovered JSONL file on disk.
type SessionFile struct {
	Path      string
	Size      int64
	ModTime   time.Time
	SessionID string // Derived from filename
}
