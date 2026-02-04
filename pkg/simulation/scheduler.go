// Package simulation implements the randomized simulation loop.
package simulation

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/cpunion/sci-bot/pkg/agent"
	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/types"
)

// ActionType defines agent actions.
type ActionType string

const (
	ActionThink   ActionType = "think"   // Independent thinking
	ActionRead    ActionType = "read"    // Read publications
	ActionPublish ActionType = "publish" // Publish content
	ActionSocial  ActionType = "social"  // Social interaction
)

// Scheduler manages randomized agent execution.
type Scheduler struct {
	mu sync.RWMutex

	agents  []*agent.Agent
	states  map[string]*agent.AgentState
	journal *publication.Journal
	forum   *publication.Forum

	rand *rand.Rand

	// Activity weights - agents with higher weights are more likely to be selected
	activityWeights map[string]float64

	// Tick counter
	tickCount int

	// Callbacks
	onTick func(tick int, agentID string, action ActionType)
}

// NewScheduler creates a new scheduler.
func NewScheduler(agents []*agent.Agent, states map[string]*agent.AgentState, journal *publication.Journal, forum *publication.Forum) *Scheduler {
	weights := make(map[string]float64)
	for _, a := range agents {
		// Initial weight based on sociability
		weights[a.ID()] = 0.5 + a.Persona.Sociability*0.5
	}

	return &Scheduler{
		agents:          agents,
		states:          states,
		journal:         journal,
		forum:           forum,
		rand:            rand.New(rand.NewSource(time.Now().UnixNano())),
		activityWeights: weights,
		tickCount:       0,
	}
}

// SetOnTick sets a callback for each tick.
func (s *Scheduler) SetOnTick(fn func(tick int, agentID string, action ActionType)) {
	s.onTick = fn
}

// Tick executes one simulation tick.
func (s *Scheduler) Tick() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tickCount++

	// 1. Select agent randomly based on weights
	selected := s.weightedRandomSelect()
	if selected == nil {
		return
	}

	// 2. Select action randomly
	action := s.randomAction(selected)

	// 3. Notify callback
	if s.onTick != nil {
		s.onTick(s.tickCount, selected.ID(), action)
	}

	// 4. Execute action
	s.executeAction(selected, action)

	// 5. Periodically decay relationships (every 10 ticks)
	if s.tickCount%10 == 0 {
		s.decayRelationships()
	}
}

// weightedRandomSelect selects an agent based on activity weights.
func (s *Scheduler) weightedRandomSelect() *agent.Agent {
	if len(s.agents) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0.0
	for _, a := range s.agents {
		totalWeight += s.activityWeights[a.ID()]
	}

	// Random selection
	r := s.rand.Float64() * totalWeight
	cumulative := 0.0
	for _, a := range s.agents {
		cumulative += s.activityWeights[a.ID()]
		if r < cumulative {
			return a
		}
	}

	// Fallback to last agent
	return s.agents[len(s.agents)-1]
}

// randomAction selects a random action for an agent.
func (s *Scheduler) randomAction(a *agent.Agent) ActionType {
	// Weights based on agent role and personality
	thinkWeight := 0.3 + a.Persona.Creativity*0.2
	readWeight := 0.3
	publishWeight := 0.2 + a.Persona.Influence*0.1
	socialWeight := 0.2 + a.Persona.Sociability*0.2

	total := thinkWeight + readWeight + publishWeight + socialWeight
	r := s.rand.Float64() * total

	if r < thinkWeight {
		return ActionThink
	}
	r -= thinkWeight

	if r < readWeight {
		return ActionRead
	}
	r -= readWeight

	if r < publishWeight {
		return ActionPublish
	}

	return ActionSocial
}

// executeAction executes the selected action.
func (s *Scheduler) executeAction(a *agent.Agent, action ActionType) {
	state := s.states[a.ID()]
	if state == nil {
		return
	}

	switch action {
	case ActionThink:
		s.doThink(a, state)
	case ActionRead:
		s.doRead(a, state)
	case ActionPublish:
		s.doPublish(a, state)
	case ActionSocial:
		s.doSocial(a, state)
	}
}

// doThink performs independent thinking.
func (s *Scheduler) doThink(a *agent.Agent, state *agent.AgentState) {
	if a.LLM == nil {
		return
	}

	// Get mastered knowledge to think about
	mastered := state.GetKnowledgeByLevel(types.KnowledgeMastered)
	if len(mastered) == 0 {
		return
	}

	// Select a random piece of knowledge to contemplate
	knowledge := mastered[s.rand.Intn(len(mastered))]
	log.Printf("[%s] Thinking about: %s", a.Name(), knowledge.TheoryTitle)

	// Update activity weight based on thinking
	s.activityWeights[a.ID()] *= 1.01 // Slightly increase activity
}

