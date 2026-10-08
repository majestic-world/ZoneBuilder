## Language

- Code and comments: English.
- Replies to the user: Brazilian Portuguese (pt-BR).
- Inflect every word to its actual count: "1 classe", "2 classes". For strings built from a runtime count, branch on the count.

## Commits

- Commit only when the user asks.
- Format: Conventional Commits. The type keeps its spec token (`feat`, `fix`); the description and body are in pt-BR.
- The user is the sole author: omit any `Co-Authored-By` trailer.

## Agent skills

### Issue tracker

Issues are tracked as local markdown files under `.scratch/<feature>/`. See `docs/agents/issue-tracker.md`.

### Triage labels

Default triage vocabulary (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`), recorded on each issue's `Status:` line. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
