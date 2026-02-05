package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type DailyEntry struct {
	Timestamp string `json:"timestamp"`
	Prompt    string `json:"prompt,omitempty"`
	Reply     string `json:"reply,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Raw       string `json:"raw,omitempty"`
}

func main() {
	dataPath := flag.String("data", "./data/adk-simulation", "Data directory")
	deleteMD := flag.Bool("delete-md", false, "Delete .md files after successful migration")
	overwrite := flag.Bool("overwrite", false, "Overwrite existing .jsonl files")
	flag.Parse()

	agentsDir := filepath.Join(*dataPath, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		fmt.Printf("Failed to read agents directory: %v\n", err)
		os.Exit(1)
	}

	migrated := 0
	for _, agent := range entries {
		if !agent.IsDir() {
			continue
		}
		dailyDir := filepath.Join(agentsDir, agent.Name(), "daily")
		mdFiles, _ := filepath.Glob(filepath.Join(dailyDir, "*.md"))
		if len(mdFiles) == 0 {
			continue
		}
		for _, mdPath := range mdFiles {
			date := strings.TrimSuffix(filepath.Base(mdPath), ".md")
			jsonPath := filepath.Join(dailyDir, date+".jsonl")
			if !*overwrite {
				if _, err := os.Stat(jsonPath); err == nil {
					continue
				}
			}
			entries, err := parseDailyMarkdown(mdPath)
			if err != nil {
				fmt.Printf("Failed to parse %s: %v\n", mdPath, err)
				continue
			}
			if len(entries) == 0 {
				continue
			}
			if err := writeJSONL(jsonPath, entries); err != nil {
				fmt.Printf("Failed to write %s: %v\n", jsonPath, err)
				continue
			}
			migrated++
			if *deleteMD {
				_ = os.Remove(mdPath)
			}
		}
	}

	fmt.Printf("Migration complete. Files migrated: %d\n", migrated)
}

var timestampRegex = regexp.MustCompile(`^\s*"?\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)

func parseDailyMarkdown(path string) ([]DailyEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines, err := readLines(file)
	if err != nil {
		return nil, err
	}

	var entries []DailyEntry
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		entry := buildEntry(current)
		if entry.Timestamp != "" || entry.Raw != "" {
			entries = append(entries, entry)
		}
		current = nil
	}

	for _, line := range lines {
		if timestampRegex.MatchString(line) {
			flush()
			current = append(current, line)
			continue
		}
		if len(current) == 0 && strings.TrimSpace(line) == "" {
			continue
		}
		current = append(current, line)
	}
	flush()

	return entries, nil
}

func readLines(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func buildEntry(lines []string) DailyEntry {
	if len(lines) == 0 {
		return DailyEntry{}
	}
	first := strings.TrimSpace(strings.TrimLeft(lines[0], "\""))
	const promptKey = " | prompt: "
	const replyKey = " | reply: "
	promptIdx := strings.Index(first, promptKey)
	if promptIdx == -1 {
		return DailyEntry{Raw: strings.TrimSpace(strings.Join(lines, "\n"))}
	}
	timestamp := strings.TrimSpace(first[:promptIdx])
	rest := first[promptIdx+len(promptKey):]
	replyIdx := strings.Index(rest, replyKey)
	prompt := ""
	reply := ""
	if replyIdx == -1 {
		prompt = strings.TrimSpace(rest)
	} else {
		prompt = strings.TrimSpace(rest[:replyIdx])
		reply = strings.TrimSpace(rest[replyIdx+len(replyKey):])
	}

	notesLines := make([]string, 0)
	if len(lines) > 1 {
		notesLines = append(notesLines, lines[1:]...)
	}
	notes := strings.TrimSpace(strings.Join(notesLines, "\n"))

	entry := DailyEntry{
		Timestamp: timestamp,
		Prompt:    prompt,
		Reply:     reply,
		Notes:     notes,
		Raw:       strings.TrimSpace(strings.Join(lines, "\n")),
	}
	return entry
}

func writeJSONL(path string, entries []DailyEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp < entries[j].Timestamp
	})

	writer := bufio.NewWriter(file)
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return writer.Flush()
}
