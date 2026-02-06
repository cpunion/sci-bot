package feed

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRebuildFromLogs(t *testing.T) {
	tmp := t.TempDir()
	log1 := filepath.Join(tmp, "logs-a.jsonl")
	log2 := filepath.Join(tmp, "logs-b.jsonl")

	// Mix ordering across two files. RebuildFromLogs sorts by sim_time then timestamp.
	lines1 := []string{
		`{"sim_time":"2026-01-02T00:00:00Z","timestamp":"2026-01-01T00:00:00Z","agent_id":"a","agent_name":"a","action":"x"}`,
		`{"sim_time":"2026-01-01T00:00:00Z","timestamp":"2026-01-01T00:00:01Z","agent_id":"a","agent_name":"a","action":"x"}`,
		`{"sim_time":"2026-01-03T00:00:00Z","timestamp":"2026-01-01T00:00:02Z","agent_id":"a","agent_name":"a","action":"x"}`,
	}
	lines2 := []string{
		`{"sim_time":"2026-01-01T12:00:00Z","timestamp":"2026-01-01T00:00:03Z","agent_id":"a","agent_name":"a","action":"x"}`,
		`{"sim_time":"2026-01-02T12:00:00Z","timestamp":"2026-01-01T00:00:04Z","agent_id":"a","agent_name":"a","action":"x"}`,
	}

	if err := os.WriteFile(log1, []byte(fmt.Sprintf("%s\n%s\n%s\n", lines1[0], lines1[1], lines1[2])), 0644); err != nil {
		t.Fatalf("WriteFile(log1): %v", err)
	}
	if err := os.WriteFile(log2, []byte(fmt.Sprintf("%s\n%s\n", lines2[0], lines2[1])), 0644); err != nil {
		t.Fatalf("WriteFile(log2): %v", err)
	}

	outDir := filepath.Join(tmp, "feed")
	idx, err := RebuildFromLogs(outDir, []string{log1, log2}, 2)
	if err != nil {
		t.Fatalf("RebuildFromLogs: %v", err)
	}
	if idx == nil {
		t.Fatalf("RebuildFromLogs returned nil index")
	}
	if idx.TotalEvents != 5 {
		t.Fatalf("TotalEvents=%d, want 5", idx.TotalEvents)
	}
	if idx.MaxEventsPerShard != 2 {
		t.Fatalf("MaxEventsPerShard=%d, want 2", idx.MaxEventsPerShard)
	}
	if len(idx.Shards) != 3 {
		t.Fatalf("Shards=%d, want 3", len(idx.Shards))
	}
	if idx.Shards[0].Events != 2 || idx.Shards[1].Events != 2 || idx.Shards[2].Events != 1 {
		t.Fatalf("shard events=%v, want [2 2 1]", []int{idx.Shards[0].Events, idx.Shards[1].Events, idx.Shards[2].Events})
	}

	for _, s := range idx.Shards {
		if _, err := os.Stat(filepath.Join(outDir, s.File)); err != nil {
			t.Fatalf("missing shard file %s: %v", s.File, err)
		}
	}
}
