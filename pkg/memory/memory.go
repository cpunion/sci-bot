// Package memory implements the memory system for Sci-Bot agents.
package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cpunion/sci-bot/pkg/types"
)

// CoreMemory represents persistent identity and values.
type CoreMemory struct {
	Identity    string            `json:"identity"`
	Values      []string          `json:"values"`
	Skills      []string          `json:"skills"`
	Beliefs     map[string]string `json:"beliefs"`
	Experiences []Experience      `json:"experiences"`
}

// SummaryMemory represents a rolling, single-entry memory snapshot.
type SummaryMemory struct {
	Snapshot  string    `json:"snapshot"`
	UpdatedAt time.Time `json:"updated_at"`
	Topics    []string  `json:"topics"`
}

// Experience represents a significant past event.
type Experience struct {
	ID         string    `json:"id"`
	Summary    string    `json:"summary"`
	Lesson     string    `json:"lesson"`
	OccurredAt time.Time `json:"occurred_at"`
}

// WorkingMemory represents the current context window.
type WorkingMemory struct {
	mu sync.RWMutex

	ContextWindow  int              `json:"context_window"` // Max tokens
	CurrentTask    *Task            `json:"current_task"`
	RecentMessages []*types.Message `json:"recent_messages"`
	ActiveTheories []string         `json:"active_theories"`
	ScratchPad     string           `json:"scratch_pad"`
}

// Task represents the current work being done.
type Task struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
}

// ExternalMemory represents retrievable knowledge.
type ExternalMemory struct {
	BasePath      string     `json:"base_path"`
	Subscriptions []string   `json:"subscriptions"`
	Bookmarks     []Bookmark `json:"bookmarks"`
}

