#!/usr/bin/env bash
set -euo pipefail

input="$(cat)"

if command -v jq >/dev/null 2>&1; then
  stop_hook_active="$(printf '%s' "$input" | jq -r '.stop_hook_active // false' 2>/dev/null || echo false)"
  transcript_path="$(printf '%s' "$input" | jq -r '.transcript_path // empty' 2>/dev/null || echo '')"
else
  stop_hook_active=false
  transcript_path=""
fi

if [ "$stop_hook_active" = "true" ]; then
  exit 0
fi

if [ -z "$transcript_path" ] || [ ! -f "$transcript_path" ]; then
  exit 0
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root" || exit 0

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  exit 0
fi

# A plan's work only counts as "done" once the tree is fully clean: either
# nothing changed, or everything the plan touched has already been committed.
# Anything still uncommitted means work is in flight, so leave plan files
# alone rather than guess.
if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
  exit 0
fi

plan_paths="$(grep -ohE 'plan has been saved to: [^"\\]+\.md' "$transcript_path" 2>/dev/null | sed -E 's/^plan has been saved to: //' | sort -u || true)"

removed=""
while IFS= read -r plan_path; do
  [ -z "$plan_path" ] && continue

  # Only ever touch files that look like Claude Code plan-mode scratch files:
  # under the user's home directory, inside a "plans" directory.
  case "$plan_path" in
    "$HOME"/*) ;;
    *) continue ;;
  esac
  case "$plan_path" in
    */plans/*.md) ;;
    *) continue ;;
  esac

  if [ -f "$plan_path" ]; then
    rm -f -- "$plan_path"
    removed="${removed}- ${plan_path}"$'\n'
  fi
done <<< "$plan_paths"

if [ -n "$removed" ]; then
  if command -v jq >/dev/null 2>&1; then
    jq -n --arg msg "Cleaned up completed plan file(s):
$removed" '{systemMessage: $msg}'
  fi
fi

exit 0
