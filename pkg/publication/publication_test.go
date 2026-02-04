package publication

import (
	"testing"

	"github.com/cpunion/sci-bot/pkg/types"
)

func TestJournal_SubmitAndApprove(t *testing.T) {
	j := NewJournal("Science", t.TempDir())

	pub := &types.Publication{
		AuthorID:   "agent-1",
		AuthorName: "Galileo",
		Title:      "On the Motion of Bodies",
		Abstract:   "A study of motion",
		Content:    "Full content here...",
	}

	// Submit
	if err := j.Submit(pub); err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	pending := j.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}

	approved := j.GetApproved()
	if len(approved) != 0 {
		t.Fatalf("expected 0 approved, got %d", len(approved))
	}

	// Approve
	if err := j.Approve(pub.ID, "agent-2"); err != nil {
		t.Fatalf("approve failed: %v", err)
	}

	pending = j.GetPending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after approval, got %d", len(pending))
	}

	approved = j.GetApproved()
	if len(approved) != 1 {
		t.Fatalf("expected 1 approved, got %d", len(approved))
	}

	if !approved[0].Approved {
		t.Error("expected publication to be marked approved")
	}
	if len(approved[0].Reviewers) != 1 {
		t.Errorf("expected 1 reviewer, got %d", len(approved[0].Reviewers))
	}
}

func TestJournal_Reject(t *testing.T) {
	j := NewJournal("Science", t.TempDir())

	pub := &types.Publication{
		AuthorID: "agent-1",
		Title:    "Bad Theory",
	}

	j.Submit(pub)

	if err := j.Reject(pub.ID, "agent-2"); err != nil {
		t.Fatalf("reject failed: %v", err)
	}

	pending := j.GetPending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after rejection, got %d", len(pending))
	}

	approved := j.GetApproved()
	if len(approved) != 0 {
		t.Fatalf("expected 0 approved, got %d", len(approved))
	}
}

func TestForum_Post(t *testing.T) {
	f := NewForum("Open Discussion", t.TempDir())

	pub := &types.Publication{
		AuthorID:   "agent-1",
		AuthorName: "Galileo",
		Title:      "My crazy idea",
		Content:    "What if...",
	}

	if err := f.Post(pub); err != nil {
		t.Fatalf("post failed: %v", err)
	}

	posts := f.GetRecent(10)
	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}

	if !posts[0].Approved {
		t.Error("forum posts should be auto-approved")
	}
	if posts[0].Channel != types.ChannelForum {
		t.Errorf("expected channel forum, got %s", posts[0].Channel)
	}
}

func TestForum_GetByAuthor(t *testing.T) {
	f := NewForum("Open Discussion", t.TempDir())

	f.Post(&types.Publication{ID: "post-a1", AuthorID: "agent-1", Title: "Post 1"})
	f.Post(&types.Publication{ID: "post-a2", AuthorID: "agent-2", Title: "Post 2"})
	f.Post(&types.Publication{ID: "post-a3", AuthorID: "agent-1", Title: "Post 3"})

	posts := f.GetByAuthor("agent-1")
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts by agent-1, got %d", len(posts))
	}
}

func TestForum_ViewsAndComments(t *testing.T) {
	f := NewForum("Open Discussion", t.TempDir())

	pub := &types.Publication{ID: "post-views", AuthorID: "agent-1", Title: "Popular post"}
	f.Post(pub)

	f.IncrementViews(pub.ID)
	f.IncrementViews(pub.ID)

	// Add a comment
	comment := &types.Publication{ID: "comment-1", AuthorID: "agent-2", Title: "Great idea!", Content: "I agree"}
	if err := f.Comment(pub.ID, comment); err != nil {
		t.Fatalf("comment failed: %v", err)
	}

	post := f.Get(pub.ID)
	if post.Views != 2 {
		t.Errorf("expected 2 views, got %d", post.Views)
	}
	if post.Comments != 1 {
		t.Errorf("expected 1 comment, got %d", post.Comments)
	}

	comments := f.GetComments(pub.ID)
	if len(comments) != 1 {
		t.Errorf("expected 1 comment, got %d", len(comments))
	}
}

