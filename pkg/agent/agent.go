// Package agent implements the core agent logic for Sci-Bot.
package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cpunion/sci-bot/pkg/memory"
	"github.com/cpunion/sci-bot/pkg/types"
)

// LLMProvider defines the interface for language model backends.
type LLMProvider interface {
	// Generate produces a response given a prompt.
	Generate(ctx context.Context, prompt string) (string, error)
}

// Agent represents a Sci-Bot network agent.
type Agent struct {
	mu sync.RWMutex

	Persona *types.Persona
	Memory  *memory.Memory
	Social  *types.SocialGraph

	// Communication channels
	Inbox  chan *types.Message
	Outbox chan *types.Message

	// LLM backend
	LLM LLMProvider

	// Control
	ctx    context.Context
	cancel context.CancelFunc

	// Timing
	thinkInterval   time.Duration
	exploreInterval time.Duration

	// State
	running bool
}

// Config holds agent configuration.
type Config struct {
	Persona         *types.Persona
	DataPath        string
	ContextWindow   int
	ThinkInterval   time.Duration
	ExploreInterval time.Duration
	InboxSize       int
	OutboxSize      int
}

// DefaultConfig returns a default configuration.
func DefaultConfig(persona *types.Persona, dataPath string) Config {
	return Config{
		Persona:         persona,
		DataPath:        dataPath,
		ContextWindow:   4096,
		ThinkInterval:   5 * time.Minute,
		ExploreInterval: 10 * time.Minute,
		InboxSize:       100,
		OutboxSize:      100,
	}
}

// New creates a new agent with the given configuration.
func New(cfg Config) *Agent {
	ctx, cancel := context.WithCancel(context.Background())

	return &Agent{
		Persona: cfg.Persona,
		Memory:  memory.NewMemory(cfg.Persona.ID, cfg.DataPath, cfg.ContextWindow),
		Social: &types.SocialGraph{
			AgentID:     cfg.Persona.ID,
			Connections: make(map[string]*types.Connection),
		},
		Inbox:           make(chan *types.Message, cfg.InboxSize),
		Outbox:          make(chan *types.Message, cfg.OutboxSize),
		ctx:             ctx,
		cancel:          cancel,
		thinkInterval:   cfg.ThinkInterval,
		exploreInterval: cfg.ExploreInterval,
	}
}

// SetLLM sets the language model provider.
func (a *Agent) SetLLM(llm LLMProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.LLM = llm
}

// Start begins the agent's main loop.
func (a *Agent) Start() error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("agent already running")
	}
	a.running = true
	a.mu.Unlock()

	// Load persisted memory
	if err := a.Memory.Load(); err != nil {
		// Ignore load errors for new agents
	}

	go a.run()
	return nil
}

// Stop stops the agent.
func (a *Agent) Stop() error {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return fmt.Errorf("agent not running")
	}
	a.running = false
	a.mu.Unlock()

	a.cancel()

	// Persist memory
	return a.Memory.Save()
}

// run is the main event loop.
func (a *Agent) run() {
	thinkTicker := time.NewTicker(a.thinkInterval)
	exploreTicker := time.NewTicker(a.exploreInterval)
	defer thinkTicker.Stop()
	defer exploreTicker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return

		case msg := <-a.Inbox:
			a.handleMessage(msg)

		case <-thinkTicker.C:
			a.autonomousThink()

		case <-exploreTicker.C:
			a.explore()
		}
	}
}

// handleMessage processes an incoming message.
func (a *Agent) handleMessage(msg *types.Message) {
	// Add to working memory
	a.Memory.AddMessage(msg)

	// Generate response based on role
	response := a.processMessage(msg)

	// Update rolling summary memory
	summaryEntry := a.buildSummaryEntry(msg, response)
	a.Memory.UpdateSummary(summaryEntry, 2000)

	if response != nil {
		a.send(response)
	}
}

// processMessage generates a response based on agent role.
func (a *Agent) processMessage(msg *types.Message) *types.Message {
	switch a.Persona.Role {
	case types.RoleExplorer:
		return a.explorerProcess(msg)
	case types.RoleBuilder:
		return a.builderProcess(msg)
	case types.RoleReviewer:
		return a.reviewerProcess(msg)
	case types.RoleSynthesizer:
		return a.synthesizerProcess(msg)
	case types.RoleCommunicator:
		return a.communicatorProcess(msg)
	default:
		return a.defaultProcess(msg)
	}
}

// explorerProcess handles messages with divergent thinking.
func (a *Agent) explorerProcess(msg *types.Message) *types.Message {
	if a.LLM == nil {
		log.Printf("[%s] LLM not configured", a.Persona.Name)
		return nil
	}

	prompt := a.buildExplorerPrompt(msg)
	log.Printf("[%s] Generating response...", a.Persona.Name)
	response, err := a.LLM.Generate(a.ctx, prompt)
	if err != nil {
		log.Printf("[%s] LLM error: %v", a.Persona.Name, err)
		return nil
	}
	log.Printf("[%s] Response generated (%d chars)", a.Persona.Name, len(response))

	return a.createResponse(msg, response)
}

// builderProcess handles messages with rigorous thinking.
func (a *Agent) builderProcess(msg *types.Message) *types.Message {
	if a.LLM == nil {
		log.Printf("[%s] LLM not configured", a.Persona.Name)
		return nil
	}

	prompt := a.buildBuilderPrompt(msg)
	log.Printf("[%s] Generating response...", a.Persona.Name)
	response, err := a.LLM.Generate(a.ctx, prompt)
	if err != nil {
		log.Printf("[%s] LLM error: %v", a.Persona.Name, err)
		return nil
	}
	log.Printf("[%s] Response generated (%d chars)", a.Persona.Name, len(response))

	return a.createResponse(msg, response)
}

