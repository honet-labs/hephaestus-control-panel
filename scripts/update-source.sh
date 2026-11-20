#!/usr/bin/env bash
# Fast-forward source updates only. Never stash/reset local installation data.
set -euo pipefail

branch="${HCP_UPDATE_BRANCH:-$(git config --local --get hcp.updateBranch || git branch --show-current)}"
if [ -z "$branch" ] || ! git check-ref-format --branch "$branch" >/dev/null 2>&1; then
    echo '[ERROR] Select an update branch explicitly with HCP_UPDATE_BRANCH (for example dev).' >&2
    exit 1
fi
if ! git diff --quiet || ! git diff --cached --quiet; then
    echo '[ERROR] Tracked files have local changes. Review/preserve them before updating; nothing was stashed or reset.' >&2
    exit 1
fi

echo "[SOURCE] Fetching origin/$branch"
git fetch --no-tags origin "refs/heads/$branch:refs/remotes/origin/$branch"
target="refs/remotes/origin/$branch"
if ! git merge-base --is-ancestor HEAD "$target"; then
    echo '[ERROR] Selected update would discard local commits or downgrade/diverge. Resolve the branch explicitly; update stopped.' >&2
    exit 1
fi
if git show-ref --verify --quiet "refs/heads/$branch" && ! git merge-base --is-ancestor "refs/heads/$branch" "$target"; then
    echo '[ERROR] The selected local branch has commits not present on the remote. Update stopped.' >&2
    exit 1
fi

if [ "$(git branch --show-current)" != "$branch" ]; then
    if git show-ref --verify --quiet "refs/heads/$branch"; then
        git switch "$branch"
    else
        git switch --create "$branch" --track "origin/$branch"
    fi
fi
git merge --ff-only "$target"
git config --local hcp.updateBranch "$branch"
echo "[SOURCE] Ready: branch=$(git branch --show-current) commit=$(git rev-parse --short HEAD)"
