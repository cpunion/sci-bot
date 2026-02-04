// Package tools provides ADK-compatible tools for Sci-Bot agents.
package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/cpunion/sci-bot/pkg/agent"
	"github.com/cpunion/sci-bot/pkg/types"
)

// SocialToolset provides tools for social interactions.
type SocialToolset struct {
	state   *agent.AgentState
	agentID string
}

// NewSocialToolset creates a new social toolset for an agent.
func NewSocialToolset(state *agent.AgentState, agentID string) *SocialToolset {
	return &SocialToolset{
		state:   state,
		agentID: agentID,
	}
}

// --- View Relationships Tool ---

// ViewRelationshipsInput is the input.
type ViewRelationshipsInput struct {
	// Filter by state: "trusted", "discussing", "new", "estranged", "forgotten", or "all"
	StateFilter string `json:"state_filter,omitempty"`
}

// RelationshipInfo represents relationship information.
type RelationshipInfo struct {
	PeerID           string   `json:"peer_id"`
	PeerName         string   `json:"peer_name"`
	State            string   `json:"state"`
	TrustScore       float64  `json:"trust_score"`
	InteractionCount int      `json:"interaction_count"`
	SharedTopics     []string `json:"shared_topics,omitempty"`
}

// ViewRelationshipsOutput is the output.
type ViewRelationshipsOutput struct {
	Relationships []RelationshipInfo `json:"relationships"`
	TotalCount    int                `json:"total_count"`
}

// ViewRelationshipsTool creates the view relationships tool.
func (st *SocialToolset) ViewRelationshipsTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input ViewRelationshipsInput) (ViewRelationshipsOutput, error) {
		var rels []*types.Relationship

		switch input.StateFilter {
		case "trusted":
			rels = st.state.GetTrustedPeers()
		case "all", "":
			rels = st.state.GetActivePeers()
		default:
			// Get all and filter
			all := st.state.GetActivePeers()
			filterState := types.RelationshipState(input.StateFilter)
			for _, r := range all {
				if r.State == filterState {
					rels = append(rels, r)
				}
			}
		}

		infos := make([]RelationshipInfo, 0, len(rels))
		for _, r := range rels {
			infos = append(infos, RelationshipInfo{
				PeerID:           r.PeerID,
				PeerName:         r.PeerName,
				State:            string(r.State),
				TrustScore:       r.TrustScore,
				InteractionCount: r.InteractionCount,
				SharedTopics:     r.SharedTopics,
			})
		}

		return ViewRelationshipsOutput{
			Relationships: infos,
			TotalCount:    len(infos),
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "view_relationships",
		Description: "查看我与其他 Agent 的关系状态。可以按关系状态筛选。",
	}, handler)
}

// --- Update Trust Tool ---

// UpdateTrustInput is the input.
type UpdateTrustInput struct {
	PeerID     string  `json:"peer_id"`
	TrustDelta float64 `json:"trust_delta"` // -1.0 to 1.0
	Reason     string  `json:"reason,omitempty"`
}

// UpdateTrustOutput is the output.
type UpdateTrustOutput struct {
	NewTrustScore float64 `json:"new_trust_score"`
	NewState      string  `json:"new_state"`
	Message       string  `json:"message"`
}

// UpdateTrustTool creates the update trust tool.
func (st *SocialToolset) UpdateTrustTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input UpdateTrustInput) (UpdateTrustOutput, error) {
		rel := st.state.GetRelationship(input.PeerID)
		if rel == nil {
			return UpdateTrustOutput{}, fmt.Errorf("no relationship with peer: %s", input.PeerID)
		}

		// Update trust score
		rel.TrustScore += input.TrustDelta
		if rel.TrustScore < 0 {
			rel.TrustScore = 0
		}
		if rel.TrustScore > 1 {
			rel.TrustScore = 1
		}

		// State transitions based on trust
		if rel.TrustScore > 0.8 && rel.State == types.RelationDiscussing {
			rel.State = types.RelationTrusted
		} else if rel.TrustScore < 0.3 && rel.State != types.RelationNew {
			rel.State = types.RelationEstranged
		}

		return UpdateTrustOutput{
			NewTrustScore: rel.TrustScore,
			NewState:      string(rel.State),
			Message:       fmt.Sprintf("已更新与 %s 的信任度", rel.PeerName),
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "update_trust",
		Description: "更新我对某个 Agent 的信任度。正值增加信任，负值降低信任。",
	}, handler)
}

// --- View Knowledge Tool ---

// ViewKnowledgeInput is the input.
type ViewKnowledgeInput struct {
	// Level filter: "heard", "learned", "mastered", or "all"
	LevelFilter string `json:"level_filter,omitempty"`
}

// KnowledgeInfo represents knowledge information.
type KnowledgeInfo struct {
	TheoryID    string  `json:"theory_id"`
	TheoryTitle string  `json:"theory_title"`
	Level       string  `json:"level"`
	Confidence  float64 `json:"confidence"`
	Source      string  `json:"source"`
}

// ViewKnowledgeOutput is the output.
type ViewKnowledgeOutput struct {
	Knowledge  []KnowledgeInfo `json:"knowledge"`
	TotalCount int             `json:"total_count"`
}

// ViewKnowledgeTool creates the view knowledge tool.
func (st *SocialToolset) ViewKnowledgeTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input ViewKnowledgeInput) (ViewKnowledgeOutput, error) {
		var items []*types.KnowledgeItem

		switch input.LevelFilter {
		case "heard":
			items = st.state.GetKnowledgeByLevel(types.KnowledgeHeard)
		case "learned":
			items = st.state.GetKnowledgeByLevel(types.KnowledgeLearned)
		case "mastered":
			items = st.state.GetKnowledgeByLevel(types.KnowledgeMastered)
		default:
			// Get all
			for _, level := range []types.KnowledgeLevel{types.KnowledgeHeard, types.KnowledgeLearned, types.KnowledgeMastered} {
				items = append(items, st.state.GetKnowledgeByLevel(level)...)
			}
		}

		infos := make([]KnowledgeInfo, 0, len(items))
		for _, k := range items {
			infos = append(infos, KnowledgeInfo{
				TheoryID:    k.TheoryID,
				TheoryTitle: k.TheoryTitle,
				Level:       string(k.Level),
				Confidence:  k.Confidence,
				Source:      k.Source,
			})
		}

		return ViewKnowledgeOutput{
			Knowledge:  infos,
			TotalCount: len(infos),
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "view_knowledge",
		Description: "查看我已掌握或了解的知识/理论。可以按掌握程度筛选。",
	}, handler)
}

// AllTools returns all social tools.
func (st *SocialToolset) AllTools() ([]tool.Tool, error) {
	viewRelTool, err := st.ViewRelationshipsTool()
	if err != nil {
		return nil, err
	}

	updateTrustTool, err := st.UpdateTrustTool()
	if err != nil {
		return nil, err
	}

	viewKnowledgeTool, err := st.ViewKnowledgeTool()
	if err != nil {
		return nil, err
	}

	return []tool.Tool{
		viewRelTool,
		updateTrustTool,
		viewKnowledgeTool,
	}, nil
}
