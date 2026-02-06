package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cpunion/sci-bot/pkg/feed"
	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/simulation"
	"github.com/cpunion/sci-bot/pkg/site"
)

func main() {
	dataPath := flag.String("data", "./data/adk-simulation", "Data directory (contains forum/, journal/, agents/)")
	feedDir := flag.String("feed", "feed", "Feed shards directory (relative to data directory). Set '-' to disable.")
	feedMaxEvents := flag.Int("feed-max-events", 200, "Max events per feed shard file")
	rebuildFeed := flag.Bool("rebuild-feed", false, "Rebuild sharded feed store from logs*.jsonl")
	feedHydrateDaily := flag.Bool("feed-hydrate-daily", true, "When rebuilding feed, hydrate prompt/response/error from per-agent daily JSONLs if available")
	flag.Parse()

	agents, err := indexAgents(*dataPath)
	if err != nil {
		log.Fatalf("Index agents: %v", err)
	}
	if err := writeAgents(*dataPath, agents); err != nil {
		log.Fatalf("Write agents index: %v", err)
	}

	feedIndexRel := ""
	if strings.TrimSpace(*feedDir) != "-" {
		dirName := strings.TrimSpace(*feedDir)
		if dirName == "" {
			dirName = "feed"
		}
		feedIndexRel = filepath.ToSlash(filepath.Join(dirName, "index.json"))
	}

	if *rebuildFeed && feedIndexRel != "" {
		if err := rebuildFeedStore(*dataPath, *feedDir, *feedMaxEvents, *feedHydrateDaily); err != nil {
			log.Fatalf("Rebuild feed store: %v", err)
		}
	}

	manifest, err := buildManifest(*dataPath, agents, feedIndexRel)
	if err != nil {
		log.Fatalf("Build manifest: %v", err)
	}
	if err := site.WriteManifest(filepath.Join(*dataPath, "site.json"), manifest); err != nil {
		log.Fatalf("Write manifest: %v", err)
	}

	fmt.Printf("Indexed %d agents -> %s\n", len(agents), filepath.Join(*dataPath, "agents", "agents.json"))
	fmt.Printf("Wrote manifest -> %s\n", filepath.Join(*dataPath, "site.json"))
}

