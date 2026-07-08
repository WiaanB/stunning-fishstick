#!/usr/bin/env bash
set -euo pipefail

input="$(cat)"

if command -v jq >/dev/null 2>&1; then
  stop_hook_active="$(printf '%s' "$input" | jq -r '.stop_hook_active // false' 2>/dev/null || echo false)"
else
  stop_hook_active=false
fi

if [ "$stop_hook_active" = "true" ]; then
  exit 0
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root" || exit 0

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  exit 0
fi

changed="$(git status --porcelain 2>/dev/null | awk '{print $2}')"

code_changed="$(printf '%s\n' "$changed" | grep -E '^(internal|cmd)/.*\.go$' || true)"
docs_changed="$(printf '%s\n' "$changed" | grep -E '^taxi-platform/' || true)"

if [ -n "$code_changed" ] && [ -z "$docs_changed" ]; then
  files_list="$(printf '%s' "$code_changed" | sed 's/^/- /')"
  reason=$(printf 'Go code changed under internal/ or cmd/ but the taxi-platform/ vault was not updated in this turn. Changed files:\n%s\n\nInvoke the vault-docs skill (.claude/skills/vault-docs/SKILL.md) to update the matching taxi-platform/Features/ note (Capabilities, Implementation, Status) before finishing.' "$files_list")

  if command -v jq >/dev/null 2>&1; then
    jq -n --arg reason "$reason" '{decision: "block", reason: $reason}'
  else
    escaped_reason=$(printf '%s' "$reason" | sed 's/\\/\\\\/g; s/"/\\"/g' | awk '{printf "%s\\n", $0}')
    printf '{"decision": "block", "reason": "%s"}\n' "$escaped_reason"
  fi
fi

exit 0
