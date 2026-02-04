// Package types defines core types for the Sci-Bot Agent Network.
package types

import "time"

// AgentRole defines the specialized role of an agent in the network.
type AgentRole string

const (
	RoleExplorer     AgentRole = "explorer"     // Divergent thinking, exploring new directions
	RoleBuilder      AgentRole = "builder"      // Rigorous derivation, building theory systems
	RoleReviewer     AgentRole = "reviewer"     // Critical review and validation
	RoleSynthesizer  AgentRole = "synthesizer"  // Cross-domain synthesis
	RoleCommunicator AgentRole = "communicator" // Knowledge dissemination
)

// ThinkingStyle defines the cognitive style of an agent.
type ThinkingStyle string

const (
	StyleDivergent  ThinkingStyle = "divergent"  // Divergent thinking
	StyleConvergent ThinkingStyle = "convergent" // Convergent thinking
	StyleLateral    ThinkingStyle = "lateral"    // Lateral thinking
	StyleAnalytical ThinkingStyle = "analytical" // Analytical thinking
	StyleIntuitive  ThinkingStyle = "intuitive"  // Intuitive thinking
)

// Persona defines the unique identity and characteristics of an agent.
type Persona struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role AgentRole `json:"role"`

	// Cognitive style - ensures diversity
	ThinkingStyle ThinkingStyle `json:"thinking_style"`
	RiskTolerance float64       `json:"risk_tolerance"` // 0.0-1.0 acceptance of unconventional ideas
	Creativity    float64       `json:"creativity"`     // 0.0-1.0 innovation tendency
	Rigor         float64       `json:"rigor"`          // 0.0-1.0 strictness level

	// Expertise
	Domains      []string `json:"domains"`       // Research domains
	AxiomSystems []string `json:"axiom_systems"` // Preferred axiom systems

	// Social characteristics
	Sociability float64 `json:"sociability"` // Social activity level
	Influence   float64 `json:"influence"`   // Influence index
}

// MessageType defines the type of message in the network.
type MessageType string

const (
	// Social messages
	MsgChat    MessageType = "chat"
	MsgMention MessageType = "mention"
	MsgReply   MessageType = "reply"

	// Academic messages
	MsgTheory    MessageType = "theory"
	MsgReview    MessageType = "review"
	MsgChallenge MessageType = "challenge"
	MsgSupport   MessageType = "support"
	MsgQuestion  MessageType = "question"

	// Collaboration messages
	MsgInvite   MessageType = "invite"
	MsgProposal MessageType = "proposal"
	MsgVote     MessageType = "vote"
)

// Visibility defines message visibility scope.
type Visibility string

const (
	VisibilityPublic      Visibility = "public"
	VisibilityConnections Visibility = "connections"
	VisibilityPrivate     Visibility = "private"
)