func indexAgents(dataPath string) ([]site.Agent, error) {
	dir := filepath.Join(dataPath, "agents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	out := make([]site.Agent, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		statePath := filepath.Join(dir, id, "state.json")
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		var st struct {
			AgentID   string `json:"agent_id"`
			AgentName string `json:"agent_name"`
		}
		if err := json.Unmarshal(data, &st); err != nil {
			continue
		}
		name := strings.TrimSpace(st.AgentName)
		if name == "" {
			name = id
		}
		role := roleFromID(id)
		out = append(out, site.Agent{
			ID:   strings.TrimSpace(st.AgentID),
			Name: name,
			Role: role,
		})
	}

	// Fill missing IDs (some state.json might be legacy).
	for i := range out {
		if out[i].ID == "" {
			out[i].ID = out[i].Name
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Role != out[j].Role {
			return out[i].Role < out[j].Role
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func writeAgents(dataPath string, agents []site.Agent) error {
	path := filepath.Join(dataPath, "agents", "agents.json")
	cat := site.AgentCatalog{
		Version:     1,
		GeneratedAt: time.Now(),
		Agents:      agents,
	}
	data, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func buildManifest(dataPath string, agents []site.Agent, feedIndexRel string) (site.Manifest, error) {
	state, _ := simulation.LoadSimState(dataPath)

	forum := publication.NewForum("自由论坛", filepath.Join(dataPath, "forum"))
	_ = forum.Load()
	journal := publication.NewJournal("科学前沿", filepath.Join(dataPath, "journal"))
	_ = journal.Load()

	logs := discoverLogs(dataPath)
	defaultLog := ""
	if len(logs) > 0 {
		defaultLog = logs[len(logs)-1]
	}

	forumThreads := 0
	for _, p := range forum.AllPosts() {
		if p != nil && !p.IsComment {
			forumThreads++
		}
	}

	m := site.Manifest{
		Version:       1,
		GeneratedAt:   time.Now(),
		AgentsPath:    "agents/agents.json",
		ForumPath:     "forum/forum.json",
		JournalPath:   "journal/journal.json",
		FeedIndexPath: feedIndexRel,
		Logs:          logs,
		DefaultLog:    defaultLog,
		Stats: site.ManifestStats{
			AgentCount:      len(agents),
			ForumThreads:    forumThreads,
			JournalApproved: len(journal.GetApproved()),
			JournalPending:  len(journal.GetPending()),
		},
	}
	if state != nil {
		m.SimTime = state.SimTime
		m.StepSeconds = state.StepSeconds
	}
	return m, nil
}

func discoverLogs(dataPath string) []string {
	entries, err := os.ReadDir(dataPath)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "logs") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func roleFromID(id string) string {
	// Convention: agent-<role>-<n>
	if strings.HasPrefix(id, "agent-") {
		parts := strings.Split(id, "-")
		if len(parts) >= 3 {
			return parts[1]
		}
	}
	roles := []string{"reviewer", "explorer", "builder", "synthesizer", "communicator"}
	for _, role := range roles {
		if strings.Contains(id, role) {
			return role
		}
	}
	return "agent"
}

func rebuildFeedStore(dataPath string, feedDir string, maxEventsPerShard int, hydrateDaily bool) error {
	dirName := strings.TrimSpace(feedDir)
	if dirName == "" {
		dirName = "feed"
	}
	if dirName == "-" {
		return nil
	}

	logNames := discoverLogs(dataPath)
	if len(logNames) == 0 {
		return nil
	}
	logPaths := make([]string, 0, len(logNames))
	for _, name := range logNames {
		logPaths = append(logPaths, filepath.Join(dataPath, name))
	}

	abs := filepath.Join(dataPath, dirName)
	tmp := abs + ".rebuild-" + time.Now().Format("20060102-150405")
	if err := rebuildFeedFromLogs(tmp, dataPath, logPaths, maxEventsPerShard, hydrateDaily); err != nil {
		return err
	}

	// Preserve any previous feed directory (do not delete data).
	if st, err := os.Stat(abs); err == nil && st.IsDir() {
		bak := abs + ".bak-" + time.Now().Format("20060102-150405")
		if err := os.Rename(abs, bak); err != nil {
			return err
		}
	}

	return os.Rename(tmp, abs)
}

type dailyLogEntry struct {
	Timestamp string `json:"timestamp"`
	Prompt    string `json:"prompt,omitempty"`
	Reply     string `json:"reply,omitempty"`
	Error     string `json:"error,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Raw       string `json:"raw,omitempty"`
}

func rebuildFeedFromLogs(outDir, dataPath string, logPaths []string, maxEventsPerShard int, hydrateDaily bool) error {
	events, err := readLogEvents(logPaths)
	if err != nil {
		return err
	}

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].SimTime.Equal(events[j].SimTime) {
			return events[i].Timestamp.Before(events[j].Timestamp)
		}
		return events[i].SimTime.Before(events[j].SimTime)
	})

	w, err := feed.OpenWriter(feed.WriterConfig{
		Dir:               outDir,
		MaxEventsPerShard: maxEventsPerShard,
		Append:            false,
	})
	if err != nil {
		return err
	}
	defer w.Close()

	dailyCache := make(map[string]map[int64]dailyLogEntry)
	loadDaily := func(agentID, dateKey string) map[int64]dailyLogEntry {
		cacheKey := agentID + "|" + dateKey
		if v, ok := dailyCache[cacheKey]; ok {
			return v
		}

		m := make(map[int64]dailyLogEntry)
		path := filepath.Join(dataPath, "agents", agentID, "daily", dateKey+".jsonl")
		data, err := os.ReadFile(path)
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var e dailyLogEntry
				if err := json.Unmarshal([]byte(line), &e); err != nil {
					continue
				}
				t, err := time.Parse(time.RFC3339, strings.TrimSpace(e.Timestamp))
				if err != nil {
					continue
				}
				m[t.Unix()] = e
			}
		}

		dailyCache[cacheKey] = m
		return m
	}

	for _, ev := range events {
		if hydrateDaily && ev.AgentID != "" && !ev.SimTime.IsZero() {
			dateKey := ev.SimTime.Format("2006-01-02")
			idx := loadDaily(ev.AgentID, dateKey)
			if entry, ok := idx[ev.SimTime.Unix()]; ok {
				if strings.TrimSpace(entry.Prompt) != "" {
					ev.Prompt = strings.TrimSpace(entry.Prompt)
				}
				if strings.TrimSpace(entry.Reply) != "" {
					ev.Response = strings.TrimSpace(entry.Reply)
				}
				if strings.TrimSpace(entry.Error) != "" {
					ev.Error = strings.TrimSpace(entry.Error)
				}
			}
		}

		line, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		if err := w.AppendJSONLine(line); err != nil {
			return err
		}
	}

	_, err = feed.LoadIndex(filepath.Join(outDir, "index.json"))
	return err
}

func readLogEvents(paths []string) ([]simulation.EventLog, error) {
	out := make([]simulation.EventLog, 0, 1024)
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
			var ev simulation.EventLog
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				continue
			}
			out = append(out, ev)
		}
		_ = f.Close()
	}
	return out, nil
}
