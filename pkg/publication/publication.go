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

// Forum represents a grassroots discussion channel.
type Forum struct {
	mu sync.RWMutex

	Name     string                        `json:"name"`
	Posts    map[string]*types.Publication `json:"posts"`
	dataPath string
}

// NewForum creates a new forum.
func NewForum(name, dataPath string) *Forum {
	return &Forum{
		Name:     name,
		Posts:    make(map[string]*types.Publication),
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

	f.Posts[pub.ID] = pub
	return nil
}

// GetRecent returns recent posts, most recent first.
func (f *Forum) GetRecent(limit int) []*types.Publication {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Collect all posts
	posts := make([]*types.Publication, 0, len(f.Posts))
	for _, p := range f.Posts {
		posts = append(posts, p)
	}

	// Sort by publish time descending (simple bubble sort for now)
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

// Reply adds a reply count (simplified - just counts).
func (f *Forum) Reply(postID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if post, ok := f.Posts[postID]; ok {
		post.Replies++
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

	return json.Unmarshal(data, f)
}

// AllPosts returns all posts.
func (f *Forum) AllPosts() []*types.Publication {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*types.Publication, 0, len(f.Posts))
	for _, p := range f.Posts {
		result = append(result, p)
	}
	return result
}
