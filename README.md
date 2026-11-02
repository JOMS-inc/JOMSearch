# JOMSearch

## Running with Docker

Copy `.env.example` to `.env` and fill in the values.

### Development (hot reload)

```sh
docker compose -f docker-compose.dev.yml up --build
```

The source code is mounted into the container, and [Air](https://github.com/air-verse/air)
rebuilds and restarts the server whenever a `.go` or `.html` file changes.

### Production

Every push to `main` builds the image, pushes it to `ghcr.io/joms-inc/jomsearch`
and redeploys it on the server (see `.github/workflows/cd.yml`). To run it by hand:

```sh
docker compose -f docker-compose.prod.yml up -d
```

## Environment variables

| Variable         | Default             | Description                          |
|------------------|---------------------|--------------------------------------|
| `SESSION_SECRET` | insecure dev key    | Key used to sign session cookies     |
| `DB_PATH`        | `../db/whoknows.db` | Path to the SQLite database          |
| `PORT`           | `8080`              | Port the server listens on           |

## Testing

See [testing.md](testing.md) for which kinds of tests we have and why.

```sh
# Unit, integration and mock tests (Go)
cd go-services && go test -race -cover ./...

# Fuzz the input parser for 30 seconds
cd go-services && go test -run '^$' -fuzz=FuzzParseInput -fuzztime=30s ./internal/api

# End-to-end tests (Playwright): start the app on a seeded test DB first
cd go-services && go run ./cmd/testdb /tmp/e2e.db && DB_PATH=/tmp/e2e.db go run ./cmd/search-api
cd e2e && npm ci && npx playwright install chromium && npx playwright test

# Only the read-only E2E tests, against production
cd e2e && BASE_URL=https://jomsearch.dk npm run test:readonly
```

Run the Go checks automatically before every push (once per clone):

```sh
git config core.hooksPath .githooks
```
