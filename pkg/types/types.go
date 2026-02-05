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
	ID   string    `json:"id"`
	Name string    `json:"name"`
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
	ID         string         `json:"id"`
	Type       MessageType    `json:"type"`
	From       string         `json:"from"`
	To         []string       `json:"to"`          // Can be specific agents or topics
	Content    string         `json:"content"`     // Message content
	InReplyTo  string         `json:"in_reply_to"` // ID of message being replied to
	Visibility Visibility     `json:"visibility"`
	Timestamp  time.Time      `json:"timestamp"`
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
	LevelTheorem     ConfidenceLevel = "theorem"     // Proven
	LevelHypothesis  ConfidenceLevel = "hypothesis"  // Under verification
	LevelConjecture  ConfidenceLevel = "conjecture"  // Preliminary
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
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Axioms      []Axiom   `json:"axioms"`
	Parent      string    `json:"parent,omitempty"` // Derived from which system
	Differences []string  `json:"differences"`      // Differences from parent
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// Definition represents a formal definition of a concept.
type Definition struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
	FormalForm string `json:"formal_form,omitempty"`
}

// Theorem represents a proven statement.
type Theorem struct {
	ID         string   `json:"id"`
	Statement  string   `json:"statement"`
	Proof      string   `json:"proof"`
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
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Authors  []string `json:"authors"`
	Abstract string   `json:"abstract"`

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
	ID          string    `json:"id"`
	TheoryID    string    `json:"theory_id"`
	ReviewerID  string    `json:"reviewer_id"`
	Verdict     string    `json:"verdict"` // approve, reject, revise
	Comments    string    `json:"comments"`
	IssuesFound []string  `json:"issues_found"`
	Suggestions []string  `json:"suggestions"`
	CreatedAt   time.Time `json:"created_at"`
}

// ConnectionType defines the strength of a social connection.
type ConnectionType string

const (
	ConnectionClose        ConnectionType = "close"        // Close connection (max 15)
	ConnectionActive       ConnectionType = "active"       // Active connection (max 150)
	ConnectionAcquaintance ConnectionType = "acquaintance" // Acquaintance (max 500)
)

// RelationshipState defines the relationship status between agents.
type RelationshipState string

const (
	RelationNew        RelationshipState = "new"        // 新认识
	RelationDiscussing RelationshipState = "discussing" // 讨论中
	RelationTrusted    RelationshipState = "trusted"    // 信任
	RelationEstranged  RelationshipState = "estranged"  // 生疏
	RelationForgotten  RelationshipState = "forgotten"  // 遗忘
)

// Relationship represents a dynamic relationship between agents.
type Relationship struct {
	PeerID           string            `json:"peer_id"`
	PeerName         string            `json:"peer_name"`
	State            RelationshipState `json:"state"`
	TrustScore       float64           `json:"trust_score"` // 0-1, affects visibility
	Familiarity      float64           `json:"familiarity"` // 0-1, how well they know each other
	LastInteraction  time.Time         `json:"last_interaction"`
	InteractionCount int               `json:"interaction_count"`
	SharedTopics     []string          `json:"shared_topics"`
}

// KnowledgeLevel defines how well an agent knows a theory.
type KnowledgeLevel string

const (
	KnowledgeHeard    KnowledgeLevel = "heard"    // 听说过
	KnowledgeLearned  KnowledgeLevel = "learned"  // 习得
	KnowledgeMastered KnowledgeLevel = "mastered" // 掌握
)

// KnowledgeItem represents an agent's knowledge of a theory.
type KnowledgeItem struct {
	TheoryID     string         `json:"theory_id"`
	TheoryTitle  string         `json:"theory_title"`
	Level        KnowledgeLevel `json:"level"`
	LearnedAt    time.Time      `json:"learned_at"`
	LastReviewed time.Time      `json:"last_reviewed"`
	Confidence   float64        `json:"confidence"` // 0-1, how confident in this knowledge
	Source       string         `json:"source"`     // Where they learned it from
}

