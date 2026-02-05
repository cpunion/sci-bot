package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/types"
)

// PublicationToolset provides tools for drafts, consensus, and journal workflows.
type PublicationToolset struct {
	workflow *publication.Workflow
	journal  *publication.Journal
	forum    *publication.Forum
	persona  *types.Persona
	dataPath string
}

// NewPublicationToolset creates a publication toolset.
func NewPublicationToolset(workflow *publication.Workflow, journal *publication.Journal, forum *publication.Forum, persona *types.Persona, dataPath string) *PublicationToolset {
	return &PublicationToolset{
		workflow: workflow,
		journal:  journal,
		forum:    forum,
		persona:  persona,
		dataPath: dataPath,
	}
}

// --- Create Draft Tool ---

type CreateDraftInput struct {
	Title        string `json:"title"`
	Abstract     string `json:"abstract,omitempty"`
	Content      string `json:"content"`
	Kind         string `json:"kind,omitempty"`         // idea | collaborative
	SourcePostID string `json:"source_post_id,omitempty"`
	ConsensusID  string `json:"consensus_id,omitempty"`
}

type CreateDraftOutput struct {
	DraftID string `json:"draft_id"`
	Message string `json:"message"`
}

func (pt *PublicationToolset) CreateDraftTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input CreateDraftInput) (CreateDraftOutput, error) {
		if pt.workflow == nil {
			return CreateDraftOutput{}, fmt.Errorf("workflow not available")
		}
		if strings.TrimSpace(input.Title) == "" {
			return CreateDraftOutput{}, fmt.Errorf("missing title")
		}
		content := strings.TrimSpace(input.Content)
		if content == "" {
			return CreateDraftOutput{}, fmt.Errorf("missing content")
		}

		kind := types.DraftKind(strings.ToLower(strings.TrimSpace(input.Kind)))
		if input.ConsensusID != "" {
			kind = types.DraftCollaborative
		}
		if kind == "" {
			kind = types.DraftIdea
		}

		authorID := ""
		authorName := ""
		if pt.persona != nil {
			authorID = pt.persona.ID
			authorName = pt.persona.Name
		}
		authors := []string{}
		if authorID != "" {
			authors = append(authors, authorID)
		} else if authorName != "" {
			authors = append(authors, authorName)
		}

		draft := &types.Draft{
			Kind:         kind,
			Status:       types.DraftOpen,
			Title:        strings.TrimSpace(input.Title),
			Abstract:     strings.TrimSpace(input.Abstract),
			Content:      content,
			Authors:      authors,
			SourcePostID: strings.TrimSpace(input.SourcePostID),
			ConsensusID:  strings.TrimSpace(input.ConsensusID),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		draftID := pt.workflow.CreateDraft(draft)
		if err := pt.workflow.Save(); err != nil {
			return CreateDraftOutput{}, err
		}

		return CreateDraftOutput{
			DraftID: draftID,
			Message: fmt.Sprintf("Draft created (%s)", kind),
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "create_draft",
		Description: "创建一份学术草案（idea 或 collaborative）。",
	}, handler)
}

// --- Request Consensus Tool ---

type RequestConsensusInput struct {
	PostID   string   `json:"post_id"`
	Reason   string   `json:"reason,omitempty"`
	Mentions []string `json:"mentions,omitempty"`
}

type RequestConsensusOutput struct {
	ConsensusID string `json:"consensus_id"`
	CommentID   string `json:"comment_id,omitempty"`
	Message     string `json:"message"`
}

