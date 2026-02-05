package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"iter"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"

	pkgagent "github.com/cpunion/sci-bot/pkg/agent"
	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/types"
)

var commentSeq int64

type mockModel struct {
	mu        sync.Mutex
	responses []*genai.Content
	requests  []*model.LLMRequest
}

func (m *mockModel) Name() string {
	return "mock"
}

func (m *mockModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	m.mu.Lock()
	m.requests = append(m.requests, req)
	m.mu.Unlock()

	return func(yield func(*model.LLMResponse, error) bool) {
		m.mu.Lock()
		if len(m.responses) == 0 {
			m.mu.Unlock()
			yield(nil, errors.New("no mock responses"))
			return
		}
		content := m.responses[0]
		m.responses = m.responses[1:]
		m.mu.Unlock()
		yield(&model.LLMResponse{Content: content}, nil)
	}
}

func TestThreadDigest_WithMockLLM(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	forum := publication.NewForum("Forum", filepath.Join(tempDir, "forum"))

	longBody := strings.Repeat("x", 2105)
	post := &types.Publication{
		AuthorID:   "author-1",
		AuthorName: "Author",
		Title:      "Long Post",
		Content:    longBody,
		Subreddit:  types.SubPhysics,
	}
	if err := forum.Post(post); err != nil {
		t.Fatalf("post failed: %v", err)
	}
	if err := forum.Comment(post.ID, &types.Publication{
		AuthorID:   "author-2",
		AuthorName: "Reply",
		Content:    "first reply",
	}); err != nil {
		t.Fatalf("comment failed: %v", err)
	}

	persona := &types.Persona{
		ID:   "agent-1",
		Name: "Agent One",
		Role: types.RoleExplorer,
	}
	state := pkgagent.NewAgentState(persona.ID, persona.Name, filepath.Join(tempDir, "agents", persona.ID))
	toolset := NewForumToolset(forum, persona.ID, persona, state)

	digestTool, err := toolset.ThreadDigestTool()
	if err != nil {
		t.Fatalf("digest tool: %v", err)
	}
	saveTool, err := toolset.SaveThreadSummaryTool()
	if err != nil {
		t.Fatalf("save tool: %v", err)
	}

	model := &mockModel{
		responses: []*genai.Content{
			genai.NewContentFromFunctionCall(
				"get_thread_digest",
				map[string]any{"post_id": post.ID},
				"model",
			),
			genai.NewContentFromFunctionCall(
				"save_thread_summary",
				map[string]any{
					"post_id": post.ID,
					"summary": "cached summary",
				},
				"model",
			),
			genai.NewContentFromText("done", "model"),
		},
	}

	adkAgent, err := llmagent.New(llmagent.Config{
		Name:                     "agent-1",
		Model:                    model,
		Instruction:              "Use tools to digest and save summaries.",
		Tools:                    []tool.Tool{digestTool, saveTool},
		DisallowTransferToParent: true,
		DisallowTransferToPeers:  true,
	})
	if err != nil {
		t.Fatalf("agent init failed: %v", err)
	}

	sessionService := session.InMemoryService()
	appName := "test-app"
	if _, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName:   appName,
		UserID:    "user",
		SessionID: "session",
	}); err != nil {
		t.Fatalf("session create failed: %v", err)
	}

	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          adkAgent,
		SessionService: sessionService,
	})
	if err != nil {
		t.Fatalf("runner init failed: %v", err)
	}

	stream := r.Run(ctx, "user", "session", genai.NewContentFromText("start", "user"), agent.RunConfig{})
	var digestResponse map[string]any
	for ev, err := range stream {
		if err != nil {
			t.Fatalf("run error: %v", err)
		}
		if ev == nil || ev.LLMResponse.Content == nil {
			continue
		}
		for _, part := range ev.LLMResponse.Content.Parts {
			if part.FunctionResponse != nil && part.FunctionResponse.Name == "get_thread_digest" {
				digestResponse = part.FunctionResponse.Response
			}
		}
	}

	if digestResponse == nil {
		t.Fatalf("missing get_thread_digest response")
	}
	if needsSummary, ok := digestResponse["needs_summary"].(bool); !ok || !needsSummary {
		t.Fatalf("expected needs_summary=true, got %v", digestResponse["needs_summary"])
	}

	summary := forum.GetThreadSummary(post.ID)
	if summary == nil {
		t.Fatalf("expected cached summary to be saved")
	}
	if summary.Summary != "cached summary" {
		t.Fatalf("unexpected cached summary: %q", summary.Summary)
	}
}

