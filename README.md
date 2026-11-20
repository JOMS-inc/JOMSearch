# JOMSearch

A small search engine for Wikipedia articles in English and Danish, live at <https://jomsearch.dk>. It's a Go rewrite of the legacy Python app in `python-legacy/`.

## How it fits together

```
browser ──> nginx (TLS) ──> search-api (Go) ──> PostgreSQL on Neon
                                │                      ▲
                                ▼ every search         │ new pages
                         logs/searches.log ──> crawler (nightly, cron) ──> Wikipedia API
```

| Part | Where | What it does |
|---|---|---|
| `search-api` | `go-services/cmd/search-api` | The web app: search page, JSON API, login/register, weather. Runs migrations on startup. |
| `crawler` | `go-services/cmd/crawler` | Reads the search log and fetches Wikipedia articles, starting with searches that found nothing |
| `sqlite2pg` | `go-services/cmd/sqlite2pg` | One-off tool that copied the old SQLite data into Postgres |
| `testdb` | `go-services/cmd/testdb` | Seeds an empty database for the E2E tests |
| Migrations | `go-services/internal/db/migrations` | The schema, as versioned goose files |

More detail:
- [database.md](database.md): why Postgres and Neon, the indexes, the ORM and migration-tool research, the cutover runbook and backups.
- [testing.md](testing.md): which kinds of tests we have and why.
- [monitoring.md](monitoring.md): Prometheus and Grafana, which user metrics we collect and why.
- [branching_strategy.md](branching_strategy.md): how we use branches.

## Running with Docker

Copy `.env.example` to `.env` and fill in the values.

### Development (hot reload)

```sh
docker compose -f docker-compose.dev.yml up --build
```

The source code is mounted into the container, and [Air](https://github.com/air-verse/air)
rebuilds and restarts the server whenever a `.go` or `.html` file changes. This also
starts a local PostgreSQL on port 5432, user and password `postgres`. The app
creates the tables itself on startup.

To run the app without Docker, start only the database:

```sh
docker compose -f docker-compose.dev.yml up -d db
cd go-services && go run ./cmd/search-api   # defaults to the dev database
```

### Production

Every push to `main` builds the image, pushes it to `ghcr.io/joms-inc/jomsearch`
and redeploys it on the server (see `.github/workflows/cd.yml`). To run it by hand:

```sh
docker compose -f docker-compose.prod.yml up -d
```

The deploy also copies `docker-compose.prod.yml` and `nginx/nginx.conf` to the
server. `.env` lives only on the server and is never copied or committed.

## Environment variables

| Variable         | Default             | Description                          |
|------------------|---------------------|--------------------------------------|
| `SESSION_SECRET` | insecure dev key    | Key used to sign session cookies     |
| `DATABASE_URL`   | local dev Postgres  | PostgreSQL connection URL (prod: Neon, see [database.md](database.md)) |
| `SEARCH_LOG`     | `../logs/searches.log` | Where searches are logged (read by the crawler) |
| `CRAWLER_CONTACT`| none                | Contact sent in the crawler's User-Agent |
| `PORT`           | `8080`              | Port the server listens on           |
| `METRICS_PORT`   | `2112`              | Port for Prometheus `/metrics` (not public) |
| `GF_SECURITY_ADMIN_PASSWORD` | none (required in prod) | Grafana admin password, see [monitoring.md](monitoring.md) |

Put quotes around `DATABASE_URL` in `.env` (`DATABASE_URL='postgres://...&...'`).
Without them, bash treats the `&` as "run in background" when you `source` the
file. Docker compose accepts the quoted form. Plain `docker run --env-file` doesn't:
it keeps the quotes as part of the value, so use `docker compose run` instead.

## Changing the database schema

Add a new file to `go-services/internal/db/migrations`, numbered after the last
one, e.g. `00002_add_page_views.sql`:

```sql
-- +goose Up
ALTER TABLE pages ADD COLUMN views INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE pages DROP COLUMN views;
```

It runs automatically the next time the app or the crawler starts, on your machine
and in production. Never edit a migration that has already been merged. Add a new
one instead. Split breaking changes (renames, drops) across two deploys so the old
and new versions both work during a deploy. See [database.md](database.md#migrations-goose).

## Search log and crawler

Every search is written as one JSON line to `SEARCH_LOG`:

```json
{"time":"2026-11-19T15:08:28Z","level":"INFO","msg":"search","query":"kubernetes","language":"en","results":0}
```

To see what people search for, on the server (`jq` needs `sudo apt-get install jq` there first):

```sh
# Most common searches
jq -r .query logs/searches.log | sort | uniq -c | sort -rn | head -20
# Searches that found nothing: the content we're missing
jq -r 'select(.results == 0) | .query' logs/searches.log | sort | uniq -c | sort -rn | head -20
```

The crawler uses the same log to decide what to fetch: searches that found nothing
come first, then the most popular. It fetches up to 20 terms per run at about one
request per second, and skips terms it fetched within the last 7 days.

```sh
# Locally
cd go-services && go run ./cmd/crawler -max-terms 5
# In production (runs nightly at 03:00 from adminuser's crontab, output in ~/crawler.log)
docker compose -f docker-compose.prod.yml run --rm crawler
```

`go run ./cmd/crawler -h` lists the flags (`-max-terms`, `-per-term`, `-delay`, `-refresh`).

## Testing

See [testing.md](testing.md) for which kinds of tests we have and why.

```sh
# Unit, integration and mock tests (Go). Database tests need a Postgres,
# e.g. the dev one: docker compose -f docker-compose.dev.yml up -d db
cd go-services && TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' go test -race -cover ./...

# Fuzz the input parser for 30 seconds
cd go-services && go test -run '^$' -fuzz=FuzzParseInput -fuzztime=30s ./internal/api

# End-to-end tests (Playwright): start the app on a fresh, seeded database first
docker compose -f docker-compose.dev.yml exec db sh -c 'dropdb -U postgres --if-exists e2e && createdb -U postgres e2e'
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/e2e?sslmode=disable'
cd go-services && go run ./cmd/testdb "$DATABASE_URL" && go run ./cmd/search-api
cd e2e && npm ci && npx playwright install chromium && npx playwright test

# Only the read-only E2E tests, against production
cd e2e && BASE_URL=https://jomsearch.dk npm run test:readonly
```

Run the Go checks automatically before every push (once per clone):

```sh
git config core.hooksPath .githooks
```
