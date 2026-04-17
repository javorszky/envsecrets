# envsecrets - Claude Project Instructions

A CLI tool for managing environment secrets with dual-backend storage: macOS Keychain (fast, local) and 1Password (durable, cross-machine sync).

## INDEX.md Workflow

`INDEX.md` is the authoritative code map and API reference. It must stay in sync with the source at all times.

**Before making changes:** consult `INDEX.md` to locate relevant files and understand existing signatures. Prefer it over a full repo scan.

**After every code change:** update the relevant section(s) of `INDEX.md` to reflect:

- New, renamed, or removed files
- New, renamed, or removed structs — including every field (name, type, one-line purpose)
- New, renamed, or removed interfaces — including every method signature
- New, renamed, or removed exported **or** key unexported functions/methods — including the full signature and a one-line description of what it does and what it governs
- Changed default values, changed error names, changed flag names
- Any change to the Architecture section's data-flow description

The level of detail required in `INDEX.md` is: **someone reading only `INDEX.md` should be able to call any function or implement any interface without opening a source file.**

## Planning for Smaller Models

When creating implementation plans, write them so that Haiku 4.5 can execute the work. This means:

- **Be explicit about file paths.** Always include the full path to every file that needs to be created or modified.
- **Show the exact code.** Don't describe what code to write — include the literal code blocks with the content to add, remove, or change. Use diff-style before/after snippets when editing existing files.
- **One step, one action.** Each step should do exactly one thing: create one file, edit one function, add one import. Never combine multiple changes into a single step.
- **Spell out the "why" inline.** A smaller model won't infer intent from context. If a step exists because of a constraint (e.g., "Keychain Set does delete-then-add to avoid duplicates"), state that reason in the step itself.
- **Reference INDEX.md entries.** Point to the specific INDEX.md entry for any file being touched so the executing model can read it for context.
- **Include verification commands.** End the plan with the exact shell commands to run (e.g., `go build ./...`, `go vet ./...`, test commands) so the model knows how to confirm the work is correct.
- **No ambiguity.** Avoid phrases like "update as needed", "add appropriate error handling", or "follow existing patterns". Instead, show the exact error handling code, the exact pattern to follow.

## Branch Workflow

After a feature branch has been merged into main, ALWAYS:

1. Fetch and reset to main: `git fetch origin main && git reset --hard origin/main`
2. Create a new working branch before making any changes: `git checkout -b <descriptive-branch-name>`
