package export

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/cnu/claude-stats/internal/parser"
)

type transcriptMessage struct {
	UUID      string
	Timestamp time.Time
	Role      string
	Model     string
	Content   []parser.ContentBlock
}

func parseTranscriptFile(path string) ([]transcriptMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open source file %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	var messages []transcriptMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		line = bytes.TrimPrefix(line, []byte{0xEF, 0xBB, 0xBF})

		var raw parser.RawJSONLLine
		if err := json.Unmarshal(line, &raw); err != nil {
			slog.Warn("skipping malformed line",
				"file", path,
				"line", lineNum,
				"error", err,
			)
			continue
		}

		if raw.Type != "user" && raw.Type != "assistant" {
			continue
		}

		ts, err := parseTimestamp(raw.Timestamp)
		if err != nil {
			slog.Warn("skipping line with bad timestamp",
				"file", path,
				"line", lineNum,
				"timestamp", raw.Timestamp,
				"error", err,
			)
			continue
		}

		role := raw.Type
		model := ""
		var content []parser.ContentBlock
		if raw.Message != nil {
			if raw.Message.Role != "" {
				role = raw.Message.Role
			}
			model = raw.Message.Model
			content = raw.Message.Content
		}

		messages = append(messages, transcriptMessage{
			UUID:      raw.UUID,
			Timestamp: ts,
			Role:      role,
			Model:     model,
			Content:   content,
		})
	}

	if err := scanner.Err(); err != nil {
		return messages, fmt.Errorf("scan source file %s: %w", path, err)
	}

	return messages, nil
}

func parseTimestamp(raw string) (time.Time, error) {
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err == nil {
		return ts, nil
	}

	ts, err = time.Parse("2006-01-02T15:04:05Z", raw)
	if err == nil {
		return ts, nil
	}

	return time.Time{}, err
}

func renderContentBlocks(blocks []parser.ContentBlock) string {
	if len(blocks) == 0 {
		return "_(no content)_"
	}

	var b strings.Builder
	for i, block := range blocks {
		if i > 0 {
			b.WriteString("\n\n")
		}

		switch block.Type {
		case "text":
			if block.Text == "" {
				b.WriteString("_(empty text block)_")
				continue
			}
			b.WriteString(block.Text)
		case "thinking":
			b.WriteString("```text\n[thinking]\n")
			if block.Thinking != "" {
				b.WriteString(block.Thinking)
				if !strings.HasSuffix(block.Thinking, "\n") {
					b.WriteString("\n")
				}
			}
			b.WriteString("```")
		case "tool_use":
			name := block.Name
			if name == "" {
				name = "(unnamed)"
			}
			b.WriteString(fmt.Sprintf("**Tool Use:** `%s`", name))
			if block.ID != "" {
				b.WriteString(fmt.Sprintf(" (`%s`)", block.ID))
			}
			b.WriteString("\n\n```json\n")
			if block.Input == nil {
				b.WriteString("{}\n")
			} else {
				payload, err := json.Marshal(block.Input)
				if err != nil {
					b.WriteString("{\"error\":\"failed to serialize tool input\"}\n")
				} else {
					b.Write(payload)
					b.WriteString("\n")
				}
			}
			b.WriteString("```")
		default:
			t := block.Type
			if t == "" {
				t = "(unknown)"
			}
			b.WriteString(fmt.Sprintf("_(unsupported content block type: %s)_", t))
		}
	}

	return b.String()
}
