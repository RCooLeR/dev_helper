# devhelper

`devhelper` manages a local PHP development stack: app directories, nginx vhosts, local TLS certs, hosts-file entries, Docker Compose services, and per-project database metadata.

## Prerequisites

- Docker Desktop and Docker Compose.
- `mkcert` for local HTTPS certificates.
- MySQL client and PostgreSQL `psql` client on the host.
- On Windows, WSL2 with Ubuntu is the default runtime filesystem.

## Configuration

Run the tool from the repository root. If `.devhelper-config.json` is missing, devhelper uses built-in defaults and `devhelper init` writes the config plus `projects/.env`.

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
  "mysql_service": "mysql-8.4",
  "mysql9_service": "mysql-9.7",
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

These paths are intentionally ignored by Git so local company/project data stays local.
