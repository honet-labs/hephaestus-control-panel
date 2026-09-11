#!/usr/bin/env bash
# Wrapper script for update-version.sh
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$DIR/update-version.sh" "$@"
