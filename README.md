# topupsh tracker

Go (Fiber v3, GORM) backend and Vue 3 (Vite, Tailwind) frontend.

The HTTP API (`/api/v1` and sign-in) is described in [openapi.yaml](openapi.yaml)
(OpenAPI 3.1). A backend test fails when a route is missing from it, so keep it
in step with `backend/internal/handler/api/api.go`.

## Base layer

- Sign in with Google, GitHub or custom OpenID Connect providers. Accounts are
  keyed by verified email, so every provider reaches the same account.
- `/setup` is unlocked with the root access token
  (`TOPUPSH_AUTH_ROOT_ACCESS_TOKEN`; a random one is logged at startup when it
  is unset). It edits the app URL, TLS, database and sign-in providers, saved
  encrypted in the settings file (`TOPUPSH_SETTINGS_FILE`).
- `/admin` (root only) shows user counts, sign-ups over 30 days and the users.
- SQLite by default, PostgreSQL when enabled. HTTPS with Let's Encrypt when
  `TOPUPSH_TLS_ENABLED` is set.
- Light, dark and system themes; installable as a PWA.

## Running

Required: `TOPUPSH_MASTER_KEY` and `TOPUPSH_AUTH_JWT_SECRET` (32+ characters
each). See `backend/cmd/server/.env.example` for the rest.

```bash
make dev      # API + Vite dev server, open http://localhost:5173
make build    # bin/topupsh-tracker with the SPA embedded
make test     # backend and frontend, 100% coverage
make lint     # golangci-lint, ESLint, copyright notice
make docker   # container image, serves on 2027
```
