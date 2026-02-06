package main

import (
	"bufio"
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

type PaperDetailResponse struct {
	JournalName string             `json:"journal_name"`
	Status      string             `json:"status"` // published | pending
	Paper       *types.Publication `json:"paper"`
}

type FeedEvent struct {
	Timestamp      time.Time `json:"timestamp"`
	SimTime        time.Time `json:"sim_time"`
	Tick           int       `json:"tick"`
	AgentID        string    `json:"agent_id"`
	AgentName      string    `json:"agent_name"`
	ModelName      string    `json:"model_name,omitempty"`
	Action         string    `json:"action"`
	Prompt         string    `json:"prompt"`
	Response       string    `json:"response"`
	ToolCalls      []string  `json:"tool_calls,omitempty"`
	ToolResponses  []string  `json:"tool_responses,omitempty"`
	TurnCount      int       `json:"turn_count"`
	BellRung       bool      `json:"bell_rung"`
	GraceRemaining int       `json:"grace_remaining"`
	Sleeping       bool      `json:"sleeping"`

	UsageEvents         int `json:"usage_events,omitempty"`
	PromptTokens        int `json:"prompt_tokens,omitempty"`
	CandidatesTokens    int `json:"candidates_tokens,omitempty"`
	ThoughtsTokens      int `json:"thoughts_tokens,omitempty"`
	ToolUsePromptTokens int `json:"tool_use_prompt_tokens,omitempty"`
	CachedContentTokens int `json:"cached_content_tokens,omitempty"`
	TotalTokens         int `json:"total_tokens,omitempty"`

	// Derived links for UI. Not part of the simulation log format.
	ActorURL     string `json:"actor_url,omitempty"`
	ContentKind  string `json:"content_kind,omitempty"`
	ContentID    string `json:"content_id,omitempty"`
	ContentTitle string `json:"content_title,omitempty"`
	ContentURL   string `json:"content_url,omitempty"`
}

type FeedResponse struct {
	Log    string      `json:"log"`
	Events []FeedEvent `json:"events"`
}

func main() {
	addr := flag.String("addr", ":8061", "Listen address")
	dataPath := flag.String("data", "./data/adk-simulation", "Data directory")
	agentsPath := flag.String("agents", "./config/agents", "Agents directory")
	webPath := flag.String("web", "./web", "Web assets directory")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/agents", withJSON(func(w http.ResponseWriter, r *http.Request) (any, int, error) {
		if r.Method != http.MethodGet {
			return nil, http.StatusMethodNotAllowed, errors.New("method not allowed")
		}
		agents, err := loadAgentsMerged(*dataPath, *agentsPath)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		return map[string]any{"agents": agents}, http.StatusOK, nil
	}))

	mux.HandleFunc("/api/stats", withJSON(func(w http.ResponseWriter, r *http.Request) (any, int, error) {
		if r.Method != http.MethodGet {
			return nil, http.StatusMethodNotAllowed, errors.New("method not allowed")
		}

		activeAgents, _ := countAgentDirs(filepath.Join(*dataPath, "agents"))
		if activeAgents == 0 {
			activeAgents, _ = countAgentDirs(*agentsPath)
		}

		var forumThreads int
		if forum, err := loadForum(*dataPath); err == nil {
			for _, p := range forum.AllPosts() {
				if p != nil && !p.IsComment {
					forumThreads++
				}
			}
		}

		var journalApproved int
		if journal, err := loadJournal(*dataPath); err == nil {
			journalApproved = len(journal.GetApproved())
		}

		return map[string]any{
			"active_agents":    activeAgents,
			"forum_threads":    forumThreads,
			"journal_approved": journalApproved,
		}, http.StatusOK, nil
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
		agent, err := loadAgentMerged(*dataPath, *agentsPath, id)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, http.StatusNotFound, err
			}
			return nil, http.StatusInternalServerError, err
		}
		resolvedID := agent.ID
		if resolvedID == "" {
			resolvedID = id
		}

		forum, _ := loadForum(*dataPath)
		journal, _ := loadJournal(*dataPath)

		var forumPosts, forumComments []*types.Publication
		if forum != nil {
			byAuthor := forum.GetByAuthor(resolvedID)
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

		approved := filterJournalByAuthor(journal, resolvedID, true)
		pending := filterJournalByAuthor(journal, resolvedID, false)

		dailyNotes := loadDailyNotes(*dataPath, resolvedID, 10)

		return AgentDetail{
			Agent:           agent,
			ForumPosts:      forumPosts,
			ForumComments:   forumComments,
			JournalApproved: approved,
			JournalPending:  pending,
			DailyNotes:      dailyNotes,
		}, http.StatusOK, nil
	}))

	mux.HandleFunc("/api/feed", withJSON(func(w http.ResponseWriter, r *http.Request) (any, int, error) {
		if r.Method != http.MethodGet {
			return nil, http.StatusMethodNotAllowed, errors.New("method not allowed")
		}

		limit := parseLimit(r.URL.Query().Get("limit"), 200, 1, 2000)
		requestedLog := strings.TrimSpace(r.URL.Query().Get("log"))

		var logName string
		var events []FeedEvent
		var err error

		if requestedLog == "" || requestedLog == "all" {
			logName = "all"
			events, err = loadFeedEventsAll(*dataPath, limit)
		} else {
			var logPath string
			logPath, logName, err = resolveFeedLog(*dataPath, requestedLog)
			if err == nil {
				events, err = loadFeedEvents(logPath, limit)
			}
		}
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, http.StatusNotFound, err
			}
			return nil, http.StatusBadRequest, err
		}

		hydrateFeedEventsFromDailyNotes(*dataPath, events)
		enrichFeedEvents(*dataPath, events)
		sort.SliceStable(events, func(i, j int) bool {
			if events[i].SimTime.Equal(events[j].SimTime) {
				return events[i].Timestamp.After(events[j].Timestamp)
			}
			return events[i].SimTime.After(events[j].SimTime)
		})

		return FeedResponse{
			Log:    logName,
			Events: events,
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

	mux.HandleFunc("/api/journal/papers/", withJSON(func(w http.ResponseWriter, r *http.Request) (any, int, error) {
		if r.Method != http.MethodGet {
			return nil, http.StatusMethodNotAllowed, errors.New("method not allowed")
		}
		journal, err := loadJournal(*dataPath)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		paperID := strings.TrimPrefix(r.URL.Path, "/api/journal/papers/")
		paperID = strings.Trim(paperID, "/")
		if paperID == "" {
			return nil, http.StatusBadRequest, errors.New("missing paper id")
		}

		var paper *types.Publication
		status := ""
		if journal != nil {
			if p, ok := journal.Publications[paperID]; ok {
				paper = p
				status = "published"
			} else if p, ok := journal.Pending[paperID]; ok {
				paper = p
				status = "pending"
			}
		}
		if paper == nil {
			return nil, http.StatusNotFound, fmt.Errorf("paper not found")
		}

		return PaperDetailResponse{
			JournalName: journal.Name,
			Status:      status,
			Paper:       paper,
		}, http.StatusOK, nil
	}))

	mux.HandleFunc("/forum", serveStaticFile(*webPath, "forum.html"))
	mux.HandleFunc("/journal", serveStaticFile(*webPath, "journal.html"))
	mux.HandleFunc("/paper/", serveStaticFile(*webPath, "paper.html"))
	mux.HandleFunc("/agent/", serveStaticFile(*webPath, "agent.html"))
	mux.HandleFunc("/feed", serveStaticFile(*webPath, "feed.html"))

	mux.Handle("/", serveStaticDir(*webPath))

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
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, path)
	}
}

