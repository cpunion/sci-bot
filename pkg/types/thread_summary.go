// Package types defines core types for the Sci-Bot Agent Network.
package types

import "time"

// ThreadSummary stores a cached summary for a forum thread.
type ThreadSummary struct {
	PostID        string    `json:"post_id"`
	Summary       string    `json:"summary"`
	UpdatedAt     time.Time `json:"updated_at"`
	LastCommentAt time.Time `json:"last_comment_at,omitempty"`
	LastCommentID string    `json:"last_comment_id,omitempty"`
	CommentCount  int       `json:"comment_count"`
	PostHash      string    `json:"post_hash,omitempty"`
}
