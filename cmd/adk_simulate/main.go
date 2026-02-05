package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/simulation"
	"github.com/cpunion/sci-bot/pkg/types"
	"github.com/joho/godotenv"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

func main() {
	_ = godotenv.Load()

	modelDefault := envOr("GOOGLE_MODEL", "gemini-3-flash-preview")
	reviewerDefault := envOr("GOOGLE_REVIEWER_MODEL", "gemini-3-pro-preview")

	ticks := flag.Int("ticks", 50, "Number of simulation ticks")
	days := flag.Int("days", 0, "Simulated days (overrides ticks when > 0)")
	step := flag.Duration("step", time.Hour, "Simulated time per tick")
	dataPath := flag.String("data", "./data/adk-simulation", "Data directory")
	modelName := flag.String("model", modelDefault, "Gemini model for general agents")
	reviewerModelName := flag.String("reviewer-model", reviewerDefault, "Gemini model for reviewer agents")
	logPath := flag.String("log", "./data/adk-simulation/logs.jsonl", "Path to JSONL log file")
	turnLimit := flag.Int("turns", 10, "Per-agent turn limit before sleep")
	graceTurns := flag.Int("grace", 3, "Grace turns after bell")
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

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		log.Fatal("GOOGLE_API_KEY not set")
	}

	clientCfg := &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	}

	defaultModel, err := gemini.NewModel(ctx, *modelName, clientCfg)
	if err != nil {
		log.Fatalf("Failed to create Gemini model (%s): %v", *modelName, err)
	}

	reviewerModel := defaultModel
	if *reviewerModelName != *modelName {
		reviewerModel, err = gemini.NewModel(ctx, *reviewerModelName, clientCfg)
		if err != nil {
			log.Fatalf("Failed to create reviewer model (%s): %v", *reviewerModelName, err)
		}
	}

	logger, err := simulation.NewJSONLLogger(*logPath)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	fmt.Println("=== Sci-Bot ADK Simulation (Gemini) ===")
	fmt.Printf("Model (default): %s\n", *modelName)
	fmt.Printf("Model (reviewer): %s\n", *reviewerModelName)
	fmt.Printf("Agents: %d\n", *agentCount)
	fmt.Printf("Ticks: %d\n", *ticks)
	fmt.Printf("Step: %s\n", step.String())
	fmt.Printf("Log: %s\n\n", *logPath)

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

	personas := simulation.GeneratePersonas(*agentCount, *seed)
	if len(forum.AllPosts()) == 0 {
		seedInitialContent(forum, personas)
	}

	sched := simulation.NewADKScheduler(simulation.ADKSchedulerConfig{
		DataPath:   *dataPath,
		Model:      defaultModel,
		Logger:     logger,
		SimStep:    *step,
		StartTime:  startTime,
		TurnLimit:  *turnLimit,
		GraceTurns: *graceTurns,
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

	if summary, err := analyzeLog(*logPath); err == nil {
		printSummary(summary)
	} else {
		log.Printf("Log analysis skipped: %v", err)
	}

	fmt.Println("\nState saved to:", *dataPath)
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
