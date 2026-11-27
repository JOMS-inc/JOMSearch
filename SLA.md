# Service Level Agreement (SLA)

**Service:** JOMSearch, <https://jomsearch.dk>
**Provider:** JOMS (student DevOps team)
**Valid from:** 2026-11-27, until the end of the exam period

JOMSearch runs on one VM with a free-tier database and is run by students alongside their studies. The targets below are set to match that, so they are deliberately modest.

## 1. Service scope

Covered:

| Service | What it is |
|---|---|
| Search (web) | `GET /` and `GET /search`: full-text search of English and Danish Wikipedia pages |
| Search API | `GET /api/search`: the same search as JSON |
| Accounts | Register, log in, log out, change password |
| Result links | `GET /go`: redirect to the chosen result |

Not covered:

- The weather page (it depends on a third-party API).
- The nightly crawler. It adds pages at 03:00, but there is no guarantee about how quickly new articles show up.
- The Grafana dashboard (`/grafana/`), which is an internal tool.
- How complete or correct search results are. The content comes from Wikipedia.

## 2. Performance metrics

| Metric | Target | How it is measured |
|---|---|---|
| **Availability** | **99.0 % per calendar month** (at most ~7 h 18 min downtime) | External Postman monitor (outside Azure). A check counts as failed when the site does not answer with HTTP 200. |
| **Search latency** | p95 under **500 ms** inside the app | `jomsearch_search_duration_seconds` histogram in Prometheus/Grafana, over a month |
| **Server errors** | Fewer than **1 %** of requests return 5xx | `jomsearch_http_requests_total{status=~"5.."}` divided by all requests |

Does not count as downtime:

- **Planned maintenance** that is announced at least 24 h in advance in the repository (an issue or the README), limited to 2 h per month. A normal deploy (CD restarting the containers) takes under a minute and also does not count.
- **Database cold starts.** Neon's free tier scales to zero after 5 minutes without traffic, so the first query after a pause can take a few seconds. This is a known limit of the free tier.
- **Outages at Azure, Neon or the DNS provider** that we cannot control.

## 3. Response and resolution

Support is given on weekdays, 09:00–17:00 (Danish time). Report problems as a GitHub issue at <https://github.com/JOMS-inc/JOMSearch/issues>.

| Severity | Example | First response | Target resolution |
|---|---|---|---|
| **Critical** | Site down, nobody can search, data leak | 4 working hours | 1 working day |
| **High** | Login or registration broken, search very slow | 1 working day | 3 working days |
| **Low** | Wrong result, layout bug, typo | 3 working days | Next release |

After each critical incident we write a short postmortem: what happened, why, and what we changed.

## 4. Security measures

- All traffic uses HTTPS (Let's Encrypt certificate). HTTP is redirected to HTTPS.
- Passwords are hashed with bcrypt. Old MD5 hashes from the legacy app are upgraded to bcrypt the next time the user logs in.
- Sessions use signed cookies. Secrets live only in `.env` on the server and in GitHub Secrets, never in git.
- The metrics port (2112) is not exposed to the internet. Grafana requires a login.
- Every change goes through CI before it is deployed: lint, tests, `govulncheck` for known vulnerabilities in dependencies, and SonarCloud.
- `/go` redirects only to URLs that are in our own database, so it cannot be used as an open redirect.
- Backups: Neon point-in-time restore, plus a nightly `pg_dump` on the VM that only the service user can read.

## 5. Compliance

- **GDPR:** the only personal data stored is username, email and a password hash. Search queries are logged without user ID or IP address and are used only to decide which pages to crawl. Users can ask for their account to be deleted through a GitHub issue. We delete it within 30 days.
- **Wikipedia:** content is used under CC BY-SA, and the crawler uses the official Wikipedia API.
- **Data location:** the VM runs in Azure. The database runs on Neon in AWS `eu-central-1` (Frankfurt), so data stays in the EU.

## 6. Compensation

None. JOMSearch is a free, non-commercial student project. A commercial SLA would normally give service credits when the availability target is missed.
