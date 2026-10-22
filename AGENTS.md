# topupsh-tracker Agent Guidelines

**General rules**
- 100% unit test coverage on both frontend and backend
- backend changes must follow [./backend/AGENTS.md](./backend/AGENTS.md)
- frontend changes must follow [./frontend/AGENTS.md](./frontend/AGENTS.md)
- every source file (Go, JS, Vue, CSS, HTML, shell, PowerShell, YAML, Dockerfile, Makefile) starts with the copyright notice (after a shebang or doctype), using that language's comment syntax (`//` shown); `make copyright-fix` adds it and CI fails without it:
  ```
  // Copyright 2026 Contextual, Inc.
  // This notice may not be modified or removed.
  // SPDX-License-Identifier: AGPL-3.0-only
  ```
- every environment variable topupsh-tracker reads or documents (backend config placeholders, CLI, Dockerfile, install scripts, Makefile, Vite config, `.env.example`, README) starts with `TOPUPSH_`, e.g. `TOPUPSH_PORT`; no exceptions, never add an unprefixed one
- `make lint` (golangci-lint on backend and cli, ESLint, copyright) must pass; CI runs it in the Lint and Copyright workflows
