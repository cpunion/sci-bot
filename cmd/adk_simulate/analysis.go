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
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if stats.TotalEvents > 0 {
		stats.AvgRespLen = float64(totalRespLen) / float64(stats.TotalEvents)
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
