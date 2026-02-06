package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cpunion/sci-bot/pkg/simulation"
)

type summaryStats struct {
	TotalEvents int
	ByAgent     map[string]int
	ByAction    map[string]int
	SleepEvents int
	ToolCalls   int
	AvgRespLen  float64

	UsageEvents         int
	PromptTokens        int
	CandidatesTokens    int
	ThoughtsTokens      int
	ToolUsePromptTokens int
	TotalTokens         int
	AvgTokensPerEvent   float64
	AvgTokensPerCall    float64
}

func analyzeLog(path string) (*summaryStats, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := &summaryStats{
		ByAgent:  make(map[string]int),
		ByAction: make(map[string]int),
	}

	scanner := bufio.NewScanner(file)
	var totalRespLen int
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var ev simulation.EventLog
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		stats.TotalEvents++
		stats.ByAgent[ev.AgentName]++
		stats.ByAction[ev.Action]++
		if ev.Sleeping {
			stats.SleepEvents++
		}
		stats.ToolCalls += len(ev.ToolCalls)
		totalRespLen += len(ev.Response)

		stats.UsageEvents += ev.UsageEvents
		stats.PromptTokens += ev.PromptTokens
		stats.CandidatesTokens += ev.CandidatesTokens
		stats.ThoughtsTokens += ev.ThoughtsTokens
		stats.ToolUsePromptTokens += ev.ToolUsePromptTokens
		stats.TotalTokens += ev.TotalTokens
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if stats.TotalEvents > 0 {
		stats.AvgRespLen = float64(totalRespLen) / float64(stats.TotalEvents)
		stats.AvgTokensPerEvent = float64(stats.TotalTokens) / float64(stats.TotalEvents)
	}
	if stats.UsageEvents > 0 {
		stats.AvgTokensPerCall = float64(stats.TotalTokens) / float64(stats.UsageEvents)
	}

	return stats, nil
}

func printSummary(stats *summaryStats) {
	if stats == nil {
		return
	}
	fmt.Println("\n=== Log Analysis ===")
	fmt.Printf("Total events: %d\n", stats.TotalEvents)
	fmt.Printf("Sleep events: %d\n", stats.SleepEvents)
	fmt.Printf("Tool calls: %d\n", stats.ToolCalls)
	fmt.Printf("Avg response length: %.1f chars\n", stats.AvgRespLen)
	if stats.TotalTokens > 0 {
		fmt.Printf("Total tokens: %d (prompt=%d, candidates=%d, tool_use_prompt=%d, thoughts=%d)\n",
			stats.TotalTokens,
			stats.PromptTokens,
			stats.CandidatesTokens,
			stats.ToolUsePromptTokens,
			stats.ThoughtsTokens,
		)
		fmt.Printf("Avg tokens/event: %.1f\n", stats.AvgTokensPerEvent)
		if stats.UsageEvents > 0 {
			fmt.Printf("Avg tokens/call: %.1f (calls=%d)\n", stats.AvgTokensPerCall, stats.UsageEvents)
		}
	}

	fmt.Println("\nEvents by agent:")
	agents := sortedKeys(stats.ByAgent)
	for _, name := range agents {
		fmt.Printf("  %s: %d\n", name, stats.ByAgent[name])
	}

	fmt.Println("\nEvents by action:")
	actions := sortedKeys(stats.ByAction)
	for _, action := range actions {
		fmt.Printf("  %s: %d\n", action, stats.ByAction[action])
	}
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