// Connection represents a social connection between agents (legacy, use Relationship instead).
type Connection struct {
	PeerID       string         `json:"peer_id"`
	Strength     float64        `json:"strength"` // Relationship strength 0-1
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

// ChannelType defines publication channel types.
type ChannelType string

const (
	ChannelJournal ChannelType = "journal" // 权威杂志，需审核
	ChannelForum   ChannelType = "forum"   // 草根论坛，自由发布
)

// Subreddit defines forum subject areas (like Reddit subreddits).
type Subreddit string

const (
	SubMathematics Subreddit = "mathematics" // 数学
	SubPhysics     Subreddit = "physics"     // 物理
	SubPhilosophy  Subreddit = "philosophy"  // 哲学
	SubBiology     Subreddit = "biology"     // 生物学
	SubComputing   Subreddit = "computing"   // 计算机科学
	SubGeneral     Subreddit = "general"     // 通用讨论
)

// AllSubreddits returns all available subreddits.
func AllSubreddits() []Subreddit {
	return []Subreddit{SubMathematics, SubPhysics, SubPhilosophy, SubBiology, SubComputing, SubGeneral}
}

// Publication represents a published work.
type Publication struct {
	ID          string      `json:"id"`
	Channel     ChannelType `json:"channel"`
	TheoryID    string      `json:"theory_id,omitempty"`
	DraftID     string      `json:"draft_id,omitempty"`
	AuthorID    string      `json:"author_id"`
	AuthorName  string      `json:"author_name"`
	Title       string      `json:"title"`
	Content     string      `json:"content"`
	Abstract    string      `json:"abstract"`
	PublishedAt time.Time   `json:"published_at"`

	// Forum specific - Reddit style
	Subreddit Subreddit `json:"subreddit,omitempty"`
	Upvotes   int       `json:"upvotes"`
	Downvotes int       `json:"downvotes"`
	Score     int       `json:"score"`               // Upvotes - Downvotes
	ParentID  string    `json:"parent_id,omitempty"` // For replies/comments
	IsComment bool      `json:"is_comment,omitempty"`
	Mentions  []string  `json:"mentions,omitempty"`

	// Journal specific
	Reviewers []string `json:"reviewers,omitempty"`
	Approved  bool     `json:"approved,omitempty"`

	// Stats
	Views    int `json:"views"`
	Comments int `json:"comments"` // Number of comments/replies
}

type DraftKind string

const (
	DraftIdea          DraftKind = "idea"
	DraftCollaborative DraftKind = "collaborative"
)

type DraftStatus string

const (
	DraftOpen   DraftStatus = "open"
	DraftLocked DraftStatus = "locked"
)

// Draft represents a working manuscript before submission.
type Draft struct {
	ID           string      `json:"id"`
	Kind         DraftKind   `json:"kind"`
	Status       DraftStatus `json:"status,omitempty"`
	Title        string      `json:"title"`
	Abstract     string      `json:"abstract,omitempty"`
	Content      string      `json:"content"`
	Authors      []string    `json:"authors"`
	SourcePostID string      `json:"source_post_id,omitempty"`
	ConsensusID  string      `json:"consensus_id,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type ConsensusStatus string

const (
	ConsensusOpen     ConsensusStatus = "open"
	ConsensusAchieved ConsensusStatus = "achieved"
	ConsensusClosed   ConsensusStatus = "closed"
)

// ConsensusRequest represents a request to build consensus around a forum post.
type ConsensusRequest struct {
	ID            string          `json:"id"`
	PostID        string          `json:"post_id"`
	CommentID     string          `json:"comment_id,omitempty"`
	RequesterID   string          `json:"requester_id"`
	RequesterName string          `json:"requester_name"`
	Reason        string          `json:"reason,omitempty"`
	Mentions      []string        `json:"mentions,omitempty"`
	Status        ConsensusStatus `json:"status,omitempty"`
	Supporters    []string        `json:"supporters,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type SubmissionStatus string

const (
	SubmissionPending        SubmissionStatus = "pending"
	SubmissionMinorRevision  SubmissionStatus = "minor_revision"
	SubmissionMajorRevision  SubmissionStatus = "major_revision"
	SubmissionAccepted       SubmissionStatus = "accepted"
	SubmissionRejected       SubmissionStatus = "rejected"
)

// Submission represents a journal submission derived from a draft.
type Submission struct {
	ID        string           `json:"id"`
	DraftID   string           `json:"draft_id,omitempty"`
	Title     string           `json:"title"`
	Abstract  string           `json:"abstract,omitempty"`
	Content   string           `json:"content"`
	AuthorID  string           `json:"author_id"`
	AuthorName string          `json:"author_name"`
	Status    SubmissionStatus `json:"status"`
	ReviewIDs []string         `json:"review_ids,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type PaperReviewVerdict string

const (
	VerdictAccept        PaperReviewVerdict = "accept"
	VerdictMinorRevision PaperReviewVerdict = "minor_revision"
	VerdictMajorRevision PaperReviewVerdict = "major_revision"
	VerdictReject        PaperReviewVerdict = "reject"
)

// PaperReviewScores captures scoring dimensions for peer review.
type PaperReviewScores struct {
	Novelty         float64 `json:"novelty"`
	Rigor           float64 `json:"rigor"`
	Falsifiability  float64 `json:"falsifiability"`
	Reproducibility float64 `json:"reproducibility"`
	CrossDomain     float64 `json:"cross_domain"`
}

// PaperReview represents one peer review for a submission.
type PaperReview struct {
	ID           string             `json:"id"`
	SubmissionID string             `json:"submission_id"`
	ReviewerID   string             `json:"reviewer_id"`
	ReviewerName string             `json:"reviewer_name"`
	Scores       PaperReviewScores  `json:"scores"`
	Verdict      PaperReviewVerdict `json:"verdict"`
	Comments     string             `json:"comments,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
}

// Vote represents a vote on a publication.
type Vote struct {
	VoterID  string    `json:"voter_id"`
	PostID   string    `json:"post_id"`
	IsUpvote bool      `json:"is_upvote"`
	VotedAt  time.Time `json:"voted_at"`
}
