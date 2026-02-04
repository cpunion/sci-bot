package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/cpunion/sci-bot/pkg/agent"
	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/simulation"
	"github.com/cpunion/sci-bot/pkg/types"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	_ = godotenv.Load()

	// Flags
	ticks := flag.Int("ticks", 100, "Number of simulation ticks")
	dataPath := flag.String("data", "./data/simulation", "Data directory")
	flag.Parse()

	fmt.Println("=== Sci-Bot Emergence Simulation ===")
	fmt.Printf("Running %d ticks...\n\n", *ticks)

	// Ensure data directory
	if err := os.MkdirAll(*dataPath, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create agents
	personas := []*types.Persona{
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

	// Initialize agents and states
	agents := make([]*agent.Agent, 0, len(personas))
	states := make(map[string]*agent.AgentState)

	for _, p := range personas {
		agentPath := filepath.Join(*dataPath, "agents", p.ID)
		cfg := agent.DefaultConfig(p, agentPath)
		a := agent.New(cfg)
		agents = append(agents, a)

		// Load or create state
		state := agent.NewAgentState(p.ID, p.Name, agentPath)
		state.Load()
		states[p.ID] = state
	}

	// Initialize publication channels
	journal := publication.NewJournal("科学前沿", filepath.Join(*dataPath, "journal"))
	journal.Load()

	forum := publication.NewForum("自由论坛", filepath.Join(*dataPath, "forum"))
	forum.Load()

	// Seed some initial content if empty
	if len(forum.AllPosts()) == 0 {
		seedInitialContent(forum, personas)
	}

	// Create scheduler
	sched := simulation.NewScheduler(agents, states, journal, forum)

	// Stats tracking
	actionCounts := make(map[simulation.ActionType]int)
	agentCounts := make(map[string]int)

	sched.SetOnTick(func(tick int, agentID string, action simulation.ActionType) {
		actionCounts[action]++
		for _, a := range agents {
			if a.ID() == agentID {
				agentCounts[a.Name()]++
				break
			}
		}
	})

	// Run simulation
	startTime := time.Now()
	sched.RunFor(*ticks)
	elapsed := time.Since(startTime)

	// Print results
	fmt.Println("\n=== Simulation Complete ===")
	fmt.Printf("Duration: %v\n", elapsed)
	fmt.Printf("Ticks: %d\n\n", *ticks)

	fmt.Println("Agent Activity:")
	for name, count := range agentCounts {
		fmt.Printf("  %s: %d actions\n", name, count)
	}

	fmt.Println("\nAction Distribution:")
	for action, count := range actionCounts {
		pct := float64(count) / float64(*ticks) * 100
		fmt.Printf("  %s: %d (%.1f%%)\n", action, count, pct)
	}

	fmt.Println("\nRelationship Summary:")
	for _, a := range agents {
		state := states[a.ID()]
		trusted := len(state.GetTrustedPeers())
		active := len(state.GetActivePeers())
		knowledge := len(state.GetKnowledgeByLevel(types.KnowledgeMastered))
		fmt.Printf("  %s: %d trusted, %d active, %d mastered theories\n",
			a.Name(), trusted, active, knowledge)
	}

	fmt.Println("\nPublication Stats:")
	fmt.Printf("  Journal: %d approved, %d pending\n",
		len(journal.GetApproved()), len(journal.GetPending()))
	fmt.Printf("  Forum: %d posts\n", len(forum.AllPosts()))

	// Save all state
	if err := sched.Save(); err != nil {
		log.Printf("Warning: Failed to save state: %v", err)
	}

	fmt.Println("\nState saved to:", *dataPath)
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
		},
		{
			ID:         "seed-2",
			AuthorID:   personas[1].ID,
			AuthorName: personas[1].Name,
			Title:      "几何学第一原理",
			Content:    "过两点有且仅有一条直线。这是不证自明的公理。",
			Abstract:   "欧几里得几何的基础",
		},
		{
			ID:         "seed-3",
			AuthorID:   personas[2].ID,
			AuthorName: personas[2].Name,
			Title:      "科学理论的可证伪性",
			Content:    "一个理论只有当它能够被证伪时，才是科学的。",
			Abstract:   "科学与非科学的划界",
		},
	}

	for _, post := range initialPosts {
		forum.Post(post)
	}

	fmt.Printf("Seeded %d initial forum posts\n", len(initialPosts))
}
