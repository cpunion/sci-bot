// Package agent implements the core agent logic for Sci-Bot.
package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cpunion/sci-bot/pkg/types"
)

// AgentState represents the persistent state of an agent.
type AgentState struct {
	mu sync.RWMutex

	AgentID       string                          `json:"agent_id"`
	AgentName     string                          `json:"agent_name"`
	Relationships map[string]*types.Relationship  `json:"relationships"`
	Knowledge     map[string]*types.KnowledgeItem `json:"knowledge"`
	Subscriptions []string                        `json:"subscriptions"`
	LastActive    time.Time                       `json:"last_active"`

	// Persistence path
	dataPath string
}

// NewAgentState creates a new agent state.
func NewAgentState(agentID, agentName, dataPath string) *AgentState {
	return &AgentState{
		AgentID:       agentID,
		AgentName:     agentName,
		Relationships: make(map[string]*types.Relationship),
		Knowledge:     make(map[string]*types.KnowledgeItem),
		Subscriptions: make([]string, 0),
		LastActive:    time.Now(),
		dataPath:      dataPath,
	}
}

// GetRelationship returns the relationship with a peer.
func (s *AgentState) GetRelationship(peerID string) *types.Relationship {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Relationships[peerID]
}

// SetRelationship updates or creates a relationship.
func (s *AgentState) SetRelationship(rel *types.Relationship) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Relationships[rel.PeerID] = rel
}

// UpdateRelationshipState transitions the relationship state.
func (s *AgentState) UpdateRelationshipState(peerID string, newState types.RelationshipState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rel, ok := s.Relationships[peerID]
	if !ok {
		return
	}
	rel.State = newState
	rel.LastInteraction = time.Now()
}

// RecordInteraction records an interaction with a peer.
func (s *AgentState) RecordInteraction(peerID, peerName string, topics []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rel, ok := s.Relationships[peerID]
	if !ok {
		// Create new relationship
		rel = &types.Relationship{
			PeerID:      peerID,
			PeerName:    peerName,
			State:       types.RelationNew,
			TrustScore:  0.5,
			Familiarity: 0.1,
		}
		s.Relationships[peerID] = rel
	}

	rel.InteractionCount++
	rel.LastInteraction = time.Now()

	// Increase familiarity with each interaction
	rel.Familiarity = min(1.0, rel.Familiarity+0.05)

	// Update shared topics
	for _, topic := range topics {
		found := false
		for _, t := range rel.SharedTopics {
			if t == topic {
				found = true
				break
			}
		}
		if !found {
			rel.SharedTopics = append(rel.SharedTopics, topic)
		}
	}

	// State transitions based on interaction count
	if rel.InteractionCount >= 10 && rel.State == types.RelationNew {
		rel.State = types.RelationDiscussing
	}
	if rel.InteractionCount >= 30 && rel.State == types.RelationDiscussing && rel.TrustScore > 0.7 {
		rel.State = types.RelationTrusted
	}
}

// DecayRelationships decays relationships based on time since last interaction.
func (s *AgentState) DecayRelationships(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rel := range s.Relationships {
		daysSinceInteraction := now.Sub(rel.LastInteraction).Hours() / 24

		if daysSinceInteraction > 90 && rel.State != types.RelationForgotten {
			rel.State = types.RelationForgotten
			rel.Familiarity *= 0.5
		} else if daysSinceInteraction > 30 && rel.State == types.RelationTrusted {
			rel.State = types.RelationEstranged
			rel.Familiarity *= 0.8
		} else if daysSinceInteraction > 14 && rel.State == types.RelationDiscussing {
			rel.State = types.RelationEstranged
			rel.Familiarity *= 0.9
		}
	}
}

// GetKnowledge returns knowledge about a theory.
func (s *AgentState) GetKnowledge(theoryID string) *types.KnowledgeItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Knowledge[theoryID]
}

// LearnTheory adds or updates knowledge about a theory.
func (s *AgentState) LearnTheory(theoryID, theoryTitle, source string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	k, ok := s.Knowledge[theoryID]
	if !ok {
		// First time hearing about it
		s.Knowledge[theoryID] = &types.KnowledgeItem{
			TheoryID:     theoryID,
			TheoryTitle:  theoryTitle,
			Level:        types.KnowledgeHeard,
			LearnedAt:    now,
			LastReviewed: now,
			Confidence:   0.3,
			Source:       source,
		}
		return
	}

	// Already know about it, potentially upgrade
	k.LastReviewed = now
	k.Confidence = min(1.0, k.Confidence+0.1)

	// Level upgrades based on reviews
	if k.Level == types.KnowledgeHeard && k.Confidence > 0.5 {
		k.Level = types.KnowledgeLearned
	} else if k.Level == types.KnowledgeLearned && k.Confidence > 0.8 {
		k.Level = types.KnowledgeMastered
	}
}

// GetKnowledgeByLevel returns all knowledge at a specific level.
func (s *AgentState) GetKnowledgeByLevel(level types.KnowledgeLevel) []*types.KnowledgeItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*types.KnowledgeItem, 0)
	for _, k := range s.Knowledge {
		if k.Level == level {
			result = append(result, k)
		}
	}
	return result
}

// GetTrustedPeers returns peers in trusted state.
func (s *AgentState) GetTrustedPeers() []*types.Relationship {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*types.Relationship, 0)
	for _, rel := range s.Relationships {
		if rel.State == types.RelationTrusted {
			result = append(result, rel)
		}
	}
	return result
}

// GetActivePeers returns peers that are not forgotten.
func (s *AgentState) GetActivePeers() []*types.Relationship {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*types.Relationship, 0)
	for _, rel := range s.Relationships {
		if rel.State != types.RelationForgotten {
			result = append(result, rel)
		}
	}
	return result
}

// Save persists the agent state to disk.
func (s *AgentState) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := os.MkdirAll(s.dataPath, 0755); err != nil {
		return err
	}

	s.LastActive = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(s.dataPath, "state.json"), data, 0644)
}

// Load loads the agent state from disk.
func (s *AgentState) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(s.dataPath, "state.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No state to load
		}
		return err
	}

	return json.Unmarshal(data, s)
}

// LoadAgentState loads an agent state from a path.
func LoadAgentState(dataPath string) (*AgentState, error) {
	state := &AgentState{
		Relationships: make(map[string]*types.Relationship),
		Knowledge:     make(map[string]*types.KnowledgeItem),
		dataPath:      dataPath,
	}

	if err := state.Load(); err != nil {
		return nil, err
	}

	return state, nil
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
