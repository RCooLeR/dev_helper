# Architecture (MVP)

- macOS/Linux runtime root: `./projects`
- Windows default runtime root (WSL): `/data/projects`
  - mirror path example: `\\wsl$\Ubuntu\data\dev-helper\projects`

The tool writes:
- vhosts to `${NGINX_CONF_ROOT}`
- certs to `${NGINX_EXTERNAL_ROOT}/certs/<domain>/`
- metadata to `${DATA_ROOT}/devhelper.store.json`
