package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cpunion/sci-bot/pkg/agent"
	"github.com/cpunion/sci-bot/pkg/knowledge"
	"github.com/cpunion/sci-bot/pkg/llm"
	"github.com/cpunion/sci-bot/pkg/network"
	"github.com/cpunion/sci-bot/pkg/types"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if exists
	_ = godotenv.Load()

	fmt.Println("=== Sci-Bot Network Demo with Gemini ===")
	fmt.Println()

	ctx := context.Background()

	// Setup data directory
	dataPath := "./data"
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize Gemini provider
	geminiCfg := llm.DefaultGeminiConfig()
	gemini, err := llm.NewGeminiProvider(ctx, geminiCfg)
	if err != nil {
		log.Fatalf("Failed to create Gemini provider: %v\nMake sure GOOGLE_API_KEY is set.", err)
	}
	fmt.Printf("Gemini provider initialized (model: %s)\n\n", gemini.Model())

	// Initialize knowledge layer
	axiomReg := knowledge.NewAxiomRegistry(dataPath)
	if err := setupDefaultAxiomSystems(axiomReg); err != nil {
		log.Fatalf("Failed to setup axiom systems: %v", err)
	}

	theoryRepo := knowledge.NewTheoryRepository(dataPath, axiomReg)

	// Initialize network
	broker := network.NewMessageBroker()

	// Create diverse agents with Gemini
	agents := createDiverseAgents(dataPath, gemini)
	for _, a := range agents {
		broker.Register(a)
		if err := a.Start(); err != nil {
			log.Printf("Failed to start agent %s: %v", a.ID(), err)
		}
	}

	fmt.Println("Registered agents:")
	for _, a := range agents {
		fmt.Printf("  - %s (%s, %s thinking, risk=%.1f)\n",
			a.Name(), a.Role(), a.Persona.ThinkingStyle, a.Persona.RiskTolerance)
	}
	fmt.Println()

	// Subscribe agents to topics
	for _, a := range agents {
		broker.Subscribe(a.ID(), "general")
		broker.Subscribe(a.ID(), "theories")
		for _, domain := range a.Persona.Domains {
			broker.Subscribe(a.ID(), domain)
		}
	}

	// Interactive mode
	fmt.Println("=== Interactive Mode ===")
	fmt.Println("Commands:")
	fmt.Println("  /ask <agent> <question>  - Ask a specific agent")
	fmt.Println("  /broadcast <message>     - Broadcast to all agents")
	fmt.Println("  /theory <title>          - Propose a theory discussion")
	fmt.Println("  /list                    - List agents")
	fmt.Println("  /axioms                  - List axiom systems")
	fmt.Println("  /quit                    - Exit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "/quit" || input == "/exit" {
			break
		}

		if input == "/list" {
			fmt.Println("Agents:")
			for _, a := range agents {
				fmt.Printf("  - %s (%s)\n", a.Name(), a.Role())
			}
			continue
		}

		if input == "/axioms" {
			fmt.Println("Axiom Systems:")
			for _, sys := range axiomReg.List() {
				fmt.Printf("  - %s: %s\n", sys.ID, sys.Name)
			}
			continue
		}

		if strings.HasPrefix(input, "/ask ") {
			parts := strings.SplitN(input[5:], " ", 2)
			if len(parts) < 2 {
				fmt.Println("Usage: /ask <agent_name> <question>")
				continue
			}
			agentName := strings.ToLower(parts[0])
			question := parts[1]

			// Find agent
			var targetAgent *agent.Agent
			for _, a := range agents {
				if strings.ToLower(a.Name()) == agentName {
					targetAgent = a
					break
				}
			}
			if targetAgent == nil {
				fmt.Printf("Agent '%s' not found\n", agentName)
				continue
			}

			// Send message directly to agent
			msg := &types.Message{
				ID:         fmt.Sprintf("user-%d", time.Now().UnixNano()),
				Type:       types.MsgQuestion,
				From:       "user",
				To:         []string{targetAgent.ID()},
				Content:    question,
				Visibility: types.VisibilityPrivate,
				Timestamp:  time.Now(),
			}

			targetAgent.Inbox <- msg
			fmt.Printf("Sent to %s. Waiting for response...\n", targetAgent.Name())

			// Wait for response
			select {
			case resp := <-targetAgent.Outbox:
				fmt.Printf("\n[%s]: %s\n\n", targetAgent.Name(), resp.Content)
			case <-time.After(30 * time.Second):
				fmt.Println("Response timeout")
			}
			continue
		}

		if strings.HasPrefix(input, "/broadcast ") {
			message := input[11:]
			msg := &types.Message{
				ID:         fmt.Sprintf("user-%d", time.Now().UnixNano()),
				Type:       types.MsgChat,
				From:       "user",
				To:         []string{},
				Content:    message,
				Visibility: types.VisibilityPublic,
				Timestamp:  time.Now(),
			}

			broker.Broadcast(msg)
			fmt.Println("Message broadcast to all agents.")
			continue
		}

		if strings.HasPrefix(input, "/theory ") {
			title := input[8:]
			theory := &types.Theory{
				ID:          fmt.Sprintf("theory-%d", time.Now().UnixNano()),
				Title:       title,
				Authors:     []string{"user"},
				Status:      types.StatusDraft,
				IsHeretical: true,
			}
			if err := theoryRepo.Propose(theory); err != nil {
				fmt.Printf("Failed to propose theory: %v\n", err)
				continue
			}
			fmt.Printf("Theory proposed: %s (ID: %s)\n", title, theory.ID)
			continue
		}

		// Default: treat as a general question to a random explorer
		for _, a := range agents {
			if a.Role() == types.RoleExplorer {
				msg := &types.Message{
					ID:         fmt.Sprintf("user-%d", time.Now().UnixNano()),
					Type:       types.MsgQuestion,
					From:       "user",
					To:         []string{a.ID()},
					Content:    input,
					Visibility: types.VisibilityPublic,
					Timestamp:  time.Now(),
				}
				a.Inbox <- msg
				fmt.Printf("Sent to %s (explorer). Waiting for response...\n", a.Name())

				select {
				case resp := <-a.Outbox:
					fmt.Printf("\n[%s]: %s\n\n", a.Name(), resp.Content)
				case <-time.After(30 * time.Second):
					fmt.Println("Response timeout")
				}
				break
			}
		}
	}

	// Cleanup
	fmt.Println("\nStopping agents...")
	for _, a := range agents {
		if err := a.Stop(); err != nil {
			log.Printf("Failed to stop agent %s: %v", a.ID(), err)
		}
	}
	fmt.Println("Done.")
}

