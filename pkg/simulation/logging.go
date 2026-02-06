package simulation

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EventLog captures one simulation step for analysis.
type EventLog struct {
	Timestamp      time.Time `json:"timestamp"`
	SimTime        time.Time `json:"sim_time"`
	Tick           int       `json:"tick"`
	AgentID        string    `json:"agent_id"`
	AgentName      string    `json:"agent_name"`
	ModelName      string    `json:"model_name,omitempty"`
	Action         string    `json:"action"`
	Prompt         string    `json:"prompt"`
	Response       string    `json:"response"`
	Error          string    `json:"error,omitempty"`
	ToolCalls      []string  `json:"tool_calls,omitempty"`
	ToolResponses  []string  `json:"tool_responses,omitempty"`
	TurnCount      int       `json:"turn_count"`
	BellRung       bool      `json:"bell_rung"`
	GraceRemaining int       `json:"grace_remaining"`
	Sleeping       bool      `json:"sleeping"`

	// Token usage (best-effort, depends on provider).
	UsageEvents         int `json:"usage_events,omitempty"`
	PromptTokens        int `json:"prompt_tokens,omitempty"`
	CandidatesTokens    int `json:"candidates_tokens,omitempty"`
	ThoughtsTokens      int `json:"thoughts_tokens,omitempty"`
	ToolUsePromptTokens int `json:"tool_use_prompt_tokens,omitempty"`
	CachedContentTokens int `json:"cached_content_tokens,omitempty"`
	TotalTokens         int `json:"total_tokens,omitempty"`
}

// EventLogger records simulation events for later analysis.
type EventLogger interface {
	LogEvent(EventLog) error
	Close() error
}

// MultiLogger broadcasts events to multiple loggers.
type MultiLogger struct {
	loggers []EventLogger
}

func NewMultiLogger(loggers ...EventLogger) *MultiLogger {
	out := make([]EventLogger, 0, len(loggers))
	for _, l := range loggers {
		if l != nil {
			out = append(out, l)
		}
	}
	return &MultiLogger{loggers: out}
}

func (m *MultiLogger) LogEvent(ev EventLog) error {
	if m == nil {
		return nil
	}
	var first error
	for _, l := range m.loggers {
		if l == nil {
			continue
		}
		if err := l.LogEvent(ev); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (m *MultiLogger) Close() error {
	if m == nil {
		return nil
	}
	var first error
	for _, l := range m.loggers {
		if l == nil {
			continue
		}
		if err := l.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// JSONLLogger writes each event as a JSON line.
type JSONLLogger struct {
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
}

// NewJSONLLogger creates a JSONL logger at the given path.
// When appendMode is true, logs are appended; otherwise, the file is truncated.
func NewJSONLLogger(path string, appendMode bool) (*JSONLLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		return nil, err
	}
	return &JSONLLogger{
		file:   file,
		writer: bufio.NewWriter(file),
	}, nil
}

// LogEvent writes a single event as JSONL.
func (l *JSONLLogger) LogEvent(ev EventLog) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if _, err := l.writer.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	return l.writer.Flush()
}

// Close closes the logger.
func (l *JSONLLogger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.writer != nil {
		_ = l.writer.Flush()
	}
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
