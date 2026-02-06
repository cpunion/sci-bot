package feed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Index is a static-friendly manifest for sharded JSONL event logs.
//
// The site loads shards incrementally (newest-first) so no single file grows
// unbounded and the frontend can paginate without a server API.
type Index struct {
	Version           int       `json:"version"`
	GeneratedAt       time.Time `json:"generated_at"`
	MaxEventsPerShard int       `json:"max_events_per_shard,omitempty"`

	// Shards are ordered oldest -> newest (append-only). UIs should load from the end.
	Shards []Shard `json:"shards"`

	TotalEvents int `json:"total_events,omitempty"`
}

type Shard struct {
	Seq    int    `json:"seq"`
	File   string `json:"file"`   // file name relative to the feed directory, e.g. "events-000001.jsonl"
	Events int    `json:"events"` // number of JSONL lines/events in the shard (best-effort)
}

func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	idx := &Index{}
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, err
	}
	if idx.Version == 0 {
		idx.Version = 1
	}
	return idx, nil
}

func SaveIndexAtomic(path string, idx *Index) error {
	if idx == nil {
		return nil
	}
	if idx.Version <= 0 {
		idx.Version = 1
	}
	idx.GeneratedAt = time.Now()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