func (pt *PublicationToolset) RequestConsensusTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input RequestConsensusInput) (RequestConsensusOutput, error) {
		if pt.workflow == nil {
			return RequestConsensusOutput{}, fmt.Errorf("workflow not available")
		}
		if pt.forum == nil {
			return RequestConsensusOutput{}, fmt.Errorf("forum not available")
		}
		postID := strings.TrimSpace(input.PostID)
		if postID == "" {
			return RequestConsensusOutput{}, fmt.Errorf("missing post_id")
		}
		post := pt.forum.Get(postID)
		if post == nil || post.IsComment {
			return RequestConsensusOutput{}, fmt.Errorf("post not found: %s", postID)
		}

		reason := strings.TrimSpace(input.Reason)
		if reason == "" {
			reason = "requesting consensus and collaborative draft"
		}

		mentions := normalizeMentions(input.Mentions)
		mentionLine := ""
		if len(mentions) > 0 {
			formatted := make([]string, 0, len(mentions))
			for _, m := range mentions {
				formatted = append(formatted, "@"+m)
			}
			mentionLine = "\n\nInviting: " + strings.Join(formatted, " ")
		}

		content := fmt.Sprintf("[Consensus Request]\n%s%s", reason, mentionLine)
		comment := &types.Publication{
			AuthorID:   personaID(pt.persona),
			AuthorName: personaName(pt.persona),
			Content:    content,
			Mentions:   extractMentions(content),
		}

		if err := pt.forum.Comment(postID, comment); err != nil {
			return RequestConsensusOutput{}, err
		}

		req := &types.ConsensusRequest{
			PostID:        postID,
			CommentID:     comment.ID,
			RequesterID:   personaID(pt.persona),
			RequesterName: personaName(pt.persona),
			Reason:        reason,
			Mentions:      mentions,
			Status:        types.ConsensusOpen,
			Supporters:    nonEmptySlice(personaID(pt.persona)),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		id := pt.workflow.AddConsensusRequest(req)
		if err := pt.workflow.Save(); err != nil {
			return RequestConsensusOutput{}, err
		}

		return RequestConsensusOutput{
			ConsensusID: id,
			CommentID:   comment.ID,
			Message:     "Consensus request posted",
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "request_consensus",
		Description: "在论坛帖子下发起共识请求（会自动发布一条评论）。",
	}, handler)
}

// --- Submit Paper Tool ---

type SubmitPaperInput struct {
	DraftID  string `json:"draft_id,omitempty"`
	Title    string `json:"title,omitempty"`
	Abstract string `json:"abstract,omitempty"`
	Content  string `json:"content,omitempty"`
}

type SubmitPaperOutput struct {
	SubmissionID string `json:"submission_id"`
	Message      string `json:"message"`
}

func (pt *PublicationToolset) SubmitPaperTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input SubmitPaperInput) (SubmitPaperOutput, error) {
		if pt.workflow == nil {
			return SubmitPaperOutput{}, fmt.Errorf("workflow not available")
		}
		if pt.journal == nil {
			return SubmitPaperOutput{}, fmt.Errorf("journal not available")
		}

		title := strings.TrimSpace(input.Title)
		abstract := strings.TrimSpace(input.Abstract)
		content := strings.TrimSpace(input.Content)
		draftID := strings.TrimSpace(input.DraftID)

		if draftID != "" {
			draft := pt.workflow.GetDraft(draftID)
			if draft == nil {
				return SubmitPaperOutput{}, fmt.Errorf("draft not found: %s", draftID)
			}
			if title == "" {
				title = draft.Title
			}
			if abstract == "" {
				abstract = draft.Abstract
			}
			if content == "" {
				content = draft.Content
			}
		}

		if title == "" || content == "" {
			return SubmitPaperOutput{}, fmt.Errorf("missing title or content")
		}

		pub := &types.Publication{
			AuthorID:   personaID(pt.persona),
			AuthorName: personaName(pt.persona),
			Title:      title,
			Abstract:   abstract,
			Content:    content,
			DraftID:    draftID,
		}

		if err := pt.journal.Submit(pub); err != nil {
			return SubmitPaperOutput{}, err
		}

		sub := &types.Submission{
			ID:         pub.ID,
			DraftID:    draftID,
			Title:      title,
			Abstract:   abstract,
			Content:    content,
			AuthorID:   personaID(pt.persona),
			AuthorName: personaName(pt.persona),
			Status:     types.SubmissionPending,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		pt.workflow.AddSubmission(sub)
		if err := pt.workflow.Save(); err != nil {
			return SubmitPaperOutput{}, err
		}

		return SubmitPaperOutput{
			SubmissionID: pub.ID,
			Message:      "Submission created and sent to journal",
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "submit_paper",
		Description: "提交草案到期刊审稿（支持 draft_id 或直接内容）。",
	}, handler)
}

// --- Assess Readiness Tool ---

type AssessReadinessInput struct {
	// WindowDays controls the lookback window (default 7, max 30)
	WindowDays int `json:"window_days,omitempty"`
}

type AssessReadinessOutput struct {
	Score          float64         `json:"score"`
	Threshold      float64         `json:"threshold"`
	Ready          bool            `json:"ready"`
	Counts         map[string]int  `json:"counts"`
	Signals        map[string]float64 `json:"signals"`
	Recommendation string          `json:"recommendation,omitempty"`
}