func TestThreadDigest_TooManyNewComments(t *testing.T) {
	ctx := context.Background()
	forum, post, summary, digestTool := setupForumWithSummary(t)

	for i := 0; i < 12; i++ {
		commentID := addComment(t, forum, post.ID, "bulk", "new comment")
		ensureAfterSummary(forum, commentID, summary.LastCommentAt)
	}

	resp := callToolResponse(t, ctx, digestTool, "get_thread_digest", map[string]any{
		"post_id":          post.ID,
		"max_new_comments": 5,
	})
	digest := decodeDigest(t, resp)
	if !digest.NeedsSummary {
		t.Fatalf("expected needs_summary (got reason=%s, truncated=%v, new_comments=%d)", digest.ResummaryReason, digest.Truncated, len(digest.NewComments))
	}
	if digest.ResummaryReason != "too_many_new_comments" {
		t.Fatalf("unexpected reason: %s", digest.ResummaryReason)
	}
	if !digest.Truncated {
		t.Fatalf("expected truncated=true")
	}
	if len(digest.NewComments) != 5 {
		t.Fatalf("expected 5 new comments, got %d", len(digest.NewComments))
	}
}

func TestThreadDigest_NewCommentTooLong(t *testing.T) {
	ctx := context.Background()
	forum, post, summary, digestTool := setupForumWithSummary(t)

	longCommentID := addComment(t, forum, post.ID, "long", strings.Repeat("l", newCommentContentLimit+50))
	ensureAfterSummary(forum, longCommentID, summary.LastCommentAt)

	resp := callToolResponse(t, ctx, digestTool, "get_thread_digest", map[string]any{
		"post_id": post.ID,
	})
	digest := decodeDigest(t, resp)
	if !digest.NeedsSummary {
		t.Fatalf("expected needs_summary")
	}
	if digest.ResummaryReason != "new_comment_too_long" {
		t.Fatalf("unexpected reason: %s", digest.ResummaryReason)
	}
	if len(digest.NewComments) == 0 {
		t.Fatalf("expected new comments")
	}
	if !strings.HasSuffix(digest.NewComments[len(digest.NewComments)-1].Content, "...") {
		t.Fatalf("expected truncated new comment")
	}
}

func TestThreadDigest_ReplyToLargeParent(t *testing.T) {
	ctx := context.Background()
	forum, post, summary, digestTool := setupForumWithSummary(t)

	parentID := addComment(t, forum, post.ID, "parent", strings.Repeat("p", parentExcerptLimit+10))
	ensureAfterSummary(forum, parentID, summary.LastCommentAt)
	replyID := addReply(t, forum, post.ID, parentID, "reply", "reply text")
	ensureAfterSummary(forum, replyID, summary.LastCommentAt.Add(2*time.Second))

	resp := callToolResponse(t, ctx, digestTool, "get_thread_digest", map[string]any{
		"post_id": post.ID,
	})
	digest := decodeDigest(t, resp)
	if !digest.NeedsSummary {
		t.Fatalf("expected needs_summary")
	}
	if digest.ResummaryReason != "reply_to_large_parent" {
		t.Fatalf("unexpected reason: %s", digest.ResummaryReason)
	}
	found := false
	for _, c := range digest.NewComments {
		if c.ID == replyID {
			found = true
			if c.ParentID != parentID {
				t.Fatalf("expected parent_id %s, got %s", parentID, c.ParentID)
			}
			if !c.ParentOversize {
				t.Fatalf("expected parent_oversize")
			}
			if c.ParentExcerpt == "" {
				t.Fatalf("expected parent excerpt")
			}
			if c.Depth < 2 {
				t.Fatalf("expected depth >= 2, got %d", c.Depth)
			}
		}
	}
	if !found {
		t.Fatalf("reply not found in new comments")
	}
}

func TestThreadDigest_PostContentChanged(t *testing.T) {
	ctx := context.Background()
	forum, post, _, digestTool := setupForumWithSummary(t)
	post.Content += " changed"
	_ = forum

	resp := callToolResponse(t, ctx, digestTool, "get_thread_digest", map[string]any{
		"post_id": post.ID,
	})
	digest := decodeDigest(t, resp)
	if !digest.NeedsSummary {
		t.Fatalf("expected needs_summary")
	}
	if digest.ResummaryReason != "post_content_changed" {
		t.Fatalf("unexpected reason: %s", digest.ResummaryReason)
	}
}

