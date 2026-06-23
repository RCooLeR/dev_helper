# Architecture

devhelper is a small Go application with two front doors:

- CLI commands in `internal/cli`
- a local HTTP UI/API in `internal/http`

Both front doors call the same provisioning layer, so project creation, deletion, cert generation, and SQL imports behave the same whether they come from the browser or the terminal.

## Package Map

- `cmd/devhelper`: process entrypoint.
- `internal/app`: repo-root detection, config loading, default config, logger setup.
- `internal/cli`: `urfave/cli` command definitions.
- `internal/http`: embedded web UI and JSON API routes.
- `internal/platform`: host/WSL path conversion, `.env` generation, hosts-file updates, file sync helpers, shell runner.
- `internal/provision`: project lifecycle, certs, database create/drop/import.
- `internal/store`: JSON-backed project metadata store.
- `internal/templates`: nginx vhost rendering.

## Config And Paths

The config file is `.devhelper-config.json` at the repository root.

On Linux/macOS, the default runtime root is the repository `projects` directory. On Windows, the runtime root defaults to `/data/projects` inside WSL, and `host_mirror_dir` points at the same location through `\\wsl$\Ubuntu\data\projects`.

The distinction matters:

- Docker and nginx use runtime paths such as `/data/projects/apps`.
- Go file operations on Windows use the UNC mirror path so they can create files inside WSL without shelling out.
- Docker Compose itself lives at `compose_dir`, a host path containing `docker-compose.yml` and `.env`.

## Project Creation Flow

`provision.Create` performs these steps:

1. Sanitize company/project names into predictable slugs.
2. Expand the default domain pattern if no custom domain was provided.
3. Create the runtime app directory.
4. On Windows, create a tiny repo-side mirror folder with a `.wsl-path` breadcrumb.
5. Render and write the nginx vhost.
6. Add hosts-file entries for IPv4 and IPv6 loopback.
7. Issue a local mkcert certificate if one is missing.
8. Create the requested database and per-project DB user.
9. Optionally import a SQL dump.
10. Upsert the project into `devhelper.store.json`.

Hosts, certs, and DB setup produce warnings instead of aborting the whole project after files are written. This lets the UI show the user what needs manual attention without losing the project record.

## Database Engines

The DB layer normalizes UI/CLI aliases before doing any work:

- `mysql` and `mysql-8.4` become `mysql-8.4`
- `mysql9`, `mysql-9.6`, and `mysql-9.7` become `mysql-9.7`
- `mariadb10` and `mariadb12` stay as named services
- `postgres` and `postgresql` become `postgres`

Each DB operation starts only the service it needs with `docker compose up -d <service>`, waits for TCP, then verifies authentication with the relevant client. SQL identifiers and string literals are quoted at the DB boundary, even though project names are sanitized earlier, because the JSON store can be edited by hand.

## Store

The store is a single JSON file at `${DATA_ROOT}/devhelper.store.json`.

`internal/store` keeps an in-memory copy guarded by a mutex. `List` returns a copy of the slice, and all mutations go through `Upsert` or `Delete`, which immediately write the JSON file back to disk.

## HTTP API

The web server embeds `internal/http/ui/*` and exposes:

- `GET /api/projects`
- `POST /api/projects`
- `POST /api/projects/drop`
- `POST /api/projects/import`
- `POST /api/cert/init`
- `POST /api/cert/issue`

The HTTP layer is deliberately thin: it decodes JSON, calls provisioning/store functions, and returns JSON responses.

## Git Hygiene

Runtime project data is intentionally ignored:

- `projects/apps/**`
- `projects/data/**`
- `projects/ssh/**`
- generated nginx vhosts and local TLS material
- local dumps/backups/archives
- non-example `.env` files

The repository should contain templates, source code, and safe examples only.
