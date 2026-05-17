# Rabhana API

Go backend for the Rabhana auction platform.

## Requirements

| Tool          | Version       | Purpose                                |
|---------------|---------------|----------------------------------------|
| Go            | 1.25+         | Compiler                               |
| PostgreSQL    | 14+           | Database                               |
| goose         | latest        | Database migrations                    |
| sqlc          | v1.25+        | Generates type-safe Go from SQL        |
| Docker        | 24+ (optional) | Container builds / deploys             |

Install tooling:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

## Setup

```bash
cp .env.example .env       # fill in DATABASE_URL and JWT_SECRET
go mod download
```

## Migrations

Migrations live in `db/migrations/` and run with goose.

```bash
goose -dir db/migrations postgres "$DATABASE_URL" up        # apply all pending
goose -dir db/migrations postgres "$DATABASE_URL" status    # show state
goose -dir db/migrations postgres "$DATABASE_URL" down      # rollback one
```

After editing any file in `db/queries/`, regenerate the sqlc code:

```bash
sqlc generate -f db/sqlc.yaml
```

Generated files in `db/sqlc/` are not hand-edited.

## Run

```bash
go run api/main.go                  # local dev
go build -o bin/api api/main.go     # build binary
docker build -t rabhana-api .       # build container (multi-stage, ~30 MB final)
docker run --env-file .env -p 8080:8080 rabhana-api
```

## Required environment

| Variable        | Required | Default       | Notes                                        |
|-----------------|----------|---------------|----------------------------------------------|
| `DATABASE_URL`  | yes      | —             | Postgres connection string                   |
| `JWT_SECRET`    | yes      | —             | Used to sign auth tokens                     |
| `SERVER_PORT`   | no       | `8080`        |                                              |
| `LOG_LEVEL`     | no       | `info`        | `debug` / `info` / `warn` / `error`          |
| `LOG_FORMAT`    | no       | `json`        | `json` or `text`                             |

### Auction tuning (all optional)

| Variable                          | Default |
|-----------------------------------|---------|
| `AUCTION_DURATION_HOURS`          | `1`     |
| `MAX_BIDS_PER_AUCTION`            | `10`    |
| `MAX_ACTIVE_BIDS_PER_USER`        | `3`     |
| `SELECTION_WINDOW_HOURS`          | `24`    |
| `MAX_CANCELLATIONS_PER_MONTH`     | `3`     |
| `MIN_INTERESTS_AT_REGISTRATION`   | `5`     |
| `MAX_NOTIFICATIONS_PER_USER`      | `10`    |

### Integrations (optional)

| Variable                       | Purpose                          |
|--------------------------------|----------------------------------|
| `FIREBASE_CREDENTIALS_PATH`    | FCM push notifications           |
| `FIREBASE_CREDENTIALS_JSON`    | FCM credentials as inline JSON   |
| `GMAIL_USER`                   | Gmail SMTP sender                |
| `GMAIL_APP_PASSWORD`           | Gmail app password               |
| `MAX_UPLOAD_SIZE_MB`           | Upload size cap (default `10`)   |
| `APP_BASE_URL`                 | Public URL of this API           |
