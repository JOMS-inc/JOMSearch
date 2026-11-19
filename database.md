# Database

whoknows moved from SQLite to **PostgreSQL**, hosted on **Neon's free tier**. Schema changes are versioned with **goose**, and we use **no ORM**. This file explains why, and how the move is done with (almost) no downtime.

## Why PostgreSQL

The choice follows from whoknows' data model:

| Data | What it needs | Fits |
|---|---|---|
| `users` | unique username/email, safe password updates | relational: constraints and transactions |
| `pages` | fixed columns; the main workload is **full-text search in English and Danish** | Postgres `tsvector` + GIN index with the built-in `english` and `danish` configs |
| `crawled_terms` | small key → value lookups | anything |
| search log | append-only lines | stays a file (`logs/searches.log`) |

- **Not MongoDB:** documents would suit pages, but users need unique fields and transactions, and Mongo's full-text search is tied to its hosted service (Atlas).
- **Not Elasticsearch/OpenSearch:** the best search engines, but they need gigabytes of RAM, which is the same problem as ELK.
- **Not MySQL:** Postgres has the stronger full-text search (stemming per language, ranking), generated columns that can hold a search index, and a wider ecosystem of managed hosts.

What we gain over SQLite:

- **Stemming:** "programs" finds "program", and "hovedstaden" finds "hovedstad". FTS5 only matched exact words.
- **No single-writer lock:** the crawler and the app write at the same time.
- **Room to grow:** the database can live on its own server, apart from the app.

## Where it runs: Neon free tier

| Option | Cost/month | Trade-off |
|---|---|---|
| Postgres in Docker on our VM | $0 extra | competes with the app for RAM, and we do backups ourselves |
| **Neon free tier** | **$0** | managed, autoscaling, 1 GB per project (we use a few MB); scales to zero after 5 min idle, so the first query after a pause takes longer |
| Azure Flexible Server B1ms | ~$12 + storage | same region as the VM, managed backups |

We chose Neon because the requirement is no cost. To upgrade later (to a paid Neon plan or Azure), we only change `DATABASE_URL`.

Setup:

1. Create a Neon project in the region closest to the Azure VM, with a database named `jomsearch`.
2. Copy the **direct** connection string, not the "pooled" one, into `DATABASE_URL` in `.env` on the server. Add `connect_timeout=15` so the app waits while Neon wakes up.
   - The pooled endpoint goes through PgBouncer in transaction mode, which breaks the advisory lock goose uses during migrations.
   - With one app container we don't need a pooler. Go's `database/sql` already keeps a connection pool.

## Indexes

| Index | Used by |
|---|---|
| `users_pkey` (id) | change password, password upgrade |
| `users_username_key` (UNIQUE) | login |
| `users_email_key` (UNIQUE) | looking up the logged-in user on every request |
| `pages_pkey` (title), `pages_url_key` (UNIQUE) | the crawler's `ON CONFLICT DO NOTHING` |
| **`pages_search_idx` (GIN on `search`)** | **every search** |
| `crawled_terms_pkey` (query, language) | the crawler's "already crawled?" check |

`pages.search` is a generated column, so Postgres updates it on every INSERT and UPDATE. That replaces the FTS5 table and its three triggers.

We added no index on `language`, because it has only two values and filters out almost nothing.

## ORM: researched, not used

| | Style | Pros | Cons |
|---|---|---|---|
| GORM | classic ORM (reflection) | most popular, fast to prototype | runtime errors, hides SQL, slower |
| ent | generated from a Go schema | type-safe, good for complex graphs | code generation and its own schema language |
| Bun | thin query builder | fast, close to SQL | still a layer on top of SQL |
| sqlc | generates Go from SQL you write | type-safe, no runtime layer | needs a code-generation step |
| `database/sql` + pgx | plain SQL | nothing to learn, full control | type mistakes only show at runtime (tests catch them) |

