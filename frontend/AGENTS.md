
## Git workflow

<!-- Outside the bmad:context block on purpose: kept across `bmad-project-context` refreshes. -->

- Branch per phase or epic (`phase-1-wails-scaffold`, `epic-1-share-one-file`). Never commit
  directly to `main`.
- Commit at every verified-green milestone — build, tests, and lint passing — rather than
  batching to the end of a task, and **push each commit to `origin`**. Work that is only
  local is invisible to the other agents on this project.
- A finished phase or epic merges into `main` with `--no-ff`, and `main` is pushed, so the
  next branch forks from a real baseline instead of stacking on its predecessor.
- Remote is `https://github.com/jaeson-sandbox/FairDrop.git`; `gh` is authenticated.
