// Package publication implements Journal and Forum publication channels.
package publication

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cpunion/sci-bot/pkg/types"
)

// Journal represents an authoritative publication channel requiring review.
type Journal struct {
	mu sync.RWMutex

	Name         string                        `json:"name"`
	Publications map[string]*types.Publication `json:"publications"`
	Pending      map[string]*types.Publication `json:"pending"` // Awaiting review
	dataPath     string
}

// NewJournal creates a new journal.
func NewJournal(name, dataPath string) *Journal {
	return &Journal{
		Name:         name,
		Publications: make(map[string]*types.Publication),
		Pending:      make(map[string]*types.Publication),
		dataPath:     dataPath,
	}
}

// Submit submits a publication for review.
func (j *Journal) Submit(pub *types.Publication) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if pub.ID == "" {
		pub.ID = fmt.Sprintf("journal-%d", time.Now().UnixNano())
	}
	pub.Channel = types.ChannelJournal
	pub.Approved = false
	j.Pending[pub.ID] = pub

	return nil
}

// GetPending returns publications awaiting review.
func (j *Journal) GetPending() []*types.Publication {
	j.mu.RLock()
	defer j.mu.RUnlock()

	result := make([]*types.Publication, 0, len(j.Pending))
	for _, pub := range j.Pending {
		result = append(result, pub)
	}
	return result
}

// Approve approves a pending publication.
func (j *Journal) Approve(pubID, reviewerID string) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	pub, ok := j.Pending[pubID]
	if !ok {
		return fmt.Errorf("publication not found in pending: %s", pubID)
	}

	pub.Approved = true
	pub.Reviewers = append(pub.Reviewers, reviewerID)
	pub.PublishedAt = time.Now()

	j.Publications[pubID] = pub
	delete(j.Pending, pubID)

	return nil
}

// Reject rejects a pending publication.
func (j *Journal) Reject(pubID, reviewerID string) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	_, ok := j.Pending[pubID]
	if !ok {
		return fmt.Errorf("publication not found in pending: %s", pubID)
	}

	delete(j.Pending, pubID)
	return nil
}

// GetApproved returns all approved publications.
func (j *Journal) GetApproved() []*types.Publication {
	j.mu.RLock()
	defer j.mu.RUnlock()

	result := make([]*types.Publication, 0, len(j.Publications))
	for _, pub := range j.Publications {
		if pub.Approved {
			result = append(result, pub)
		}
	}
	return result
}

// Get returns a specific publication.
func (j *Journal) Get(pubID string) *types.Publication {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.Publications[pubID]
}

// IncrementViews increments view count.
func (j *Journal) IncrementViews(pubID string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if pub, ok := j.Publications[pubID]; ok {
		pub.Views++
	}
}

// Save persists the journal to disk.
func (j *Journal) Save() error {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if err := os.MkdirAll(j.dataPath, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(j.dataPath, "journal.json"), data, 0644)
}

// Load loads the journal from disk.
func (j *Journal) Load() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(j.dataPath, "journal.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, j)
}

// Forum represents a grassroots discussion channel (Reddit-style).
type Forum struct {
	mu sync.RWMutex

	Name     string                        `json:"name"`
	Posts    map[string]*types.Publication `json:"posts"`
	Votes    map[string]*types.Vote        `json:"votes"` // key: "voterID:postID"
	dataPath string
}

// NewForum creates a new forum.
func NewForum(name, dataPath string) *Forum {
	return &Forum{
		Name:     name,
		Posts:    make(map[string]*types.Publication),
		Votes:    make(map[string]*types.Vote),
		dataPath: dataPath,
	}
}

// Post publishes a post without review.
func (f *Forum) Post(pub *types.Publication) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if pub.ID == "" {
		pub.ID = fmt.Sprintf("forum-%d", time.Now().UnixNano())
	}
	pub.Channel = types.ChannelForum
	pub.PublishedAt = time.Now()
	pub.Approved = true // Forum posts don't need approval
	pub.Upvotes = 1     // Author's implicit upvote
	pub.Score = 1       // Author's implicit upvote

	if pub.Subreddit == "" {
		pub.Subreddit = types.SubGeneral
	}

	f.Posts[pub.ID] = pub
	return nil
}

// Comment adds a comment to a post.
func (f *Forum) Comment(parentID string, comment *types.Publication) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	parent, ok := f.Posts[parentID]
	if !ok {
		return fmt.Errorf("parent post not found: %s", parentID)
	}

	if comment.ID == "" {
		comment.ID = fmt.Sprintf("comment-%d", time.Now().UnixNano())
	}
	comment.Channel = types.ChannelForum
	comment.PublishedAt = time.Now()
	comment.Approved = true
	comment.ParentID = parentID
	comment.IsComment = true
	comment.Subreddit = parent.Subreddit
	comment.Score = 1

	f.Posts[comment.ID] = comment
	parent.Comments++

	return nil
}

// Upvote upvotes a post.
func (f *Forum) Upvote(voterID, postID string) error {
	return f.vote(voterID, postID, true)
}

// Downvote downvotes a post.
func (f *Forum) Downvote(voterID, postID string) error {
	return f.vote(voterID, postID, false)
}