// Message represents a communication unit in the network.
type Message struct {
	ID         string      `json:"id"`
	Type       MessageType `json:"type"`
	From       string      `json:"from"`
	To         []string    `json:"to"`           // Can be specific agents or topics
	Content    string      `json:"content"`      // Message content
	InReplyTo  string      `json:"in_reply_to"`  // ID of message being replied to
	Visibility Visibility  `json:"visibility"`
	Timestamp  time.Time   `json:"timestamp"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// TheoryStatus defines the validation status of a theory.
type TheoryStatus string

const (
	StatusDraft       TheoryStatus = "draft"
	StatusProposed    TheoryStatus = "proposed"
	StatusUnderReview TheoryStatus = "under_review"
	StatusValidated   TheoryStatus = "validated"
	StatusDisputed    TheoryStatus = "disputed"
	StatusRefuted     TheoryStatus = "refuted"
	StatusRevised     TheoryStatus = "revised"
)

// ConfidenceLevel indicates how certain a claim is.
type ConfidenceLevel string

const (
	LevelTheorem    ConfidenceLevel = "theorem"    // Proven
	LevelHypothesis ConfidenceLevel = "hypothesis" // Under verification
	LevelConjecture ConfidenceLevel = "conjecture" // Preliminary
	LevelInspiration ConfidenceLevel = "inspiration" // Initial idea
)

// Axiom represents a fundamental assumption in an axiom system.
type Axiom struct {
	ID          string   `json:"id"`
	Statement   string   `json:"statement"`   // Axiom statement
	FormalForm  string   `json:"formal_form"` // Formal expression
	Assumptions []string `json:"assumptions"` // Prerequisites
}

// AxiomSystem represents a complete axiom system.
type AxiomSystem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Axioms      []Axiom  `json:"axioms"`
	Parent      string   `json:"parent,omitempty"` // Derived from which system
	Differences []string `json:"differences"`      // Differences from parent
	CreatedBy   string   `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// Definition represents a formal definition of a concept.
type Definition struct {
	Term        string `json:"term"`
	Definition  string `json:"definition"`
	FormalForm  string `json:"formal_form,omitempty"`
}

// Theorem represents a proven statement.
type Theorem struct {
	ID         string `json:"id"`
	Statement  string `json:"statement"`
	Proof      string `json:"proof"`
	References []string `json:"references"`
}

// Hypothesis represents an unverified claim.
type Hypothesis struct {
	ID          string   `json:"id"`
	Statement   string   `json:"statement"`
	Reasoning   string   `json:"reasoning"`
	Predictions []string `json:"predictions"`
	Confidence  float64  `json:"confidence"` // 0-100
}

// Conjecture represents a preliminary idea.
type Conjecture struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
	Intuition string `json:"intuition"`
}

// Theory represents a complete theory in the network.
type Theory struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Authors     []string `json:"authors"`
	Abstract    string `json:"abstract"`

	// Axiom foundation
	AxiomSystem  string  `json:"axiom_system"`  // Based on which axiom system
	CustomAxioms []Axiom `json:"custom_axioms"` // Additional axioms

	// Content layers
	Definitions []Definition `json:"definitions"`
	Theorems    []Theorem    `json:"theorems"`
	Hypotheses  []Hypothesis `json:"hypotheses"`
	Conjectures []Conjecture `json:"conjectures"`

	// Validation status
	Status  TheoryStatus `json:"status"`
	Reviews []Review     `json:"reviews"`

	// Metadata
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
	Citations    []string  `json:"citations"`
	CitedBy      []string  `json:"cited_by"`
	NoveltyScore float64   `json:"novelty_score"`
	IsHeretical  bool      `json:"is_heretical"`
}

// Review represents a peer review of a theory.
type Review struct {
	ID         string    `json:"id"`
	TheoryID   string    `json:"theory_id"`
	ReviewerID string    `json:"reviewer_id"`
	Verdict    string    `json:"verdict"` // approve, reject, revise
	Comments   string    `json:"comments"`
	IssuesFound []string `json:"issues_found"`
	Suggestions []string `json:"suggestions"`
	CreatedAt  time.Time `json:"created_at"`
}

// ConnectionType defines the strength of a social connection.
type ConnectionType string

const (
	ConnectionClose       ConnectionType = "close"       // Close connection (max 15)
	ConnectionActive      ConnectionType = "active"      // Active connection (max 150)
	ConnectionAcquaintance ConnectionType = "acquaintance" // Acquaintance (max 500)
)

// Connection represents a social connection between agents.
type Connection struct {
	PeerID       string         `json:"peer_id"`
	Strength     float64        `json:"strength"`      // Relationship strength 0-1
	Type         ConnectionType `json:"type"`
	SharedTopics []string       `json:"shared_topics"` // Common interests
	LastContact  time.Time      `json:"last_contact"`
	TrustScore   float64        `json:"trust_score"`
}

// SocialGraph represents an agent's social network.
type SocialGraph struct {
	AgentID     string                 `json:"agent_id"`
	Connections map[string]*Connection `json:"connections"`
}

// Social limits (Dunbar's number for agents)
const (
	MaxCloseConnections  = 15
	MaxActiveConnections = 150
	MaxAcquaintances     = 500
)