func (pt *PublicationToolset) AssessReadinessTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input AssessReadinessInput) (AssessReadinessOutput, error) {
		if pt.persona == nil {
			return AssessReadinessOutput{}, fmt.Errorf("persona not available")
		}
		windowDays := input.WindowDays
		if windowDays <= 0 {
			windowDays = 7
		}
		if windowDays > 30 {
			windowDays = 30
		}
		windowStart := time.Now().AddDate(0, 0, -windowDays)

		activityDays := 0
		keywordHits := 0
		if pt.dataPath != "" {
			activityDays, keywordHits = scanDailyNotes(pt.dataPath, pt.persona.ID, windowStart)
		}

		postCount, commentCount := 0, 0
		if pt.forum != nil {
			for _, pub := range pt.forum.AllPublications() {
				if pub == nil || pub.AuthorID != pt.persona.ID {
					continue
				}
				if pub.PublishedAt.Before(windowStart) {
					continue
				}
				if pub.IsComment {
					commentCount++
				} else {
					postCount++
				}
			}
		}

		activityScore := clamp01(float64(activityDays) / 3.0)
		postScore := clamp01(float64(postCount) / 2.0)
		commentScore := clamp01(float64(commentCount) / 6.0)
		keywordScore := clamp01(float64(keywordHits) / 3.0)

		score := 0.35*activityScore + 0.35*postScore + 0.2*commentScore + 0.1*keywordScore
		threshold := 0.65
		ready := score >= threshold

		rec := ""
		if ready {
			rec = "Ready to draft. Consider using create_draft."
		} else if postCount == 0 {
			rec = "Need more public discussion. Post at least one hypothesis on the forum."
		} else if activityDays < 2 {
			rec = "Increase daily accumulation before drafting."
		} else {
			rec = "Deepen evidence and add comments from peers before drafting."
		}

		return AssessReadinessOutput{
			Score:     round2(score),
			Threshold: threshold,
			Ready:     ready,
			Counts: map[string]int{
				"activity_days": activityDays,
				"posts":         postCount,
				"comments":      commentCount,
				"keyword_hits":  keywordHits,
			},
			Signals: map[string]float64{
				"activity": activityScore,
				"posts":    postScore,
				"comments": commentScore,
				"keywords": keywordScore,
			},
			Recommendation: rec,
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "assess_readiness",
		Description: "评估个人科研想法是否达到起草草案的成熟度。",
	}, handler)
}

// --- Assess Consensus Tool ---

type AssessConsensusInput struct {
	PostID string `json:"post_id"`
}

type AssessConsensusOutput struct {
	Score          float64         `json:"score"`
	Threshold      float64         `json:"threshold"`
	Status         string          `json:"status"`
	Counts         map[string]int  `json:"counts"`
	Signals        map[string]float64 `json:"signals"`
	ConsensusID    string          `json:"consensus_id,omitempty"`
	Recommendation string          `json:"recommendation,omitempty"`
}

func (pt *PublicationToolset) AssessConsensusTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input AssessConsensusInput) (AssessConsensusOutput, error) {
		postID := strings.TrimSpace(input.PostID)
		if postID == "" {
			return AssessConsensusOutput{}, fmt.Errorf("missing post_id")
		}
		if pt.forum == nil {
			return AssessConsensusOutput{}, fmt.Errorf("forum not available")
		}
		post := pt.forum.Get(postID)
		if post == nil || post.IsComment {
			return AssessConsensusOutput{}, fmt.Errorf("post not found: %s", postID)
		}

		comments := pt.forum.GetThreadComments(postID)
		uniqueCommenters := map[string]struct{}{}
		reviewerCommenters := map[string]struct{}{}
		supportMentions := 0
		maxDepth := 0
		for _, c := range comments {
			if c == nil {
				continue
			}
			if c.AuthorID != "" && c.AuthorID != post.AuthorID {
				uniqueCommenters[c.AuthorID] = struct{}{}
				if looksLikeReviewer(c.AuthorID, c.AuthorName) {
					reviewerCommenters[c.AuthorID] = struct{}{}
				}
			}
			if hasSupportSignal(c.Content) {
				supportMentions++
			}
			depth := computeCommentDepth(pt.forum, c, postID)
			if depth > maxDepth {
				maxDepth = depth
			}
		}

		uniqueCount := len(uniqueCommenters)
		reviewerCount := len(reviewerCommenters)

		uniqueScore := clamp01(float64(uniqueCount) / 4.0)
		reviewerScore := clamp01(float64(reviewerCount) / 2.0)
		depthScore := clamp01(float64(maxDepth) / 3.0)
		supportScore := clamp01(float64(supportMentions) / 3.0)

		score := 0.4*uniqueScore + 0.3*reviewerScore + 0.2*depthScore + 0.1*supportScore
		threshold := 0.7

		status := "low"
		if score >= threshold {
			status = "ready"
		} else if score >= 0.4 {
			status = "partial"
		}

		consensusID := ""
		if pt.workflow != nil {
			for id, req := range pt.workflow.Consensus {
				if req.PostID == postID && req.Status != types.ConsensusClosed {
					consensusID = id
					break
				}
			}
		}

		rec := ""
		if status == "ready" {
			if consensusID != "" {
				rec = "Consensus ready. Consider create_draft with consensus_id."
			} else {
				rec = "Consensus ready. Consider request_consensus to formalize and draft."
			}
		} else if status == "partial" {
			rec = "Discussion forming. Invite reviewers or deepen thread."
		} else {
			rec = "Consensus weak. Encourage more discussion and reviewer feedback."
		}

		return AssessConsensusOutput{
			Score:     round2(score),
			Threshold: threshold,
			Status:    status,
			ConsensusID: consensusID,
			Counts: map[string]int{
				"unique_commenters": uniqueCount,
				"reviewer_commenters": reviewerCount,
				"support_mentions": supportMentions,
				"max_depth": maxDepth,
			},
			Signals: map[string]float64{
				"unique": uniqueScore,
				"reviewer": reviewerScore,
				"depth": depthScore,
				"support": supportScore,
			},
			Recommendation: rec,
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "assess_consensus",
		Description: "评估论坛线程的共识成熟度，用于决定是否进入协作草案。",
	}, handler)
}