func TestForum_Voting(t *testing.T) {
	f := NewForum("Open Discussion", t.TempDir())

	pub := &types.Publication{ID: "post-vote", AuthorID: "agent-1", Title: "Controversial"}
	f.Post(pub)

	// Initial: author's implicit upvote (Upvotes=1, Score=1)
	post := f.Get(pub.ID)
	if post.Upvotes != 1 || post.Score != 1 {
		t.Errorf("expected initial upvotes=1 score=1, got upvotes=%d score=%d", post.Upvotes, post.Score)
	}

	// Another agent upvotes
	f.Upvote("agent-2", pub.ID)
	post = f.Get(pub.ID)
	if post.Upvotes != 2 || post.Score != 2 {
		t.Errorf("after upvote: expected upvotes=2 score=2, got upvotes=%d score=%d", post.Upvotes, post.Score)
	}

	// Downvote from another agent
	f.Downvote("agent-3", pub.ID)
	post = f.Get(pub.ID)
	if post.Downvotes != 1 || post.Score != 1 {
		t.Errorf("after downvote: expected downvotes=1 score=1, got downvotes=%d score=%d", post.Downvotes, post.Score)
	}

	// Same agent (agent-2) upvotes again (removes their upvote)
	f.Upvote("agent-2", pub.ID)
	post = f.Get(pub.ID)
	if post.Upvotes != 1 || post.Score != 0 {
		t.Errorf("after removing upvote: expected upvotes=1 score=0, got upvotes=%d score=%d", post.Upvotes, post.Score)
	}

	// agent-3 changes vote from downvote to upvote
	f.Upvote("agent-3", pub.ID)
	post = f.Get(pub.ID)
	if post.Upvotes != 2 || post.Downvotes != 0 || post.Score != 2 {
		t.Errorf("after changing vote: expected upvotes=2 downvotes=0 score=2, got upvotes=%d downvotes=%d score=%d",
			post.Upvotes, post.Downvotes, post.Score)
	}
}

func TestForum_Subreddits(t *testing.T) {
	f := NewForum("Open Discussion", t.TempDir())

	f.Post(&types.Publication{ID: "p1", AuthorID: "a1", Title: "Math 1", Subreddit: types.SubMathematics})
	f.Post(&types.Publication{ID: "p2", AuthorID: "a2", Title: "Physics 1", Subreddit: types.SubPhysics})
	f.Post(&types.Publication{ID: "p3", AuthorID: "a3", Title: "Math 2", Subreddit: types.SubMathematics})

	mathPosts := f.GetBySubreddit(types.SubMathematics, 10)
	if len(mathPosts) != 2 {
		t.Errorf("expected 2 math posts, got %d", len(mathPosts))
	}

	physicsPosts := f.GetBySubreddit(types.SubPhysics, 10)
	if len(physicsPosts) != 1 {
		t.Errorf("expected 1 physics post, got %d", len(physicsPosts))
	}

	stats := f.GetSubredditStats()
	if stats[types.SubMathematics] != 2 {
		t.Errorf("expected 2 in math subreddit, got %d", stats[types.SubMathematics])
	}
}

func TestForum_HotPosts(t *testing.T) {
	f := NewForum("Open Discussion", t.TempDir())

	f.Post(&types.Publication{ID: "p1", AuthorID: "a1", Title: "Normal"})
	f.Post(&types.Publication{ID: "p2", AuthorID: "a2", Title: "Popular"})
	f.Post(&types.Publication{ID: "p3", AuthorID: "a3", Title: "Unpopular"})

	// Make p2 popular
	f.Upvote("v1", "p2")
	f.Upvote("v2", "p2")
	f.Upvote("v3", "p2")

	// Make p3 unpopular
	f.Downvote("v1", "p3")
	f.Downvote("v2", "p3")

	hot := f.GetHot(10)
	if len(hot) < 3 {
		t.Fatalf("expected 3 posts, got %d", len(hot))
	}

	// Popular should be first
	if hot[0].ID != "p2" {
		t.Errorf("expected p2 (Popular) first, got %s", hot[0].ID)
	}
	// Unpopular should be last
	if hot[2].ID != "p3" {
		t.Errorf("expected p3 (Unpopular) last, got %s", hot[2].ID)
	}
}

func TestJournal_Persistence(t *testing.T) {
	tempDir := t.TempDir()
	j := NewJournal("Science", tempDir)

	pub := &types.Publication{AuthorID: "agent-1", Title: "Theory"}
	j.Submit(pub)
	j.Approve(pub.ID, "agent-2")

	if err := j.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Load into new journal
	j2 := NewJournal("Science", tempDir)
	if err := j2.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(j2.Publications) != 1 {
		t.Errorf("expected 1 publication after load, got %d", len(j2.Publications))
	}
}

func TestForum_Persistence(t *testing.T) {
	tempDir := t.TempDir()
	f := NewForum("Discussion", tempDir)

	f.Post(&types.Publication{ID: "post-1", AuthorID: "agent-1", Title: "Post 1"})
	f.Post(&types.Publication{ID: "post-2", AuthorID: "agent-2", Title: "Post 2"})

	if err := f.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	f2 := NewForum("Discussion", tempDir)
	if err := f2.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(f2.Posts) != 2 {
		t.Errorf("expected 2 posts after load, got %d", len(f2.Posts))
	}
}
