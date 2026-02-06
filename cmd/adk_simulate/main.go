package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	ailibmodel "github.com/cpunion/ailib/adk/model"
	"github.com/cpunion/sci-bot/pkg/feed"
	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/simulation"
	"github.com/cpunion/sci-bot/pkg/site"
	"github.com/cpunion/sci-bot/pkg/types"
	"github.com/joho/godotenv"
	"google.golang.org/adk/model"
)

func main() {
	_ = godotenv.Load()

	modelDefault := envOr("GOOGLE_MODEL", "gemini-3-flash-preview")
	reviewerDefault := envOr("GOOGLE_REVIEWER_MODEL", "gemini-3-pro-preview")

	ticks := flag.Int("ticks", 50, "Number of simulation ticks")
	days := flag.Int("days", 0, "Simulated days (overrides ticks when > 0)")
	step := flag.Duration("step", time.Hour, "Simulated time per tick")
	dataPath := flag.String("data", "./data/adk-simulation", "Data directory")
	modelName := flag.String("model", modelDefault, "LLM model spec for general agents (e.g. gemini:gemini-3-flash-preview)")
	reviewerModelName := flag.String("reviewer-model", reviewerDefault, "LLM model spec for reviewer agents (e.g. gemini:gemini-3-pro-preview)")
	logPath := flag.String("log", "./data/adk-simulation/logs.jsonl", "Path to JSONL log file")
	logAppend := flag.Bool("log-append", true, "Append to log file instead of truncating")
	feedDir := flag.String("feed", "feed", "Feed shards directory (relative to data directory). Set '-' to disable.")
	feedMaxEvents := flag.Int("feed-max-events", 200, "Max events per feed shard file")
	turnLimit := flag.Int("turns", 10, "Per-agent turn limit before sleep")
	graceTurns := flag.Int("grace", 3, "Grace turns after bell")
	agentsPerTick := flag.Int("per-tick", 1, "Number of agents to run per tick")
	checkpointEvery := flag.Int("checkpoint", 1, "Checkpoint every N ticks (0 disables)")
	agentCount := flag.Int("agents", 5, "Number of agents")
	seed := flag.Int64("seed", time.Now().UnixNano(), "Random seed for personas")
	flag.Parse()

	if *days > 0 {
		*ticks = int(math.Ceil(float64(time.Duration(*days)*24*time.Hour) / float64(*step)))
	}

	ctx := context.Background()

	if err := os.MkdirAll(*dataPath, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	defaultModel, err := newLLM(ctx, normalizeModelSpec(*modelName))
	if err != nil {
		log.Fatalf("Failed to create model (%s): %v", *modelName, err)
	}

	reviewerModel := defaultModel
	if *reviewerModelName != *modelName {
		reviewerModel, err = newLLM(ctx, normalizeModelSpec(*reviewerModelName))
		if err != nil {
			log.Fatalf("Failed to create reviewer model (%s): %v", *reviewerModelName, err)
		}
	}

	var fileLogger simulation.EventLogger
	if strings.TrimSpace(*logPath) != "" {
		fileLogger, err = simulation.NewJSONLLogger(*logPath, *logAppend)
		if err != nil {
			log.Fatalf("Failed to create logger: %v", err)
		}
	}

	var feedLogger simulation.EventLogger
	feedIndexRel := ""
	if strings.TrimSpace(*feedDir) != "-" {
		dirName := strings.TrimSpace(*feedDir)
		if dirName == "" {
			dirName = "feed"
		}
		feedIndexRel = filepath.ToSlash(filepath.Join(dirName, "index.json"))
		abs := filepath.Join(*dataPath, dirName)
		fw, err := feed.OpenWriter(feed.WriterConfig{
			Dir:               abs,
			MaxEventsPerShard: *feedMaxEvents,
			Append:            true,
		})
		if err != nil {
			log.Fatalf("Failed to create feed store: %v", err)
		}
		feedLogger = &feedEventLogger{w: fw}
	}

	logger := simulation.NewMultiLogger(fileLogger, feedLogger)
	defer func() {
		_ = logger.Close()
	}()

	fmt.Println("=== Sci-Bot ADK Simulation ===")
	fmt.Printf("Model (default): %s\n", *modelName)
	fmt.Printf("Model (reviewer): %s\n", *reviewerModelName)
	fmt.Printf("Agents: %d\n", *agentCount)
	fmt.Printf("Ticks: %d\n", *ticks)
	fmt.Printf("Step: %s\n", step.String())
	fmt.Printf("Agents per tick: %d\n", *agentsPerTick)
	fmt.Printf("Checkpoint every: %d\n", *checkpointEvery)
	if strings.TrimSpace(*logPath) != "" {
		fmt.Printf("Log: %s\n", *logPath)
	}
	if feedIndexRel != "" {
		fmt.Printf("Feed index: %s\n", feedIndexRel)
	}
	fmt.Println()

	journal := publication.NewJournal("科学前沿", filepath.Join(*dataPath, "journal"))
	_ = journal.Load()

	forum := publication.NewForum("自由论坛", filepath.Join(*dataPath, "forum"))
	_ = forum.Load()

	startTime := time.Now()
	if state, err := simulation.LoadSimState(*dataPath); err == nil && !state.SimTime.IsZero() {
		startTime = state.SimTime
		if state.StepSeconds > 0 && time.Duration(state.StepSeconds)*time.Second != *step {
			log.Printf("Warning: sim step changed (prev %s, now %s)", time.Duration(state.StepSeconds)*time.Second, step.String())
		}
		fmt.Printf("Resume sim time: %s\n", startTime.Format(time.RFC3339))
	}

	personas, err := loadOrCreatePersonas(*dataPath, *agentCount, *seed)
	if err != nil {
		log.Printf("Warning: failed to load personas index, using generated personas: %v", err)
		personas = simulation.GeneratePersonas(*agentCount, *seed)
	}

	// Keep a static agents index for the frontend (no server API required).
	if err := site.WriteAgentCatalog(filepath.Join(*dataPath, "agents", "agents.json"), personas); err != nil {
		log.Printf("Warning: failed to write agents index: %v", err)
	}
	if len(forum.AllPosts()) == 0 {
		seedInitialContent(forum, personas)
	}

	sched := simulation.NewADKScheduler(simulation.ADKSchedulerConfig{
		DataPath:        *dataPath,
		Model:           defaultModel,
		Logger:          logger,
		SimStep:         *step,
		StartTime:       startTime,
		TurnLimit:       *turnLimit,
		GraceTurns:      *graceTurns,
		AgentsPerTick:   *agentsPerTick,
		CheckpointEvery: *checkpointEvery,
		ModelForPersona: func(p *types.Persona) model.LLM {
			if p.Role == types.RoleReviewer {
				return reviewerModel
			}
			return defaultModel
		},
	})
	sched.SetJournal(journal)
	sched.SetForum(forum)

	for _, p := range personas {
		if err := sched.AddAgent(ctx, p); err != nil {
			log.Fatalf("Failed to add agent %s: %v", p.ID, err)
		}
	}

	start := time.Now()
	if err := sched.RunFor(ctx, *ticks); err != nil {
		log.Fatalf("Simulation failed: %v", err)
	}
	elapsed := time.Since(start)

	stats := sched.Stats()
	fmt.Println("\n=== Simulation Complete ===")
	fmt.Printf("Duration: %v\n", elapsed)
	fmt.Printf("Ticks: %v\n", stats["ticks"])
	fmt.Printf("Agents: %v\n", stats["agents"])
	fmt.Printf("Actions: %v\n", stats["action_stats"])

	if err := sched.Save(); err != nil {
		log.Printf("Warning: failed to save state: %v", err)
	}

	// Persist personas so resumed runs keep consistent identities without needing flags.
	if err := savePersonas(*dataPath, *seed, personas); err != nil {
		log.Printf("Warning: failed to write personas.json: %v", err)
	}
	// Re-write the public agents index after the run, because agent names may
	// have been updated from persisted state during AddAgent.
	if err := site.WriteAgentCatalog(filepath.Join(*dataPath, "agents", "agents.json"), personas); err != nil {
		log.Printf("Warning: failed to write agents index: %v", err)
	}

	// Write a static site manifest so a purely-static frontend can discover files.
	if err := writeStaticManifest(*dataPath, *logPath, feedIndexRel, forum, journal, personas); err != nil {
		log.Printf("Warning: failed to write site manifest: %v", err)
	}

	if strings.TrimSpace(*logPath) != "" {
		if summary, err := analyzeLog(*logPath); err == nil {
			printSummary(summary)
		} else {
			log.Printf("Log analysis skipped: %v", err)
		}
	}

	fmt.Println("\nState saved to:", *dataPath)
}

func loadOrCreatePersonas(dataPath string, count int, seed int64) ([]*types.Persona, error) {
	path := filepath.Join(dataPath, "personas.json")
	data, err := os.ReadFile(path)
	if err == nil {
		var store struct {
			Version  int              `json:"version"`
			Seed     int64            `json:"seed,omitempty"`
			Personas []*types.Persona `json:"personas"`
		}
		if jsonErr := json.Unmarshal(data, &store); jsonErr == nil && len(store.Personas) > 0 {
			out := store.Personas
			if seed == 0 && store.Seed != 0 {
				seed = store.Seed
			}
			if count > 0 && count < len(out) {
				return out[:count], nil
			}
			if count > 0 && count > len(out) {
				expanded := simulation.GeneratePersonas(count, seed)
				byID := make(map[string]*types.Persona, len(out))
				for _, p := range out {
					if p != nil && p.ID != "" {
						byID[p.ID] = p
					}
				}
				merged := make([]*types.Persona, 0, len(expanded))
				for _, p := range expanded {
					if p == nil || p.ID == "" {
						continue
					}
					if existing, ok := byID[p.ID]; ok {
						merged = append(merged, existing)
					} else {
						merged = append(merged, p)
					}
				}
				_ = savePersonas(dataPath, seed, merged)
				return merged, nil
			}
			return out, nil
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	personas := simulation.GeneratePersonas(count, seed)
	if writeErr := savePersonas(dataPath, seed, personas); writeErr != nil {
		return personas, writeErr
	}
	return personas, nil
}

func savePersonas(dataPath string, seed int64, personas []*types.Persona) error {
	store := struct {
		Version     int              `json:"version"`
		GeneratedAt time.Time        `json:"generated_at"`
		Seed        int64            `json:"seed,omitempty"`
		Personas    []*types.Persona `json:"personas"`
	}{
		Version:     1,
		GeneratedAt: time.Now(),
		Seed:        seed,
		Personas:    personas,
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataPath, "personas.json"), data, 0644)
}

func writeStaticManifest(dataPath string, logPath string, feedIndexRel string, forum *publication.Forum, journal *publication.Journal, personas []*types.Persona) error {
	state, _ := simulation.LoadSimState(dataPath)

	logs := discoverLogs(dataPath)
	defaultLog := filepath.Base(logPath)
	if defaultLog != "" && defaultLog != logPath {
		// non-empty base
	} else if defaultLog == "" {
		defaultLog = ""
	}
	if defaultLog != "" {
		found := false
		for _, n := range logs {
			if n == defaultLog {
				found = true
				break
			}
		}
		if !found {
			logs = append([]string{defaultLog}, logs...)
		}
	}

	var forumThreads int
	if forum != nil {
		for _, p := range forum.AllPosts() {
			if p != nil && !p.IsComment {
				forumThreads++
			}
		}
	}

	jApproved := 0
	jPending := 0
	if journal != nil {
		jApproved = len(journal.GetApproved())
		jPending = len(journal.GetPending())
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
			AgentCount:      len(personas),
			ForumThreads:    forumThreads,
			JournalApproved: jApproved,
			JournalPending:  jPending,
		},
	}
	if state != nil {
		m.SimTime = state.SimTime
		m.StepSeconds = state.StepSeconds
	}

	return site.WriteManifest(filepath.Join(dataPath, "site.json"), m)
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

func normalizeModelSpec(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return spec
	}
	// If the provider is explicitly specified, keep it.
	if strings.Contains(spec, ":") {
		return spec
	}
	// If the spec contains a slash, treat it as an OpenRouter-style model name and
	// let the factory default to openrouter (backward compatible).
	if strings.Contains(spec, "/") {
		return spec
	}
	// Backward compatible default: our project historically assumes Gemini.
	return ailibmodel.ProviderGemini + ":" + spec
}

func newLLM(ctx context.Context, modelSpec string) (model.LLM, error) {
	provider, _ := ailibmodel.ParseModelString(modelSpec)
	// Keep compatibility with existing .env (`GOOGLE_API_KEY`) while still
	// allowing provider-specific env vars for other providers.
	//
	// ailib expects GEMINI_API_KEY for Gemini provider, while this repo uses
	// GOOGLE_API_KEY historically.
	if provider == ailibmodel.ProviderGemini {
		if os.Getenv("GEMINI_API_KEY") == "" && os.Getenv("GOOGLE_API_KEY") != "" {
			_ = os.Setenv("GEMINI_API_KEY", os.Getenv("GOOGLE_API_KEY"))
		}
	}

	return ailibmodel.New(ctx, modelSpec)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func seedInitialContent(forum *publication.Forum, personas []*types.Persona) {
	if len(personas) < 3 {
		return
	}
	initialPosts := []*types.Publication{
		{
			ID:         "seed-1",
			AuthorID:   personas[0].ID,
			AuthorName: personas[0].Name,
			Title:      "关于自由落体的思考",
			Content:    "如果在真空中，羽毛和铁球会以相同速度下落吗？",
			Abstract:   "对亚里士多德物理学的质疑",
			Subreddit:  types.SubPhysics,
		},
		{
			ID:         "seed-2",
			AuthorID:   personas[1].ID,
			AuthorName: personas[1].Name,
			Title:      "几何学第一原理",
			Content:    "过两点有且仅有一条直线。这是不证自明的公理。",
			Abstract:   "欧几里得几何的基础",
			Subreddit:  types.SubMathematics,
		},
		{
			ID:         "seed-3",
			AuthorID:   personas[2].ID,
			AuthorName: personas[2].Name,
			Title:      "理论可证伪性的重要性",
			Content:    "任何科学理论都必须允许被实验推翻，否则只是形而上学。",
			Abstract:   "科学方法的核心要求",
			Subreddit:  types.SubPhilosophy,
		},
	}

	for _, post := range initialPosts {
		forum.Post(post)
	}
}
