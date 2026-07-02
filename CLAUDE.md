# Agent instructions — poe-fissure

> Guidance for AI coding agents working in this repo.

## Task tracking — use GitHub Issues

Track all work in **GitHub Issues**, not local markdown. Do not create or maintain a
`TODO.md`/task list for new work. (The `TODO.md` here, if present, is a one-time seed
that's already imported — Issues are the source of truth.)

- Find work:  `gh issue list`  (filter by `--label`, `--milestone`, state).
- File work:  `gh issue create --title "<imperative>" --body "<what + where + acceptance>" --label <area>,<type>,priority:<high|medium|low> [--milestone "Phase N"]`
- Close it in the PR/commit that finishes it (`Fixes #<n>`), or `gh issue close <n>`.
- Labels: type = feature|bug|chore|docs|research; priority:high|medium|low; area labels
  lower-case. Reuse existing labels — check `gh label list` first.