// createDiverseAgents creates a diverse set of agents with Gemini.
func createDiverseAgents(dataPath string, llmProvider agent.LLMProvider) []*agent.Agent {
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
			Sociability:   0.7,
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
			Sociability:   0.5,
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
			Sociability:   0.6,
		},
	}

	agents := make([]*agent.Agent, 0, len(personas))
	for _, p := range personas {
		cfg := agent.DefaultConfig(p, filepath.Join(dataPath, "agents", p.ID))
		cfg.ThinkInterval = 10 * time.Minute
		cfg.ExploreInterval = 20 * time.Minute
		a := agent.New(cfg)
		a.SetLLM(llmProvider)
		agents = append(agents, a)
	}

	return agents
}

// setupDefaultAxiomSystems creates some default axiom systems.
func setupDefaultAxiomSystems(reg *knowledge.AxiomRegistry) error {
	systems := []*types.AxiomSystem{
		{
			ID:          "euclidean-geometry",
			Name:        "欧几里得几何公理体系",
			Description: "经典平面几何的公理体系",
			Axioms: []types.Axiom{
				{ID: "e1", Statement: "两点确定一条直线"},
				{ID: "e2", Statement: "线段可以无限延长"},
				{ID: "e3", Statement: "以任意点为圆心、任意长度为半径可以画圆"},
				{ID: "e4", Statement: "所有直角都相等"},
				{ID: "e5", Statement: "平行公设：过直线外一点有且仅有一条平行线"},
			},
			CreatedBy: "system",
			CreatedAt: time.Now(),
		},
		{
			ID:          "hyperbolic-geometry",
			Name:        "双曲几何公理体系",
			Description: "罗巴切夫斯基几何，否定第五公设",
			Axioms: []types.Axiom{
				{ID: "h1", Statement: "两点确定一条直线"},
				{ID: "h2", Statement: "线段可以无限延长"},
				{ID: "h3", Statement: "以任意点为圆心、任意长度为半径可以画圆"},
				{ID: "h4", Statement: "所有直角都相等"},
				{ID: "h5", Statement: "过直线外一点有无穷多条平行线"},
			},
			Parent:      "euclidean-geometry",
			Differences: []string{"修改第五公设：允许无穷多平行线"},
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "zfc-set-theory",
			Name:        "ZFC 集合论公理体系",
			Description: "策梅洛-弗兰克尔集合论（含选择公理）",
			Axioms: []types.Axiom{
				{ID: "zfc1", Statement: "外延公理：两个集合相等当且仅当它们有相同的元素"},
				{ID: "zfc2", Statement: "空集公理：存在一个不包含任何元素的集合"},
				{ID: "zfc3", Statement: "配对公理：对于任意两个集合，存在一个恰好包含它们作为元素的集合"},
			},
			CreatedBy: "system",
			CreatedAt: time.Now(),
		},
	}

	for _, sys := range systems {
		_ = reg.Register(sys) // Ignore already exists errors
	}

	// Save as JSON for reference
	data, _ := json.MarshalIndent(systems, "", "  ")
	os.WriteFile("data/axiom_systems_reference.json", data, 0644)

	return nil
}
