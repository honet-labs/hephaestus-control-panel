# Branch-aware updates and recovery from the legacy updater

The legacy `update-version.sh` fetched `main`, stashed local files and reset to
`origin/main` regardless of which branch had been pulled. Running it after
`git pull origin dev` could therefore rebuild the old UI. Pulling a named branch
also does not rename/switch the current branch.

The corrected updater uses `scripts/update-source.sh`: select the branch via
`HCP_UPDATE_BRANCH`, then the saved local Git setting `hcp.updateBranch`, otherwise
the current branch. It switches to that branch, allows only fast-forward updates,
and saves the successful selection for subsequent updates. A fetch failure, dirty
tracked file, unpushed commit, divergence or downgrade stops the update before a
build. There is no automatic stash or hard reset. Untracked files such as `certs/`
are retained; a checkout conflict stops Git rather than deleting those files.

## One-time recovery on an installation still running the old updater

From the actual installation directory, first review `git status --short`. If the
only changes are expected untracked certificates, use:

```bash
git fetch origin
git switch dev
git merge --ff-only origin/dev
HCP_UPDATE_BRANCH=dev bash ./update-version.sh
```

Stop on any error; do not continue to the next command or use reset/clean to
force it. Review/preserve tracked local modifications first. Crucially, fetch and
switch to the corrected updater before running it: setting HCP_UPDATE_BRANCH on
the old updater does not change its hardcoded main behavior.

Subsequent updates can use `bash ./update-version.sh`; the selected `dev` branch
is saved in this repository's local Git configuration. The script prints source
branch and commit before rebuilding. A source pull alone does not replace running
Docker containers. Existing `.env`, certificates, database volumes and encryption
keys must be preserved. Hard refresh the browser after a successful frontend
rebuild; if the old sidebar persists, verify the installation/Compose project and
reverse proxy actually serve the updated panel.

`bash tests/scripts/update-source-test.sh` exercises disposable Git repositories:
explicit/saved dev selection, preservation of certificates/local edits, rejection
of downgrade and unpushed commits, and failed fetch. No production repository or
Docker service is touched by this test.
