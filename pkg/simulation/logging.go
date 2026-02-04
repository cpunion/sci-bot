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
	Action         string    `json:"action"`
	Prompt         string    `json:"prompt"`
	Response       string    `json:"response"`
	ToolCalls      []string  `json:"tool_calls,omitempty"`
	ToolResponses  []string  `json:"tool_responses,omitempty"`
	TurnCount      int       `json:"turn_count"`
	BellRung       bool      `json:"bell_rung"`
	GraceRemaining int       `json:"grace_remaining"`
	Sleeping       bool      `json:"sleeping"`
}

// EventLogger records simulation events for later analysis.
type EventLogger interface {
	LogEvent(EventLog) error
	Close() error
}

// JSONLLogger writes each event as a JSON line.
type JSONLLogger struct {
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
}

// NewJSONLLogger creates a JSONL logger at the given path.
func NewJSONLLogger(path string) (*JSONLLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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
