// Package tools provides ADK-compatible tools for Sci-Bot agents.
package tools

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/cpunion/sci-bot/pkg/agent"
	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/types"
)

// ForumToolset provides tools for interacting with the forum.
type ForumToolset struct {
	forum   *publication.Forum
	agentID string
	persona *types.Persona
	state   *agent.AgentState
	rng     *rand.Rand
}

// NewForumToolset creates a new forum toolset for an agent.
func NewForumToolset(forum *publication.Forum, agentID string, persona *types.Persona, state *agent.AgentState) *ForumToolset {
	return &ForumToolset{
		forum:   forum,
		agentID: agentID,
		persona: persona,
		state:   state,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano() + hashSeed(agentID))),
	}
}

// --- Browse Forum Tool ---

// BrowseForumInput is the input for browsing the forum.
type BrowseForumInput struct {
	// Subreddit to browse (optional, empty means all)
	Subreddit string `json:"subreddit,omitempty"`
	// SortBy can be "hot" or "recent"
	SortBy string `json:"sort_by,omitempty"`
	// Limit number of posts to return
	Limit int `json:"limit,omitempty"`
}

// BrowseForumOutput is the output of browsing the forum.
type BrowseForumOutput struct {
	Posts []PostSummary `json:"posts"`
}

// PostSummary is a summary of a forum post.
type PostSummary struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	AuthorName string `json:"author_name"`
	Subreddit  string `json:"subreddit"`
	Score      int    `json:"score"`
	Comments   int    `json:"comments"`
	Abstract   string `json:"abstract,omitempty"`
}

// BrowseForumTool creates the browse forum tool.
func (ft *ForumToolset) BrowseForumTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input BrowseForumInput) (BrowseForumOutput, error) {
		limit := input.Limit
		if limit <= 0 || limit > 20 {
			limit = 10
		}

		posts := ft.personalizedFeed(input)
		if limit > len(posts) {
			limit = len(posts)
		}
		posts = posts[:limit]

		summaries := make([]PostSummary, 0, len(posts))
		for _, p := range posts {
			summaries = append(summaries, PostSummary{
				ID:         p.ID,
				Title:      p.Title,
				AuthorName: p.AuthorName,
				Subreddit:  string(p.Subreddit),
				Score:      p.Score,
				Comments:   p.Comments,
				Abstract:   p.Abstract,
			})
		}

		return BrowseForumOutput{Posts: summaries}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "browse_forum",
		Description: "浏览论坛帖子。会根据你的兴趣与关系个性化推荐，可按板块筛选。",
	}, handler)
}

// --- Read Post Tool ---

// ReadPostInput is the input for reading a post.
type ReadPostInput struct {
	PostID string `json:"post_id"`
}

// ReadPostOutput is the output of reading a post.
type ReadPostOutput struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Content    string           `json:"content"`
	AuthorName string           `json:"author_name"`
	AuthorID   string           `json:"author_id"`
	Subreddit  string           `json:"subreddit"`
	Score      int              `json:"score"`
	Comments   []CommentSummary `json:"comments"`
}

// CommentSummary is a summary of a comment.
type CommentSummary struct {
	ID         string `json:"id"`
	Content    string `json:"content"`
	AuthorName string `json:"author_name"`
	Score      int    `json:"score"`
	ParentID   string `json:"parent_id,omitempty"`
	Depth      int    `json:"depth"`
}

// ReadPostTool creates the read post tool.
func (ft *ForumToolset) ReadPostTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input ReadPostInput) (ReadPostOutput, error) {
		post := ft.forum.Get(input.PostID)
		if post == nil {
			return ReadPostOutput{}, fmt.Errorf("post not found: %s", input.PostID)
		}

		// Increment views
		ft.forum.IncrementViews(input.PostID)
		ft.recordInteraction(post)

		// Get threaded comments
		commentPubs := ft.forum.GetThreadComments(input.PostID)
		comments := make([]CommentSummary, 0, len(commentPubs))
		for _, c := range commentPubs {
			depth := computeCommentDepth(ft.forum, c, input.PostID)
			comments = append(comments, CommentSummary{
				ID:         c.ID,
				Content:    c.Content,
				AuthorName: c.AuthorName,
				Score:      c.Score,
				ParentID:   c.ParentID,
				Depth:      depth,
			})
		}

		return ReadPostOutput{
			ID:         post.ID,
			Title:      post.Title,
			Content:    post.Content,
			AuthorName: post.AuthorName,
			AuthorID:   post.AuthorID,
			Subreddit:  string(post.Subreddit),
			Score:      post.Score,
			Comments:   comments,
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "read_post",
		Description: "阅读帖子详情，包括完整内容和树形评论（含 parent_id 与 depth）。",
	}, handler)
}

