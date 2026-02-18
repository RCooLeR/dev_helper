# devhelper

## Prereqs
- Windows host: Docker Desktop + WSL2 (Ubuntu)
- `mkcert` installed on Windows (for HTTPS cert generation)
- MySQL + Postgres CLI on Windows (`mysql.exe`, `psql.exe`)

## 1) Config
Create `.devhelper-config.json` in the repo root (same folder where you run `devhelper.exe`).

Example:

```json
{
  "root_dir": "/data/dev-helper/projects",
  "host_mirror_dir": "\\\\wsl$\\\\Ubuntu\\\\data\\\\dev-helper\\\\projects",
  "wsl_distro": "Ubuntu",
  "apps_root": "/data/dev-helper/projects/apps",
  "data_root": "/data/dev-helper/projects/data",
  "nginx_conf_root": "/data/dev-helper/projects/containers/nginx/conf.d",
  "nginx_external_root": "/data/dev-helper/projects/containers/nginx/external",
  "compose_dir": "E:\\\\Development\\\\projects",
  "default_domain_pattern": "<project>.<company>.local",
  "mysql_service": "mysql",
  "mysql_host": "127.0.0.1",
  "mysql_port": 3306,
  "mysql_root_pass": "change-me",
  "mysql_cli": "C:\\\\Program Files\\\\MySQL\\\\MySQL Server 8.4\\\\bin\\\\mysql.exe",
  "postgres_service": "postgres",
  "postgres_host": "127.0.0.1",
  "postgres_port": 5432,
  "postgres_super_pass": "change-me",
  "psql_cli": "C:\\\\Program Files\\\\PostgreSQL\\\\18\\\\bin\\\\psql.exe",
  "docker_cli": "docker",
  "mkcert_cli": "mkcert",
  "store_file": "/data/dev-helper/projects/data/devhelper.store.json",
  "log_level": "debug"
}
```

## 2) Init
From repo root (Windows):

```powershell
devhelper.exe init
```

## 3) Start Docker stack
From WSL (Ubuntu):

```bash
cd /mnt/<drive>/<repo>/projects
docker compose up -d --build
```

## 4) Run Web UI
From repo root (Windows):

```powershell
devhelper.exe serve --addr 127.0.0.1:8787
```

Open:
- http://127.0.0.1:8787
