package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type urlset struct {
	XMLName xml.Name   `xml:"urlset"`
	Xmlns   string     `xml:"xmlns,attr"`
	URLs    []urlEntry `xml:"url"`
}

type urlEntry struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type agentsIndex struct {
	Agents []struct {
		ID string `json:"id"`
	} `json:"agents"`
}

type journalIndex struct {
	Publications map[string]struct {
		ID          string `json:"id"`
		PublishedAt string `json:"published_at"`
	} `json:"publications"`
	Pending map[string]struct {
		ID          string `json:"id"`
		PublishedAt string `json:"published_at"`
	} `json:"pending"`
}

type forumIndex struct {
	Posts map[string]struct {
		ID        string `json:"id"`
		IsComment bool   `json:"is_comment"`
		// We only use this when it parses cleanly; otherwise omit lastmod.
		PublishedAt string `json:"published_at"`
	} `json:"posts"`
}

func mustReadJSON(path string, dst any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func parseLastMod(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return ""
	}
	// Omit empty/default timestamps (many pending papers use year 1).
	if t.Year() <= 1970 {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func joinBase(base string, path string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return path
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	path = strings.TrimPrefix(path, "/")
	return base + path
}

func main() {
	var dataDir string
	var outDir string
	var baseURL string
	var includeAgents bool
	var includePapers bool
	var includeForumPosts bool

	flag.StringVar(&dataDir, "data", "./data/adk-simulation", "simulation data directory")
	flag.StringVar(&outDir, "out", "./public", "output directory (site root)")
	flag.StringVar(&baseURL, "base", "https://cpunion.github.io/sci-bot/", "canonical base URL (must include /sci-bot/ for GitHub Pages project site)")
	flag.BoolVar(&includeAgents, "agents", true, "include agent pages in sitemap")
	flag.BoolVar(&includePapers, "papers", true, "include paper pages in sitemap")
	flag.BoolVar(&includeForumPosts, "forum-posts", true, "include forum thread pages in sitemap")
	flag.Parse()

	// Ensure output directory exists.
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir out:", err)
		os.Exit(2)
	}

	// Basic site pages.
	now := time.Now().UTC().Format("2006-01-02")
	entries := []urlEntry{
		{Loc: joinBase(baseURL, ""), LastMod: now, ChangeFreq: "daily", Priority: "1.0"},
		{Loc: joinBase(baseURL, "forum.html"), LastMod: now, ChangeFreq: "daily", Priority: "0.9"},
		{Loc: joinBase(baseURL, "journal.html"), LastMod: now, ChangeFreq: "daily", Priority: "0.9"},
		{Loc: joinBase(baseURL, "feed.html"), LastMod: now, ChangeFreq: "daily", Priority: "0.8"},
	}

	// Agents.
	if includeAgents {
		var agents agentsIndex
		if err := mustReadJSON(filepath.Join(dataDir, "agents", "agents.json"), &agents); err == nil {
			for _, a := range agents.Agents {
				id := strings.TrimSpace(a.ID)
				if id == "" {
					continue
				}
				entries = append(entries, urlEntry{
					Loc:        joinBase(baseURL, "agent.html?id="+url.QueryEscape(id)),
					ChangeFreq: "weekly",
					Priority:   "0.6",
				})
			}
		}
	}

	// Papers.
	if includePapers {
		var journal journalIndex
		if err := mustReadJSON(filepath.Join(dataDir, "journal", "journal.json"), &journal); err == nil {
			addPaper := func(id string, publishedAt string, priority string) {
				id = strings.TrimSpace(id)
				if id == "" {
					return
				}
				entries = append(entries, urlEntry{
					Loc:        joinBase(baseURL, "paper.html?id="+url.QueryEscape(id)),
					LastMod:    parseLastMod(publishedAt),
					ChangeFreq: "monthly",
					Priority:   priority,
				})
			}
			for _, p := range journal.Publications {
				addPaper(p.ID, p.PublishedAt, "0.8")
			}
			for _, p := range journal.Pending {
				addPaper(p.ID, p.PublishedAt, "0.5")
			}
		}
	}

	// Forum threads.
	if includeForumPosts {
		var forum forumIndex
		if err := mustReadJSON(filepath.Join(dataDir, "forum", "forum.json"), &forum); err == nil {
			for _, p := range forum.Posts {
				if p.IsComment {
					continue
				}
				id := strings.TrimSpace(p.ID)
				if id == "" {
					continue
				}
				entries = append(entries, urlEntry{
					Loc:        joinBase(baseURL, "forum.html?post="+url.QueryEscape(id)),
					LastMod:    parseLastMod(p.PublishedAt),
					ChangeFreq: "weekly",
					Priority:   "0.7",
				})
			}
		}
	}

	// Deterministic ordering helps diffs and debugging.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Loc < entries[j].Loc
	})

	s := urlset{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  entries,
	}
	outPath := filepath.Join(outDir, "sitemap.xml")
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create sitemap:", err)
		os.Exit(2)
	}
	defer f.Close()

	if _, err := f.WriteString(xml.Header); err != nil {
		fmt.Fprintln(os.Stderr, "write header:", err)
		os.Exit(2)
	}
	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.Encode(s); err != nil {
		fmt.Fprintln(os.Stderr, "encode sitemap:", err)
		os.Exit(2)
	}
	if err := enc.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, "flush sitemap:", err)
		os.Exit(2)
	}
}