// --- Review Paper Tool ---

type ReviewPaperInput struct {
	SubmissionID string            `json:"submission_id"`
	Scores       types.PaperReviewScores `json:"scores"`
	Verdict      string            `json:"verdict"`
	Comments     string            `json:"comments,omitempty"`
}

type ReviewPaperOutput struct {
	ReviewID string `json:"review_id"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

func (pt *PublicationToolset) ReviewPaperTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input ReviewPaperInput) (ReviewPaperOutput, error) {
		if pt.workflow == nil {
			return ReviewPaperOutput{}, fmt.Errorf("workflow not available")
		}
		if pt.journal == nil {
			return ReviewPaperOutput{}, fmt.Errorf("journal not available")
		}
		if pt.persona == nil || pt.persona.Role != types.RoleReviewer {
			return ReviewPaperOutput{}, fmt.Errorf("reviewer role required")
		}
		subID := strings.TrimSpace(input.SubmissionID)
		if subID == "" {
			return ReviewPaperOutput{}, fmt.Errorf("missing submission_id")
		}

		sub := pt.workflow.GetSubmission(subID)
		if sub == nil {
			pending := findPendingSubmission(pt.journal, subID)
			if pending == nil {
				return ReviewPaperOutput{}, fmt.Errorf("submission not found: %s", subID)
			}
			sub = &types.Submission{
				ID:         pending.ID,
				DraftID:    pending.DraftID,
				Title:      pending.Title,
				Abstract:   pending.Abstract,
				Content:    pending.Content,
				AuthorID:   pending.AuthorID,
				AuthorName: pending.AuthorName,
				Status:     types.SubmissionPending,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			pt.workflow.AddSubmission(sub)
		}

		verdict := parseVerdict(input.Verdict)
		if verdict == "" {
			return ReviewPaperOutput{}, fmt.Errorf("invalid verdict: %s", input.Verdict)
		}

		review := &types.PaperReview{
			SubmissionID: subID,
			ReviewerID:   pt.persona.ID,
			ReviewerName: pt.persona.Name,
			Scores:       input.Scores,
			Verdict:      verdict,
			Comments:     strings.TrimSpace(input.Comments),
			CreatedAt:    time.Now(),
		}

		reviewID := pt.workflow.AddReview(review)
		pt.workflow.AttachReview(subID, reviewID)

		status := string(sub.Status)
		switch verdict {
		case types.VerdictAccept:
			if err := pt.journal.Approve(subID, pt.persona.ID); err != nil {
				return ReviewPaperOutput{}, err
			}
			pt.workflow.UpdateSubmissionStatus(subID, types.SubmissionAccepted)
			status = string(types.SubmissionAccepted)
		case types.VerdictReject:
			if err := pt.journal.Reject(subID, pt.persona.ID); err != nil {
				return ReviewPaperOutput{}, err
			}
			pt.workflow.UpdateSubmissionStatus(subID, types.SubmissionRejected)
			status = string(types.SubmissionRejected)
		case types.VerdictMinorRevision:
			pt.workflow.UpdateSubmissionStatus(subID, types.SubmissionMinorRevision)
			status = string(types.SubmissionMinorRevision)
		case types.VerdictMajorRevision:
			pt.workflow.UpdateSubmissionStatus(subID, types.SubmissionMajorRevision)
			status = string(types.SubmissionMajorRevision)
		}

		if err := pt.workflow.Save(); err != nil {
			return ReviewPaperOutput{}, err
		}

		return ReviewPaperOutput{
			ReviewID: reviewID,
			Status:   status,
			Message:  "Review recorded",
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "review_paper",
		Description: "对投稿进行审稿（Reviewer 角色）。",
	}, handler)
}

// AllTools returns all publication tools.
func (pt *PublicationToolset) AllTools() ([]tool.Tool, error) {
	createDraft, err := pt.CreateDraftTool()
	if err != nil {
		return nil, err
	}
	requestConsensus, err := pt.RequestConsensusTool()
	if err != nil {
		return nil, err
	}
	assessReadiness, err := pt.AssessReadinessTool()
	if err != nil {
		return nil, err
	}
	assessConsensus, err := pt.AssessConsensusTool()
	if err != nil {
		return nil, err
	}
	submitPaper, err := pt.SubmitPaperTool()
	if err != nil {
		return nil, err
	}
	reviewPaper, err := pt.ReviewPaperTool()
	if err != nil {
		return nil, err
	}

	return []tool.Tool{
		assessReadiness,
		assessConsensus,
		createDraft,
		requestConsensus,
		submitPaper,
		reviewPaper,
	}, nil
}

func personaID(p *types.Persona) string {
	if p == nil {
		return ""
	}
	return p.ID
}

func personaName(p *types.Persona) string {
	if p == nil {
		return ""
	}
	return p.Name
}

func parseVerdict(value string) types.PaperReviewVerdict {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case string(types.VerdictAccept):
		return types.VerdictAccept
	case string(types.VerdictMinorRevision):
		return types.VerdictMinorRevision
	case string(types.VerdictMajorRevision):
		return types.VerdictMajorRevision
	case string(types.VerdictReject):
		return types.VerdictReject
	default:
		return ""
	}
}

func findPendingSubmission(journal *publication.Journal, id string) *types.Publication {
	if journal == nil {
		return nil
	}
	for _, p := range journal.GetPending() {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func normalizeMentions(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		trim := strings.TrimSpace(item)
		trim = strings.TrimPrefix(trim, "@")
		if trim == "" {
			continue
		}
		out = append(out, strings.ToLower(trim))
	}
	return uniqueStrings(out)
}

func nonEmptySlice(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

type dailyNoteEntry struct {
	Timestamp string `json:"timestamp"`
	Prompt    string `json:"prompt,omitempty"`
	Reply     string `json:"reply,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Raw       string `json:"raw,omitempty"`
}