// --- Create Post Tool ---

// CreatePostInput is the input for creating a post.
type CreatePostInput struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	Abstract  string `json:"abstract,omitempty"`
	Subreddit string `json:"subreddit"`
}

// CreatePostOutput is the output of creating a post.
type CreatePostOutput struct {
	PostID  string `json:"post_id"`
	Message string `json:"message"`
}

// CreatePostTool creates the create post tool.
func (ft *ForumToolset) CreatePostTool(agentName string) (tool.Tool, error) {
	handler := func(ctx tool.Context, input CreatePostInput) (CreatePostOutput, error) {
		sub := types.Subreddit(input.Subreddit)
		if sub == "" {
			sub = types.SubGeneral
		}

		pub := &types.Publication{
			AuthorID:   ft.agentID,
			AuthorName: agentName,
			Title:      input.Title,
			Content:    input.Content,
			Abstract:   input.Abstract,
			Subreddit:  sub,
		}

		if err := ft.forum.Post(pub); err != nil {
			return CreatePostOutput{}, err
		}

		return CreatePostOutput{
			PostID:  pub.ID,
			Message: fmt.Sprintf("帖子已发布到 r/%s", sub),
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "create_post",
		Description: "在论坛发布新帖子。需要指定标题、内容和板块。",
	}, handler)
}

// --- Vote Tool ---

// VoteInput is the input for voting.
type VoteInput struct {
	PostID string `json:"post_id"`
	// VoteType is "upvote" or "downvote"
	VoteType string `json:"vote_type"`
}

// VoteOutput is the output of voting.
type VoteOutput struct {
	NewScore int    `json:"new_score"`
	Message  string `json:"message"`
}

// VoteTool creates the vote tool.
func (ft *ForumToolset) VoteTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input VoteInput) (VoteOutput, error) {
		var err error
		if input.VoteType == "upvote" {
			err = ft.forum.Upvote(ft.agentID, input.PostID)
		} else if input.VoteType == "downvote" {
			err = ft.forum.Downvote(ft.agentID, input.PostID)
		} else {
			return VoteOutput{}, fmt.Errorf("invalid vote type: %s (use 'upvote' or 'downvote')", input.VoteType)
		}

		if err != nil {
			return VoteOutput{}, err
		}

		post := ft.forum.Get(input.PostID)
		ft.recordInteraction(post)
		return VoteOutput{
			NewScore: post.Score,
			Message:  fmt.Sprintf("已%s", input.VoteType),
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "vote",
		Description: "对帖子投票。可以顶（upvote）或踩（downvote）。",
	}, handler)
}

// --- Comment Tool ---

