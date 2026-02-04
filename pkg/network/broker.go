package network

import (
	"fmt"
	"sync"

	"github.com/cpunion/sci-bot/pkg/agent"
	"github.com/cpunion/sci-bot/pkg/types"
)

// MessageBroker routes messages between agents.
type MessageBroker struct {
	mu sync.RWMutex

	agents    map[string]*agent.Agent
	topics    map[string][]string // topic -> subscriber agent IDs
	diversity *DiversityEngine
}

// NewMessageBroker creates a new message broker.
func NewMessageBroker() *MessageBroker {
	return &MessageBroker{
		agents:    make(map[string]*agent.Agent),
		topics:    make(map[string][]string),
		diversity: NewDiversityEngine(),
	}
}

// Register registers an agent with the broker.
func (b *MessageBroker) Register(a *agent.Agent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.agents[a.ID()] = a
}

// Unregister removes an agent from the broker.
func (b *MessageBroker) Unregister(agentID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.agents, agentID)

	// Remove from all topic subscriptions
	for topic, subs := range b.topics {
		newSubs := make([]string, 0, len(subs))
		for _, s := range subs {
			if s != agentID {
				newSubs = append(newSubs, s)
			}
		}
		b.topics[topic] = newSubs
	}
}

// Subscribe subscribes an agent to a topic.
func (b *MessageBroker) Subscribe(agentID, topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.topics[topic]
	for _, s := range subs {
		if s == agentID {
			return // Already subscribed
		}
	}
	b.topics[topic] = append(subs, agentID)
}

// Unsubscribe removes an agent from a topic.
func (b *MessageBroker) Unsubscribe(agentID, topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.topics[topic]
	newSubs := make([]string, 0, len(subs))
	for _, s := range subs {
		if s != agentID {
			newSubs = append(newSubs, s)
		}
	}
	b.topics[topic] = newSubs
}

// Route sends a message to its recipients.
func (b *MessageBroker) Route(msg *types.Message) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	recipients := make(map[string]bool)

	// Direct recipients
	for _, to := range msg.To {
		if _, ok := b.agents[to]; ok {
			recipients[to] = true
		} else {
			// Check if it's a topic
			if subs, ok := b.topics[to]; ok {
				for _, sub := range subs {
					if sub != msg.From {
						recipients[sub] = true
					}
				}
			}
		}
	}

	// Public messages go to connections based on visibility
	if msg.Visibility == types.VisibilityPublic && len(msg.To) == 0 {
		// Broadcast to all agents
		for id := range b.agents {
			if id != msg.From {
				recipients[id] = true
			}
		}
	}

	// Send to all recipients
	for recipientID := range recipients {
		if agent, ok := b.agents[recipientID]; ok {
			select {
			case agent.Inbox <- msg:
			default:
				// Inbox full, drop message for this recipient
			}
		}
	}

	return nil
}

// Broadcast sends a message to all agents.
func (b *MessageBroker) Broadcast(msg *types.Message) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for id, agent := range b.agents {
		if id != msg.From {
			select {
			case agent.Inbox <- msg:
			default:
				// Inbox full
			}
		}
	}
}

// GetAgent returns an agent by ID.
func (b *MessageBroker) GetAgent(agentID string) (*agent.Agent, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if a, ok := b.agents[agentID]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("agent not found: %s", agentID)
}

// ListAgents returns all registered agents.
func (b *MessageBroker) ListAgents() []*agent.Agent {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]*agent.Agent, 0, len(b.agents))
	for _, a := range b.agents {
		result = append(result, a)
	}
	return result
}

// ListTopics returns all topics.
func (b *MessageBroker) ListTopics() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]string, 0, len(b.topics))
	for topic := range b.topics {
		result = append(result, topic)
	}
	return result
}

// Forward collects outgoing messages from all agents and routes them.
func (b *MessageBroker) Forward() {
	b.mu.RLock()
	agents := make([]*agent.Agent, 0, len(b.agents))
	for _, a := range b.agents {
		agents = append(agents, a)
	}
	b.mu.RUnlock()

	for _, a := range agents {
		select {
		case msg := <-a.Outbox:
			b.Route(msg)
		default:
			// No outgoing messages
		}
	}
}
