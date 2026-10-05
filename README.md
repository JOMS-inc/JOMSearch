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
