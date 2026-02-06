package feed

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type rawEvent struct {
	SimTime   time.Time
	Timestamp time.Time
	Line      []byte
}

// RebuildFromLogs reads JSONL log files (EventLog-compatible) and writes them
// into a sharded feed store under outDir.
//
// outDir should be empty/new. This function never touches the original logs.
func RebuildFromLogs(outDir string, logPaths []string, maxEventsPerShard int) (*Index, error) {
	events, err := readLogs(logPaths)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].SimTime.Equal(events[j].SimTime) {
			return events[i].Timestamp.Before(events[j].Timestamp)
		}
		return events[i].SimTime.Before(events[j].SimTime)
	})

	w, err := OpenWriter(WriterConfig{
		Dir:               outDir,
		MaxEventsPerShard: maxEventsPerShard,
		Append:            false,
	})
	if err != nil {
		return nil, err
	}
	defer w.Close()

	for _, ev := range events {
		if err := w.AppendJSONLine(ev.Line); err != nil {
			return nil, err
		}
	}

	return LoadIndex(filepath.Join(outDir, "index.json"))
}

func readLogs(paths []string) ([]rawEvent, error) {
	out := make([]rawEvent, 0, 1024)

	type keyOnly struct {
		SimTime   time.Time `json:"sim_time"`
		Timestamp time.Time `json:"timestamp"`
	}

	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 256*1024), 8*1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var k keyOnly
			if err := json.Unmarshal([]byte(line), &k); err != nil {
				continue
			}
			out = append(out, rawEvent{
				SimTime:   k.SimTime,
				Timestamp: k.Timestamp,
				Line:      []byte(line),
			})
		}
		_ = f.Close()
	}
	return out, nil
}