// Bookmark represents a saved reference.
type Bookmark struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Reference string    `json:"reference"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
}

// Memory is the complete memory system for an agent.
type Memory struct {
	AgentID  string          `json:"agent_id"`
	Core     *CoreMemory     `json:"core"`
	Summary  *SummaryMemory  `json:"summary"`
	Working  *WorkingMemory  `json:"working"`
	External *ExternalMemory `json:"external"`

	// Persistence
	dataPath string
}

// NewMemory creates a new memory system for an agent.
func NewMemory(agentID string, dataPath string, contextWindow int) *Memory {
	return &Memory{
		AgentID: agentID,
		Core: &CoreMemory{
			Beliefs:     make(map[string]string),
			Experiences: make([]Experience, 0),
		},
		Summary: &SummaryMemory{
			Snapshot: "",
			Topics:   make([]string, 0),
		},
		Working: &WorkingMemory{
			ContextWindow:  contextWindow,
			RecentMessages: make([]*types.Message, 0),
			ActiveTheories: make([]string, 0),
		},
		External: &ExternalMemory{
			BasePath:      filepath.Join(dataPath, "external"),
			Subscriptions: make([]string, 0),
			Bookmarks:     make([]Bookmark, 0),
		},
		dataPath: dataPath,
	}
}

// AddMessage adds a message to working memory, respecting context limits.
func (m *Memory) AddMessage(msg *types.Message) {
	m.Working.mu.Lock()
	defer m.Working.mu.Unlock()

	m.Working.RecentMessages = append(m.Working.RecentMessages, msg)

	// Trim to keep within context window (simple approximation: 100 tokens per message)
	maxMessages := m.Working.ContextWindow / 100
	if maxMessages < 10 {
		maxMessages = 10
	}
	if len(m.Working.RecentMessages) > maxMessages {
		m.Working.RecentMessages = m.Working.RecentMessages[len(m.Working.RecentMessages)-maxMessages:]
	}
}

// GetRecentMessages returns recent messages from working memory.
func (m *Memory) GetRecentMessages(limit int) []*types.Message {
	m.Working.mu.RLock()
	defer m.Working.mu.RUnlock()

	if limit <= 0 || limit > len(m.Working.RecentMessages) {
		limit = len(m.Working.RecentMessages)
	}
	start := len(m.Working.RecentMessages) - limit
	if start < 0 {
		start = 0
	}
	return m.Working.RecentMessages[start:]
}

// SetCurrentTask sets the current task in working memory.
func (m *Memory) SetCurrentTask(task *Task) {
	m.Working.mu.Lock()
	defer m.Working.mu.Unlock()
	m.Working.CurrentTask = task
}

// ClearCurrentTask clears the current task.
func (m *Memory) ClearCurrentTask() {
	m.Working.mu.Lock()
	defer m.Working.mu.Unlock()
	m.Working.CurrentTask = nil
}

// AddExperience adds a significant experience to core memory.
func (m *Memory) AddExperience(exp Experience) {
	m.Core.Experiences = append(m.Core.Experiences, exp)
}

// UpdateBelief updates or adds a belief.
func (m *Memory) UpdateBelief(key, value string) {
	m.Core.Beliefs[key] = value
}

// AddBookmark adds a bookmark to external memory.
func (m *Memory) AddBookmark(bm Bookmark) {
	m.External.Bookmarks = append(m.External.Bookmarks, bm)
}

// Subscribe adds a topic subscription.
func (m *Memory) Subscribe(topic string) {
	for _, t := range m.External.Subscriptions {
		if t == topic {
			return
		}
	}
	m.External.Subscriptions = append(m.External.Subscriptions, topic)
}

// Save persists the memory to disk.
func (m *Memory) Save() error {
	if err := os.MkdirAll(m.dataPath, 0755); err != nil {
		return err
	}

	// Save core memory
	corePath := filepath.Join(m.dataPath, "core_memory.json")
	coreData, err := json.MarshalIndent(m.Core, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(corePath, coreData, 0644); err != nil {
		return err
	}

	// Save summary memory
	summaryPath := filepath.Join(m.dataPath, "summary.json")
	summaryData, err := json.MarshalIndent(m.Summary, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(summaryPath, summaryData, 0644); err != nil {
		return err
	}

	// Save external memory
	extPath := filepath.Join(m.dataPath, "external_memory.json")
	extData, err := json.MarshalIndent(m.External, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(extPath, extData, 0644)
}

// Load loads the memory from disk.
func (m *Memory) Load() error {
	// Load core memory
	corePath := filepath.Join(m.dataPath, "core_memory.json")
	if data, err := os.ReadFile(corePath); err == nil {
		if err := json.Unmarshal(data, m.Core); err != nil {
			return err
		}
	}

	// Load summary memory
	summaryPath := filepath.Join(m.dataPath, "summary.json")
	if data, err := os.ReadFile(summaryPath); err == nil {
		if err := json.Unmarshal(data, m.Summary); err != nil {
			return err
		}
	}

	// Load external memory
	extPath := filepath.Join(m.dataPath, "external_memory.json")
	if data, err := os.ReadFile(extPath); err == nil {
		if err := json.Unmarshal(data, m.External); err != nil {
			return err
		}
	}

	return nil
}

// GetScratchPad returns the scratch pad content.
func (m *Memory) GetScratchPad() string {
	m.Working.mu.RLock()
	defer m.Working.mu.RUnlock()
	return m.Working.ScratchPad
}

// SetScratchPad sets the scratch pad content.
func (m *Memory) SetScratchPad(content string) {
	m.Working.mu.Lock()
	defer m.Working.mu.Unlock()
	m.Working.ScratchPad = content
}

// UpdateSummary appends a new entry to the rolling summary.
func (m *Memory) UpdateSummary(entry string, maxChars int) {
	if entry == "" {
		return
	}
	if maxChars <= 0 {
		maxChars = 2000
	}
	if m.Summary == nil {
		m.Summary = &SummaryMemory{}
	}

	if m.Summary.Snapshot == "" {
		m.Summary.Snapshot = entry
	} else {
		m.Summary.Snapshot = m.Summary.Snapshot + "\n" + entry
	}

	m.Summary.Snapshot = trimToLastRunes(m.Summary.Snapshot, maxChars)
	m.Summary.UpdatedAt = time.Now()
}

// AddActiveTheory adds a theory to the active theories list.
func (m *Memory) AddActiveTheory(theoryID string) {
	m.Working.mu.Lock()
	defer m.Working.mu.Unlock()
	for _, t := range m.Working.ActiveTheories {
		if t == theoryID {
			return
		}
	}
	m.Working.ActiveTheories = append(m.Working.ActiveTheories, theoryID)
}

// GetContextSummary returns a summary of the current context for prompts.
func (m *Memory) GetContextSummary() map[string]any {
	m.Working.mu.RLock()
	defer m.Working.mu.RUnlock()

	summary := ""
	if m.Summary != nil {
		summary = m.Summary.Snapshot
	}

	return map[string]any{
		"identity":        m.Core.Identity,
		"values":          m.Core.Values,
		"skills":          m.Core.Skills,
		"summary":         summary,
		"current_task":    m.Working.CurrentTask,
		"active_theories": m.Working.ActiveTheories,
		"subscriptions":   m.External.Subscriptions,
		"recent_messages": len(m.Working.RecentMessages),
	}
}

func trimToLastRunes(s string, maxChars int) string {
	if maxChars <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[len(runes)-maxChars:])
}
