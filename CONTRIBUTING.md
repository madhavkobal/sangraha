# Contributing to sangraha

Thank you for your interest in contributing. This document covers the essentials for getting started.

## Before You Open a PR

- Read [CLAUDE.md](CLAUDE.md) — it is the authoritative guide for code conventions, architecture decisions, and security rules.
- Read [PLAN.md](PLAN.md) — it describes which phase/sprint a feature belongs to. Please align your contribution with the current phase.
- For significant new features or architectural changes, open an issue first to discuss before writing code.
- Search existing issues and PRs to avoid duplicating effort.

## Developer Certificate of Origin (DCO)

All commits must be signed off:

```bash
git commit -s -m "feat: add presigned URL generation"
```

By signing off, you certify that you wrote the code or have the right to contribute it under the Apache 2.0 license. See [https://developercertificate.org](https://developercertificate.org).

## Development Setup

```bash
git clone https://github.com/madhavkobal/sangraha
cd sangraha
go mod download
make build
make test
```

## Pull Request Checklist

Before opening a PR, ensure:

- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run` exits 0
- [ ] `go vet ./...` exits 0
- [ ] New packages have ≥ 80% test coverage
- [ ] All new exported symbols have doc comments
- [ ] No secrets are logged or hardcoded
- [ ] S3 operations return XML errors (never JSON from S3 endpoints)
- [ ] CLAUDE.md updated if a new convention was established

## Commit Message Format

```
<type>: <short imperative description>

[optional body]
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `perf`, `security`

Examples:
```
feat: add presigned URL generation for GetObject
fix: correct ETag quoting in multipart complete response
test: add integration test for ListObjectsV2 pagination
```

## Reporting Security Issues

Do **not** file a public GitHub issue for security vulnerabilities. Use [GitHub Security Advisories](https://github.com/madhavkobal/sangraha/security/advisories/new) to report privately.

## Code of Conduct

Be respectful and constructive. We follow the [Contributor Covenant](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).