// doRead reads publications.
func (s *Scheduler) doRead(a *agent.Agent, state *agent.AgentState) {
	// Get visible content based on relationships
	allPosts := s.getVisibleContent(a, state)
	if len(allPosts) == 0 {
		return
	}

	// Read a random post
	post := allPosts[s.rand.Intn(len(allPosts))]

	// Record view
	if post.Channel == types.ChannelJournal {
		s.journal.IncrementViews(post.ID)
	} else {
		s.forum.IncrementViews(post.ID)
	}

	// Learn from it
	if post.TheoryID != "" {
		state.LearnTheory(post.TheoryID, post.Title, post.AuthorID)
	}

	// Record interaction with author
	state.RecordInteraction(post.AuthorID, post.AuthorName, []string{})

	log.Printf("[%s] Read: %s by %s", a.Name(), post.Title, post.AuthorName)
}

// doPublish publishes content.
func (s *Scheduler) doPublish(a *agent.Agent, state *agent.AgentState) {
	// Decide based on role whether to publish to journal or forum
	if a.Persona.Role == types.RoleBuilder || a.Persona.Rigor > 0.7 {
		// Prefer journal for rigorous agents
		log.Printf("[%s] Would publish to journal (needs content)", a.Name())
	} else {
		// Prefer forum for others
		log.Printf("[%s] Would publish to forum (needs content)", a.Name())
	}
}

// doSocial performs social interaction.
func (s *Scheduler) doSocial(a *agent.Agent, state *agent.AgentState) {
	// Choose between interacting with known peer or discovering new one
	if s.rand.Float64() < 0.7 && len(state.GetActivePeers()) > 0 {
		// Interact with existing peer
		peers := state.GetActivePeers()
		peer := peers[s.rand.Intn(len(peers))]
		state.RecordInteraction(peer.PeerID, peer.PeerName, []string{})
		log.Printf("[%s] Interacted with: %s", a.Name(), peer.PeerName)
	} else {
		// Browse forum to discover new agents
		posts := s.forum.GetRecent(20)
		for _, post := range posts {
			if post.AuthorID != a.ID() && state.GetRelationship(post.AuthorID) == nil {
				state.RecordInteraction(post.AuthorID, post.AuthorName, []string{})
				log.Printf("[%s] Discovered new agent: %s via forum", a.Name(), post.AuthorName)
				break
			}
		}
	}
}

// getVisibleContent returns content visible to an agent based on relationships.
func (s *Scheduler) getVisibleContent(a *agent.Agent, state *agent.AgentState) []*types.Publication {
	result := make([]*types.Publication, 0)

	// All journal publications are visible (authoritative)
	result = append(result, s.journal.GetApproved()...)

	// Forum posts filtered by relationship
	for _, post := range s.forum.AllPosts() {
		if post.AuthorID == a.ID() {
			continue // Skip own posts
		}

		visibility := s.calculateVisibility(a, state, post)
		if s.rand.Float64() < visibility {
			result = append(result, post)
		}
	}

	return result
}

// calculateVisibility calculates how visible a post is to an agent.
func (s *Scheduler) calculateVisibility(a *agent.Agent, state *agent.AgentState, post *types.Publication) float64 {
	base := 0.3 // Base visibility for forum posts

	rel := state.GetRelationship(post.AuthorID)
	if rel == nil {
		return base // Unknown author, base visibility
	}

	switch rel.State {
	case types.RelationTrusted:
		return 0.95 // Almost always see trusted peers
	case types.RelationDiscussing:
		return 0.7
	case types.RelationNew:
		return 0.5
	case types.RelationEstranged:
		return 0.2
	case types.RelationForgotten:
		return 0.05 // Rarely see forgotten peers
	}

	return base
}

// decayRelationships decays relationships for all agents.
func (s *Scheduler) decayRelationships() {
	now := time.Now()
	for _, state := range s.states {
		state.DecayRelationships(now)
	}
}

// GetTickCount returns the current tick count.
func (s *Scheduler) GetTickCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tickCount
}

// RunFor runs the simulation for n ticks.
func (s *Scheduler) RunFor(n int) {
	for i := 0; i < n; i++ {
		s.Tick()
	}
}

// Save persists all states.
func (s *Scheduler) Save() error {
	for _, state := range s.states {
		if err := state.Save(); err != nil {
			return err
		}
	}
	if err := s.journal.Save(); err != nil {
		return err
	}
	return s.forum.Save()
}
