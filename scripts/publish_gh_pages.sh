#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

REMOTE="${REMOTE:-origin}"
BRANCH="${BRANCH:-gh-pages}"
DATA_DIR="${DATA_DIR:-./data/adk-simulation}"
WEB_DIR="${WEB_DIR:-./web}"
OUT_DIR="${OUT_DIR:-./public}"
WORKTREE_DIR="${WORKTREE_DIR:-./.worktrees/gh-pages}"

usage() {
  cat <<'EOF'
publish_gh_pages.sh

Export the static site and publish it to a `gh-pages` branch using `git worktree`.

Environment variables:
  REMOTE=origin
  BRANCH=gh-pages
  DATA_DIR=./data/adk-simulation
  WEB_DIR=./web
  OUT_DIR=./public
  WORKTREE_DIR=./.worktrees/gh-pages

Usage:
  scripts/publish_gh_pages.sh
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

scripts/export_static.sh -data "$DATA_DIR" -web "$WEB_DIR" -out "$OUT_DIR"

mkdir -p "$(dirname "$WORKTREE_DIR")"

if [[ -d "$WORKTREE_DIR" ]]; then
  git worktree remove --force "$WORKTREE_DIR" >/dev/null 2>&1 || true
  rm -rf "$WORKTREE_DIR"
fi

OUT_ABS="$(cd "$OUT_DIR" && pwd)"

if git ls-remote --heads "$REMOTE" "$BRANCH" | grep -q .; then
  git fetch "$REMOTE" "$BRANCH"
  git worktree add -B "$BRANCH" "$WORKTREE_DIR" "$REMOTE/$BRANCH"
else
  git worktree add --orphan -b "$BRANCH" "$WORKTREE_DIR"
fi

(
  cd "$WORKTREE_DIR"

  # Remove everything except the worktree's .git pointer.
  find . -mindepth 1 -maxdepth 1 ! -name '.git' -exec rm -rf {} +

  rsync -a "$OUT_ABS"/ ./

  touch .nojekyll

  git add -A

  if git diff --cached --quiet; then
    echo "No changes to publish."
    exit 0
  fi

  msg="Deploy $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  git commit -m "$msg"
)

git push "$REMOTE" "$BRANCH"

echo "Published to $REMOTE/$BRANCH"
