# AI Usage

This project is developed collaboratively between one human and Claude (Anthropic). This document is an honest account of what each party contributes.

---

## What Claude does

**All implementation.** Every Go source file in this repository — the `internal/keychain`, `internal/onepassword`, `internal/secrets`, `internal/config`, and `cmd` packages, along with their tests — was written by Claude. This includes the interfaces, the error types, the `security` and `op` CLI wrappers, the Cobra command wiring, the TOML config loading, and the test stubs.

**All documentation.** The README, INDEX.md, BASH_FUNCTIONS.md, CLAUDE.md, and this file were written by Claude. So were the PR descriptions, commit messages, and inline code comments.

**Research.** Competitive analysis (the comparison table in the README) was produced by Claude fetching and reading the homepages and docs of each listed tool.

**Planning.** Before larger changes, Claude produces explicit implementation plans with file paths, exact code snippets, and verification commands. These are stored in `.claude/plans/` and serve as working documents during implementation.

**Git operations.** Branching, committing, pushing, and opening pull requests are all done by Claude through the shell. The human merges.

---

## What the human does

**Decides what to build.** Every feature, every constraint, and every design direction comes from the human. Claude does not propose features unprompted.

**Prompts.** The human describes what they want in plain language — sometimes a single sentence, sometimes with specific requirements. Claude translates that into working code and documentation.

**Reviews.** Every pull request is read by the human before it is merged. The code, the docs, the PR description — all of it. Nothing ships without human eyes on it.

**Validates in the real world.** The human runs the actual binary on their machine. This is where real bugs surface. For example: a `exit status 44` error appeared when the macOS login keychain entry for the vault password was missing — a failure mode that only shows up against a real Keychain, not in tests. The human reported it; Claude diagnosed and fixed it.

**Guides and corrects.** When Claude's approach is technically valid but wrong for the product, the human says so. One concrete example: an early recovery strategy for a missing keychain password involved deleting and recreating the keychain file. The human rejected this immediately — *"I cannot allow that file to ever become unrecoverable just by the user using the CLI tool"* — and specified the correct approach: write an access-details file at creation time and fall back to it transparently. Claude implemented that instead.

**Makes judgment calls.** Decisions with product implications belong to the human. Which defaults to ship, what goes in the README, whether a feature is worth adding — all of that is human-decided.

---

## How a typical session works

1. The human opens a session and describes what they want.
2. Claude reads the relevant source files and `INDEX.md` to understand the current state.
3. For non-trivial changes, Claude writes an implementation plan and the human approves it (or redirects).
4. Claude implements the change, runs the pre-commit hooks (`golangci-lint`, `go test -race`), and opens a PR.
5. The human reviews and merges (or asks for changes).
6. Claude syncs `main` and starts the next branch.

Sessions are short — typically one feature or one document at a time. The human stays in the loop at every step.

---

## What this means for the codebase

The code and documentation reflect Claude's understanding of what the human asked for, filtered through the human's review. Bugs that survive to `main` are bugs that neither party caught — Claude in implementation, the human in review and real-world testing.

The architectural decisions (dedicated keychain file per vault, fallback to access-details file, 1Password as best-effort not required, macOS-only for now) are the human's. The implementations of those decisions are Claude's.
