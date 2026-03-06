package parser

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// messageTypes are the JSONL event types we care about.
var messageTypes = map[string]bool{
	"user":      true,
	"assistant": true,
}

// ParseLine parses a single JSONL line into a ParsedMessage.
// Returns nil, nil for lines that should be skipped (non-message events, empty lines).
func ParseLine(data []byte) (*ParsedMessage, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}

	// Strip UTF-8 BOM if present
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var raw RawJSONLLine
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	// Filter to only message types we care about
	if !messageTypes[raw.Type] {
		return nil, nil
	}

	ts, err := time.Parse(time.RFC3339Nano, raw.Timestamp)
	if err != nil {
		// Try alternate formats
		ts, err = time.Parse("2006-01-02T15:04:05Z", raw.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse timestamp %q: %w", raw.Timestamp, err)
		}
	}

	msg := &ParsedMessage{
		SessionID: raw.SessionID,
		UUID:      raw.UUID,
		Timestamp: ts,
		CWD:       raw.CWD,
		Version:   raw.Version,
		GitBranch: raw.GitBranch,
		CostUSD:   raw.CostUSD,
	}

	if raw.ParentUUID != nil {
		msg.ParentUUID = *raw.ParentUUID
	}

	if raw.Duration != nil {
		msg.DurationMs = *raw.Duration
	}

	if raw.Message != nil {
		msg.Role = raw.Message.Role
		msg.Model = raw.Message.Model

		if raw.Message.Usage != nil {
			msg.Usage = *raw.Message.Usage
		}

		msg.ContentPreview = extractContentPreview(raw.Message.Content)
		msg.ToolUses = extractToolUses(raw.Message.Content)
	} else {
		// Use the top-level type as role fallback
		msg.Role = raw.Type
	}

	return msg, nil
}

// ParseFile parses all message lines from a JSONL file.
// Malformed lines are logged and skipped.
func ParseFile(path string) ([]ParsedMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", path, err)
	}
	defer f.Close()

	var messages []ParsedMessage
	scanner := bufio.NewScanner(f)
	// Increase buffer size for potentially large JSON lines
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		msg, err := ParseLine(line)
		if err != nil {
			slog.Warn("skipping malformed line",
				"file", path,
				"line", lineNum,
				"error", err,
			)
			continue
		}
		if msg == nil {
			continue // Non-message event or empty line
		}

		messages = append(messages, *msg)
	}

	if err := scanner.Err(); err != nil {
		return messages, fmt.Errorf("scan file %s: %w", path, err)
	}

	return messages, nil
}

// extractContentPreview returns the first 200 characters of the first text block.
func extractContentPreview(blocks []ContentBlock) string {
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			if len(b.Text) > 200 {
				return b.Text[:200]
			}
			return b.Text
		}
	}
	return ""
}

// extractToolUses pulls all tool_use blocks from content.
func extractToolUses(blocks []ContentBlock) []ToolUse {
	var tools []ToolUse
	for _, b := range blocks {
		if b.Type != "tool_use" {
			continue
		}

		tu := ToolUse{
			ID:   b.ID,
			Name: b.Name,
		}

		// Serialize input for preview
		if b.Input != nil {
			inputBytes, err := json.Marshal(b.Input)
			if err == nil {
				preview := string(inputBytes)
				if len(preview) > 200 {
					preview = preview[:200]
				}
				tu.InputPreview = preview
			}
		}

		tools = append(tools, tu)
	}
	return tools
}
