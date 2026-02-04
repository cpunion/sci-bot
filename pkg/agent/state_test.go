package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cpunion/sci-bot/pkg/types"
)

func TestAgentState_Relationships(t *testing.T) {
	state := NewAgentState("agent-1", "Galileo", t.TempDir())

	// Test recording interaction creates new relationship
	state.RecordInteraction("agent-2", "Euclid", []string{"geometry"})

	rel := state.GetRelationship("agent-2")
	if rel == nil {
		t.Fatal("expected relationship to be created")
	}
	if rel.State != types.RelationNew {
		t.Errorf("expected state New, got %s", rel.State)
	}
	if rel.InteractionCount != 1 {
		t.Errorf("expected interaction count 1, got %d", rel.InteractionCount)
	}

	// Test multiple interactions increase familiarity
	for i := 0; i < 9; i++ {
		state.RecordInteraction("agent-2", "Euclid", []string{"geometry"})
	}

	rel = state.GetRelationship("agent-2")
	if rel.State != types.RelationDiscussing {
		t.Errorf("expected state Discussing after 10 interactions, got %s", rel.State)
	}
	if rel.Familiarity < 0.5 {
		t.Errorf("expected familiarity to increase, got %f", rel.Familiarity)
	}
}

func TestAgentState_RelationshipDecay(t *testing.T) {
	state := NewAgentState("agent-1", "Galileo", t.TempDir())

	// Create a trusted relationship
	state.SetRelationship(&types.Relationship{
		PeerID:          "agent-2",
		PeerName:        "Euclid",
		State:           types.RelationTrusted,
		TrustScore:      0.9,
		Familiarity:     0.8,
		LastInteraction: time.Now().Add(-35 * 24 * time.Hour), // 35 days ago
	})

	// Decay relationships
	state.DecayRelationships(time.Now())

	rel := state.GetRelationship("agent-2")
	if rel.State != types.RelationEstranged {
		t.Errorf("expected state Estranged after 35 days without interaction, got %s", rel.State)
	}

	// Decay more - 100 days
	state.Relationships["agent-2"].LastInteraction = time.Now().Add(-100 * 24 * time.Hour)
	state.DecayRelationships(time.Now())

	rel = state.GetRelationship("agent-2")
	if rel.State != types.RelationForgotten {
		t.Errorf("expected state Forgotten after 100 days, got %s", rel.State)
	}
}

func TestAgentState_Knowledge(t *testing.T) {
	state := NewAgentState("agent-1", "Galileo", t.TempDir())

	// First time learning
	state.LearnTheory("theory-1", "Pythagorean Theorem", "Euclid")

	k := state.GetKnowledge("theory-1")
	if k == nil {
		t.Fatal("expected knowledge to be created")
	}
	if k.Level != types.KnowledgeHeard {
		t.Errorf("expected level Heard, got %s", k.Level)
	}

	// Learn again - should increase confidence
	state.LearnTheory("theory-1", "Pythagorean Theorem", "Euclid")
	state.LearnTheory("theory-1", "Pythagorean Theorem", "Euclid")
	state.LearnTheory("theory-1", "Pythagorean Theorem", "Euclid")

	k = state.GetKnowledge("theory-1")
	if k.Level != types.KnowledgeLearned {
		t.Errorf("expected level Learned after multiple reviews, got %s", k.Level)
	}

	// More reviews to master
	for i := 0; i < 5; i++ {
		state.LearnTheory("theory-1", "Pythagorean Theorem", "Euclid")
	}

	k = state.GetKnowledge("theory-1")
	if k.Level != types.KnowledgeMastered {
		t.Errorf("expected level Mastered after many reviews, got %s", k.Level)
	}
}

func TestAgentState_Persistence(t *testing.T) {
	tempDir := t.TempDir()

	// Create and save state
	state := NewAgentState("agent-1", "Galileo", tempDir)
	state.RecordInteraction("agent-2", "Euclid", []string{"geometry"})
	state.LearnTheory("theory-1", "Pythagorean Theorem", "Euclid")

	if err := state.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(tempDir, "state.json")); os.IsNotExist(err) {
		t.Fatal("state file not created")
	}

	// Load into new state
	loaded, err := LoadAgentState(tempDir)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.AgentID != "agent-1" {
		t.Errorf("expected agent ID agent-1, got %s", loaded.AgentID)
	}
	if len(loaded.Relationships) != 1 {
		t.Errorf("expected 1 relationship, got %d", len(loaded.Relationships))
	}
	if len(loaded.Knowledge) != 1 {
		t.Errorf("expected 1 knowledge item, got %d", len(loaded.Knowledge))
	}
}

func TestAgentState_GetPeers(t *testing.T) {
	state := NewAgentState("agent-1", "Galileo", t.TempDir())

	state.SetRelationship(&types.Relationship{
		PeerID: "agent-2", State: types.RelationTrusted,
	})
	state.SetRelationship(&types.Relationship{
		PeerID: "agent-3", State: types.RelationDiscussing,
	})
	state.SetRelationship(&types.Relationship{
		PeerID: "agent-4", State: types.RelationForgotten,
	})

	trusted := state.GetTrustedPeers()
	if len(trusted) != 1 {
		t.Errorf("expected 1 trusted peer, got %d", len(trusted))
	}

	active := state.GetActivePeers()
	if len(active) != 2 {
		t.Errorf("expected 2 active peers (not forgotten), got %d", len(active))
	}
}
