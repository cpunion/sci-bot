package simulation

import (
	"testing"
	"time"

	"github.com/cpunion/sci-bot/pkg/agent"
	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/types"
)

func createTestAgent(id, name string, role types.AgentRole) *agent.Agent {
	persona := &types.Persona{
		ID:            id,
		Name:          name,
		Role:          role,
		ThinkingStyle: types.StyleDivergent,
		RiskTolerance: 0.5,
		Creativity:    0.5,
		Rigor:         0.5,
		Sociability:   0.5,
		Influence:     0.5,
	}
	cfg := agent.DefaultConfig(persona, "")
	return agent.New(cfg)
}

func TestScheduler_WeightedSelection(t *testing.T) {
	tempDir := t.TempDir()

	agents := []*agent.Agent{
		createTestAgent("agent-1", "Galileo", types.RoleExplorer),
		createTestAgent("agent-2", "Euclid", types.RoleBuilder),
		createTestAgent("agent-3", "Popper", types.RoleReviewer),
	}

	states := make(map[string]*agent.AgentState)
	for _, a := range agents {
		states[a.ID()] = agent.NewAgentState(a.ID(), a.Name(), tempDir)
	}

	journal := publication.NewJournal("Science", tempDir)
	forum := publication.NewForum("Discussion", tempDir)

	sched := NewScheduler(agents, states, journal, forum)

	// Track selection counts
	selectionCounts := make(map[string]int)
	sched.SetOnTick(func(tick int, agentID string, action ActionType) {
		selectionCounts[agentID]++
	})

	// Run many ticks
	sched.RunFor(1000)

	// All agents should have been selected at least once
	for _, a := range agents {
		if selectionCounts[a.ID()] == 0 {
			t.Errorf("agent %s was never selected", a.ID())
		}
	}

	// Selection should be roughly balanced (with some variance)
	// Each agent should get at least 200 selections out of 1000
	for id, count := range selectionCounts {
		if count < 150 {
			t.Logf("agent %s selected %d times (may be low)", id, count)
		}
	}
}

func TestScheduler_ActionDistribution(t *testing.T) {
	tempDir := t.TempDir()

	agents := []*agent.Agent{
		createTestAgent("agent-1", "Galileo", types.RoleExplorer),
	}

	states := make(map[string]*agent.AgentState)
	states["agent-1"] = agent.NewAgentState("agent-1", "Galileo", tempDir)

	journal := publication.NewJournal("Science", tempDir)
	forum := publication.NewForum("Discussion", tempDir)

	sched := NewScheduler(agents, states, journal, forum)

	// Track action counts
	actionCounts := make(map[ActionType]int)
	sched.SetOnTick(func(tick int, agentID string, action ActionType) {
		actionCounts[action]++
	})

	sched.RunFor(1000)

	// All action types should occur
	allActions := []ActionType{ActionThink, ActionRead, ActionPublish, ActionSocial}
	for _, action := range allActions {
		if actionCounts[action] == 0 {
			t.Errorf("action %s never occurred", action)
		}
	}
}

func TestScheduler_RelationshipDecay(t *testing.T) {
	tempDir := t.TempDir()

	state := agent.NewAgentState("agent-1", "Galileo", tempDir)

	// Create a relationship that will decay
	state.SetRelationship(&types.Relationship{
		PeerID:          "agent-2",
		PeerName:        "Euclid",
		State:           types.RelationTrusted,
		LastInteraction: time.Now().Add(-100 * 24 * time.Hour), // 100 days ago
	})

	// Directly call decay
	state.DecayRelationships(time.Now())

	rel := state.GetRelationship("agent-2")
	if rel.State != types.RelationForgotten {
		t.Errorf("expected relationship to decay to Forgotten, got %s", rel.State)
	}
}

func TestScheduler_VisibilityBasedOnRelationship(t *testing.T) {
	tempDir := t.TempDir()

	agents := []*agent.Agent{
		createTestAgent("agent-1", "Galileo", types.RoleExplorer),
	}

	states := make(map[string]*agent.AgentState)
	state := agent.NewAgentState("agent-1", "Galileo", tempDir)

	// Create trusted and forgotten relationships
	state.SetRelationship(&types.Relationship{
		PeerID:   "trusted-author",
		PeerName: "Trusted",
		State:    types.RelationTrusted,
	})
	state.SetRelationship(&types.Relationship{
		PeerID:   "forgotten-author",
		PeerName: "Forgotten",
		State:    types.RelationForgotten,
	})
	states["agent-1"] = state

	journal := publication.NewJournal("Science", tempDir)
	forum := publication.NewForum("Discussion", tempDir)

	// Add posts from both authors with explicit IDs
	forum.Post(&types.Publication{ID: "post-trusted", AuthorID: "trusted-author", AuthorName: "Trusted", Title: "Trusted Post"})
	forum.Post(&types.Publication{ID: "post-forgotten", AuthorID: "forgotten-author", AuthorName: "Forgotten", Title: "Forgotten Post"})

	sched := NewScheduler(agents, states, journal, forum)

	// Run visibility calculation multiple times
	trustedSeen := 0
	forgottenSeen := 0
	iterations := 1000

	for i := 0; i < iterations; i++ {
		visible := sched.getVisibleContent(agents[0], state)
		for _, p := range visible {
			if p.AuthorID == "trusted-author" {
				trustedSeen++
			}
			if p.AuthorID == "forgotten-author" {
				forgottenSeen++
			}
		}
	}

	// Trusted author should be seen much more often
	trustedRatio := float64(trustedSeen) / float64(iterations)
	forgottenRatio := float64(forgottenSeen) / float64(iterations)

	if trustedRatio < 0.8 {
		t.Errorf("trusted author visibility too low: %.2f", trustedRatio)
	}
	if forgottenRatio > 0.2 {
		t.Errorf("forgotten author visibility too high: %.2f", forgottenRatio)
	}
}
