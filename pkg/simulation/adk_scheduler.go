// Package simulation provides the ADK-based simulation scheduler.
package simulation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
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
	workflow *publication.Workflow
	dataPath string

	// Configuration
	model           model.LLM
	modelForPersona func(*types.Persona) model.LLM
	turnLimit       int
	graceTurns      int
	logger          EventLogger
	simTime         time.Time
	simStep         time.Duration
	agentsPerTick   int
	checkpointEvery int

	// Stats
	ticks       int
	actionStats map[string]int
}

type agentRunner struct {
	persona   *types.Persona
	state     *pkgagent.AgentState
	runner    *runner.Runner
	sessionID string
	appName   string
	session   session.Service

	actionWeights  map[string]float64
	turnCount      int
	bellRung       bool
	graceRemaining int
}

// ADKSchedulerConfig configures the ADK scheduler.
type ADKSchedulerConfig struct {
	DataPath        string
	Model           model.LLM
	ModelForPersona func(*types.Persona) model.LLM
	Workflow        *publication.Workflow
	TurnLimit       int
	GraceTurns      int
	Logger          EventLogger
	SimStep         time.Duration
	StartTime       time.Time
	AgentsPerTick   int
	CheckpointEvery int
}

// NewADKScheduler creates a new ADK-based scheduler.
func NewADKScheduler(cfg ADKSchedulerConfig) *ADKScheduler {
	rand.Seed(time.Now().UnixNano())
	turnLimit := cfg.TurnLimit
	if turnLimit <= 0 {
		turnLimit = 10
	}
	graceTurns := cfg.GraceTurns
	if graceTurns <= 0 {
		graceTurns = 3
	}
	simStep := cfg.SimStep
	if simStep <= 0 {
		simStep = time.Hour
	}
	startTime := cfg.StartTime
	if startTime.IsZero() {
		startTime = time.Now()
	}

	checkpointEvery := cfg.CheckpointEvery
	if checkpointEvery <= 0 {
		checkpointEvery = 1
	}

	workflow := cfg.Workflow
	if workflow == nil && cfg.DataPath != "" {
		workflow = publication.NewWorkflow(filepath.Join(cfg.DataPath, "workflow"))
		if err := workflow.Load(); err != nil {
			log.Printf("Failed to load workflow: %v", err)
		}
	}

	return &ADKScheduler{
		runners:         make(map[string]*agentRunner),
		dataPath:        cfg.DataPath,
		model:           cfg.Model,
		modelForPersona: cfg.ModelForPersona,
		turnLimit:       turnLimit,
		graceTurns:      graceTurns,
		logger:          cfg.Logger,
		simTime:         startTime,
		simStep:         simStep,
		agentsPerTick:   maxInt(cfg.AgentsPerTick, 1),
		checkpointEvery: checkpointEvery,
		workflow:        workflow,
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

// SetWorkflow sets the workflow store.
func (s *ADKScheduler) SetWorkflow(workflow *publication.Workflow) {
	s.workflow = workflow
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
	forumToolset := tools.NewForumToolset(s.forum, persona.ID, persona, state)
	socialToolset := tools.NewSocialToolset(state, persona.ID)
	publicationToolset := tools.NewPublicationToolset(s.workflow, s.journal, s.forum, persona, s.dataPath)

	forumTools, err := forumToolset.AllTools(persona.Name)
	if err != nil {
		return fmt.Errorf("failed to create forum tools: %w", err)
	}

	socialTools, err := socialToolset.AllTools()
	if err != nil {
		return fmt.Errorf("failed to create social tools: %w", err)
	}

	publicationTools, err := publicationToolset.AllTools()
	if err != nil {
		return fmt.Errorf("failed to create publication tools: %w", err)
	}

	allTools := append(forumTools, socialTools...)
	allTools = append(allTools, publicationTools...)

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
		State: map[string]any{
			"agent_summary": "",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	s.runners[persona.ID] = &agentRunner{
		persona:        persona,
		state:          state,
		runner:         r,
		sessionID:      sess.Session.ID(),
		appName:        "sci-bot",
		session:        sessionService,
		actionWeights:  buildActionWeights(persona),
		graceRemaining: s.graceTurns,
		bellRung:       false,
		turnCount:      0,
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
- read_post: 阅读帖子详情和树形评论（含 parent_id 与 depth，可用于理解讨论层级）
- get_thread_digest: 多帖汇总专用：线程摘要+摘要后的新回复（如无摘要会提示 needs_summary）
- save_thread_summary: 保存线程摘要缓存（仅在你完成该线程总结后调用）
- browse_mentions: 查看与你相关的 @ 提及或回复，优先处理
- create_post: 发表新帖子（需要标题、内容和板块）
- vote: 对帖子投票（upvote 或 downvote）
- comment: 发表评论或回复评论（使用 parent_id 回复某条评论，否则用 post_id 回复顶层）

### 发表工具
- assess_readiness: 评估个人想法成熟度
- assess_consensus: 评估论坛线程共识成熟度
- create_draft: 创建学术草案（idea 或 collaborative）
- request_consensus: 在论坛帖子下发起共识请求（自动发布评论）
- submit_paper: 提交草案到期刊审稿
- review_paper: 对投稿进行审稿（Reviewer 角色）

### 社交工具
- view_relationships: 查看与其他科学家的关系
- update_trust: 更新对某人的信任度
- view_knowledge: 查看已掌握的知识

## 行为准则
1. 以科学家的身份参与讨论
2. 发表有价值、有深度的观点
3. 尊重其他科学家，但敢于质疑
4. 建立有意义的学术关系
5. 持续学习和分享知识

## 发言规则
- 只有在被点名、能提供新见解/证据、纠错或总结时才发言
- 如果没有增量贡献，请简短说明继续观察
- 若被 @ 提及或有人回复你，请优先处理

## 阅读规则
- 单个帖子内分析请使用 read_post，不要做摘要
- 多帖汇总时优先使用 get_thread_digest
- 若 needs_summary=true，请逐帖 read_post 后总结，并调用 save_thread_summary 记录

## 创新导向
- 在科研相关问题上主动创新、提出新假设或改进建议
- 非科研话题可根据个人性格自由选择参与方式

## 摘要记忆（单条滚动沉淀）
{agent_summary?}

## 作息规则
- 当出现“晚钟/敲钟/夜间休息”的提示时，需礼貌收尾并立即休息，不再展开新话题。%s`,
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

	// Select random eligible agent
	ids := s.eligibleAgentIDs()
	if len(ids) == 0 {
		return nil
	}

	perTick := s.agentsPerTick
	if perTick <= 0 {
		perTick = 1
	}
	if perTick > len(ids) {
		perTick = len(ids)
	}

	rand.Shuffle(len(ids), func(i, j int) {
		ids[i], ids[j] = ids[j], ids[i]
	})

	for _, selectedID := range ids[:perTick] {
		ar := s.runners[selectedID]
		if ar == nil {
			continue
		}
		// Generate a prompt based on random action
		prompt := s.selectActionPrompt(ar)
		s.actionStats[prompt.action]++

		log.Printf("[Tick %d] %s: %s", s.ticks, ar.persona.Name, prompt.action)

		// Run the agent
		msg := &genai.Content{
			Role: "user",
			Parts: []*genai.Part{
				{Text: prompt.text},
			},
		}

		var responseText string
		toolCalls := make([]string, 0)
		toolResponses := make([]string, 0)
		for event, err := range ar.runner.Run(ctx, ar.persona.ID, ar.sessionID, msg, agent.RunConfig{}) {
			if err != nil {
				log.Printf("Agent error: %v", err)
				continue
			}
			if event != nil && event.Content != nil {
				for _, part := range event.Content.Parts {
					if part.Text != "" {
						log.Printf("  [%s] %s", ar.persona.Name, truncate(part.Text, 100))
						responseText += part.Text
					}
					if part.FunctionCall != nil {
						log.Printf("  [%s] Calling: %s", ar.persona.Name, part.FunctionCall.Name)
						toolCalls = append(toolCalls, part.FunctionCall.Name)
					}
					if part.FunctionResponse != nil {
						toolResponses = append(toolResponses, part.FunctionResponse.Name)
					}
				}
			}
		}

		s.updateAgentSummary(ctx, ar, prompt.text, responseText)
		s.logEvent(ar, prompt, responseText, toolCalls, toolResponses)
	}
	s.simTime = s.simTime.Add(s.simStep)
	if s.checkpointEvery > 0 && s.ticks%s.checkpointEvery == 0 {
		if err := s.checkpointLocked(false); err != nil {
			log.Printf("Checkpoint failed: %v", err)
		}
	}

	return nil
}

type actionPrompt struct {
	action string
	text   string
}

func (s *ADKScheduler) selectActionPrompt(ar *agentRunner) actionPrompt {
	if ar == nil {
		return actionPrompt{action: "idle", text: "请保持待命。"}
	}

	if ar.turnCount >= s.turnLimit {
		if !ar.bellRung {
			ar.bellRung = true
			ar.graceRemaining = s.graceTurns
			ar.turnCount++
			return actionPrompt{action: "sleep", text: "夜间敲钟：今天到此为止，请简短收尾并休息。"}
		}
		if ar.graceRemaining <= 0 {
			return actionPrompt{action: "sleep", text: "夜已深，请立即休息，不再展开新话题。"}
		}
		ar.graceRemaining--
		ar.turnCount++
		return actionPrompt{action: "sleep", text: "夜间敲钟已响，请礼貌结束并去休息。"}
	}

	action := weightedSelect(ar.actionWeights)
	promptText := pickActionText(action)
	ar.turnCount++
	return actionPrompt{action: action, text: promptText}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *ADKScheduler) eligibleAgentIDs() []string {
	ids := make([]string, 0, len(s.runners))
	for id, ar := range s.runners {
		if ar == nil {
			continue
		}
		if ar.bellRung && ar.graceRemaining <= 0 {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func buildActionWeights(p *types.Persona) map[string]float64 {
	weights := map[string]float64{
		"browse":   0.3,
		"read":     0.3,
		"post":     0.2,
		"interact": 0.2,
		"review":   0.15,
		"observe":  0.2,
	}

	if p != nil {
		weights["browse"] += p.RiskTolerance*0.2 + p.Creativity*0.1
		weights["read"] += p.Rigor * 0.1
		weights["post"] += p.Creativity*0.2 + p.Influence*0.1 - p.Rigor*0.05
		weights["interact"] += p.Sociability * 0.3
		weights["review"] += p.Rigor * 0.3
		weights["observe"] += (1.0 - p.Sociability) * 0.2
		if p.Role == types.RoleReviewer {
			weights["review"] += 0.4
			weights["observe"] += 0.1
		}
	}

	for k, v := range weights {
		jitter := 0.8 + rand.Float64()*0.4
		weights[k] = v * jitter
	}

	return weights
}

func weightedSelect(weights map[string]float64) string {
	if len(weights) == 0 {
		return "browse"
	}
	total := 0.0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total == 0 {
		return "browse"
	}
	r := rand.Float64() * total
	for k, w := range weights {
		if w <= 0 {
			continue
		}
		r -= w
		if r <= 0 {
			return k
		}
	}
	for k := range weights {
		return k
	}
	return "browse"
}

func pickActionText(action string) string {
	switch action {
	case "browse":
		return pickOne([]string{
			"请浏览论坛，看看有什么有趣的讨论。",
			"查看最新的热门帖子。",
			"看看与你研究领域相关的新讨论。",
		})
	case "read":
		return pickOne([]string{
			"找一篇有趣的帖子阅读并评论。",
			"阅读一篇与你领域相关的帖子，给出简短反馈。",
		})
	case "post":
		return pickOne([]string{
			"在论坛发表一个你最近思考的科学问题。",
			"发布一个简短的研究想法或假设，邀请讨论。",
		})
	case "interact":
		return pickOne([]string{
			"查看你的人际关系，并与一位科学家互动。",
			"选择一位你信任的同行进行学术交流。",
		})
	case "review":
		return pickOne([]string{
			"阅读一篇帖子并投票。",
			"对一篇讨论进行审慎评估，给出立场。",
		})
	case "observe":
		return pickOne([]string{
			"保持观察。如果没有新增贡献，请简短说明继续关注。",
			"暂不发言，记录你认为重要的线索。",
		})
	default:
		return "请保持待命。"
	}
}

func pickOne(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[rand.Intn(len(items))]
}

func (s *ADKScheduler) logEvent(ar *agentRunner, prompt actionPrompt, response string, toolCalls, toolResponses []string) {
	if s.logger == nil || ar == nil {
		return
	}
	ev := EventLog{
		Timestamp:      time.Now(),
		SimTime:        s.simTime,
		Tick:           s.ticks,
		AgentID:        ar.persona.ID,
		AgentName:      ar.persona.Name,
		Action:         prompt.action,
		Prompt:         truncateRunes(prompt.text, 500),
		Response:       truncateRunes(response, 500),
		ToolCalls:      toolCalls,
		ToolResponses:  toolResponses,
		TurnCount:      ar.turnCount,
		BellRung:       ar.bellRung,
		GraceRemaining: ar.graceRemaining,
		Sleeping:       prompt.action == "sleep",
	}
	if err := s.logger.LogEvent(ev); err != nil {
		log.Printf("Failed to log event: %v", err)
	}
}

func (s *ADKScheduler) updateAgentSummary(ctx context.Context, ar *agentRunner, promptText, responseText string) {
	entry := buildSummaryEntry(s.simTime, promptText, responseText)
	if entry == "" || ar.session == nil {
		return
	}

	sessResp, err := ar.session.Get(ctx, &session.GetRequest{
		AppName:   ar.appName,
		UserID:    ar.persona.ID,
		SessionID: ar.sessionID,
	})
	if err != nil {
		log.Printf("Failed to load session for summary: %v", err)
		return
	}

	current := ""
	if v, err := sessResp.Session.State().Get("agent_summary"); err == nil {
		if str, ok := v.(string); ok {
			current = str
		}
	}

	updated := appendSummary(current, entry, 2000)
	event := session.NewEvent("summary-update")
	event.Author = ar.persona.ID
	event.Actions.StateDelta["agent_summary"] = updated
	if err := ar.session.AppendEvent(ctx, sessResp.Session, event); err != nil {
		log.Printf("Failed to append summary event: %v", err)
	}

	if err := s.appendDailyLog(ar.persona.ID, promptText, responseText, entry); err != nil {
		log.Printf("Failed to append daily log: %v", err)
	}
}

func buildSummaryEntry(at time.Time, promptText, responseText string) string {
	promptText = strings.TrimSpace(promptText)
	responseText = strings.TrimSpace(responseText)
	if promptText == "" && responseText == "" {
		return ""
	}

	entry := fmt.Sprintf("%s | prompt: %s", at.Format(time.RFC3339), truncateRunes(promptText, 200))
	if responseText != "" {
		entry = fmt.Sprintf("%s | reply: %s", entry, truncateRunes(responseText, 200))
	}
	return entry
}

func appendSummary(current, entry string, maxChars int) string {
	if entry == "" {
		return current
	}
	if current == "" {
		return truncateRunes(entry, maxChars)
	}
	return truncateRunes(current+"\n"+entry, maxChars)
}

func truncateRunes(s string, maxChars int) string {
	if maxChars <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[len(runes)-maxChars:])
}

func (s *ADKScheduler) appendDailyLog(agentID, promptText, responseText, entry string) error {
	if entry == "" {
		return nil
	}
	if s.dataPath == "" {
		return nil
	}
	dateKey := s.simTime.Format("2006-01-02")
	dir := filepath.Join(s.dataPath, "agents", agentID, "daily")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	jsonPath := filepath.Join(dir, dateKey+".jsonl")
	jsonFile, err := os.OpenFile(jsonPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	record := dailyLogEntry{
		Timestamp: s.simTime.Format(time.RFC3339),
		Prompt:    strings.TrimSpace(promptText),
		Reply:     strings.TrimSpace(responseText),
		Raw:       entry,
		Notes:     "",
	}
	if err := writeJSONLine(jsonFile, record); err != nil {
		return err
	}
	return nil
}

type dailyLogEntry struct {
	Timestamp string `json:"timestamp"`
	Prompt    string `json:"prompt,omitempty"`
	Reply     string `json:"reply,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Raw       string `json:"raw,omitempty"`
}

func writeJSONLine(f *os.File, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *ADKScheduler) saveSimState() error {
	if s.dataPath == "" {
		return nil
	}
	if err := os.MkdirAll(s.dataPath, 0755); err != nil {
		return err
	}
	state := SimState{
		SimTime:     s.simTime,
		Ticks:       s.ticks,
		StepSeconds: int(s.simStep.Seconds()),
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dataPath, "sim_state.json"), data, 0644)
}

func (s *ADKScheduler) checkpointLocked(closeLogger bool) error {
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

	if err := s.saveSimState(); err != nil {
		return fmt.Errorf("failed to save sim state: %w", err)
	}

	if closeLogger && s.logger != nil {
		if err := s.logger.Close(); err != nil {
			return fmt.Errorf("failed to close logger: %w", err)
		}
	}

	return nil
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

	return s.checkpointLocked(true)
}

// Checkpoint persists current state without closing the logger.
func (s *ADKScheduler) Checkpoint() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.checkpointLocked(false)
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
func GetAllTools(forum *publication.Forum, state *pkgagent.AgentState, persona *types.Persona, agentID, agentName string) ([]tool.Tool, error) {
	forumToolset := tools.NewForumToolset(forum, agentID, persona, state)
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
