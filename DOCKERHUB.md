# SprintBoard

A self-hosted, no-nonsense task and sprint tracker. Projects, cycles (sprints),
modules (epics), sub-tasks, assignees, labels, comments with @mentions, file
attachments, and email + in-app notifications. Ships as a single binary with
the frontend and database migrations embedded, so one container is the whole
app.

SprintBoard is a fork of [Bureaucat](https://github.com/bureaucatorg/bureaucat)
(built in-house at Zerodha and open-sourced), adapted for sprint-centric teams.
Source for this fork: https://github.com/mannam95/bureaucat

## Supported tags

- `X.Y.Z` (e.g. `1.3.0`) — an immutable release. **Pin this in production.**
- `sha-<commit>` — the exact commit a release was built from.
- `latest` — the most recent release. Convenient, but moves under you.

Versioning is [SemVer](https://semver.org/): patch = fixes, minor = new
features, major = breaking changes.

## Quick start

Postgres plus the app, via `docker-compose.yml`:

```yaml
services:
  postgres:
    image: postgres:18
    environment:
      POSTGRES_USER: sprintboard
      POSTGRES_PASSWORD: change-me
      POSTGRES_DB: sprintboard
    volumes:
      - pgdata:/var/lib/postgresql/data

  app:
    image: mvsrinath/sprintboard:1.3.0
    depends_on: [postgres]
    ports:
      - "1341:1341"
    environment:
      DATABASE_URL: postgres://sprintboard:change-me@postgres:5432/sprintboard?sslmode=disable
      JWT_SECRET: replace-with-a-random-string-of-32-plus-characters

volumes:
  pgdata:
```

```bash
docker compose up -d
```

Then open http://localhost:1341 and create your first account. Database
migrations run automatically on start (the image runs `serve --migrate`).

## Configuration

### Required

| Variable       | Description                                             |
| -------------- | ------------------------------------------------------- |
| `DATABASE_URL` | PostgreSQL connection string.                           |
| `JWT_SECRET`   | Secret for signing access tokens. Use 32+ characters.   |

### Optional — core

| Variable                    | Default | Description                                          |
| --------------------------- | ------- | ---------------------------------------------------- |
| `API_HOST`                  | `0.0.0.0` | Interface the server binds to.                     |
| `ACCESS_TOKEN_EXPIRY_MINS`  | `5`     | Access-token lifetime, minutes.                      |
| `REFRESH_TOKEN_EXPIRY_DAYS` | `7`     | Refresh-token lifetime, days.                        |

The container listens on port **1341**.

### Optional — file attachments (S3-compatible)

Attachments need an S3-compatible bucket. Works with AWS S3, MinIO, Garage, etc.

| Variable                         | Description                                                            |
| -------------------------------- | --------------------------------------------------------------------- |
| `S3_ENDPOINT`                    | Bucket endpoint URL.                                                   |
| `FILES_BUCKET_NAME`              | Bucket name.                                                           |
| `FILES_BUCKET_ACCESS_KEY_ID`     | Access key.                                                            |
| `FILES_BUCKET_SECRET_ACCESS_KEY` | Secret key.                                                            |
| `FILES_BUCKET_REGION`            | SigV4 signing region. **Required for real AWS S3** — must match the bucket's region (e.g. `eu-central-1`), or requests fail with `SignatureDoesNotMatch`. Optional for MinIO/Garage. |
| `MAX_UPLOAD_SIZE`                | Max upload size in bytes. Default 10 MB.                               |

### Optional — email notifications (SMTP)

Leave unset to disable email; in-app notifications still work.

| Variable            | Description                                                        |
| ------------------- | ----------------------------------------------------------------- |
| `SMTP_HOST`         | SMTP server host.                                                 |
| `SMTP_PORT`         | Port. `465` uses implicit TLS; others use STARTTLS when offered.  |
| `SMTP_USERNAME`     | Username (omit for an unauthenticated relay).                     |
| `SMTP_PASSWORD`     | Password.                                                         |
| `SMTP_FROM_ADDRESS` | From address, e.g. `SprintBoard <no-reply@example.com>`.          |

## Upgrading

Change the image tag to the new version and recreate the container. Migrations
for the new version run automatically on start. Back up your database first.

## License

AGPL-3.0, same as upstream Bureaucat. This is a modified fork; the complete
corresponding source is at https://github.com/mannam95/bureaucat
