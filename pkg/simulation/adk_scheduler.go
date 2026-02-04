// Package simulation provides the ADK-based simulation scheduler.
package simulation

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"

	pkgagent "github.com/cpunion/sci-bot/pkg/agent"
	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/tools"
	"github.com/cpunion/sci-bot/pkg/types"
)

// ADKScheduler manages ADK agents in a simulation.
type ADKScheduler struct {
	mu sync.Mutex

	// Agent runners
	runners map[string]*agentRunner

	// Shared resources
	journal  *publication.Journal
	forum    *publication.Forum
	dataPath string

	// Configuration
	model           model.LLM
	modelForPersona func(*types.Persona) model.LLM

	// Stats
	ticks       int
	actionStats map[string]int
}

type agentRunner struct {
	persona   *types.Persona
	state     *pkgagent.AgentState
	runner    *runner.Runner
	sessionID string
}

// ADKSchedulerConfig configures the ADK scheduler.
type ADKSchedulerConfig struct {
	DataPath        string
	Model           model.LLM
	ModelForPersona func(*types.Persona) model.LLM
}

// NewADKScheduler creates a new ADK-based scheduler.
func NewADKScheduler(cfg ADKSchedulerConfig) *ADKScheduler {
	rand.Seed(time.Now().UnixNano())
	return &ADKScheduler{
		runners:         make(map[string]*agentRunner),
		dataPath:        cfg.DataPath,
		model:           cfg.Model,
		modelForPersona: cfg.ModelForPersona,
		actionStats:     make(map[string]int),
	}
}

// SetJournal sets the journal for publication.
func (s *ADKScheduler) SetJournal(journal *publication.Journal) {
	s.journal = journal
}

// SetForum sets the forum for publication.
func (s *ADKScheduler) SetForum(forum *publication.Forum) {
	s.forum = forum
}

// AddAgent adds an agent to the scheduler.
func (s *ADKScheduler) AddAgent(ctx context.Context, persona *types.Persona) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	modelForAgent := s.resolveModel(persona)
	if modelForAgent == nil {
		return fmt.Errorf("no LLM model configured for agent %s", persona.ID)
	}

	// Create agent state
	agentPath := filepath.Join(s.dataPath, "agents", persona.ID)
	state := pkgagent.NewAgentState(persona.ID, persona.Name, agentPath)
	if err := state.Load(); err != nil {
		log.Printf("Creating new state for %s", persona.Name)
	}

	// Create tools
	forumToolset := tools.NewForumToolset(s.forum, persona.ID)
	socialToolset := tools.NewSocialToolset(state, persona.ID)

	forumTools, err := forumToolset.AllTools(persona.Name)
	if err != nil {
		return fmt.Errorf("failed to create forum tools: %w", err)
	}

	socialTools, err := socialToolset.AllTools()
	if err != nil {
		return fmt.Errorf("failed to create social tools: %w", err)
	}

	allTools := append(forumTools, socialTools...)

	// Create LLM agent
	instruction := buildInstruction(persona)
	adkAgent, err := llmagent.New(llmagent.Config{
		Name:        persona.ID,
		Model:       modelForAgent,
		Description: fmt.Sprintf("%s - %s", persona.Name, persona.Role),
		Instruction: instruction,
		Tools:       allTools,
	})
	if err != nil {
		return fmt.Errorf("failed to create ADK agent: %w", err)
	}

	// Create session service
	sessionService := session.InMemoryService()

	// Create runner
	r, err := runner.New(runner.Config{
		AppName:        "sci-bot",
		Agent:          adkAgent,
		SessionService: sessionService,
	})
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	// Create session
	sess, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName:   "sci-bot",
		UserID:    persona.ID,
		SessionID: persona.ID + "-session",
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	s.runners[persona.ID] = &agentRunner{
		persona:   persona,
		state:     state,
		runner:    r,
		sessionID: sess.Session.ID(),
	}

	return nil
}

func (s *ADKScheduler) resolveModel(persona *types.Persona) model.LLM {
	if s.modelForPersona != nil {
		if m := s.modelForPersona(persona); m != nil {
			return m
		}
	}
	return s.model
}

