#!/usr/bin/env bash
# Fail when the exported API changed incompatibly since BASE and the same
# change doesn't add a BREAKING entry to CHANGELOG.md.
#
#   .github/scripts/api-compat.sh <base-commit>
set -euo pipefail

base="${1:-}"
if [ -z "$base" ] || [ "$base" = "0000000000000000000000000000000000000000" ]; then
  echo "No base commit; skipping"
  exit 0
fi

module="github.com/DrewBradfordXYZ/quickbase-go/v2"
apidiff="golang.org/x/exp/cmd/apidiff@v0.0.0-20261109185158-a52e3aef5920"
work="$(mktemp -d)"
trap 'git worktree remove --force "$work/base" >/dev/null 2>&1 || true; rm -rf "$work"' EXIT

git worktree add --quiet --detach "$work/base" "$base"
(cd "$work/base" && go run "$apidiff" -m -w "$work/base.api" "$module")
go run "$apidiff" -m -incompatible "$work/base.api" "$module" > "$work/incompatible.txt"

if [ ! -s "$work/incompatible.txt" ]; then
  echo "No incompatible API changes since $base"
  exit 0
fi

echo "Incompatible API changes since $base ($(grep -c '^- ' "$work/incompatible.txt")):"
cat "$work/incompatible.txt"
if git diff "$base" HEAD -- CHANGELOG.md | grep -q '^+.*BREAKING'; then
  echo "CHANGELOG.md adds a BREAKING entry for them."
  exit 0
fi
echo "::error::Incompatible API changes without a BREAKING entry added to CHANGELOG.md"
exit 1
