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

	f.Post(&types.Publication{AuthorID: "agent-1", Title: "Post 1"})
	f.Post(&types.Publication{AuthorID: "agent-2", Title: "Post 2"})
	f.Post(&types.Publication{AuthorID: "agent-1", Title: "Post 3"})

	posts := f.GetByAuthor("agent-1")
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts by agent-1, got %d", len(posts))
	}
}

func TestForum_ViewsAndReplies(t *testing.T) {
	f := NewForum("Open Discussion", t.TempDir())

	pub := &types.Publication{AuthorID: "agent-1", Title: "Popular post"}
	f.Post(pub)

	f.IncrementViews(pub.ID)
	f.IncrementViews(pub.ID)
	f.Reply(pub.ID)

	post := f.Get(pub.ID)
	if post.Views != 2 {
		t.Errorf("expected 2 views, got %d", post.Views)
	}
	if post.Replies != 1 {
		t.Errorf("expected 1 reply, got %d", post.Replies)
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
