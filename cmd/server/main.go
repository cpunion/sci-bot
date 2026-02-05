package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/types"
)

type AgentInfo struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Role                string   `json:"role"`
	ThinkingStyle       string   `json:"thinking_style"`
	Domains             []string `json:"domains"`
	Creativity          float64  `json:"creativity"`
	Rigor               float64  `json:"rigor"`
	RiskTolerance       float64  `json:"risk_tolerance"`
	Sociability         float64  `json:"sociability"`
	Influence           float64  `json:"influence"`
	ResearchOrientation string   `json:"research_orientation"`
}

type DailyNote struct {
	Date    string       `json:"date"`
	Content string       `json:"content"`
	Entries []DailyEntry `json:"entries,omitempty"`
}

type DailyEntry struct {
	Timestamp string `json:"timestamp"`
	Prompt    string `json:"prompt,omitempty"`
	Reply     string `json:"reply,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Raw       string `json:"raw,omitempty"`
}

type AgentDetail struct {
	Agent           AgentInfo            `json:"agent"`
	ForumPosts      []*types.Publication `json:"forum_posts"`
	ForumComments   []*types.Publication `json:"forum_comments"`
	JournalApproved []*types.Publication `json:"journal_approved"`
	JournalPending  []*types.Publication `json:"journal_pending"`
	DailyNotes      []DailyNote          `json:"daily_notes"`
}

type ForumResponse struct {
	Name           string               `json:"name"`
	Posts          []*types.Publication `json:"posts"`
	SubredditStats map[string]int       `json:"subreddit_stats"`
}

type ForumPostResponse struct {
	Post     *types.Publication   `json:"post"`
	Comments []*types.Publication `json:"comments"`
}

type JournalResponse struct {
	Name     string               `json:"name"`
	Approved []*types.Publication `json:"approved"`
	Pending  []*types.Publication `json:"pending"`
}

func main() {
	addr := flag.String("addr", ":8080", "Listen address")
	dataPath := flag.String("data", "./data/adk-simulation", "Data directory")
	agentsPath := flag.String("agents", "./config/agents", "Agents directory")
	webPath := flag.String("web", "./web", "Web assets directory")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/agents", withJSON(func(w http.ResponseWriter, r *http.Request) (any, int, error) {
		if r.Method != http.MethodGet {
			return nil, http.StatusMethodNotAllowed, errors.New("method not allowed")
		}
		agents, err := loadAgents(*agentsPath)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		return map[string]any{"agents": agents}, http.StatusOK, nil
	}))

	mux.HandleFunc("/api/agents/", withJSON(func(w http.ResponseWriter, r *http.Request) (any, int, error) {
		if r.Method != http.MethodGet {
			return nil, http.StatusMethodNotAllowed, errors.New("method not allowed")
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/agents/")
		id = strings.Trim(id, "/")
		if id == "" {
			return nil, http.StatusBadRequest, errors.New("missing agent id")
		}
		agent, err := loadAgent(*agentsPath, id)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, http.StatusNotFound, err
			}
			return nil, http.StatusInternalServerError, err
		}

		forum, _ := loadForum(*dataPath)
		journal, _ := loadJournal(*dataPath)

		var forumPosts, forumComments []*types.Publication
		if forum != nil {
			byAuthor := forum.GetByAuthor(id)
			for _, p := range byAuthor {
				if p.IsComment {
					forumComments = append(forumComments, p)
				} else {
					forumPosts = append(forumPosts, p)
				}
			}
			sortPublicationsByTimeDesc(forumPosts)
			sortPublicationsByTimeDesc(forumComments)
		}

		approved := filterJournalByAuthor(journal, id, true)
		pending := filterJournalByAuthor(journal, id, false)

		dailyNotes := loadDailyNotes(*dataPath, id, 10)

		return AgentDetail{
			Agent:           agent,
			ForumPosts:      forumPosts,
			ForumComments:   forumComments,
			JournalApproved: approved,
			JournalPending:  pending,
			DailyNotes:      dailyNotes,
		}, http.StatusOK, nil
	}))

	mux.HandleFunc("/api/forum", withJSON(func(w http.ResponseWriter, r *http.Request) (any, int, error) {
		if r.Method != http.MethodGet {
			return nil, http.StatusMethodNotAllowed, errors.New("method not allowed")
		}
		forum, err := loadForum(*dataPath)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		limit := parseLimit(r.URL.Query().Get("limit"), 30, 1, 200)
		sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
		subreddit := strings.TrimSpace(r.URL.Query().Get("subreddit"))

		posts := selectForumPosts(forum, subreddit, sortBy, limit)
		stats := forum.GetSubredditStats()
		statsOut := make(map[string]int, len(stats))
		for k, v := range stats {
			statsOut[string(k)] = v
		}

		return ForumResponse{
			Name:           forum.Name,
			Posts:          posts,
			SubredditStats: statsOut,
		}, http.StatusOK, nil
	}))

	mux.HandleFunc("/api/forum/posts/", withJSON(func(w http.ResponseWriter, r *http.Request) (any, int, error) {
		if r.Method != http.MethodGet {
			return nil, http.StatusMethodNotAllowed, errors.New("method not allowed")
		}
		forum, err := loadForum(*dataPath)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		postID := strings.TrimPrefix(r.URL.Path, "/api/forum/posts/")
		postID = strings.Trim(postID, "/")
		if postID == "" {
			return nil, http.StatusBadRequest, errors.New("missing post id")
		}
		post := forum.Get(postID)
		if post == nil {
			return nil, http.StatusNotFound, fmt.Errorf("post not found")
		}
		comments := forum.GetThreadComments(postID)
		return ForumPostResponse{Post: post, Comments: comments}, http.StatusOK, nil
	}))

	mux.HandleFunc("/api/journal", withJSON(func(w http.ResponseWriter, r *http.Request) (any, int, error) {
		if r.Method != http.MethodGet {
			return nil, http.StatusMethodNotAllowed, errors.New("method not allowed")
		}
		journal, err := loadJournal(*dataPath)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}

		approved := journal.GetApproved()
		pending := journal.GetPending()
		sortPublicationsByTimeDesc(approved)
		sortPublicationsByTimeDesc(pending)

		limit := parseLimit(r.URL.Query().Get("limit"), 50, 1, 200)
		if limit < len(approved) {
			approved = approved[:limit]
		}

		return JournalResponse{
			Name:     journal.Name,
			Approved: approved,
			Pending:  pending,
		}, http.StatusOK, nil
	}))

	mux.HandleFunc("/forum", serveStaticFile(*webPath, "forum.html"))
	mux.HandleFunc("/journal", serveStaticFile(*webPath, "journal.html"))
	mux.HandleFunc("/agent/", serveStaticFile(*webPath, "agent.html"))

	fs := http.FileServer(http.Dir(*webPath))
	mux.Handle("/", fs)

	log.Printf("Web server listening on %s", *addr)
	if err := http.ListenAndServe(*addr, logRequest(mux)); err != nil {
		log.Fatal(err)
	}
}

func withJSON(handler func(http.ResponseWriter, *http.Request) (any, int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, status, err := handler(w, r)
		if err != nil {
			writeJSON(w, status, map[string]any{
				"error": err.Error(),
			})
			return
		}
		writeJSON(w, status, payload)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func serveStaticFile(webPath, file string) http.HandlerFunc {
	path := filepath.Join(webPath, file)
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}

func loadForum(dataPath string) (*publication.Forum, error) {
	forum := publication.NewForum("自由论坛", filepath.Join(dataPath, "forum"))
	if err := forum.Load(); err != nil {
		return nil, err
	}
	return forum, nil
}

func loadJournal(dataPath string) (*publication.Journal, error) {
	journal := publication.NewJournal("科学前沿", filepath.Join(dataPath, "journal"))
	if err := journal.Load(); err != nil {
		return nil, err
	}
	return journal, nil
}

func loadAgents(agentsPath string) ([]AgentInfo, error) {
	entries, err := os.ReadDir(agentsPath)
	if err != nil {
		return nil, err
	}
	agents := make([]AgentInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		agent, err := loadAgent(agentsPath, id)
		if err != nil {
			continue
		}
		agents = append(agents, agent)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Name < agents[j].Name })
	return agents, nil
}

func loadAgent(agentsPath, id string) (AgentInfo, error) {
	identityPath := filepath.Join(agentsPath, id, "IDENTITY.md")
	data, err := os.ReadFile(identityPath)
	if err != nil {
		return AgentInfo{}, err
	}
	info := parseIdentity(string(data))
	if info.ID == "" {
		info.ID = id
	}
	return info, nil
}

func parseIdentity(content string) AgentInfo {
	lines := strings.Split(content, "\n")
	info := AgentInfo{}
	captureOrientation := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			captureOrientation = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, "##")), "Research Orientation")
			continue
		}
		if captureOrientation {
			if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "#") {
				continue
			}
			info.ResearchOrientation = line
			captureOrientation = false
			continue
		}
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimPrefix(line, "- ")
		}
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch strings.ToLower(key) {
		case "name":
			info.Name = value
		case "agent id":
			info.ID = value
		case "role":
			info.Role = value
		case "thinking style":
			info.ThinkingStyle = value
		case "domains":
			info.Domains = splitCSV(value)
		case "creativity":
			info.Creativity = parseFloat(value)
		case "rigor":
			info.Rigor = parseFloat(value)
		case "risk tolerance":
			info.RiskTolerance = parseFloat(value)
		case "sociability":
			info.Sociability = parseFloat(value)
		case "influence":
			info.Influence = parseFloat(value)
		}
	}
	return info
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trim := strings.TrimSpace(p)
		if trim != "" {
			out = append(out, trim)
		}
	}
	return out
}

func parseFloat(value string) float64 {
	val, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return val
}

func selectForumPosts(forum *publication.Forum, subreddit, sortBy string, limit int) []*types.Publication {
	if subreddit != "" {
		return filterForumBySubreddit(forum, subreddit, sortBy, limit)
	}
	if sortBy == "recent" || sortBy == "new" {
		return forum.GetRecent(limit)
	}
	return forum.GetHot(limit)
}

func filterForumBySubreddit(forum *publication.Forum, subreddit, sortBy string, limit int) []*types.Publication {
	target := types.Subreddit(subreddit)
	if sortBy == "recent" || sortBy == "new" {
		posts := make([]*types.Publication, 0)
		for _, p := range forum.AllPosts() {
			if p.Subreddit == target {
				posts = append(posts, p)
			}
		}
		sortPublicationsByTimeDesc(posts)
		if limit < len(posts) {
			posts = posts[:limit]
		}
		return posts
	}
	return forum.GetBySubreddit(target, limit)
}

func sortPublicationsByTimeDesc(items []*types.Publication) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})
}

func sortPublicationsByScoreDesc(items []*types.Publication) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
}

func filterJournalByAuthor(journal *publication.Journal, authorID string, approved bool) []*types.Publication {
	if journal == nil {
		return nil
	}
	result := make([]*types.Publication, 0)
	if approved {
		for _, p := range journal.GetApproved() {
			if p.AuthorID == authorID {
				result = append(result, p)
			}
		}
	} else {
		for _, p := range journal.GetPending() {
			if p.AuthorID == authorID {
				result = append(result, p)
			}
		}
	}
	sortPublicationsByTimeDesc(result)
	return result
}

func loadDailyNotes(dataPath, agentID string, limit int) []DailyNote {
	dir := filepath.Join(dataPath, "agents", agentID, "daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	notesByDate := make(map[string]*DailyNote)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		date := strings.TrimSuffix(name, ".jsonl")
		entries, err := readDailyEntries(filepath.Join(dir, name))
		if err != nil || len(entries) == 0 {
			continue
		}
		note := &DailyNote{Date: date, Entries: entries}
		notesByDate[date] = note
	}
	notes := make([]DailyNote, 0, len(notesByDate))
	for _, note := range notesByDate {
		notes = append(notes, *note)
	}
	if len(notes) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 10
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].Date > notes[j].Date })
	if limit < len(notes) {
		notes = notes[:limit]
	}
	return notes
}

func readDailyEntries(path string) ([]DailyEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	out := make([]DailyEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry DailyEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func parseLimit(value string, fallback, min, max int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}
