#!/usr/bin/env bash
# goreleaser-prepare.sh <version>
# Stamps the release version into plugin.json and validates strict contracts.
# Sets a trap so plugin.json is restored on any error, matching the cleanup
# done by the GoReleaser after-hook on success.
set -euo pipefail

VERSION="${1:?usage: goreleaser-prepare.sh <version>}"

cp plugin.json plugin.json.orig
cleanup() { mv plugin.json.orig plugin.json; }
trap cleanup ERR

# Stamp version (use temp file to avoid .bak intermediates).
tmp=$(mktemp)
sed "s/\"version\": \".*\"/\"version\": \"${VERSION}\"/" plugin.json > "$tmp" && mv "$tmp" plugin.json

# Update download URLs.
tmp=$(mktemp)
sed "s|releases/download/v[^/]*/|releases/download/v${VERSION}/|g" plugin.json > "$tmp" && mv "$tmp" plugin.json

# Validate stamped manifest against strict contracts.
go run github.com/GoCodeAlone/workflow/cmd/wfctl@v0.20.1 plugin validate --file plugin.json --strict-contracts
