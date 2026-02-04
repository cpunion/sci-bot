package main

import (
	"context"
	"flag"
	"fmt"
	"log"
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
	dataPath := flag.String("data", "./data/adk-simulation", "Data directory")
	modelName := flag.String("model", modelDefault, "Gemini model for general agents")
	reviewerModelName := flag.String("reviewer-model", reviewerDefault, "Gemini model for reviewer agents")
	flag.Parse()

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

	fmt.Println("=== Sci-Bot ADK Simulation (Gemini) ===")
	fmt.Printf("Model (default): %s\n", *modelName)
	fmt.Printf("Model (reviewer): %s\n", *reviewerModelName)
	fmt.Printf("Ticks: %d\n\n", *ticks)

	journal := publication.NewJournal("科学前沿", filepath.Join(*dataPath, "journal"))
	_ = journal.Load()

	forum := publication.NewForum("自由论坛", filepath.Join(*dataPath, "forum"))
	_ = forum.Load()

	personas := defaultPersonas()
	if len(forum.AllPosts()) == 0 {
		seedInitialContent(forum, personas)
	}

	sched := simulation.NewADKScheduler(simulation.ADKSchedulerConfig{
		DataPath: *dataPath,
		Model:    defaultModel,
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

	fmt.Println("\nState saved to:", *dataPath)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func defaultPersonas() []*types.Persona {
	return []*types.Persona{
		{
			ID:            "agent-explorer-1",
			Name:          "Galileo",
			Role:          types.RoleExplorer,
			ThinkingStyle: types.StyleDivergent,
			RiskTolerance: 0.9,
			Creativity:    0.9,
			Rigor:         0.4,
			Domains:       []string{"physics", "astronomy"},
			Sociability:   0.8,
			Influence:     0.7,
		},
		{
			ID:            "agent-builder-1",
			Name:          "Euclid",
			Role:          types.RoleBuilder,
			ThinkingStyle: types.StyleConvergent,
			RiskTolerance: 0.3,
			Creativity:    0.5,
			Rigor:         0.95,
			Domains:       []string{"mathematics", "geometry"},
			Sociability:   0.4,
			Influence:     0.8,
		},
		{
			ID:            "agent-reviewer-1",
			Name:          "Popper",
			Role:          types.RoleReviewer,
			ThinkingStyle: types.StyleAnalytical,
			RiskTolerance: 0.4,
			Creativity:    0.4,
			Rigor:         0.9,
			Domains:       []string{"philosophy", "methodology"},
			Sociability:   0.5,
			Influence:     0.6,
		},
		{
			ID:            "agent-synthesizer-1",
			Name:          "Darwin",
			Role:          types.RoleSynthesizer,
			ThinkingStyle: types.StyleLateral,
			RiskTolerance: 0.7,
			Creativity:    0.8,
			Rigor:         0.6,
			Domains:       []string{"biology", "evolution"},
			Sociability:   0.6,
			Influence:     0.7,
		},
		{
			ID:            "agent-communicator-1",
			Name:          "Feynman",
			Role:          types.RoleCommunicator,
			ThinkingStyle: types.StyleIntuitive,
			RiskTolerance: 0.6,
			Creativity:    0.85,
			Rigor:         0.7,
			Domains:       []string{"physics", "education"},
			Sociability:   0.95,
			Influence:     0.9,
		},
	}
}

func seedInitialContent(forum *publication.Forum, personas []*types.Persona) {
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
