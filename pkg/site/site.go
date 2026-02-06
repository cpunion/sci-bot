package site

import "time"

// Manifest is a small index file used by the static frontend. It allows a
// purely-static site to discover which data files exist without a server API.
type Manifest struct {
	Version     int       `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`

	// Simulation clock (optional but useful for UI that needs a reference "now").
	SimTime     time.Time `json:"sim_time,omitempty"`
	StepSeconds int       `json:"step_seconds,omitempty"`

	// Relative paths, anchored at the data root (the directory that contains this file).
	AgentsPath  string   `json:"agents_path,omitempty"`  // e.g. "agents/agents.json"
	ForumPath   string   `json:"forum_path,omitempty"`   // e.g. "forum/forum.json"
	JournalPath string   `json:"journal_path,omitempty"` // e.g. "journal/journal.json"
	Logs        []string `json:"logs,omitempty"`         // e.g. ["logs.jsonl", "logs-10d-...jsonl"]
	DefaultLog  string   `json:"default_log,omitempty"`  // best-effort

	Stats ManifestStats `json:"stats,omitempty"`
}

type ManifestStats struct {
	AgentCount      int `json:"agent_count,omitempty"`
	ForumThreads    int `json:"forum_threads,omitempty"`
	JournalApproved int `json:"journal_approved,omitempty"`
	JournalPending  int `json:"journal_pending,omitempty"`
}

// AgentCatalog is a static index of agents used by the frontend (homepage + mention resolution).
type AgentCatalog struct {
	Version     int       `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
	Agents      []Agent   `json:"agents"`
}

// Agent mirrors the fields needed by the UI (similar to cmd/server AgentInfo).
type Agent struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Role                string   `json:"role"`
	ThinkingStyle       string   `json:"thinking_style"`
	Domains             []string `json:"domains"`
	Creativity          float64  `json:"creativity"`
	Rigor               float64  `json:"rigor"`
	RiskTolerance       float64  `json:"risk_tolerance"`
	Sociability         float64  `json:"sociability"`
	Influence           float64  `json:"influence"`
	ResearchOrientation string   `json:"research_orientation"`
}
