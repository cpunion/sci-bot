// Package network implements communication and diversity mechanisms.
package network

import (
	"math/rand"
	"sync"

	"github.com/cpunion/sci-bot/pkg/types"
)

// DiversityEngine ensures content diversity across the network.
type DiversityEngine struct {
	mu sync.RWMutex

	// Diversity parameters
	RandomExposureRate  float64 // Probability of random exposure to unrelated content
	CrossDomainBonus    float64 // Weight bonus for cross-domain content
	HeresyAmplification float64 // Amplification for unconventional ideas
	MinorityVoiceBoost  float64 // Boost for minority viewpoints
	NoveltyReward       float64 // Reward coefficient for originality
	RepetitionPenalty   float64 // Penalty for repetitive content
}

// NewDiversityEngine creates a new diversity engine with default settings.
func NewDiversityEngine() *DiversityEngine {
	return &DiversityEngine{
		RandomExposureRate:  0.1, // 10% chance of random content
		CrossDomainBonus:    0.3, // 30% bonus for cross-domain
		HeresyAmplification: 0.5, // 50% amplification for heretical ideas
		MinorityVoiceBoost:  0.2, // 20% boost for minority views
		NoveltyReward:       0.4, // 40% reward for novelty
		RepetitionPenalty:   0.3, // 30% penalty for repetition
	}
}

// Content represents a piece of content in the network.
type Content struct {
	ID              string   `json:"id"`
	Type            string   `json:"type"`
	TheoryID        string   `json:"theory_id,omitempty"`
	MessageID       string   `json:"message_id,omitempty"`
	AuthorID        string   `json:"author_id"`
	Domains         []string `json:"domains"`
	Content         string   `json:"content"`
	IsHeretical     bool     `json:"is_heretical"`
	NoveltyScore    float64  `json:"novelty_score"`
	PopularityScore float64  `json:"popularity_score"`
}

// SelectContentFor selects personalized content for an agent.
func (d *DiversityEngine) SelectContentFor(persona *types.Persona, pool []*Content, limit int) []*Content {
	if len(pool) == 0 {
		return nil
	}

	// Score each content item for this agent
	scored := make([]scoredContent, 0, len(pool))
	for _, c := range pool {
		score := d.scoreContent(c, persona)
		scored = append(scored, scoredContent{content: c, score: score})
	}

	// Sort by score descending
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Select top items with some randomization
	result := make([]*Content, 0, limit)
	for i := 0; i < len(scored) && len(result) < limit; i++ {
		// Random exploration: sometimes include lower-ranked content
		if rand.Float64() < d.RandomExposureRate && i > 0 {
			// Pick a random item from remaining
			randIdx := rand.Intn(len(scored)-i) + i
			result = append(result, scored[randIdx].content)
		} else {
			result = append(result, scored[i].content)
		}
	}

	return result
}

type scoredContent struct {
	content *Content
	score   float64
}

// scoreContent calculates how relevant/valuable content is for an agent.
func (d *DiversityEngine) scoreContent(content *Content, persona *types.Persona) float64 {
	score := 0.0

	// 1. Domain relevance
	domainMatch := 0.0
	crossDomain := true
	for _, cd := range content.Domains {
		for _, pd := range persona.Domains {
			if cd == pd {
				domainMatch += 1.0
				crossDomain = false
			}
		}
	}
	if len(persona.Domains) > 0 {
		score += domainMatch / float64(len(persona.Domains)) * 0.3
	}

	// 2. Cross-domain bonus (higher for agents with high creativity)
	if crossDomain {
		score += d.CrossDomainBonus * persona.Creativity
	}

	// 3. Heretical content bonus (based on risk tolerance)
	if content.IsHeretical {
		score += d.HeresyAmplification * persona.RiskTolerance
	}

	// 4. Novelty reward
	score += content.NoveltyScore * d.NoveltyReward

	// 5. Minority voice boost (inverse of popularity)
	if content.PopularityScore < 0.3 {
		score += d.MinorityVoiceBoost * (1 - content.PopularityScore)
	}

	// 6. Thinking style alignment
	switch persona.ThinkingStyle {
	case types.StyleDivergent:
		// Divergent thinkers prefer novel, heretical content
		if content.IsHeretical || content.NoveltyScore > 0.7 {
			score += 0.2
		}
	case types.StyleConvergent:
		// Convergent thinkers prefer validated, popular content
		if content.PopularityScore > 0.7 {
			score += 0.2
		}
	case types.StyleLateral:
		// Lateral thinkers prefer cross-domain content
		if crossDomain {
			score += 0.2
		}
	}

	return score
}

// ComputeNoveltyScore computes novelty score for new content.
func (d *DiversityEngine) ComputeNoveltyScore(content *Content, existingContent []*Content) float64 {
	if len(existingContent) == 0 {
		return 1.0 // First content is fully novel
	}

	// Simple novelty: inverse of similarity to existing content
	maxSimilarity := 0.0
	for _, existing := range existingContent {
		sim := d.computeSimilarity(content, existing)
		if sim > maxSimilarity {
			maxSimilarity = sim
		}
	}

	return 1.0 - maxSimilarity
}

// computeSimilarity computes domain-based similarity between two content items.
func (d *DiversityEngine) computeSimilarity(a, b *Content) float64 {
	if len(a.Domains) == 0 || len(b.Domains) == 0 {
		return 0.0
	}

	matches := 0
	for _, da := range a.Domains {
		for _, db := range b.Domains {
			if da == db {
				matches++
			}
		}
	}

	return float64(matches) / float64(max(len(a.Domains), len(b.Domains)))
}

// MutationEngine generates variations of theories.
type MutationEngine struct {
	MutationRate           float64 // Base mutation probability
	HallucinationTolerance float64 // Tolerance for unverified ideas
}

// NewMutationEngine creates a new mutation engine.
func NewMutationEngine() *MutationEngine {
	return &MutationEngine{
		MutationRate:           0.1,
		HallucinationTolerance: 0.3,
	}
}

// ShouldMutate decides if a mutation should occur.
func (m *MutationEngine) ShouldMutate() bool {
	return rand.Float64() < m.MutationRate
}

// MutationType defines types of mutations.
type MutationType string

const (
	MutationAxiom      MutationType = "axiom"      // Modify axioms
	MutationDefinition MutationType = "definition" // Redefine concepts
	MutationDerivation MutationType = "derivation" // Try different proof paths
)

// SuggestMutation suggests a type of mutation.
func (m *MutationEngine) SuggestMutation() MutationType {
	r := rand.Float64()
	if r < 0.3 {
		return MutationAxiom
	} else if r < 0.6 {
		return MutationDefinition
	}
	return MutationDerivation
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
