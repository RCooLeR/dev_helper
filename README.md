# devhelper

`devhelper` manages a local PHP development stack: app directories, nginx vhosts, local TLS certs, hosts-file entries, Docker Compose services, and per-project database metadata.

## Prerequisites

- Docker Desktop and Docker Compose.
- `mkcert` for local HTTPS certificates.
- Database clients are used inside the selected Compose container by default. Set `mysql_cli` / `psql_cli` to use a host client instead.
- On Windows, WSL2 with Ubuntu is the default runtime filesystem.

## Configuration

Run the tool from the repository root. If `.devhelper-config.json` is missing, devhelper uses built-in defaults and `devhelper init` writes the config, creates local `projects/docker-compose.yml` from `projects/docker-compose.example.yml` when needed, and writes `projects/.env`.

Example Windows config:

```json
{
  "root_dir": "/data/projects",
  "host_mirror_dir": "\\\\wsl$\\Ubuntu\\data\\projects",
  "wsl_distro": "Ubuntu",
  "apps_root": "/data/projects/apps",
  "data_root": "/data/projects/data",
  "nginx_conf_root": "/data/projects/containers/nginx/conf.d",
  "nginx_external_root": "/data/projects/containers/nginx/external",
  "compose_dir": "D:\\Development\\projects",
  "default_domain_pattern": "<project>.<company>.local",
  "mysql_service": "mysql",
  "mysql9_service": "mysql9",
  "mariadb10_service": "mariadb10",
  "mariadb12_service": "mariadb12",
  "postgres_service": "postgres",
  "mysql_host": "127.0.0.1",
  "mysql_port": 3384,
  "mysql9_port": 3396,
  "mariadb10_port": 33106,
  "mariadb12_port": 3312,
  "mysql_root_pass": "Passw0rd",
  "mysql_cli": "C:\\Program Files\\MySQL\\MySQL Server 8.4\\bin\\mysql.exe",
  "postgres_host": "127.0.0.1",
  "postgres_port": 5434,
  "postgres_super_pass": "Passw0rd",
  "psql_cli": "C:\\Program Files\\PostgreSQL\\18\\bin\\psql.exe",
  "docker_cli": "docker",
  "mkcert_cli": "mkcert",
  "store_file": "D:\\Development\\projects\\data\\devhelper.store.json",
  "log_level": "debug"
}
```

## Setup

Initialize config and Compose environment:

```powershell
devhelper.exe init
```

`projects/docker-compose.yml` is local-only and ignored by Git. Edit it freely for private services, mounts, or tunnels. Keep reusable stack changes in `projects/docker-compose.example.yml`.

Start the stack:

```powershell
cd D:\Development\projects
docker compose up -d --build
```

Run the web UI:

```powershell
devhelper.exe serve --http 127.0.0.1:8787
```

Open `http://127.0.0.1:8787`.

## CLI

Create a project:

```powershell
devhelper.exe project create --company company --project app --php 8.3 --db mysql9
```

Supported DB aliases:

- `none`
- `mysql` or `mysql-8.4`
- `mysql9`, `mysql-9.6`, or `mysql-9.7` (stored as `mysql-9.7`)
- `mariadb10`
- `mariadb12`
- `postgres`

Import a dump:

```powershell
devhelper.exe db import --company company --project app --file D:\dumps\app.sql
```

Repair a database missing from an existing project (including projects saved by older versions after a failed database creation):

```powershell
devhelper.exe db ensure --company company --project app
```

This reuses the stored database name and credentials, creates missing database/user objects, and preserves tables. Shared legacy database/login ownership is reported for manual resolution. Repeating `project create` for an existing project is rejected to protect credentials and configuration.

Drop a project:

```powershell
devhelper.exe project drop --company company --project app
```

## Runtime Files

devhelper writes runtime files outside Git-tracked project data:

- app directories under `${APPS_ROOT}/<company>/<project>`
- nginx vhosts under `${NGINX_CONF_ROOT}`
- certs under `${NGINX_EXTERNAL_ROOT}/certs/<domain>`
- metadata in `${DATA_ROOT}/devhelper.store.json`
- Docker Compose env in `${COMPOSE_DIR}/.env`
- local Docker Compose file at `${COMPOSE_DIR}/docker-compose.yml`

These paths are intentionally ignored by Git so local company/project data stays local.

Project metadata keeps the managed per-project credentials in `db_user` / `db_pass`. `db_connections` records observed application database dependencies and their configuration sources without passwords; its observed login can differ from the managed login. `db_host` / `db_port` describe the primary endpoint, while `db_external` marks a primary database as externally managed. These endpoint fields are inventory; local operations still target the configured Compose service or native client endpoint.

Drop, replacement, and `db ensure` check other projects' local secondary database references as well as primary ownership. Ordinary imports do not perform this shared-database check. An external primary database blocks ensure, import, and replacement; dropping its project may remove local files and metadata but skips database cleanup.

## Build and test

Go 1.27.1 or newer is required. The Go toolchain can download the required version automatically. The production web assets are checked in, so a normal Go build needs no Node.js installation:

```powershell
go build -o devhelper.exe ./cmd/devhelper
go test ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
```

Use these scoped package patterns in a working development stack: `./...` also walks potentially very large local `projects/apps` and `projects/data` directories. On a clean checkout, `go test ./...` works as usual.

For UI changes, follow [frontend/README.md](frontend/README.md), rebuild the assets, then rebuild Go. The embedded React UI works without external CDN connections.

See [the migration and analysis report](docs/MIGRATION_2026-09.md) for dependency versions, behavior changes, test coverage, and compatibility notes.
