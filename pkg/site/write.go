package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cpunion/sci-bot/pkg/types"
)

func WriteAgentCatalog(path string, personas []*types.Persona) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	agents := make([]Agent, 0, len(personas))
	for _, p := range personas {
		if p == nil {
			continue
		}
		agents = append(agents, Agent{
			ID:                  strings.TrimSpace(p.ID),
			Name:                strings.TrimSpace(p.Name),
			Role:                string(p.Role),
			ThinkingStyle:       string(p.ThinkingStyle),
			Domains:             append([]string(nil), p.Domains...),
			Creativity:          p.Creativity,
			Rigor:               p.Rigor,
			RiskTolerance:       p.RiskTolerance,
			Sociability:         p.Sociability,
			Influence:           p.Influence,
			ResearchOrientation: roleFocus(p),
		})
	}

	sort.Slice(agents, func(i, j int) bool {
		if agents[i].Role != agents[j].Role {
			return agents[i].Role < agents[j].Role
		}
		if agents[i].Name != agents[j].Name {
			return agents[i].Name < agents[j].Name
		}
		return agents[i].ID < agents[j].ID
	})

	cat := AgentCatalog{
		Version:     1,
		GeneratedAt: time.Now(),
		Agents:      agents,
	}
	data, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func roleFocus(p *types.Persona) string {
	if p == nil {
		return ""
	}
	switch p.Role {
	case types.RoleExplorer:
		return "High-variance discovery and speculative hypotheses."
	case types.RoleBuilder:
		return "Formal structure, proofs, and consistency checking."
	case types.RoleReviewer:
		return "Rigor, falsifiability, and error detection."
	case types.RoleSynthesizer:
		return "Cross-domain synthesis and conceptual bridges."
	case types.RoleCommunicator:
		return "Clarity, teaching, and outreach to non-experts."
	default:
		return ""
	}
}

func WriteManifest(path string, manifest Manifest) error {
	if manifest.Version <= 0 {
		manifest.Version = 1
	}
	if manifest.GeneratedAt.IsZero() {
		manifest.GeneratedAt = time.Now()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