func addComment(t *testing.T, forum *publication.Forum, postID, author, content string) string {
	t.Helper()
	comment := &types.Publication{
		ID:         fmt.Sprintf("comment-test-%d", atomic.AddInt64(&commentSeq, 1)),
		AuthorID:   author,
		AuthorName: author,
		Content:    content,
	}
	if err := forum.Comment(postID, comment); err != nil {
		t.Fatalf("comment failed: %v", err)
	}
	return comment.ID
}

func addReply(t *testing.T, forum *publication.Forum, postID, parentID, author, content string) string {
	t.Helper()
	comment := &types.Publication{
		ID:         fmt.Sprintf("comment-test-%d", atomic.AddInt64(&commentSeq, 1)),
		AuthorID:   author,
		AuthorName: author,
		Content:    content,
	}
	if err := forum.Comment(parentID, comment); err != nil {
		t.Fatalf("reply failed: %v", err)
	}
	return comment.ID
}

func ensureAfterSummary(forum *publication.Forum, commentID string, after time.Time) {
	comment := forum.Get(commentID)
	if comment == nil {
		return
	}
	if comment.PublishedAt.After(after) {
		return
	}
	comment.PublishedAt = after.Add(time.Second)
}

func setupForumWithSummary(t *testing.T) (*publication.Forum, *types.Publication, *types.ThreadSummary, tool.Tool) {
	t.Helper()
	tempDir := t.TempDir()
	forum := publication.NewForum("Forum", filepath.Join(tempDir, "forum"))
	post := &types.Publication{
		AuthorID:   "author-1",
		AuthorName: "Author",
		Title:      "Short Post",
		Content:    "short content",
		Subreddit:  types.SubPhysics,
	}
	if err := forum.Post(post); err != nil {
		t.Fatalf("post failed: %v", err)
	}
	addComment(t, forum, post.ID, "seed", "seed comment")
	if _, err := forum.SaveThreadSummary(post.ID, "seed summary"); err != nil {
		t.Fatalf("save summary failed: %v", err)
	}
	summary := forum.GetThreadSummary(post.ID)
	if summary == nil {
		t.Fatalf("missing summary")
	}

	persona := &types.Persona{ID: "agent-1", Name: "Agent One", Role: types.RoleExplorer}
	state := pkgagent.NewAgentState(persona.ID, persona.Name, filepath.Join(tempDir, "agents", persona.ID))
	toolset := NewForumToolset(forum, persona.ID, persona, state)
	digestTool, err := toolset.ThreadDigestTool()
	if err != nil {
		t.Fatalf("digest tool: %v", err)
	}
	return forum, post, summary, digestTool
}

func callToolResponse(t *testing.T, ctx context.Context, callTool tool.Tool, toolName string, args map[string]any) map[string]any {
	t.Helper()
	model := &mockModel{
		responses: []*genai.Content{
			genai.NewContentFromFunctionCall(toolName, args, "model"),
			genai.NewContentFromText("done", "model"),
		},
	}

	adkAgent, err := llmagent.New(llmagent.Config{
		Name:                     "agent-1",
		Model:                    model,
		Instruction:              "Call the tool.",
		Tools:                    []tool.Tool{callTool},
		DisallowTransferToParent: true,
		DisallowTransferToPeers:  true,
	})
	if err != nil {
		t.Fatalf("agent init failed: %v", err)
	}

	sessionService := session.InMemoryService()
	appName := "test-app"
	if _, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName:   appName,
		UserID:    "user",
		SessionID: "session",
	}); err != nil {
		t.Fatalf("session create failed: %v", err)
	}

	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          adkAgent,
		SessionService: sessionService,
	})
	if err != nil {
		t.Fatalf("runner init failed: %v", err)
	}

	stream := r.Run(ctx, "user", "session", genai.NewContentFromText("start", "user"), agent.RunConfig{})
	for ev, err := range stream {
		if err != nil {
			t.Fatalf("run error: %v", err)
		}
		if ev == nil || ev.LLMResponse.Content == nil {
			continue
		}
		for _, part := range ev.LLMResponse.Content.Parts {
			if part.FunctionResponse != nil && part.FunctionResponse.Name == toolName {
				return part.FunctionResponse.Response
			}
		}
	}
	return nil
}

func decodeDigest(t *testing.T, resp map[string]any) ThreadDigestOutput {
	t.Helper()
	var out ThreadDigestOutput
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return out
}
