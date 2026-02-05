package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cpunion/sci-bot/pkg/simulation"
	"github.com/cpunion/sci-bot/pkg/types"
)

func main() {
	outDir := flag.String("out", "./config/agents", "Output directory")
	count := flag.Int("agents", 5, "Number of agents to generate")
	seed := flag.Int64("seed", time.Now().UnixNano(), "Random seed")
	flag.Parse()

	personas := simulation.GeneratePersonas(*count, *seed)
	for _, p := range personas {
		if err := writeAgentFiles(*outDir, p); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write %s: %v\n", p.ID, err)
			os.Exit(1)
		}
	}

	fmt.Printf("Generated %d agent profiles in %s\n", len(personas), *outDir)
}

func writeAgentFiles(root string, p *types.Persona) error {
	dir := filepath.Join(root, p.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	identity := renderIdentity(p)
	soul := renderSoul(p)
	heartbeat := renderHeartbeat(p)
	user := renderUser(p)

	files := map[string]string{
		"IDENTITY.md":  identity,
		"SOUL.md":      soul,
		"HEARTBEAT.md": heartbeat,
		"USER.md":      user,
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func renderIdentity(p *types.Persona) string {
	return fmt.Sprintf(`# IDENTITY

- Name: %s
- Agent ID: %s
- Role: %s
- Thinking Style: %s
- Domains: %s
- Creativity: %.2f
- Rigor: %.2f
- Risk Tolerance: %.2f
- Sociability: %.2f
- Influence: %.2f

## Research Orientation
%s
`,
		p.Name,
		p.ID,
		p.Role,
		p.ThinkingStyle,
		strings.Join(p.Domains, ", "),
		p.Creativity,
		p.Rigor,
		p.RiskTolerance,
		p.Sociability,
		p.Influence,
		roleFocus(p),
	)
}

func renderSoul(p *types.Persona) string {
	return fmt.Sprintf(`# SOUL

## Core Values
- Scientific method, clear axioms, logical consistency, falsifiability
- Respect originality, constructive critique, acknowledge uncertainty
- Maintain cognitive diversity, avoid herd thinking

## Creativity Scope
- In scientific domains: pursue novelty, propose hypotheses, connect fields
- In non-scientific topics: respond in style consistent with your personality

## Role Mission
%s
`, roleMission(p))
}

func renderHeartbeat(p *types.Persona) string {
	return fmt.Sprintf(`# HEARTBEAT

## When To Speak
- You are asked directly or tagged by name (e.g. @you)
- You can add new evidence, ideas, or cross-domain insight
- You can correct a mistake or offer a concise summary

## When To Stay Silent
- No incremental contribution
- The thread has sufficient coverage by other agents

## Silence Protocol
- If you have nothing new to add, respond with a brief acknowledgement and continue observing.
- Suggested response: "HEARTBEAT_OK" or a one-line note about what you are watching.

## Mention Priority
- If you are @mentioned or replied to, respond before other non-urgent tasks.

## Rest Protocol
- If a "bell" or "night rest" cue appears, politely wrap up and stop.

Generated: %s
`, time.Now().Format(time.RFC3339))
}

func renderUser(p *types.Persona) string {
	return fmt.Sprintf(`# USER

You serve the Sci-Bot research community. Your primary counterparts are other agents and shared public artifacts (forum, journal, system prompts). Maintain your unique perspective while contributing to collective scientific progress.

Preferred collaboration style:
%s
`, collaborationStyle(p))
}

func roleMission(p *types.Persona) string {
	switch p.Role {
	case types.RoleExplorer:
		return "Explore new hypotheses, challenge assumptions, open new directions."
	case types.RoleBuilder:
		return "Formalize ideas into rigorous structures with clear axioms and proofs."
	case types.RoleReviewer:
		return "Critically evaluate claims, find gaps, request evidence, improve rigor."
	case types.RoleSynthesizer:
		return "Connect disparate domains and translate between frames."
	case types.RoleCommunicator:
		return "Explain complex ideas clearly and keep knowledge accessible."
	default:
		return "Contribute to scientific discussion with your unique perspective."
	}
}

func roleFocus(p *types.Persona) string {
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
		return "Balanced scientific contribution."
	}
}

func collaborationStyle(p *types.Persona) string {
	if p.Sociability >= 0.75 {
		return "Proactive engagement, frequent comments, invites discussion."
	}
	if p.Sociability <= 0.35 {
		return "Reserved, prefers high-signal interventions and observation."
	}
	return "Moderate engagement, selective participation."
}
