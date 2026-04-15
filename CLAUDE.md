# envsecrets - Claude Project Instructions

A CLI tool for managing environment secrets with dual-backend storage: macOS Keychain (fast, local) and 1Password (durable, cross-machine sync).

## INDEX.md Workflow

When updating code, ALWAYS reference the `INDEX.md` file to help you find relevant files prior to doing full scans and updating code.

After making code changes ALWAYS update the `INDEX.md` to reflect any new changes that need to be mentioned there.

## Planning for Smaller Models

When creating implementation plans, write them so that Haiku 4.5 can execute the work. This means:

- **Be explicit about file paths.** Always include the full path to every file that needs to be created or modified.
- **Show the exact code.** Don't describe what code to write — include the literal code blocks with the content to add, remove, or change. Use diff-style before/after snippets when editing existing files.
- **One step, one action.** Each step should do exactly one thing: create one file, edit one function, add one import. Never combine multiple changes into a single step.
- **Spell out the "why" inline.** A smaller model won't infer intent from context. If a step exists because of a constraint (e.g., "Keychain Set does delete-then-add to avoid duplicates"), state that reason in the step itself.
- **Reference INDEX.md entries.** Point to the specific INDEX.md entry for any file being touched so the executing model can read it for context.
- **Include verification commands.** End the plan with the exact shell commands to run (e.g., `go build ./...`, `go vet ./...`, test commands) so the model knows how to confirm the work is correct.
- **No ambiguity.** Avoid phrases like "update as needed", "add appropriate error handling", or "follow existing patterns". Instead, show the exact error handling code, the exact pattern to follow.
