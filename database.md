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

## Moving production from SQLite to Neon (done 2026-11-19)

`sqlite2pg` copies users, pages and crawled terms in one transaction. It keeps user ids, so existing session cookies keep working, and it can safely run more than once (existing rows are updated, not duplicated). That is what made a short switch possible: copy everything while the old app keeps serving, then copy once more right before the new version goes live.

What we did:

1. **Neon project:** in `eu-central-1`, Frankfurt. Its direct connection string went into `.env` on the server and on a laptop. The old app ignores `DATABASE_URL`, so adding it early was safe.
2. **Test run:** migrations, copy and searches against Neon from the laptop. Afterwards the test rows were removed (`TRUNCATE users, pages, crawled_terms RESTART IDENTITY`).
3. **First real copy:** from a copy of the production file, run from the laptop:
   ```sh
   scp guts:JOMSearch/db/whoknows.db ~/whoknows-prod.db
   cd go-services && set -a && . ../.env && set +a
   go run ./cmd/sqlite2pg -sqlite ~/whoknows-prod.db -pg "$DATABASE_URL"
   ```
   It copied 2,379 users and 51 pages in 54 s. Each row is one round trip to Frankfurt; the same copy into a local Postgres took 0.3 s. Counts, the highest user id and the number of flagged users were compared between SQLite and Neon.
4. **Final copy:** the same command again with a fresh copy of the file, once CI was green.
5. **Switch:** merged the PR right after the final copy. CD deployed the new image, which reads `DATABASE_URL`. We checked the live site: search for "programs" finds "program" (stemming, so it really is Postgres), and the SQLite file was no longer modified.

Downtime was only the container restart during the deploy, a few seconds. A user who registered between the final copy and the deploy (a few minutes) would have been lost. We watched the user count beforehand and nobody registered in that time.

**Don't run `sqlite2pg` again now that production writes to Postgres.** It would overwrite newer rows, such as changed passwords, with the old values from SQLite.

For reference, to run `sqlite2pg` on the server instead of a laptop, go through compose so `.env` is read correctly:

```sh
docker compose -f docker-compose.prod.yml run --rm -v ./db:/data app \
  sh -c './sqlite2pg -sqlite /data/whoknows.db -pg "$DATABASE_URL"'
```

Don't use `docker run --env-file .env`: unlike compose, it keeps the quotes around `DATABASE_URL` as part of the value.

**Rollback:** `sqlite2pg` only reads the SQLite file (`mode=ro`), and it is still on the server at `db/whoknows.db`. To go back, revert the merge commit on `main`. CD then redeploys the SQLite version, including the old compose file with the `./db` mount. Users who registered or changed their password on Postgres after the switch would have to be copied back by hand. Once we trust the new setup, the SQLite file can be deleted.

## Backups

- **Neon:** keeps point-in-time history, which lets you restore to an earlier moment. The free tier keeps only a short window, so check the current limit in the Neon console.
- **Our own copy:** a nightly logical dump on the VM, so we also have a copy outside Neon. In `adminuser`'s crontab (`crontab -e`):
  ```sh
  0 4 * * * cd $HOME/JOMSearch && umask 077 && set -a && . ./.env && set +a && docker run --rm -e DATABASE_URL postgres:18-alpine sh -c 'pg_dump "$DATABASE_URL"' | gzip > backups/jomsearch-$(date +\%F).sql.gz
  ```
  `umask 077` makes the dump readable only by `adminuser`, because it contains the password hashes. `. ./.env` loads the file the way bash does, so the quotes are stripped. `-e DATABASE_URL` passes the value into the container without it showing up in the host's process list. The `postgres` image's `pg_dump` must be at least the server's version (Neon runs 18).
