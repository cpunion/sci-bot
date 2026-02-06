package feed

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Writer struct {
	mu sync.Mutex

	dir               string
	indexPath         string
	maxEventsPerShard int

	idx *Index

	curFile   *os.File
	curWriter *bufio.Writer
	curSeq    int
	curEvents int
}

type WriterConfig struct {
	Dir               string
	MaxEventsPerShard int
	Append            bool
}

func OpenWriter(cfg WriterConfig) (*Writer, error) {
	if cfg.Dir == "" {
		return nil, errors.New("feed dir is required")
	}
	if cfg.MaxEventsPerShard <= 0 {
		cfg.MaxEventsPerShard = 200
	}
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, err
	}

	w := &Writer{
		dir:               cfg.Dir,
		indexPath:         filepath.Join(cfg.Dir, "index.json"),
		maxEventsPerShard: cfg.MaxEventsPerShard,
		idx: &Index{
			Version:           1,
			MaxEventsPerShard: cfg.MaxEventsPerShard,
			Shards:            nil,
		},
	}

	if cfg.Append {
		if idx, err := LoadIndex(w.indexPath); err == nil && idx != nil {
			w.idx = idx
			if w.idx.MaxEventsPerShard == 0 {
				w.idx.MaxEventsPerShard = cfg.MaxEventsPerShard
			}
			// Best-effort: keep TotalEvents consistent even if older index files lacked it.
			sum := 0
			for _, s := range w.idx.Shards {
				sum += s.Events
			}
			if w.idx.TotalEvents < sum {
				w.idx.TotalEvents = sum
			}
		}
	}

	if err := w.openForAppend(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) openForAppend() error {
	last := w.lastShard()
	if last != nil {
		seq := last.Seq
		path := filepath.Join(w.dir, last.File)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		w.curFile = f
		w.curWriter = bufio.NewWriter(f)
		w.curSeq = seq
		w.curEvents = last.Events
		if w.curEvents <= 0 {
			// Repair missing shard counts by scanning the file once.
			w.curEvents = countLines(path)
			for i := range w.idx.Shards {
				if w.idx.Shards[i].Seq == w.curSeq {
					w.idx.Shards[i].Events = w.curEvents
					break
				}
			}
			sum := 0
			for _, s := range w.idx.Shards {
				sum += s.Events
			}
			w.idx.TotalEvents = sum
			_ = SaveIndexAtomic(w.indexPath, w.idx)
		}
		return nil
	}

	if cfgSeq := detectMaxSeq(w.dir); cfgSeq > 0 {
		// Index missing but shards exist. Rebuild index from disk and resume on the newest shard.
		w.idx = rebuildIndexFromDisk(w.dir, w.maxEventsPerShard)
		_ = SaveIndexAtomic(w.indexPath, w.idx)
		return w.openForAppend()
	}
	return w.rotateTo(1)
}

func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Individual lines should be small, but be safe.
	scanner.Buffer(make([]byte, 0, 256*1024), 8*1024*1024)
	n := 0
	for scanner.Scan() {
		if len(bytes.TrimSpace(scanner.Bytes())) > 0 {
			n++
		}
	}
	return n
}

func detectMaxSeq(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	maxSeq := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		seq := parseShardSeq(name)
		if seq > maxSeq {
			maxSeq = seq
		}
	}
	return maxSeq
}

func rebuildIndexFromDisk(dir string, maxEventsPerShard int) *Index {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return &Index{Version: 1, MaxEventsPerShard: maxEventsPerShard}
	}

	shards := make([]Shard, 0, 16)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		seq := parseShardSeq(name)
		if seq <= 0 {
			continue
		}
		path := filepath.Join(dir, name)
		n := countLines(path)
		shards = append(shards, Shard{Seq: seq, File: name, Events: n})
	}
	sort.Slice(shards, func(i, j int) bool { return shards[i].Seq < shards[j].Seq })

	total := 0
	for _, s := range shards {
		total += s.Events
	}

	return &Index{
		Version:           1,
		MaxEventsPerShard: maxEventsPerShard,
		Shards:            shards,
		TotalEvents:       total,
	}
}

func parseShardSeq(name string) int {
	// events-000123.jsonl
	if !strings.HasPrefix(name, "events-") || !strings.HasSuffix(name, ".jsonl") {
		return 0
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(name, "events-"), ".jsonl")
	n, err := strconv.Atoi(mid)
	if err != nil {
		return 0
	}
	return n
}

func shardFileName(seq int) string {
	return fmt.Sprintf("events-%06d.jsonl", seq)
}

func (w *Writer) lastShard() *Shard {
	if w.idx == nil || len(w.idx.Shards) == 0 {
		return nil
	}
	// Shards are append-only; last is newest.
	return &w.idx.Shards[len(w.idx.Shards)-1]
}

func (w *Writer) rotateTo(seq int) error {
	if w.curWriter != nil {
		_ = w.curWriter.Flush()
	}
	if w.curFile != nil {
		_ = w.curFile.Close()
	}

	file := shardFileName(seq)
	path := filepath.Join(w.dir, file)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	w.curFile = f
	w.curWriter = bufio.NewWriter(f)
	w.curSeq = seq
	w.curEvents = 0

	// Ensure shard is registered in the index.
	if w.idx == nil {
		w.idx = &Index{Version: 1, MaxEventsPerShard: w.maxEventsPerShard}
	}
	if w.idx.MaxEventsPerShard == 0 {
		w.idx.MaxEventsPerShard = w.maxEventsPerShard
	}

	found := false
	for i := range w.idx.Shards {
		if w.idx.Shards[i].Seq == seq {
			w.idx.Shards[i].File = file
			found = true
			break
		}
	}
	if !found {
		w.idx.Shards = append(w.idx.Shards, Shard{Seq: seq, File: file, Events: 0})
		sort.Slice(w.idx.Shards, func(i, j int) bool { return w.idx.Shards[i].Seq < w.idx.Shards[j].Seq })
	}
	return SaveIndexAtomic(w.indexPath, w.idx)
}

func (w *Writer) AppendJSONLine(line []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.curWriter == nil {
		return errors.New("writer not initialized")
	}

	// Rotate only when we are about to write into a full shard. This avoids
	// persisting empty "next" shards in the index.
	if w.curEvents >= w.maxEventsPerShard {
		if err := w.rotateTo(w.curSeq + 1); err != nil {
			return err
		}
	}

	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil
	}
	if !bytes.HasSuffix(trimmed, []byte("\n")) {
		trimmed = append(trimmed, '\n')
	}

	if _, err := w.curWriter.Write(trimmed); err != nil {
		return err
	}
	if err := w.curWriter.Flush(); err != nil {
		return err
	}

	w.curEvents++
	w.idx.TotalEvents++
	for i := range w.idx.Shards {
		if w.idx.Shards[i].Seq == w.curSeq {
			w.idx.Shards[i].Events = w.curEvents
			break
		}
	}
	if err := SaveIndexAtomic(w.indexPath, w.idx); err != nil {
		return err
	}
	return nil
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var err error
	if w.curWriter != nil {
		err = w.curWriter.Flush()
	}
	if w.curFile != nil {
		closeErr := w.curFile.Close()
		if err == nil {
			err = closeErr
		}
	}
	if w.idx != nil {
		_ = SaveIndexAtomic(w.indexPath, w.idx)
	}
	return err
}
