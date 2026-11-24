#!/usr/bin/env bash
set -euo pipefail

pr="${1:?usage: check-pr.sh <pr-number>}"
repo="${GITHUB_REPOSITORY:-autobrr/dashbrr}"
failed=0

labels=$(gh pr view "$pr" --repo "$repo" --json labels --jq '.labels | length')
if [ "$labels" -eq 0 ]; then
  echo "::error::PR #$pr has no labels. Pick labels from 'gh label list' and add them with 'gh pr edit $pr --add-label <name>'."
  failed=1
fi

attribution='generated (with|by) \[?(claude|codex|copilot)|co-authored-by: *(claude|codex|copilot)|noreply@anthropic\.com|claude\.ai/code/session_|claude-session:'
text=$(gh pr view "$pr" --repo "$repo" --json title,body,commits \
  --jq '.title, .body, (.commits[] | .messageHeadline, .messageBody)')
if matches=$(grep -inE "$attribution" <<<"$text"); then
  echo "::error::PR #$pr has an AI attribution line in its title, body, or a commit message. Remove it:"
  echo "$matches"
  failed=1
fi

exit "$failed"