// vote handles voting logic.
func (f *Forum) vote(voterID, postID string, isUpvote bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	post, ok := f.Posts[postID]
	if !ok {
		return fmt.Errorf("post not found: %s", postID)
	}

	voteKey := voterID + ":" + postID
	existingVote, hasVoted := f.Votes[voteKey]

	if hasVoted {
		// Already voted - update or remove vote
		if existingVote.IsUpvote == isUpvote {
			// Same vote, remove it
			if isUpvote {
				post.Upvotes--
			} else {
				post.Downvotes--
			}
			delete(f.Votes, voteKey)
		} else {
			// Changing vote
			if isUpvote {
				post.Downvotes--
				post.Upvotes++
			} else {
				post.Upvotes--
				post.Downvotes++
			}
			existingVote.IsUpvote = isUpvote
			existingVote.VotedAt = time.Now()
		}
	} else {
		// New vote
		if isUpvote {
			post.Upvotes++
		} else {
			post.Downvotes++
		}
		f.Votes[voteKey] = &types.Vote{
			VoterID:  voterID,
			PostID:   postID,
			IsUpvote: isUpvote,
			VotedAt:  time.Now(),
		}
	}

	post.Score = post.Upvotes - post.Downvotes
	return nil
}

// GetBySubreddit returns posts from a specific subreddit.
func (f *Forum) GetBySubreddit(sub types.Subreddit, limit int) []*types.Publication {
	f.mu.RLock()
	defer f.mu.RUnlock()

	posts := make([]*types.Publication, 0)
	for _, p := range f.Posts {
		if p.Subreddit == sub && !p.IsComment {
			posts = append(posts, p)
		}
	}

	// Sort by score descending (hot posts)
	f.sortByScore(posts)

	if limit > len(posts) {
		limit = len(posts)
	}
	return posts[:limit]
}

// GetHot returns hot posts (highest score) across all subreddits.
func (f *Forum) GetHot(limit int) []*types.Publication {
	f.mu.RLock()
	defer f.mu.RUnlock()

	posts := make([]*types.Publication, 0)
	for _, p := range f.Posts {
		if !p.IsComment {
			posts = append(posts, p)
		}
	}

	f.sortByScore(posts)

	if limit > len(posts) {
		limit = len(posts)
	}
	return posts[:limit]
}

// GetRecent returns recent posts, most recent first.
func (f *Forum) GetRecent(limit int) []*types.Publication {
	f.mu.RLock()
	defer f.mu.RUnlock()

	posts := make([]*types.Publication, 0, len(f.Posts))
	for _, p := range f.Posts {
		if !p.IsComment {
			posts = append(posts, p)
		}
	}

	// Sort by publish time descending
	for i := 0; i < len(posts)-1; i++ {
		for j := i + 1; j < len(posts); j++ {
			if posts[j].PublishedAt.After(posts[i].PublishedAt) {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}

	if limit > len(posts) {
		limit = len(posts)
	}
	return posts[:limit]
}

// GetComments returns comments for a post.
func (f *Forum) GetComments(postID string) []*types.Publication {
	f.mu.RLock()
	defer f.mu.RUnlock()

	comments := make([]*types.Publication, 0)
	for _, p := range f.Posts {
		if p.ParentID == postID {
			comments = append(comments, p)
		}
	}

	f.sortByScore(comments)
	return comments
}

// GetThreadComments returns all comments under a root post, including nested replies.
func (f *Forum) GetThreadComments(rootID string) []*types.Publication {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if _, ok := f.Posts[rootID]; !ok {
		return nil
	}

	related := make(map[string]struct{})
	related[rootID] = struct{}{}

	changed := true
	for changed {
		changed = false
		for _, p := range f.Posts {
			if !p.IsComment {
				continue
			}
			if _, ok := related[p.ID]; ok {
				continue
			}
			if _, ok := related[p.ParentID]; ok {
				related[p.ID] = struct{}{}
				changed = true
			}
		}
	}

	comments := make([]*types.Publication, 0)
	for id := range related {
		if id == rootID {
			continue
		}
		if p, ok := f.Posts[id]; ok && p.IsComment {
			comments = append(comments, p)
		}
	}

	return comments
}

// sortByScore sorts posts by score descending.
func (f *Forum) sortByScore(posts []*types.Publication) {
	for i := 0; i < len(posts)-1; i++ {
		for j := i + 1; j < len(posts); j++ {
			if posts[j].Score > posts[i].Score {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}
}

// GetByAuthor returns posts by a specific author.
func (f *Forum) GetByAuthor(authorID string) []*types.Publication {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*types.Publication, 0)
	for _, p := range f.Posts {
		if p.AuthorID == authorID {
			result = append(result, p)
		}
	}
	return result
}

// Get returns a specific post.
func (f *Forum) Get(postID string) *types.Publication {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.Posts[postID]
}

// IncrementViews increments view count.
func (f *Forum) IncrementViews(postID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if post, ok := f.Posts[postID]; ok {
		post.Views++
	}
}

// Save persists the forum to disk.
func (f *Forum) Save() error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := os.MkdirAll(f.dataPath, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(f.dataPath, "forum.json"), data, 0644)
}

// Load loads the forum from disk.
func (f *Forum) Load() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(f.dataPath, "forum.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, f); err != nil {
		return err
	}

	// Ensure maps are initialized
	if f.Posts == nil {
		f.Posts = make(map[string]*types.Publication)
	}
	if f.Votes == nil {
		f.Votes = make(map[string]*types.Vote)
	}

	return nil
}

// AllPosts returns all top-level posts (not comments).
func (f *Forum) AllPosts() []*types.Publication {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*types.Publication, 0, len(f.Posts))
	for _, p := range f.Posts {
		if !p.IsComment {
			result = append(result, p)
		}
	}
	return result
}

// GetSubredditStats returns stats for each subreddit.
func (f *Forum) GetSubredditStats() map[types.Subreddit]int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	stats := make(map[types.Subreddit]int)
	for _, p := range f.Posts {
		if !p.IsComment {
			stats[p.Subreddit]++
		}
	}
	return stats
}