whoknows has 3 tables, no relations between them, and fewer than 15 queries. Its most important query (`to_tsquery`, `@@`, `ts_rank`) is Postgres-specific and would be raw SQL in any ORM anyway. An ORM would add a dependency and a layer to learn without saving any code. **We use `database/sql` with the pgx driver.** If type-safety becomes a problem, sqlc is the next step, because it keeps our SQL.

The numbers behind this are in the benchmark repo (`jomsearch-db-bench`).

## Migrations: goose

| Tool | Style | Verdict |
|---|---|---|
| **goose** | versioned `.sql` files (or Go functions), library + CLI | chosen: plain SQL, embeds in our binary, Postgres advisory lock |
| golang-migrate | versioned up/down `.sql` files, CLI + library | very similar; separate up/down files, no Go migrations |
| Atlas | declarative: describe the schema, it writes the migration | strongest (linting of destructive changes), but more than we need |
| Flyway / Liquibase | Java, versioned SQL / XML | industry standard, but needs a JVM next to a Go app |

How it works here:

- **Where migrations live:** `go-services/internal/db/migrations/NNNNN_name.sql`, compiled into the binary.
- **When they run:** the app and the crawler call `db.Migrate` on startup, which applies pending files and records them in `goose_db_version`.
- **Concurrent starts:** an advisory lock lets only one process migrate at a time.
- **Changing the schema:** add a new file, e.g. `00002_add_page_views.sql` with `-- +goose Up` / `-- +goose Down` sections. Never edit a file that has already run.
- **Avoiding downtime on schema changes:** split breaking changes in two (expand, then contract). For example, add the new column, deploy code that uses it, and only drop the old column in a later migration. That way the old and new app versions both work during a deploy.

Data from SQLite is moved by `cmd/sqlite2pg`, described next.

## Moving production from SQLite to Neon (runbook)

`sqlite2pg` copies users, pages and crawled terms in one transaction. It keeps user ids, so existing session cookies keep working, and it can safely run more than once. Running it twice is what keeps downtime short.

On the server (`~/JOMSearch`):

```sh
# 0. Backup of the old database (keep it until we are sure)
cp db/whoknows.db db/whoknows.db.bak-$(date +%F)

# 1. Pause the crawler: comment out its cron line (crontab -e)

# 2. First copy, while the old app keeps serving users
docker run --rm -v ./db:/data --env-file .env ghcr.io/joms-inc/jomsearch:<new-tag> \
  sh -c './sqlite2pg -sqlite /data/whoknows.db -pg "$DATABASE_URL"'

# 3. Check: counts must match
sqlite3 db/whoknows.db 'select count(*) from users; select count(*) from pages;'
psql "$DATABASE_URL" -c 'select count(*) from users' -c 'select count(*) from pages'

# 4. Second copy, then switch. Only users can change in between
#    (registrations, password changes), and they are few rows, so this
#    copy takes well under a second.
docker run --rm -v ./db:/data --env-file .env ghcr.io/joms-inc/jomsearch:<new-tag> \
  sh -c './sqlite2pg -sqlite /data/whoknows.db -pg "$DATABASE_URL"' \
  && docker compose -f docker-compose.prod.yml up -d   # new image, now on Postgres

# 5. Smoke test, then re-enable the crawler
curl -s 'https://jomsearch.dk/api/search?q=python' | head -c 200
```

Downtime is only the container restart in step 4, a few seconds. A user who registers between the second copy and the restart would be lost. Writes can't be frozen without a maintenance page, so we accept that tiny window.

**Rollback:** the SQLite file is never written to (`mode=ro`). To go back, deploy the previous image tag, which still reads `db/whoknows.db`.

## Backups

- **Neon:** keeps point-in-time history, which lets you restore to an earlier moment. The free tier keeps only a short window, so check the current limit in the Neon console.
- **Our own copy:** a nightly logical dump on the VM, so we also have a copy outside Neon.
  ```sh
  0 4 * * * cd ~/JOMSearch && docker run --rm --env-file .env postgres:18-alpine \
    sh -c 'pg_dump "$DATABASE_URL"' | gzip > backups/jomsearch-$(date +\%F).sql.gz
  ```
