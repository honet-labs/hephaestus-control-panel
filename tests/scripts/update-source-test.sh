#!/usr/bin/env bash
set -euo pipefail
helper="$(cd "$(dirname "$0")/../.." && pwd)/scripts/update-source.sh"
tmp="$(mktemp -d)"
trap 'rm -rf -- "$tmp"' EXIT
export GIT_AUTHOR_NAME='Updater Test' GIT_COMMITTER_NAME='Updater Test'
export GIT_AUTHOR_EMAIL='fixture@example.test' GIT_COMMITTER_EMAIL='fixture@example.test'
unset HCP_UPDATE_BRANCH

git init --bare -q "$tmp/origin.git"
git init -q -b main "$tmp/seed"
cd "$tmp/seed"
echo main > source.txt
git add source.txt
git commit -q -m initial
git remote add origin "$tmp/origin.git"
git push -q origin main
git switch -q -c dev
echo dev >> source.txt
git commit -qam dev
git push -q origin dev

git clone -q --branch main "$tmp/origin.git" "$tmp/install"
cd "$tmp/install"
mkdir certs
echo preserve-certificate > certs/test.pem
HCP_UPDATE_BRANCH=dev bash "$helper"
test "$(git branch --show-current)" = dev
test "$(git config --local --get hcp.updateBranch)" = dev
test "$(cat certs/test.pem)" = preserve-certificate
echo 'PASS: explicit dev selection and untracked certificates preserved'

cd "$tmp/seed"
echo newer >> source.txt
git commit -qam newer
git push -q origin dev
expected="$(git rev-parse HEAD)"
cd "$tmp/install"
bash "$helper"
test "$(git rev-parse HEAD)" = "$expected"
echo 'PASS: subsequent update follows saved dev branch'

if HCP_UPDATE_BRANCH=main bash "$helper"; then echo 'FAIL: downgrade allowed'; exit 1; fi
test "$(git rev-parse HEAD)" = "$expected"
test "$(git branch --show-current)" = dev
echo 'PASS: accidental downgrade to older main refused'

echo local-change >> source.txt
if bash "$helper"; then echo 'FAIL: dirty tracked file accepted'; exit 1; fi
grep -q local-change source.txt
test -z "$(git stash list)"
echo 'PASS: local edits retained, never stashed/reset'

git clone -q --branch dev "$tmp/origin.git" "$tmp/diverged"
cd "$tmp/diverged"
echo local-only >> source.txt
git commit -qam local-only
local_head="$(git rev-parse HEAD)"
if HCP_UPDATE_BRANCH=dev bash "$helper"; then echo 'FAIL: unpushed commits accepted'; exit 1; fi
test "$(git rev-parse HEAD)" = "$local_head"
echo 'PASS: unpushed local commits retained and update refused'

git clone -q --branch dev "$tmp/origin.git" "$tmp/unavailable"
cd "$tmp/unavailable"
git remote set-url origin "$tmp/no-such-repository"
if HCP_UPDATE_BRANCH=dev bash "$helper"; then echo 'FAIL: fetch failure ignored'; exit 1; fi
test "$(git rev-parse HEAD)" = "$expected"
echo 'PASS: fetch errors stop update instead of rebuilding stale source'
