#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

DATA_DIR="./data/adk-simulation"
WEB_DIR="./web"
OUT_DIR="./public"
REBUILD_FEED=1
SITE_BASE_URL="${SITE_BASE_URL:-https://cpunion.github.io/sci-bot/}"

usage() {
  cat <<'EOF'
export_static.sh

Export the static site (HTML + assets + data/) into an output directory so it
can be hosted on GitHub Pages or any static file server.

Usage:
  scripts/export_static.sh [-data <dir>] [-web <dir>] [-out <dir>] [-no-rebuild-feed]

Defaults:
  -data ./data/adk-simulation
  -web  ./web
  -out  ./public
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -data)
      DATA_DIR="${2:-}"
      shift 2
      ;;
    -web)
      WEB_DIR="${2:-}"
      shift 2
      ;;
    -out)
      OUT_DIR="${2:-}"
      shift 2
      ;;
    -no-rebuild-feed)
      REBUILD_FEED=0
      shift 1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ ! -d "$DATA_DIR" ]]; then
  echo "Data dir not found: $DATA_DIR" >&2
  exit 2
fi
if [[ ! -d "$WEB_DIR" ]]; then
  echo "Web dir not found: $WEB_DIR" >&2
  exit 2
fi

INDEX_ARGS=(-data "$DATA_DIR")
if [[ "$REBUILD_FEED" -eq 1 ]]; then
  INDEX_ARGS+=(-rebuild-feed)
fi
go run ./cmd/index_data "${INDEX_ARGS[@]}"

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR/assets" "$OUT_DIR/data"

rsync -a "$WEB_DIR"/*.html "$OUT_DIR"/
rsync -a --delete "$WEB_DIR"/assets/ "$OUT_DIR"/assets/

# Copy only the files the static UI needs (avoid publishing rebuild backups).
rsync -a --delete "$DATA_DIR"/agents/ "$OUT_DIR"/data/agents/
rsync -a --delete "$DATA_DIR"/forum/ "$OUT_DIR"/data/forum/
rsync -a --delete "$DATA_DIR"/journal/ "$OUT_DIR"/data/journal/
rsync -a --delete "$DATA_DIR"/feed/ "$OUT_DIR"/data/feed/
rsync -a "$DATA_DIR"/site.json "$OUT_DIR"/data/site.json

if [[ -f "$DATA_DIR/sim_state.json" ]]; then
  rsync -a "$DATA_DIR"/sim_state.json "$OUT_DIR"/data/sim_state.json
fi

shopt -s nullglob
for f in "$DATA_DIR"/logs*.jsonl; do
  rsync -a "$f" "$OUT_DIR"/data/
done
shopt -u nullglob

touch "$OUT_DIR/.nojekyll"

go run ./cmd/generate_sitemap -data "$DATA_DIR" -out "$OUT_DIR" -base "$SITE_BASE_URL" >/dev/null

echo "Exported static site -> $OUT_DIR"
echo "Test locally: python -m http.server -d \"$OUT_DIR\" 8000"
