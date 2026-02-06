package feed

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWriter_RotationAndResume(t *testing.T) {
	dir := t.TempDir()

	w, err := OpenWriter(WriterConfig{
		Dir:               dir,
		MaxEventsPerShard: 3,
		Append:            false,
	})
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}

	for i := 0; i < 7; i++ {
		line := []byte(fmt.Sprintf(`{"sim_time":"2026-01-01T00:00:%02dZ","timestamp":"2026-01-01T00:00:%02dZ","tick":%d,"agent_id":"a","agent_name":"a","action":"x"}`, i, i, i))
		if err := w.AppendJSONLine(line); err != nil {
			t.Fatalf("AppendJSONLine(%d): %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	idx, err := LoadIndex(filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if idx.TotalEvents != 7 {
		t.Fatalf("TotalEvents=%d, want 7", idx.TotalEvents)
	}
	if len(idx.Shards) != 3 {
		t.Fatalf("Shards=%d, want 3", len(idx.Shards))
	}
	if idx.Shards[0].File != "events-000001.jsonl" || idx.Shards[0].Events != 3 {
		t.Fatalf("shard1=%+v, want file events-000001.jsonl events 3", idx.Shards[0])
	}
	if idx.Shards[1].File != "events-000002.jsonl" || idx.Shards[1].Events != 3 {
		t.Fatalf("shard2=%+v, want file events-000002.jsonl events 3", idx.Shards[1])
	}
	if idx.Shards[2].File != "events-000003.jsonl" || idx.Shards[2].Events != 1 {
		t.Fatalf("shard3=%+v, want file events-000003.jsonl events 1", idx.Shards[2])
	}
	for _, s := range idx.Shards {
		if _, err := os.Stat(filepath.Join(dir, s.File)); err != nil {
			t.Fatalf("missing shard file %s: %v", s.File, err)
		}
	}

	// Resume with append and verify we keep writing into the last shard until full.
	w2, err := OpenWriter(WriterConfig{
		Dir:               dir,
		MaxEventsPerShard: 3,
		Append:            true,
	})
	if err != nil {
		t.Fatalf("OpenWriter(resume): %v", err)
	}
	for i := 7; i < 9; i++ {
		line := []byte(fmt.Sprintf(`{"sim_time":"2026-01-01T00:00:%02dZ","timestamp":"2026-01-01T00:00:%02dZ","tick":%d,"agent_id":"a","agent_name":"a","action":"x"}`, i, i, i))
		if err := w2.AppendJSONLine(line); err != nil {
			t.Fatalf("AppendJSONLine(resume %d): %v", i, err)
		}
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close(resume): %v", err)
	}

	idx2, err := LoadIndex(filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatalf("LoadIndex(resume): %v", err)
	}
	if idx2.TotalEvents != 9 {
		t.Fatalf("TotalEvents(resume)=%d, want 9", idx2.TotalEvents)
	}
	if len(idx2.Shards) != 3 {
		t.Fatalf("Shards(resume)=%d, want 3", len(idx2.Shards))
	}
	if idx2.Shards[2].Events != 3 {
		t.Fatalf("shard3 events=%d, want 3", idx2.Shards[2].Events)
	}
}
