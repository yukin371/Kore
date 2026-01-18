package agent

import (
	"fmt"
	"strings"

	"github.com/yukin/kore/internal/core"
)

// ContextStrategy defines how to prune or compress conversation history.
type ContextStrategy interface {
	Apply(history *core.ConversationHistory) error
}

// RollingWindowStrategy keeps recent messages and summarizes older ones.
type RollingWindowStrategy struct {
	MaxMessages int
}

func (s *RollingWindowStrategy) Apply(history *core.ConversationHistory) error {
	if history == nil {
		return fmt.Errorf("history is nil")
	}

	if s.MaxMessages <= 0 {
		return nil
	}

	messages := history.GetMessages()
	if len(messages) <= s.MaxMessages {
		return nil
	}

	var preserved []core.Message
	start := 0
	if len(messages) > 0 && messages[0].Role == "system" {
		preserved = append(preserved, messages[0])
		start = 1
	}

	cut := len(messages) - s.MaxMessages
	if cut < start {
		cut = start
	}

	summary := summarizeMessages(messages[start:cut])
	if summary != "" {
		preserved = append(preserved, core.Message{
			Role:    "system",
			Content: summary,
		})
	}

	trimmed := messages[cut:]
	preserved = append(preserved, trimmed...)
	history.ReplaceMessages(preserved)

	return nil
}

func summarizeMessages(messages []core.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var summary strings.Builder
	summary.WriteString("Summary of earlier context:\n")
	limit := 12
	count := 0

	for _, msg := range messages {
		line := strings.TrimSpace(msg.Content)
		if line == "" {
			continue
		}
		if len(line) > 120 {
			line = line[:120] + "..."
		}
		summary.WriteString(fmt.Sprintf("- %s: %s\n", msg.Role, line))
		count++
		if count >= limit {
			break
		}
	}

	return strings.TrimSpace(summary.String())
}
