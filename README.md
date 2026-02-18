# devhelper

## Windows quick start

### 1) Prereqs
- Docker Desktop
- WSL2 (Ubuntu) + Docker Desktop WSL integration enabled
- `mkcert` installed on Windows (optional, for HTTPS without warnings)
- MySQL client + Postgres client installed on Windows (for DB create/import)

### 2) Create config: `.devhelper-config.json`
Example (edit paths/passwords to your setup):

```json
{
  "root_dir": "/data/dev-helper/projects",
  "host_mirror_dir": "\\wsl$\\Ubuntu\\data\\dev-helper\\projects",
  "wsl_distro": "Ubuntu",

  "apps_root": "/data/dev-helper/projects/apps",
  "data_root": "/data/dev-helper/projects/data",
  "nginx_conf_root": "/data/dev-helper/projects/containers/nginx/conf.d",
  "nginx_external_root": "/data/dev-helper/projects/containers/nginx/external",

  "compose_dir": "E:\\Development\\projects",
  "default_domain_pattern": "<project>.<company>.local",

  "mysql_service": "mysql",
  "mysql_host": "127.0.0.1",
  "mysql_port": 3306,
  "mysql_root_pass": "change-me",
  "mysql_cli": "C:\\Program Files\\MySQL\\MySQL Server 8.4\\bin\\mysql.exe",

  "postgres_service": "postgres",
  "postgres_host": "127.0.0.1",
  "postgres_port": 5432,
  "postgres_super_pass": "change-me",
  "psql_cli": "C:\\Program Files\\PostgreSQL\\16\\bin\\psql.exe",

  "docker_cli": "docker",
  "mkcert_cli": "mkcert",

  "store_file": "/data/dev-helper/projects/data/devhelper.store.json",
  "log_level": "debug"
}
```

### 3) Init (creates runtime dirs + writes `projects/.env`)
From repo root:

```powershell
devhelper.exe init
```

### 4) Start Docker stack
```powershell
cd E:\Development\projects
docker compose up -d
```

### 5) Run devhelper (web UI)
```powershell
devhelper.exe serve --addr 127.0.0.1:8787
```

Open:
- http://127.0.0.1:8787

## What happens when you create a project in UI

- Windows mirror directory:
  - `projects/apps/<company>/<project>/`
- WSL runtime directory (fast FS):
  - `/data/dev-helper/projects/apps/<company>/<project>/`
- Nginx vhost generated into:
  - `/data/dev-helper/projects/containers/nginx/conf.d/`
- Hosts file entry added on Windows:
  - `127.0.0.1 <domain>`
- DB created via Windows `mysql.exe` / `psql.exe` over TCP:
  - MySQL: `127.0.0.1:3306`
  - Postgres: `127.0.0.1:5432`