func buildInstruction(persona *types.Persona) string {
	roleGuidance := ""
	switch persona.Role {
	case types.RoleReviewer:
		roleGuidance = "作为专业审稿人，请重点评估论证是否自洽、证据是否充分、结论是否过度外推，并给出可操作的改进建议。"
	case types.RoleBuilder:
		roleGuidance = "作为严谨构建者，请关注定义清晰、推导步骤完整、可证伪性与可复现性。"
	case types.RoleExplorer:
		roleGuidance = "作为探索者，请提出新颖假设、跨领域联想，并清楚标注不确定性。"
	case types.RoleSynthesizer:
		roleGuidance = "作为综合者，请连接不同领域观点，指出潜在统一框架与冲突点。"
	case types.RoleCommunicator:
		roleGuidance = "作为传播者，请将复杂观点转化为清晰易懂的解释，并保持准确性。"
	}

	roleBlock := ""
	if roleGuidance != "" {
		roleBlock = fmt.Sprintf("\n\n## 角色要求\n%s", roleGuidance)
	}

	return fmt.Sprintf(`你是 %s，一位科学探索者。

## 你的性格
- 角色: %s
- 思维方式: %s
- 创造力: %.0f%%
- 风险承受能力: %.0f%%
- 研究领域: %v

## 你的能力
你可以使用以下工具与科学社区互动：

### 论坛工具
- browse_forum: 浏览论坛帖子（按热度或时间排序，可选板块筛选）
- read_post: 阅读帖子详情和评论
- create_post: 发表新帖子（需要标题、内容和板块）
- vote: 对帖子投票（upvote 或 downvote）
- comment: 发表评论

### 社交工具
- view_relationships: 查看与其他科学家的关系
- update_trust: 更新对某人的信任度
- view_knowledge: 查看已掌握的知识

## 行为准则
1. 以科学家的身份参与讨论
2. 发表有价值、有深度的观点
3. 尊重其他科学家，但敢于质疑
4. 建立有意义的学术关系
5. 持续学习和分享知识%s`,
		persona.Name,
		persona.Role,
		persona.ThinkingStyle,
		persona.Creativity*100,
		persona.RiskTolerance*100,
		persona.Domains,
		roleBlock,
	)
}

// RunTick runs a single simulation tick.
func (s *ADKScheduler) RunTick(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ticks++

	// Select random agent
	ids := make([]string, 0, len(s.runners))
	for id := range s.runners {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}

	selectedID := ids[rand.Intn(len(ids))]
	ar := s.runners[selectedID]

	// Generate a prompt based on random action
	prompt := generateActionPrompt()
	s.actionStats[prompt.action]++

	log.Printf("[Tick %d] %s: %s", s.ticks, ar.persona.Name, prompt.action)

	// Run the agent
	msg := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: prompt.text},
		},
	}

	for event, err := range ar.runner.Run(ctx, ar.persona.ID, ar.sessionID, msg, agent.RunConfig{}) {
		if err != nil {
			log.Printf("Agent error: %v", err)
			continue
		}
		if event != nil && event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					log.Printf("  [%s] %s", ar.persona.Name, truncate(part.Text, 100))
				}
				if part.FunctionCall != nil {
					log.Printf("  [%s] Calling: %s", ar.persona.Name, part.FunctionCall.Name)
				}
			}
		}
	}

	return nil
}

type actionPrompt struct {
	action string
	text   string
}

func generateActionPrompt() actionPrompt {
	prompts := []actionPrompt{
		{"browse", "请浏览论坛，看看有什么有趣的讨论。"},
		{"browse", "查看最新的热门帖子。"},
		{"read", "找一篇有趣的帖子阅读并评论。"},
		{"post", "在论坛发表一个你最近思考的科学问题。"},
		{"interact", "查看你的人际关系，并与一位科学家互动。"},
		{"review", "阅读一篇帖子并投票。"},
	}
	return prompts[rand.Intn(len(prompts))]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// RunFor runs the simulation for n ticks.
func (s *ADKScheduler) RunFor(ctx context.Context, n int) error {
	for i := 0; i < n; i++ {
		if err := s.RunTick(ctx); err != nil {
			return err
		}
		// Small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// Save saves all agent states and publications.
func (s *ADKScheduler) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ar := range s.runners {
		if err := ar.state.Save(); err != nil {
			return fmt.Errorf("failed to save state for %s: %w", ar.persona.Name, err)
		}
	}

	if s.journal != nil {
		if err := s.journal.Save(); err != nil {
			return fmt.Errorf("failed to save journal: %w", err)
		}
	}

	if s.forum != nil {
		if err := s.forum.Save(); err != nil {
			return fmt.Errorf("failed to save forum: %w", err)
		}
	}

	return nil
}

// Stats returns simulation statistics.
func (s *ADKScheduler) Stats() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	agentStats := make(map[string]int)
	for id, ar := range s.runners {
		agentStats[ar.persona.Name] = len(s.runners) // placeholder
		_ = id
	}

	return map[string]interface{}{
		"ticks":        s.ticks,
		"agents":       len(s.runners),
		"action_stats": s.actionStats,
	}
}

// GetAllTools is a helper to get all available tools for an agent.
func GetAllTools(forum *publication.Forum, state *pkgagent.AgentState, agentID, agentName string) ([]tool.Tool, error) {
	forumToolset := tools.NewForumToolset(forum, agentID)
	socialToolset := tools.NewSocialToolset(state, agentID)

	forumTools, err := forumToolset.AllTools(agentName)
	if err != nil {
		return nil, err
	}

	socialTools, err := socialToolset.AllTools()
	if err != nil {
		return nil, err
	}

	return append(forumTools, socialTools...), nil
}
