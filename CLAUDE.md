# CLAUDE.md — AI Assistant Guide for sangraha

> Last updated: 2026-03-04
> Repository: [madhavkobal/sangraha](http://local_proxy@127.0.0.1:60135/git/madhavkobal/sangraha)

This file provides essential context for AI assistants (Claude Code and similar tools) working in this repository. Keep this document up-to-date as the project evolves.

---

## Repository Status

This repository is currently in its **initial state** — no source files, build system, or application code exist yet. As features are added, update the relevant sections below.

---

## Git Workflow

### Branch Naming

Feature branches created by Claude Code follow a strict naming convention:

```
claude/<slug>-<session-id>
```

Example: `claude/claude-md-mmby5ut451fg1eh7-PRdg9`

**Rules:**
- All Claude Code development branches **must** start with `claude/` and end with the matching session ID
- Pushing to any other pattern will result in a **403 HTTP error**
- Never push to `main` or `master` directly without explicit permission

### Commit Discipline

- Write clear, descriptive commit messages that explain *why*, not just *what*
- One logical change per commit
- Format: `<type>: <short description>` (e.g., `feat: add user authentication`, `fix: handle null token case`)

### Push Procedure

```bash
# Always use -u on first push
git push -u origin <branch-name>

# On network failure, retry with exponential backoff:
# Attempt 1 → wait 2s → Attempt 2 → wait 4s → Attempt 3 → wait 8s → Attempt 4 → wait 16s
```

### Fetch/Pull

```bash
# Prefer fetching specific branches
git fetch origin <branch-name>
git pull origin <branch-name>
```

---

## Codebase Structure

> This section will be filled in once the project structure is established.

```
sangraha/
├── CLAUDE.md          ← This file
└── (source files TBD)
```

When the codebase is set up, document:
- Top-level directories and their purpose
- Entry points (e.g., `main.py`, `index.ts`, `cmd/server/main.go`)
- Key packages/modules and their responsibilities

---

## Technology Stack

> To be determined once the project is initialized.

Document here:
- Primary language(s) and version requirements
- Framework(s) in use
- Key dependencies and why they were chosen

---

## Development Setup

> To be completed once the project has a build system.

Typical sections to add:
1. Prerequisites (runtime versions, tools)
2. Installation steps
3. Environment variable configuration (`.env` / `.env.example`)
4. Running locally
5. Running tests

---

## Testing

> To be documented once tests exist.

Include:
- How to run the full test suite
- How to run a single test
- Coverage thresholds (if enforced)
- Where tests live relative to source code

---

## Linting & Formatting

> To be documented once tooling is configured.

Include:
- Linters in use and how to invoke them
- Auto-formatters and how to run them
- Whether these are enforced in CI

---

## Key Conventions

These conventions apply once code exists. AI assistants should follow them strictly:

### General
- Prefer editing existing files over creating new ones
- Keep changes minimal and focused — do not refactor or improve code beyond what is asked
- Do not add comments, docstrings, or type annotations to code you did not change
- Do not add error handling for scenarios that cannot happen in normal flow

### Security
- Never introduce command injection, XSS, SQL injection, or other OWASP Top 10 vulnerabilities
- Validate at system boundaries (user input, external APIs); trust internal code
- Do not commit secrets or credentials — check for `.env`, key files, etc. before staging

### Reversibility
- Prefer reversible actions; confirm with user before destructive operations
- Never use `--no-verify` to skip hooks without explicit user approval
- Never force-push to shared branches without explicit permission

---

## CI/CD

> To be documented once pipelines are configured.

Include:
- Pipeline provider (GitHub Actions, GitLab CI, etc.)
- Key workflow files and what they do
- How to interpret failures

---

## Updating This File

When the project grows, update CLAUDE.md to reflect:
- New directories or major modules added
- Changes to build/test commands
- New linting rules or formatting requirements
- Architectural decisions and their rationale

AI assistants should treat this file as the authoritative source of project conventions.
