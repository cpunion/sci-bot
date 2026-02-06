package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// DailyNotesIndex is a per-agent index file used by the static frontend.
//
// Without this, agent profile pages would have to probe for recent days by
// guessing filenames, which produces a lot of 404s in static hosting.
type DailyNotesIndex struct {
	Version     int       `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
	Dates       []string  `json:"dates"` // YYYY-MM-DD, sorted ascending
}

var dailyFileRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.jsonl$`)

func buildDailyNotesIndex(dailyDir string) (DailyNotesIndex, error) {
	idx := DailyNotesIndex{Version: 1, GeneratedAt: time.Now(), Dates: []string{}}

	entries, err := os.ReadDir(dailyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return idx, nil
		}
		return idx, err
	}

	dates := make([]string, 0, len(entries))
	for _, e := range entries {
		if e == nil || e.IsDir() {
			continue
		}
		name := e.Name()
		if !dailyFileRe.MatchString(name) {
			continue
		}
		dateKey := strings.TrimSuffix(name, ".jsonl")
		if dateKey == "" {
			continue
		}
		dates = append(dates, dateKey)
	}
	sort.Strings(dates)
	idx.Dates = dates
	return idx, nil
}

func writeDailyNotesIndex(path string, idx DailyNotesIndex) error {
	if idx.Version <= 0 {
		idx.Version = 1
	}
	if idx.GeneratedAt.IsZero() {
		idx.GeneratedAt = time.Now()
	}
	if idx.Dates == nil {
		idx.Dates = []string{}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// WriteDailyNotesIndexes writes `agents/<agent_id>/daily/index.json` for all agents
// found under `dataPath/agents`.
func WriteDailyNotesIndexes(dataPath string) error {
	agentsDir := filepath.Join(dataPath, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e == nil || !e.IsDir() {
			continue
		}
		agentID := e.Name()
		dailyDir := filepath.Join(agentsDir, agentID, "daily")
		if err := os.MkdirAll(dailyDir, 0755); err != nil {
			return err
		}
		idx, err := buildDailyNotesIndex(dailyDir)
		if err != nil {
			return err
		}
		if err := writeDailyNotesIndex(filepath.Join(dailyDir, "index.json"), idx); err != nil {
			return err
		}
	}
	return nil
}
