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

// Workflow manages drafts, consensus requests, submissions, and reviews.
type Workflow struct {
	mu          sync.RWMutex
	Drafts      map[string]*types.Draft
	Consensus   map[string]*types.ConsensusRequest
	Submissions map[string]*types.Submission
	Reviews     map[string][]*types.PaperReview
	dataPath    string
}

type workflowStore struct {
	Drafts      map[string]*types.Draft            `json:"drafts"`
	Consensus   map[string]*types.ConsensusRequest `json:"consensus"`
	Submissions map[string]*types.Submission       `json:"submissions"`
	Reviews     map[string][]*types.PaperReview    `json:"reviews"`
}

// NewWorkflow creates a workflow store rooted at dataPath.
func NewWorkflow(dataPath string) *Workflow {
	return &Workflow{
		Drafts:      make(map[string]*types.Draft),
		Consensus:   make(map[string]*types.ConsensusRequest),
		Submissions: make(map[string]*types.Submission),
		Reviews:     make(map[string][]*types.PaperReview),
		dataPath:    dataPath,
	}
}

// Load loads workflow data from disk.
func (w *Workflow) Load() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	path := filepath.Join(w.dataPath, "workflow.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var store workflowStore
	if err := json.Unmarshal(data, &store); err != nil {
		return err
	}
	if store.Drafts != nil {
		w.Drafts = store.Drafts
	}
	if store.Consensus != nil {
		w.Consensus = store.Consensus
	}
	if store.Submissions != nil {
		w.Submissions = store.Submissions
	}
	if store.Reviews != nil {
		w.Reviews = store.Reviews
	}
	return nil
}

// Save persists workflow data to disk.
func (w *Workflow) Save() error {
	w.mu.RLock()
	store := workflowStore{
		Drafts:      w.Drafts,
		Consensus:   w.Consensus,
		Submissions: w.Submissions,
		Reviews:     w.Reviews,
	}
	w.mu.RUnlock()

	if err := os.MkdirAll(w.dataPath, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(w.dataPath, "workflow.json"), data, 0644)
}

// CreateDraft registers a new draft.
func (w *Workflow) CreateDraft(draft *types.Draft) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if draft.ID == "" {
		draft.ID = fmt.Sprintf("draft-%d", time.Now().UnixNano())
	}
	if draft.CreatedAt.IsZero() {
		draft.CreatedAt = time.Now()
	}
	draft.UpdatedAt = time.Now()
	w.Drafts[draft.ID] = draft
	return draft.ID
}

// AddConsensusRequest registers a new consensus request.
func (w *Workflow) AddConsensusRequest(req *types.ConsensusRequest) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if req.ID == "" {
		req.ID = fmt.Sprintf("consensus-%d", time.Now().UnixNano())
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	req.UpdatedAt = time.Now()
	w.Consensus[req.ID] = req
	return req.ID
}

// AddSubmission registers a new submission.
func (w *Workflow) AddSubmission(sub *types.Submission) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if sub.ID == "" {
		sub.ID = fmt.Sprintf("submission-%d", time.Now().UnixNano())
	}
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = time.Now()
	}
	sub.UpdatedAt = time.Now()
	w.Submissions[sub.ID] = sub
	return sub.ID
}

// AddReview registers a new review.
func (w *Workflow) AddReview(review *types.PaperReview) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if review.ID == "" {
		review.ID = fmt.Sprintf("review-%d", time.Now().UnixNano())
	}
	if review.CreatedAt.IsZero() {
		review.CreatedAt = time.Now()
	}
	w.Reviews[review.SubmissionID] = append(w.Reviews[review.SubmissionID], review)
	return review.ID
}

// GetDraft returns a draft by ID.
func (w *Workflow) GetDraft(id string) *types.Draft {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Drafts[id]
}

// GetSubmission returns a submission by ID.
func (w *Workflow) GetSubmission(id string) *types.Submission {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Submissions[id]
}

// UpdateSubmissionStatus updates the status of a submission.
func (w *Workflow) UpdateSubmissionStatus(id string, status types.SubmissionStatus) {
	w.mu.Lock()
	defer w.mu.Unlock()
	sub, ok := w.Submissions[id]
	if !ok {
		return
	}
	sub.Status = status
	sub.UpdatedAt = time.Now()
}

// AttachReview links a review to a submission.
func (w *Workflow) AttachReview(submissionID, reviewID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	sub, ok := w.Submissions[submissionID]
	if !ok {
		return
	}
	sub.ReviewIDs = append(sub.ReviewIDs, reviewID)
	sub.UpdatedAt = time.Now()
}
