# Working in this repo

This is a Claude-driven project. These rules exist so the user can trust code
that lands here without reviewing every line — follow them by default, don't
ask permission to follow them.

## Spec-driven development

- No work happens without a spec. Specs live in `/plans`, one file per spec,
  numbered `NNNN-short-name.md` (see `plans/0001-v1-exporter.md` for the
  format: motivation, goals, non-goals, architecture, stages, decisions log).
- Multiple specs can exist. A spec builds on prior ones rather than
  duplicating their scope.
- If asked to build something with no corresponding spec (or a change that
  doesn't fit the current one), stop and write/extend the spec first. Get it
  reviewed before implementing.
- Each spec is broken into stages, each independently mergeable with a stated,
  checkable "done when" criterion. Work one stage at a time.
- **At the end of every stage: stop.** Run tests, `vet`, and `fmt`, confirm
  the stage's "done when" criterion is actually met, summarize what changed,
  and commit. Do not silently continue to the next stage in the same sitting
  — pause so the user can evaluate and redirect.
- Record real decisions (not obvious ones) in the spec's own decisions log as
  they're made, not after the fact from memory.
- Update a spec's `Status` field (`draft` → `in-progress` → `done`) as work
  progresses.

## Code style

- Minimal comments. Only when the *why* isn't obvious from the code itself —
  never restate what a line does.
- Concise, simple over clever. No abstractions, config knobs, or error
  handling for cases that can't happen. Three similar lines beat a premature
  helper.
- Prefer the standard library. Reach for a third-party dependency only when
  the stdlib genuinely can't do it, and note why in the spec's decisions log.
- Match the conventions already established in the active spec (error
  wrapping, logging shape, package boundaries, etc.) rather than introducing
  new ones per file.

## Testing

- Tests must pass before every commit — no exceptions, no `--no-verify`.
- Run `vet` and `fmt` before committing too; fix anything they flag rather
  than working around it.
- Test what has logic (parsing, validation, collectors), skip pure wiring
  (`main.go`, generated code, descriptor-only files) — see each spec's
  testing strategy section for specifics.

## Git

- Claude may commit directly — no need to ask before each commit.
- Work directly on `main`. No feature branches per spec/stage.
- **Do not push.** Commit locally only; the user pushes when ready.
- One commit per completed stage (or per meaningful fix within a stage).
  Don't bundle unrelated stages into one commit.
- Commit message format: conventional-commit prefix + 3-6 words, no body.
  e.g. `feat: add pod domain collector`, `test: cover vec reset on error`,
  `fix: correct gpu index label`.
- **No Claude attribution.** No `Co-Authored-By: Claude`, no `Claude-Session:`
  link, no "Generated with Claude Code" trailer, nothing that marks a commit
  as AI-authored. Commits use the repo's normal local git identity, same as
  any other commit here.
- Never amend or force-push. If a pre-commit check fails, fix and make a new
  commit.

## When blocked

If something is genuinely ambiguous — the spec doesn't say, or two
reasonable approaches conflict — stop and ask rather than guessing and
writing it down as a decision after the fact.
