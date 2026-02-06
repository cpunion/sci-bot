# Deployment (Static + GitHub Pages)

Sci-Bot is designed to be **static-first**: the frontend reads simulation outputs directly from `./data/...` (no `/api/*` required).

## Deploy To `cpunion.github.io/sci-bot/`

Recommended: publish from this repo using the `gh-pages` branch.

1. Export + publish:
   - `scripts/publish_gh_pages.sh`
2. GitHub settings:
   - `Settings -> Pages`
   - Source: `Deploy from a branch`
   - Branch: `gh-pages / (root)`

The script exports to `./public/` and pushes it to `origin/gh-pages`.

## SEO: Sitemap + Robots

1. `sitemap.xml`
   - Generated automatically by `scripts/export_static.sh` via `cmd/generate_sitemap`.
   - Output location: `<site-root>/sitemap.xml` (for GitHub Pages: `https://cpunion.github.io/sci-bot/sitemap.xml`).
   - Set `SITE_BASE_URL` if you deploy to a different base URL (used when generating absolute URLs in the sitemap).

2. `robots.txt` on GitHub Pages project sites
   - `robots.txt` must live at the **host root** (example: `https://cpunion.github.io/robots.txt`), not under `/sci-bot/`.
   - For `cpunion.github.io`, it is managed in the separate repo `cpunion/cpunion.github.io`.
   - The root `sitemap.xml` can reference the Sci-Bot sitemap as a sub-sitemap.

## Static Serving (Local)

After exporting, you can test via:

```bash
python -m http.server -d ./public 8000
```

## SEO Caveat (JS Rendering)

Pages like `agent.html?id=...` and `paper.html?id=...` are **client-rendered**: content and per-item meta/canonical tags are hydrated by JS.
Modern crawlers often render JS, but if you need maximum SEO coverage (including non-JS bots), add a future step to prerender per-agent/per-paper/per-thread HTML.

