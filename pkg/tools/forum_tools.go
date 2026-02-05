// Package tools provides ADK-compatible tools for Sci-Bot agents.
package tools

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"regexp"
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
	Mentioned  bool   `json:"mentioned,omitempty"`
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
				Mentioned:  ft.postMentionsAgent(p),
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

// --- Mentions Tool ---

// BrowseMentionsInput is the input for browsing mentions.
type BrowseMentionsInput struct {
	// Limit number of mentions to return
	Limit int `json:"limit,omitempty"`
}

// MentionSummary is a summary of a mention.
type MentionSummary struct {
	PostID      string `json:"post_id"`
	CommentID   string `json:"comment_id,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
	AuthorName  string `json:"author_name"`
	Subreddit   string `json:"subreddit"`
	Excerpt     string `json:"excerpt"`
	IsComment   bool   `json:"is_comment"`
	PublishedAt string `json:"published_at"`
	Reason      string `json:"reason"`
}

// BrowseMentionsOutput is the output of browsing mentions.
type BrowseMentionsOutput struct {
	Mentions []MentionSummary `json:"mentions"`
}

// BrowseMentionsTool creates the mentions tool.
func (ft *ForumToolset) BrowseMentionsTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input BrowseMentionsInput) (BrowseMentionsOutput, error) {
		if ft.forum == nil {
			return BrowseMentionsOutput{}, nil
		}
		limit := input.Limit
		if limit <= 0 || limit > 30 {
			limit = 10
		}

		items := make([]mentionItem, 0)
		for _, pub := range ft.forum.AllPublications() {
			if pub == nil {
				continue
			}
			reason := ft.mentionReason(pub)
			if reason == "" {
				continue
			}
			items = append(items, mentionItem{
				pub:    pub,
				reason: reason,
			})
		}

		sort.Slice(items, func(i, j int) bool {
			return items[i].pub.PublishedAt.After(items[j].pub.PublishedAt)
		})

		if limit > len(items) {
			limit = len(items)
		}

		out := make([]MentionSummary, 0, limit)
		for _, item := range items[:limit] {
			pub := item.pub
			subreddit := string(pub.Subreddit)
			if subreddit == "" {
				subreddit = string(types.SubGeneral)
			}
			excerpt := pub.Content
			if len(excerpt) > 180 {
				excerpt = excerpt[:180] + "..."
			}
			out = append(out, MentionSummary{
				PostID:      ft.rootPostID(pub),
				CommentID:   ft.commentID(pub),
				ParentID:    pub.ParentID,
				AuthorName:  pub.AuthorName,
				Subreddit:   subreddit,
				Excerpt:     strings.TrimSpace(excerpt),
				IsComment:   pub.IsComment,
				PublishedAt: pub.PublishedAt.Format(time.RFC3339),
				Reason:      item.reason,
			})
		}

		return BrowseMentionsOutput{Mentions: out}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "browse_mentions",
		Description: "查看与你相关的 @ 提及或回复，优先处理这些内容。",
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
			Mentions:   extractMentions(input.Title + "\n" + input.Abstract + "\n" + input.Content),
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
			Mentions:   extractMentions(input.Content),
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

	mentionTool, err := ft.BrowseMentionsTool()
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
		mentionTool,
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
	score += ft.mentionBoost(post)
	score += ft.randomness()

	return score
}

type mentionItem struct {
	pub    *types.Publication
	reason string
}

func (ft *ForumToolset) mentionBoost(post *types.Publication) float64 {
	if post == nil || ft.forum == nil {
		return 0
	}
	if ft.publicationMentionsAgent(post) {
		return 5.0
	}
	comments := ft.forum.GetThreadComments(post.ID)
	for _, c := range comments {
		if ft.publicationMentionsAgent(c) {
			return 4.0
		}
		if ft.replyTargetsAgent(c) {
			return 2.5
		}
	}
	return 0
}

func (ft *ForumToolset) postMentionsAgent(post *types.Publication) bool {
	if post == nil || ft.forum == nil {
		return false
	}
	if ft.publicationMentionsAgent(post) {
		return true
	}
	comments := ft.forum.GetThreadComments(post.ID)
	for _, c := range comments {
		if ft.publicationMentionsAgent(c) || ft.replyTargetsAgent(c) {
			return true
		}
	}
	return false
}

func (ft *ForumToolset) mentionReason(pub *types.Publication) string {
	if pub == nil {
		return ""
	}
	if ft.publicationMentionsAgent(pub) {
		return "mention"
	}
	if pub.IsComment && ft.replyTargetsAgent(pub) {
		return "reply"
	}
	return ""
}

func (ft *ForumToolset) replyTargetsAgent(pub *types.Publication) bool {
	if pub == nil || pub.ParentID == "" || ft.forum == nil {
		return false
	}
	parent := ft.forum.Get(pub.ParentID)
	if parent == nil {
		return false
	}
	return parent.AuthorID == ft.agentID
}

func (ft *ForumToolset) publicationMentionsAgent(pub *types.Publication) bool {
	if pub == nil {
		return false
	}
	keys := ft.mentionKeys()
	if len(keys) == 0 {
		return false
	}
	keySet := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		if k == "" {
			continue
		}
		keySet[strings.ToLower(k)] = struct{}{}
	}

	mentions := pub.Mentions
	if len(mentions) == 0 {
		mentions = extractMentions(pub.Title + "\n" + pub.Abstract + "\n" + pub.Content)
	}
	for _, m := range mentions {
		if _, ok := keySet[strings.ToLower(m)]; ok {
			return true
		}
	}
	return false
}

func (ft *ForumToolset) mentionKeys() []string {
	keys := make([]string, 0, 3)
	if ft.agentID != "" {
		keys = append(keys, strings.ToLower(ft.agentID))
	}
	if ft.persona != nil {
		name := strings.TrimSpace(ft.persona.Name)
		if name != "" {
			keys = append(keys, strings.ToLower(name))
			keys = append(keys, strings.ToLower(strings.ReplaceAll(name, " ", "")))
		}
	}
	return uniqueStrings(keys)
}

func (ft *ForumToolset) rootPostID(pub *types.Publication) string {
	if pub == nil || ft.forum == nil {
		return ""
	}
	if !pub.IsComment {
		return pub.ID
	}
	parentID := pub.ParentID
	seen := map[string]struct{}{}
	for parentID != "" {
		if _, ok := seen[parentID]; ok {
			break
		}
		seen[parentID] = struct{}{}
		parent := ft.forum.Get(parentID)
		if parent == nil {
			break
		}
		if !parent.IsComment {
			return parent.ID
		}
		parentID = parent.ParentID
	}
	return pub.ParentID
}

func (ft *ForumToolset) commentID(pub *types.Publication) string {
	if pub == nil {
		return ""
	}
	if pub.IsComment {
		return pub.ID
	}
	return ""
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

var mentionPattern = regexp.MustCompile(`(?i)@([a-z0-9][a-z0-9_./-]{0,63})`)

func extractMentions(text string) []string {
	if text == "" {
		return nil
	}
	matches := mentionPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		token := strings.TrimSpace(m[1])
		if token == "" {
			continue
		}
		out = append(out, strings.ToLower(token))
	}
	return uniqueStrings(out)
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
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