func scanDailyNotes(dataPath, agentID string, windowStart time.Time) (int, int) {
	dir := filepath.Join(dataPath, "agents", agentID, "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	daySet := map[string]struct{}{}
	keywordHits := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var rec dailyNoteEntry
			if err := jsonUnmarshal(line, &rec); err != nil {
				continue
			}
			ts, err := time.Parse(time.RFC3339, rec.Timestamp)
			if err != nil || ts.Before(windowStart) {
				continue
			}
			daySet[ts.Format("2006-01-02")] = struct{}{}
			if hasIdeaSignal(rec.Prompt) || hasIdeaSignal(rec.Reply) || hasIdeaSignal(rec.Notes) || hasIdeaSignal(rec.Raw) {
				keywordHits++
			}
		}
		file.Close()
	}
	return len(daySet), keywordHits
}

func hasIdeaSignal(text string) bool {
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	signals := []string{
		"hypothesis", "conjecture", "theory", "experiment", "falsify", "evidence", "model",
		"假说", "猜想", "理论", "实验", "证伪", "证据", "模型",
	}
	for _, s := range signals {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func hasSupportSignal(text string) bool {
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	signals := []string{
		"support", "agree", "endorse", "consensus", "co-sign",
		"支持", "赞同", "同意", "共识", "赞成",
	}
	for _, s := range signals {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func looksLikeReviewer(id, name string) bool {
	candidates := []string{strings.ToLower(id), strings.ToLower(name)}
	for _, v := range candidates {
		if v == "" {
			continue
		}
		if strings.Contains(v, "reviewer") || strings.Contains(v, "review") {
			return true
		}
	}
	return false
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func jsonUnmarshal(line string, dst any) error {
	return json.Unmarshal([]byte(line), dst)
}
