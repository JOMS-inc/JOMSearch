# Testing

**Created:** 2. nov. 2026

**Last revised:** 2. nov. 2026

We set up a variety of test types instead of covering one type exhaustively. Tests run at three points in our development process:

1. **Locally before pushing:** a git pre-push hook (`.githooks/pre-push`) runs gofmt, `go vet` and all Go tests.
2. **In CI on every pull request:** all Go tests, plus the full Playwright E2E suite against the app started on the runner with a fresh test database.
3. **In CD after every deploy:** a cURL smoke test, then the read-only E2E tests against https://jomsearch.dk.

## What we implemented

| Type | What it tests | Where | Runs locally | CI | After deploy |
|---|---|---|---|---|---|
| Unit | Password checks (`checkPassword`, `looksLikeMD5`) and input parsing (`parseInput`) | `go-services/internal/api/login_test.go`, `input_test.go` | ✓ | ✓ | |
| Fuzz | `parseInput` with random bodies; must never panic | `FuzzParseInput` in `input_test.go` | ✓ (seed corpus; full fuzzing by hand) | ✓ (seed corpus) | |
| Integration | Real router, handlers, templates, sessions and a real SQLite DB over HTTP: pages, search API, register/login/logout, error cases, legacy MD5 → bcrypt upgrade | `internal/api/integration_test.go` | ✓ | ✓ | |
| Mock | The weather feature with a fake Open-Meteo server: response mapping, bad coordinates, upstream errors | `internal/api/weather_test.go` | ✓ | ✓ | |
| End-to-end | A real browser (Chromium via Playwright) clicking through search, navigation, layout and the full auth flow | `e2e/tests/*.spec.ts` | ✓ | ✓ (all) | ✓ (`@readonly` only) |
| Smoke | `/health`, `/`, `/about`, the stylesheet and search respond with 200 | `smoke-test` job in `.github/workflows/cd.yml` | | | ✓ |
| Static analysis | gofmt, `go vet`, golangci-lint, govulncheck, hadolint, SonarCloud | `.github/workflows/ci.yml` | partly (hook) | ✓ | |

Shared test data: `internal/testdb` builds a database from the real `schema/schema.sql` plus a few known pages. The Go tests use it directly, and the E2E tests get the same data through `go run ./cmd/testdb`.

### Why these choices

- **Integration tests are the core of the suite.** Most of our code is HTTP handlers talking to SQLite, so the bugs that matter happen where those parts meet. The tests run the real app in memory (`httptest`), which takes seconds, so they can run before every push.
- **Unit tests only where there is real logic.** The password functions decide who can log in, including the legacy MD5 path, so they get table tests for every branch.
- **Mocking only external services.** We mock Open-Meteo because it is outside our control: the real API would make tests slow, flaky and dependent on the actual weather. We don't mock our own database, because SQLite in a temp file is fast and tests the real SQL.
- **The E2E tests are split with a `@readonly` tag.** Tests that only read also run against production after deploy. Tests that register users only run against the throwaway CI database, so production doesn't fill up with test accounts.
- **Fuzzing** fits `parseInput`, because it takes untrusted request bodies. It shows the technique at almost no cost.

### Bugs the tests found

- The search page and the weather page always showed "Log in" even when the user was logged in, because their handlers never passed the current user to the template. The integration tests caught it, and we fixed it in `root.go` and `weather_page.go`.
- Earlier, the smoke test work revealed that `/static/style.css` and `/about` returned 404 in production. Both are now covered by tests.

## Considered but not implemented

| Type | Why not (for now) |
|---|---|
| Load / performance testing (k6, JMeter) | We have one small server and almost no traffic, so the numbers wouldn't tell us much yet. It could become relevant if we want to compare the SQLite `LIKE` search with full-text search (fts5). |
| Security testing / pen-testing (OWASP ZAP) | This is partly covered by govulncheck, SonarCloud security rules and the tests for password hashing. A ZAP scan against production is a candidate for later, but it needs care so we don't attack our own live site in every pipeline run. |
| Contract testing (Pact) | Contract tests check an agreement between a provider and its consumers. Nobody else consumes our API, and our own frontend is server-rendered by the same Go code, so there is no contract to protect. |
| Visual regression (screenshot diffs) | The UI is small and changes often while we work on it, so screenshot baselines would mostly produce noise. The E2E tests check the layout that matters, such as the footer at the bottom and the stylesheet loading. |
| Mutation testing | Too heavy for the course. We ran a manual version instead: we broke `checkPassword` on purpose and confirmed that both unit and integration tests fail. |
| Accessibility testing (axe) | Relevant for a public site, but not part of this assignment. It could be added cheaply later with `@axe-core/playwright`. |
| Acceptance / user acceptance testing | We have no customer or product owner to sign off. Our E2E tests follow user stories (search, register, log in), which is the closest equivalent. |
| Chaos / resilience testing | We run a single server without redundancy, so there is nothing to fail over to. The mock tests cover the one external dependency failing (the weather API being down). |
| Unit tests for the database layer (`internal/db`) | `db.Open` is three lines around `sql.Open`. The integration tests use it for every test, so a separate unit test would add nothing. |