// reviewerProcess handles messages with critical thinking.
func (a *Agent) reviewerProcess(msg *types.Message) *types.Message {
	if a.LLM == nil {
		log.Printf("[%s] LLM not configured", a.Persona.Name)
		return nil
	}

	prompt := a.buildReviewerPrompt(msg)
	log.Printf("[%s] Generating response...", a.Persona.Name)
	response, err := a.LLM.Generate(a.ctx, prompt)
	if err != nil {
		log.Printf("[%s] LLM error: %v", a.Persona.Name, err)
		return nil
	}
	log.Printf("[%s] Response generated (%d chars)", a.Persona.Name, len(response))

	return a.createResponse(msg, response)
}

// synthesizerProcess handles messages with cross-domain thinking.
func (a *Agent) synthesizerProcess(msg *types.Message) *types.Message {
	if a.LLM == nil {
		return nil
	}

	prompt := a.buildSynthesizerPrompt(msg)
	response, err := a.LLM.Generate(a.ctx, prompt)
	if err != nil {
		return nil
	}

	return a.createResponse(msg, response)
}

// communicatorProcess handles messages for knowledge dissemination.
func (a *Agent) communicatorProcess(msg *types.Message) *types.Message {
	if a.LLM == nil {
		return nil
	}

	prompt := a.buildCommunicatorPrompt(msg)
	response, err := a.LLM.Generate(a.ctx, prompt)
	if err != nil {
		return nil
	}

	return a.createResponse(msg, response)
}

// defaultProcess handles messages with default behavior.
func (a *Agent) defaultProcess(msg *types.Message) *types.Message {
	if a.LLM == nil {
		return nil
	}

	prompt := a.buildDefaultPrompt(msg)
	response, err := a.LLM.Generate(a.ctx, prompt)
	if err != nil {
		return nil
	}

	return a.createResponse(msg, response)
}

// autonomousThink performs independent thinking.
func (a *Agent) autonomousThink() {
	if a.LLM == nil {
		return
	}

	prompt := a.buildThinkPrompt()
	thought, err := a.LLM.Generate(a.ctx, prompt)
	if err != nil {
		return
	}

	// Store thought in scratch pad
	a.Memory.SetScratchPad(thought)
}

// explore discovers new content.
func (a *Agent) explore() {
	// This would connect to the knowledge layer to find new content
	// For now, it's a placeholder
}

// send sends a message through the outbox.
func (a *Agent) send(msg *types.Message) {
	select {
	case a.Outbox <- msg:
	default:
		// Outbox full, drop message
	}
}

// createResponse creates a response message.
func (a *Agent) createResponse(original *types.Message, content string) *types.Message {
	return &types.Message{
		ID:         fmt.Sprintf("%s-%d", a.Persona.ID, time.Now().UnixNano()),
		Type:       types.MsgReply,
		From:       a.Persona.ID,
		To:         []string{original.From},
		Content:    content,
		InReplyTo:  original.ID,
		Visibility: original.Visibility,
		Timestamp:  time.Now(),
	}
}

func (a *Agent) buildSummaryEntry(msg *types.Message, response *types.Message) string {
	timestamp := time.Now().Format(time.RFC3339)
	entry := fmt.Sprintf("%s | from %s: %s", timestamp, msg.From, truncateText(msg.Content, 200))
	if response != nil && response.Content != "" {
		entry = fmt.Sprintf("%s | reply: %s", entry, truncateText(response.Content, 200))
	}
	return entry
}

func truncateText(s string, maxChars int) string {
	if maxChars <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars]) + "..."
}

// Connect establishes a connection with another agent.
func (a *Agent) Connect(peerID string, connType types.ConnectionType) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check limits
	counts := a.countConnectionsByType()
	switch connType {
	case types.ConnectionClose:
		if counts[types.ConnectionClose] >= types.MaxCloseConnections {
			return fmt.Errorf("max close connections reached (%d)", types.MaxCloseConnections)
		}
	case types.ConnectionActive:
		if counts[types.ConnectionActive] >= types.MaxActiveConnections {
			return fmt.Errorf("max active connections reached (%d)", types.MaxActiveConnections)
		}
	case types.ConnectionAcquaintance:
		if counts[types.ConnectionAcquaintance] >= types.MaxAcquaintances {
			return fmt.Errorf("max acquaintances reached (%d)", types.MaxAcquaintances)
		}
	}

	a.Social.Connections[peerID] = &types.Connection{
		PeerID:      peerID,
		Strength:    0.5,
		Type:        connType,
		LastContact: time.Now(),
		TrustScore:  0.5,
	}

	return nil
}

// Disconnect removes a connection with another agent.
func (a *Agent) Disconnect(peerID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.Social.Connections, peerID)
}

// GetConnections returns all connections.
func (a *Agent) GetConnections() map[string]*types.Connection {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Social.Connections
}

// countConnectionsByType counts connections by type.
func (a *Agent) countConnectionsByType() map[types.ConnectionType]int {
	counts := make(map[types.ConnectionType]int)
	for _, conn := range a.Social.Connections {
		counts[conn.Type]++
	}
	return counts
}

// ID returns the agent's ID.
func (a *Agent) ID() string {
	return a.Persona.ID
}

// Name returns the agent's name.
func (a *Agent) Name() string {
	return a.Persona.Name
}

// Role returns the agent's role.
func (a *Agent) Role() types.AgentRole {
	return a.Persona.Role
}
