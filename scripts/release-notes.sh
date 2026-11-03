#!/usr/bin/env bash
# Prints release notes (markdown) for a tag, built from the changelog lines
# that each sync commit carries in its message body:
#
#   - feat(auth): log logins, failed logins and logouts
#   - fix(db): bind each request to its database
#
# Lines between the previous tag and this one are grouped by Conventional
# Commit type. Used by .github/workflows/release.yml; run locally with
#   scripts/release-notes.sh v1.2.0
set -euo pipefail

TAG="${1:?usage: $0 <tag>}"
REPO="${GITHUB_REPOSITORY:-virlatinus/UrSchema-Community}"
IMAGE="ghcr.io/$(echo "$REPO" | tr '[:upper:]' '[:lower:]')"

PREV="$(git describe --tags --abbrev=0 "$TAG^" 2>/dev/null || true)"
RANGE="${PREV:+$PREV..}$TAG"

# "- type(scope)!: subject" lines, first occurrence only, in history order.
ENTRIES="$(git log --reverse --format=%B "$RANGE" \
  | grep -E '^- [a-z]+(\([^)]+\))?!?: ' \
  | sed 's/^- //' \
  | awk '!seen[$0]++' || true)"

section() { # title, type regex
  local lines
  lines="$(printf '%s\n' "$ENTRIES" | grep -E "^($2)(\(|!|:)" || true)"
  [[ -z "$lines" ]] && return 0
  printf '### %s\n\n' "$1"
  # "fix(db): thing" -> "- **db:** thing"; "!" before ":" marks a breaking change
  printf '%s\n' "$lines" | sed -E \
    -e 's/^[a-z]+\(([^)]+)\)!: (.*)$/- **\1:** **(breaking)** \2/' \
    -e 's/^[a-z]+\(([^)]+)\): (.*)$/- **\1:** \2/' \
    -e 's/^[a-z]+!: (.*)$/- **(breaking)** \1/' \
    -e 's/^[a-z]+: (.*)$/- \1/'
  printf '\n'
}

{
  if [[ -z "$ENTRIES" ]]; then
    printf 'No changelog entries were recorded for this release; see the full changelog below.\n\n'
  else
    printf '## Changes\n\n'
    section "Features" "feat"
    section "Fixes" "fix"
    section "Security" "sec|security"
    section "Performance" "perf"
    section "Documentation" "docs"
    section "Maintenance" "build|ci|chore|refactor|test|style|revert"
  fi

  VERSION="${TAG#v}"
  printf '## Install\n\n'
  printf '**Docker:** `%s:%s`\n\n' "$IMAGE" "$VERSION"
  printf '**Tarballs:** Linux amd64 and arm64, attached below. Unpack, then start `./main` from `app/bin`.\n\n'
  printf 'See the [README](https://github.com/%s#-getting-started) for setup and the [Security section](https://github.com/%s#-security) before exposing it on a network.\n\n' "$REPO" "$REPO"

  if [[ -n "$PREV" ]]; then
    printf '**Full Changelog**: https://github.com/%s/compare/%s...%s\n' "$REPO" "$PREV" "$TAG"
  else
    printf '**Full Changelog**: https://github.com/%s/commits/%s\n' "$REPO" "$TAG"
  fi
}
