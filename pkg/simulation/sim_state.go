package simulation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// SimState captures the persisted simulation clock.
type SimState struct {
	SimTime     time.Time `json:"sim_time"`
	Ticks       int       `json:"ticks"`
	StepSeconds int       `json:"step_seconds"`
}

// LoadSimState reads the persisted simulation state if present.
func LoadSimState(dataPath string) (*SimState, error) {
	path := filepath.Join(dataPath, "sim_state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	state := &SimState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, err
	}
	return state, nil
}