func serveStaticDir(webPath string) http.Handler {
	fs := http.FileServer(http.Dir(webPath))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" ||
			strings.HasSuffix(r.URL.Path, ".html") ||
			strings.HasSuffix(r.URL.Path, ".js") ||
			strings.HasSuffix(r.URL.Path, ".css") {
			w.Header().Set("Cache-Control", "no-store")
		}
		fs.ServeHTTP(w, r)
	})
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

func loadAgentsMerged(dataPath, agentsPath string) ([]AgentInfo, error) {
	ids := make(map[string]struct{})

	for _, path := range []string{
		agentsPath,
		filepath.Join(dataPath, "agents"),
	} {
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			ids[entry.Name()] = struct{}{}
		}
	}

	agents := make([]AgentInfo, 0, len(ids))
	for id := range ids {
		agent, err := loadAgentMerged(dataPath, agentsPath, id)
		if err != nil {
			continue
		}
		agents = append(agents, agent)
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

	return agents, nil
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

func countAgentDirs(path string) (int, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count, nil
}

func loadAgentMerged(dataPath, agentsPath, id string) (AgentInfo, error) {
	state, stateErr := loadAgentFromState(dataPath, id)
	cfg, cfgErr := loadAgent(agentsPath, id)

	// Prefer persisted simulation state. Config identities are only trusted to
	// enrich state when the names match, so we don't mix personas.
	if stateErr == nil && state.ID != "" {
		if cfgErr == nil && cfg.ID != "" {
			cfgName := strings.TrimSpace(cfg.Name)
			stateName := strings.TrimSpace(state.Name)
			if stateName == "" || strings.EqualFold(cfgName, stateName) {
				merged := cfg
				if state.ID != "" {
					merged.ID = state.ID
				}
				if state.Name != "" {
					merged.Name = state.Name
				}
				if merged.Role == "" {
					merged.Role = state.Role
				}
				return merged, nil
			}
		}
		return state, nil
	}

	if cfgErr == nil && cfg.ID != "" {
		return cfg, nil
	}

	// If this id doesn't exist in either config or persisted state, allow resolving
	// an alias like agent name ("Agent-73") to the real agent id ("agent-reviewer-15").
	if errors.Is(stateErr, os.ErrNotExist) && errors.Is(cfgErr, os.ErrNotExist) {
		resolvedID, err := resolveAgentAlias(dataPath, agentsPath, id)
		if err != nil {
			return AgentInfo{}, err
		}
		if resolvedID != "" && resolvedID != id {
			return loadAgentMerged(dataPath, agentsPath, resolvedID)
		}
	}

	if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
		return AgentInfo{}, stateErr
	}
	return AgentInfo{}, cfgErr
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

func loadAgentFromState(dataPath, id string) (AgentInfo, error) {
	statePath := filepath.Join(dataPath, "agents", id, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return AgentInfo{}, err
	}

	var state struct {
		AgentID   string `json:"agent_id"`
		AgentName string `json:"agent_name"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return AgentInfo{}, err
	}

	info := AgentInfo{
		ID:   strings.TrimSpace(state.AgentID),
		Name: strings.TrimSpace(state.AgentName),
		Role: roleFromID(id),
	}
	if info.ID == "" {
		info.ID = id
	}
	if info.Name == "" {
		info.Name = id
	}
	if info.Role == "" {
		info.Role = "agent"
	}

	return info, nil
}

func roleFromID(id string) string {
	// Convention: agent-<role>-<n>
	if strings.HasPrefix(id, "agent-") {
		parts := strings.Split(id, "-")
		if len(parts) >= 3 {
			return parts[1]
		}
	}
	roles := []string{"reviewer", "explorer", "builder", "synthesizer", "communicator"}
	for _, role := range roles {
		if strings.Contains(id, role) {
			return role
		}
	}
	return ""
}

func resolveAgentAlias(dataPath, agentsPath, raw string) (string, error) {
	key := strings.TrimSpace(raw)
	key = strings.TrimPrefix(key, "@")
	if key == "" {
		return "", os.ErrNotExist
	}

	stateMatches := make(map[string]struct{})

	// Prefer resolving from persisted simulation agents (authoritative for UI).
	if entries, err := os.ReadDir(filepath.Join(dataPath, "agents")); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			id := entry.Name()
			info, err := loadAgentFromState(dataPath, id)
			if err != nil {
				continue
			}
			if strings.EqualFold(info.Name, key) || strings.EqualFold(info.ID, key) {
				stateMatches[info.ID] = struct{}{}
			}
		}
	}

	if len(stateMatches) == 1 {
		for id := range stateMatches {
			return id, nil
		}
	}
	if len(stateMatches) > 1 {
		ids := make([]string, 0, len(stateMatches))
		for id := range stateMatches {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return "", fmt.Errorf("ambiguous agent alias %q (matches: %s)", key, strings.Join(ids, ", "))
	}

	cfgMatches := make(map[string]struct{})

	// Fall back to config identities (useful for manually curated agents not present in state).
	if entries, err := os.ReadDir(agentsPath); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			id := entry.Name()
			info, err := loadAgent(agentsPath, id)
			if err != nil {
				continue
			}
			if strings.EqualFold(info.Name, key) || strings.EqualFold(info.ID, key) {
				if info.ID != "" {
					cfgMatches[info.ID] = struct{}{}
				} else {
					cfgMatches[id] = struct{}{}
				}
			}
		}
	}

	if len(cfgMatches) == 0 {
		return "", os.ErrNotExist
	}
	if len(cfgMatches) > 1 {
		ids := make([]string, 0, len(cfgMatches))
		for id := range cfgMatches {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return "", fmt.Errorf("ambiguous agent alias %q (matches: %s)", key, strings.Join(ids, ", "))
	}

	for id := range cfgMatches {
		return id, nil
	}
	return "", os.ErrNotExist
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

func resolveFeedLog(dataPath, requested string) (path string, name string, err error) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		name = filepath.Base(requested)
		if name != requested {
			return "", "", fmt.Errorf("invalid log path")
		}
		if !strings.HasPrefix(name, "logs") || !strings.HasSuffix(name, ".jsonl") {
			return "", "", fmt.Errorf("invalid log file")
		}
		path = filepath.Join(dataPath, name)
		st, statErr := os.Stat(path)
		if statErr != nil {
			return "", "", statErr
		}
		if st.IsDir() {
			return "", "", fmt.Errorf("log is a directory")
		}
		return path, name, nil
	}

	entries, err := os.ReadDir(dataPath)
	if err != nil {
		return "", "", err
	}

	var newestName string
	var newestMod time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		n := entry.Name()
		if !strings.HasPrefix(n, "logs") || !strings.HasSuffix(n, ".jsonl") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		if newestName == "" || info.ModTime().After(newestMod) {
			newestName = n
			newestMod = info.ModTime()
		}
	}

	if newestName == "" {
		return "", "", os.ErrNotExist
	}

	return filepath.Join(dataPath, newestName), newestName, nil
}

func loadFeedEvents(path string, limit int) ([]FeedEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Use a ring buffer so we don't keep the whole log in memory.
	ring := make([]FeedEvent, limit)
	idx := 0
	n := 0

	scanner := bufio.NewScanner(file)
	// JSONL lines can be large because prompt/response are logged verbatim.
	scanner.Buffer(make([]byte, 0, 256*1024), 8*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev FeedEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// File may be actively appended; ignore partial lines.
			continue
		}
		ring[idx] = ev
		idx = (idx + 1) % limit
		if n < limit {
			n++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	out := make([]FeedEvent, 0, n)
	if n == 0 {
		return out, nil
	}

	if n < limit {
		out = append(out, ring[:n]...)
		return out, nil
	}

	// Oldest entry is at idx when the ring is full.
	for i := 0; i < limit; i++ {
		out = append(out, ring[(idx+i)%limit])
	}
	return out, nil
}

func loadFeedEventsAll(dataPath string, limit int) ([]FeedEvent, error) {
	entries, err := os.ReadDir(dataPath)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "logs") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		paths = append(paths, filepath.Join(dataPath, name))
	}
	if len(paths) == 0 {
		return nil, os.ErrNotExist
	}

	all := make([]FeedEvent, 0, limit*minInt(len(paths), 10))
	for _, path := range paths {
		evs, err := loadFeedEvents(path, limit)
		if err != nil {
			continue
		}
		all = append(all, evs...)
	}

	sort.SliceStable(all, func(i, j int) bool {
		if all[i].SimTime.Equal(all[j].SimTime) {
			return all[i].Timestamp.After(all[j].Timestamp)
		}
		return all[i].SimTime.After(all[j].SimTime)
	})
	if len(all) > limit {
		all = all[:limit]
	}

	return all, nil
}

func hydrateFeedEventsFromDailyNotes(dataPath string, events []FeedEvent) {
	if len(events) == 0 {
		return
	}

	// Cache: daily file path -> timestamp -> entry
	cache := make(map[string]map[string]DailyEntry)
	missing := make(map[string]bool)

	for i := range events {
		ev := &events[i]
		if ev.AgentID == "" || ev.SimTime.IsZero() {
			continue
		}

		dateKey := ev.SimTime.Format("2006-01-02")
		dailyPath := filepath.Join(dataPath, "agents", ev.AgentID, "daily", dateKey+".jsonl")
		if missing[dailyPath] {
			continue
		}

		entriesByTS, ok := cache[dailyPath]
		if !ok {
			entries, err := readDailyEntries(dailyPath)
			if err != nil {
				missing[dailyPath] = true
				continue
			}
			entriesByTS = make(map[string]DailyEntry, len(entries))
			for _, entry := range entries {
				if entry.Timestamp == "" {
					continue
				}
				entriesByTS[entry.Timestamp] = entry
			}
			cache[dailyPath] = entriesByTS
		}

		// Daily notes use RFC3339 (seconds precision). Feed events use time.Time
		// marshaled with RFC3339Nano; normalize to seconds.
		tsKey := ev.SimTime.Truncate(time.Second).Format(time.RFC3339)
		entry, ok := entriesByTS[tsKey]
		if !ok {
			continue
		}
		if entry.Prompt != "" {
			ev.Prompt = entry.Prompt
		}
		if entry.Reply != "" {
			ev.Response = entry.Reply
		}
	}
}

func enrichFeedEvents(dataPath string, events []FeedEvent) {
	if len(events) == 0 {
		return
	}

	for i := range events {
		if events[i].AgentID != "" {
			events[i].ActorURL = "/agent/" + events[i].AgentID
		}
	}

	forum, err := loadForum(dataPath)
	if err != nil || forum == nil {
		return
	}

	postsByAuthor := make(map[string][]*types.Publication)
	commentsByAuthor := make(map[string][]*types.Publication)
	for _, pub := range forum.AllPublications() {
		if pub == nil {
			continue
		}
		if pub.AuthorID == "" {
			continue
		}
		if pub.IsComment {
			commentsByAuthor[pub.AuthorID] = append(commentsByAuthor[pub.AuthorID], pub)
			continue
		}
		postsByAuthor[pub.AuthorID] = append(postsByAuthor[pub.AuthorID], pub)
	}

	for _, pubs := range postsByAuthor {
		sortPublicationsByTimeDesc(pubs)
	}
	for _, pubs := range commentsByAuthor {
		sortPublicationsByTimeDesc(pubs)
	}

	maxDelta := 10 * time.Minute
	for i := range events {
		ev := &events[i]
		if ev.AgentID == "" {
			continue
		}
		if ev.ContentURL != "" {
			continue
		}

		// Prioritize linking to actual content created in this event.
		if containsString(ev.ToolCalls, "create_post") {
			if post := findClosestByTime(postsByAuthor[ev.AgentID], ev.Timestamp, maxDelta); post != nil {
				ev.ContentKind = "forum_post"
				ev.ContentID = post.ID
				ev.ContentTitle = post.Title
				ev.ContentURL = "/forum?post=" + post.ID
				continue
			}
		}

		if containsAnyString(ev.ToolCalls, []string{"comment", "request_consensus"}) {
			if comment := findClosestByTime(commentsByAuthor[ev.AgentID], ev.Timestamp, maxDelta); comment != nil {
				rootID := forum.ResolveRootPostID(comment.ID)
				if rootID == "" {
					rootID = comment.ParentID
				}
				if rootID != "" {
					root := forum.Get(rootID)
					title := ""
					if root != nil {
						title = root.Title
					}
					if title == "" {
						title = "Open thread"
					}

					ev.ContentKind = "forum_comment"
					ev.ContentID = comment.ID
					ev.ContentTitle = title
					ev.ContentURL = "/forum?post=" + rootID + "#" + comment.ID
				}
			}
		}
	}
}

func containsString(items []string, target string) bool {
	for _, v := range items {
		if v == target {
			return true
		}
	}
	return false
}

func containsAnyString(items []string, targets []string) bool {
	for _, t := range targets {
		if containsString(items, t) {
			return true
		}
	}
	return false
}

func findClosestByTime(items []*types.Publication, at time.Time, maxDelta time.Duration) *types.Publication {
	if len(items) == 0 || at.IsZero() {
		return nil
	}
	var best *types.Publication
	var bestDelta time.Duration
	for _, item := range items {
		if item == nil || item.PublishedAt.IsZero() {
			continue
		}
		delta := item.PublishedAt.Sub(at)
		if delta < 0 {
			delta = -delta
		}
		if maxDelta > 0 && delta > maxDelta {
			continue
		}
		if best == nil || delta < bestDelta {
			best = item
			bestDelta = delta
		}
	}
	return best
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