// CommentInput is the input for commenting.
type CommentInput struct {
	PostID   string `json:"post_id,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
	Content  string `json:"content"`
}

// CommentOutput is the output of commenting.
type CommentOutput struct {
	CommentID string `json:"comment_id"`
	Message   string `json:"message"`
}

// CommentTool creates the comment tool.
func (ft *ForumToolset) CommentTool(agentName string) (tool.Tool, error) {
	handler := func(ctx tool.Context, input CommentInput) (CommentOutput, error) {
		parentID := input.ParentID
		if parentID == "" {
			parentID = input.PostID
		}
		if parentID == "" {
			return CommentOutput{}, fmt.Errorf("missing parent_id or post_id")
		}
		comment := &types.Publication{
			AuthorID:   ft.agentID,
			AuthorName: agentName,
			Content:    input.Content,
		}

		if err := ft.forum.Comment(parentID, comment); err != nil {
			return CommentOutput{}, err
		}

		post := ft.forum.Get(input.PostID)
		ft.recordInteraction(post)

		return CommentOutput{
			CommentID: comment.ID,
			Message:   "评论已发布",
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "comment",
		Description: "对帖子或评论回复。可传 parent_id 指定要回复的评论，否则使用 post_id 回复顶层。",
	}, handler)
}

// AllTools returns all forum tools.
func (ft *ForumToolset) AllTools(agentName string) ([]tool.Tool, error) {
	browseTool, err := ft.BrowseForumTool()
	if err != nil {
		return nil, err
	}

	readTool, err := ft.ReadPostTool()
	if err != nil {
		return nil, err
	}

	createTool, err := ft.CreatePostTool(agentName)
	if err != nil {
		return nil, err
	}

	voteTool, err := ft.VoteTool()
	if err != nil {
		return nil, err
	}

	commentTool, err := ft.CommentTool(agentName)
	if err != nil {
		return nil, err
	}

	return []tool.Tool{
		browseTool,
		readTool,
		createTool,
		voteTool,
		commentTool,
	}, nil
}

func (ft *ForumToolset) personalizedFeed(input BrowseForumInput) []*types.Publication {
	var candidates []*types.Publication
	if input.Subreddit != "" {
		candidates = ft.forum.GetBySubreddit(types.Subreddit(input.Subreddit), 200)
	} else {
		candidates = ft.forum.AllPosts()
	}

	if len(candidates) == 0 {
		return candidates
	}

	scored := make([]scoredPost, 0, len(candidates))
	for _, post := range candidates {
		score := ft.scorePost(post, input.SortBy)
		scored = append(scored, scoredPost{post: post, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	posts := make([]*types.Publication, 0, len(scored))
	for _, sp := range scored {
		posts = append(posts, sp.post)
	}

	return posts
}

func computeCommentDepth(forum *publication.Forum, comment *types.Publication, rootID string) int {
	if comment == nil {
		return 0
	}
	depth := 0
	seen := map[string]struct{}{}
	parentID := comment.ParentID
	for parentID != "" && parentID != rootID {
		if _, ok := seen[parentID]; ok {
			break
		}
		seen[parentID] = struct{}{}
		parent := forum.Get(parentID)
		if parent == nil {
			break
		}
		depth++
		parentID = parent.ParentID
	}
	if parentID == rootID {
		depth++
	}
	if depth == 0 {
		return 1
	}
	return depth
}

type scoredPost struct {
	post  *types.Publication
	score float64
}

func (ft *ForumToolset) scorePost(post *types.Publication, sortBy string) float64 {
	base := float64(post.Score) * 0.6
	recency := recencyScore(post.PublishedAt)

	if sortBy == "recent" {
		base = base * 0.3
		recency *= 2.0
	}

	score := base + recency
	score += ft.domainScore(post.Subreddit)
	score += ft.relationshipScore(post.AuthorID)
	score += ft.noveltyScore(post)
	score += ft.randomness()

	return score
}

func (ft *ForumToolset) domainScore(sub types.Subreddit) float64 {
	if ft.persona == nil {
		return 0
	}
	interests := personaSubreddits(ft.persona)
	if interests[sub] {
		return 2.0
	}
	// Mild cross-domain bonus for highly creative agents
	if ft.persona.Creativity > 0.7 {
		return 0.3
	}
	return -0.1
}

func (ft *ForumToolset) relationshipScore(authorID string) float64 {
	if ft.state == nil {
		return 0
	}
	rel := ft.state.GetRelationship(authorID)
	if rel == nil {
		return 0
	}

	score := rel.TrustScore*1.2 + rel.Familiarity*0.8
	switch rel.State {
	case types.RelationTrusted:
		score += 0.6
	case types.RelationDiscussing:
		score += 0.2
	case types.RelationEstranged:
		score -= 0.2
	case types.RelationForgotten:
		score -= 0.4
	}
	return score
}

func (ft *ForumToolset) noveltyScore(post *types.Publication) float64 {
	if ft.persona == nil {
		return 0
	}
	if post.Score <= 1 {
		return 0.5 * ft.persona.RiskTolerance
	}
	return 0.2 * ft.persona.Creativity
}

func (ft *ForumToolset) randomness() float64 {
	if ft.rng == nil {
		return 0
	}
	amp := 0.2
	if ft.persona != nil {
		amp += ft.persona.Creativity * 0.3
	}
	return (ft.rng.Float64()*2 - 1) * amp
}

func (ft *ForumToolset) recordInteraction(post *types.Publication) {
	if ft.state == nil || post == nil || post.AuthorID == "" {
		return
	}
	topics := []string{}
	if post.Subreddit != "" {
		topics = append(topics, string(post.Subreddit))
	}
	ft.state.RecordInteraction(post.AuthorID, post.AuthorName, topics)
}

func recencyScore(t time.Time) float64 {
	ageHours := time.Since(t).Hours()
	return 1.0 / (1.0 + math.Max(ageHours, 0)/12.0)
}

func personaSubreddits(p *types.Persona) map[types.Subreddit]bool {
	result := make(map[types.Subreddit]bool)
	for _, domain := range p.Domains {
		d := strings.ToLower(domain)
		switch {
		case strings.Contains(d, "math") || strings.Contains(d, "geom"):
			result[types.SubMathematics] = true
		case strings.Contains(d, "physics") || strings.Contains(d, "astronomy"):
			result[types.SubPhysics] = true
		case strings.Contains(d, "philosophy") || strings.Contains(d, "method"):
			result[types.SubPhilosophy] = true
		case strings.Contains(d, "biology") || strings.Contains(d, "evolution"):
			result[types.SubBiology] = true
		case strings.Contains(d, "comput") || strings.Contains(d, "ai"):
			result[types.SubComputing] = true
		}
	}
	return result
}

func hashSeed(s string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum32())
}
