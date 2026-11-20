# Monitoring

Postman monitors whether the site is **up**. This setup answers a different
question: **how is JOMSearch used, and does it help the people using it?**

```
search-api ──:2112/metrics──> Prometheus ──> Grafana ──> https://jomsearch.dk/grafana/
 (Go, client_golang)          (scrapes every 15s,        (provisioned dashboard,
                               keeps 120 days / 1 GB)     login required)
```

## Questions we asked, and the metric that answers each

We started from questions, not from "what can we measure", so every metric
has a reason to exist.

| Question | Metric | Dashboard panel |
|---|---|---|
| Is search used, and in which language? Browser or API (simulator)? | `jomsearch_searches_total{language, source, outcome}` | Searches per minute |
| How often do we fail the user? | same counter, `outcome="zero"` / all | Zero-result rate |
| Does the nightly crawler fix those failures? | zero-result rate over days | Zero-result rate (should drop after crawls) |
| Do results actually help? | `jomsearch_result_clicks_total{language, rank}` / searches with hits | Click-through rate |
| Is our ranking good? | clicks by `rank` (1, 2, 3, 4-10, 11+) | Clicks by result position |
| How many results do people get? | `jomsearch_search_results` (histogram) | Results per search |
| Is search fast enough? | `jomsearch_search_duration_seconds` (histogram) | Search latency p50 / p95 |
| Which features are used, and where do users hit errors? | `jomsearch_http_requests_total{route, method, status}` | Requests by page, Errors by route |
| Do people sign up and log in? Any brute force? | `jomsearch_logins_total{result}`, `jomsearch_registrations_total{result}` | Logins, Registrations |

### How clicks are counted

Search results used to link straight to Wikipedia, so the app never knew if
anyone opened a result. Now they link to `/go?url=...&rank=N`, which counts
the click and redirects. `/go` only redirects to URLs that are in our `pages`
table, so it can't be abused as an open redirect to a phishing site.

### Design choices

- **The search text is not a label.** Every distinct label value is a new
  time series in Prometheus. Free text from users would grow without limit
  and eat the server's memory. The actual queries are already in
  `logs/searches.log`, which the crawler reads.
- **Routes are labelled by pattern** (`GET /search`), not raw path. Requests
  that match no route (404s, bots probing `/wp-admin`) all share the label
  `unmatched` for the same reason.
- **`/metrics` is on its own port (2112)**, which is not published. Port 8080
  and nginx are public; the metrics port is only reachable by Prometheus on
  the Docker network.
- **One VM for everything.** The server has ~900 MB RAM. Prometheus and Grafana
  use ~110 MB together at our scale, and both have `mem_limit: 256m` so they
  can't push the app out of memory. Monitoring on the same machine dies with
  it, but outages are Postman's job (it checks from outside); usage data from
  a dead server doesn't exist anyway. In a company, monitoring would run on
  separate infrastructure.

## Using it

- Production: <https://jomsearch.dk/grafana/>, user `admin`, password is
  `GF_SECURITY_ADMIN_PASSWORD` in the server's `.env` (only read on Grafana's
  first start).
- Locally: `docker compose -f docker-compose.dev.yml up --build`, then
  Grafana on <http://localhost:3000> (admin/admin) and Prometheus on
  <http://localhost:9090>.

The dashboard is provisioned from `monitoring/grafana/dashboards/jomsearch-usage.json`.
To change it, edit it in Grafana, export the JSON (Share → Export) and commit
it over that file.

## Findings

Fill this in during the semester (with dates and screenshots in
`docs/screenshots/`). These are what to bring up at the exam.

- _e.g. 2026-12-01: zero-result rate for `da` was 40%, the crawler only fetched English pages…_
