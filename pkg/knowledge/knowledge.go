// Package knowledge implements axiom systems and theory management.
package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cpunion/sci-bot/pkg/types"
)

// AxiomRegistry manages axiom systems.
type AxiomRegistry struct {
	mu sync.RWMutex

	systems  map[string]*types.AxiomSystem
	dataPath string
}

// NewAxiomRegistry creates a new axiom registry.
func NewAxiomRegistry(dataPath string) *AxiomRegistry {
	return &AxiomRegistry{
		systems:  make(map[string]*types.AxiomSystem),
		dataPath: dataPath,
	}
}

// Register registers a new axiom system.
func (r *AxiomRegistry) Register(system *types.AxiomSystem) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.systems[system.ID]; exists {
		return fmt.Errorf("axiom system already exists: %s", system.ID)
	}

	r.systems[system.ID] = system
	return r.save(system)
}

// Get retrieves an axiom system by ID.
func (r *AxiomRegistry) Get(id string) (*types.AxiomSystem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if system, ok := r.systems[id]; ok {
		return system, nil
	}
	return nil, fmt.Errorf("axiom system not found: %s", id)
}

// List returns all axiom systems.
func (r *AxiomRegistry) List() []*types.AxiomSystem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*types.AxiomSystem, 0, len(r.systems))
	for _, s := range r.systems {
		result = append(result, s)
	}
	return result
}

// GetDerived returns axiom systems derived from a parent.
func (r *AxiomRegistry) GetDerived(parentID string) []*types.AxiomSystem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*types.AxiomSystem, 0)
	for _, s := range r.systems {
		if s.Parent == parentID {
			result = append(result, s)
		}
	}
	return result
}

// save persists an axiom system to disk.
func (r *AxiomRegistry) save(system *types.AxiomSystem) error {
	dir := filepath.Join(r.dataPath, "axiom_systems")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(system, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, system.ID+".json"), data, 0644)
}

// Load loads all axiom systems from disk.
func (r *AxiomRegistry) Load() error {
	dir := filepath.Join(r.dataPath, "axiom_systems")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // No systems to load
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var system types.AxiomSystem
		if err := json.Unmarshal(data, &system); err != nil {
			continue
		}

		r.systems[system.ID] = &system
	}

	return nil
}

// TheoryRepository manages theories.
type TheoryRepository struct {
	mu sync.RWMutex

	theories map[string]*types.Theory
	dataPath string
	axiomReg *AxiomRegistry
}

// NewTheoryRepository creates a new theory repository.
func NewTheoryRepository(dataPath string, axiomReg *AxiomRegistry) *TheoryRepository {
	return &TheoryRepository{
		theories: make(map[string]*types.Theory),
		dataPath: dataPath,
		axiomReg: axiomReg,
	}
}

// Propose submits a new theory.
func (r *TheoryRepository) Propose(theory *types.Theory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.theories[theory.ID]; exists {
		return fmt.Errorf("theory already exists: %s", theory.ID)
	}

	// Validate axiom system exists
	if theory.AxiomSystem != "" && theory.AxiomSystem != "custom" {
		if _, err := r.axiomReg.Get(theory.AxiomSystem); err != nil {
			return fmt.Errorf("unknown axiom system: %s", theory.AxiomSystem)
		}
	}

	theory.Status = types.StatusProposed
	theory.Created = time.Now()
	theory.Updated = time.Now()

	r.theories[theory.ID] = theory
	return r.save(theory)
}

// Update updates an existing theory.
func (r *TheoryRepository) Update(theoryID string, updateFn func(*types.Theory) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	theory, ok := r.theories[theoryID]
	if !ok {
		return fmt.Errorf("theory not found: %s", theoryID)
	}

	if err := updateFn(theory); err != nil {
		return err
	}

	theory.Updated = time.Now()
	return r.save(theory)
}

// Get retrieves a theory by ID.
func (r *TheoryRepository) Get(id string) (*types.Theory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if theory, ok := r.theories[id]; ok {
		return theory, nil
	}
	return nil, fmt.Errorf("theory not found: %s", id)
}

// List returns all theories.
func (r *TheoryRepository) List() []*types.Theory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*types.Theory, 0, len(r.theories))
	for _, t := range r.theories {
		result = append(result, t)
	}
	return result
}

// Search searches theories by criteria.
func (r *TheoryRepository) Search(query TheoryQuery) []*types.Theory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*types.Theory, 0)
	for _, t := range r.theories {
		if r.matchesQuery(t, query) {
			result = append(result, t)
		}
	}
	return result
}

// TheoryQuery defines search criteria for theories.
type TheoryQuery struct {
	AxiomSystem string
	Status      types.TheoryStatus
	AuthorID    string
	IsHeretical *bool
	MinNovelty  float64
}

// matchesQuery checks if a theory matches a query.
func (r *TheoryRepository) matchesQuery(t *types.Theory, q TheoryQuery) bool {
	if q.AxiomSystem != "" && t.AxiomSystem != q.AxiomSystem {
		return false
	}
	if q.Status != "" && t.Status != q.Status {
		return false
	}
	if q.AuthorID != "" {
		found := false
		for _, a := range t.Authors {
			if a == q.AuthorID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if q.IsHeretical != nil && t.IsHeretical != *q.IsHeretical {
		return false
	}
	if t.NoveltyScore < q.MinNovelty {
		return false
	}
	return true
}

// GetByAxiomSystem returns theories based on an axiom system.
func (r *TheoryRepository) GetByAxiomSystem(axiomID string) []*types.Theory {
	return r.Search(TheoryQuery{AxiomSystem: axiomID})
}

// AddReview adds a review to a theory.
func (r *TheoryRepository) AddReview(theoryID string, review *types.Review) error {
	return r.Update(theoryID, func(t *types.Theory) error {
		t.Reviews = append(t.Reviews, *review)

		// Update status based on reviews
		approves := 0
		rejects := 0
		for _, rev := range t.Reviews {
			switch rev.Verdict {
			case "approve":
				approves++
			case "reject":
				rejects++
			}
		}

		if approves >= 3 && rejects == 0 {
			t.Status = types.StatusValidated
		} else if rejects >= 2 {
			t.Status = types.StatusDisputed
		} else if len(t.Reviews) > 0 {
			t.Status = types.StatusUnderReview
		}

		return nil
	})
}

// save persists a theory to disk.
func (r *TheoryRepository) save(theory *types.Theory) error {
	dir := filepath.Join(r.dataPath, "theories", theory.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(theory, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "theory.json"), data, 0644)
}

// Load loads all theories from disk.
func (r *TheoryRepository) Load() error {
	dir := filepath.Join(r.dataPath, "theories")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		theoryPath := filepath.Join(dir, entry.Name(), "theory.json")
		data, err := os.ReadFile(theoryPath)
		if err != nil {
			continue
		}

		var theory types.Theory
		if err := json.Unmarshal(data, &theory); err != nil {
			continue
		}

		r.theories[theory.ID] = &theory
	}

	return nil
}
