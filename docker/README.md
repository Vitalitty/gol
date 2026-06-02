# Docker Runtime

Use the published `ghcr.io/vitalitty/gol` image instead of downloading the old
upstream binary during a Docker build. The image is built from this fork, embeds
the current frontend, and can be configured at container start.

## Build Locally

```sh
docker build -f docker/Dockerfile -t ghcr.io/vitalitty/gol:local .
```

## Environment Arguments

The image entrypoint adds container-safe defaults:

```text
--host=0.0.0.0 --port=${GOL_PORT:-3003} --open=false
```

Set these variables to generate `gol` flags:

- `GOL_FILE_PATTERNS`: newline-separated `-f=` values.
- `GOL_SSH_TARGETS`: newline-separated `-s=` values.
- `GOL_DOCKER_ALL=true`: adds `-d=` for all running containers.
- `GOL_DOCKER_TARGETS`: newline-separated `-d=` values.
- `GOL_BASE_URL`, `GOL_EVERY`, `GOL_LIMIT`, and `GOL_ACCESS` map to their matching server flags.

Any `docker run` arguments or Compose `command:` values are appended after the
environment-generated flags.

In `docker/.env`, use `\n` between repeated values because `.env` files are
line-based:

```env
GOL_FILE_PATTERNS=/logs/*.log\n/logs/*.log.*
```

## Run With Host Log Files

```sh
docker run --rm \
  -p 3003:3003 \
  -v "$PWD/docker/logs:/logs:ro" \
  -e GOL_FILE_PATTERNS="/logs/*.log" \
  ghcr.io/vitalitty/gol:latest
```

Multiple file patterns:

```sh
docker run --rm \
  -p 3003:3003 \
  -v /home/vitalitty/docker/proxy/data/npm/logs:/logs:ro \
  -e GOL_FILE_PATTERNS="/logs/*.log
/logs/*.log.*" \
  ghcr.io/vitalitty/gol:latest
```

Direct flag override style also works:

```sh
docker run --rm \
  -p 3003:3003 \
  -v /home/vitalitty/docker/proxy/data/npm/logs:/logs:ro \
  ghcr.io/vitalitty/gol:latest \
  -f=/logs/*.log -f=/logs/*.log.*
```

## Run With Docker Logs

Docker log mode needs access to the host Docker socket. Mounting the socket
gives the container control over Docker on the host, so use it only for trusted
deployments.

```sh
docker run --rm \
  -p 3003:3003 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --user root \
  -e GOL_DOCKER_ALL=true \
  ghcr.io/vitalitty/gol:latest
```

Read a specific file inside a container:

```sh
docker run --rm \
  -p 3003:3003 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --user root \
  -e GOL_DOCKER_TARGETS="container-id /app/logs.log" \
  ghcr.io/vitalitty/gol:latest
```

## Run With SSH Logs

Mount SSH keys read-only and reference the mounted key path:

```sh
docker run --rm \
  -p 3003:3003 \
  -v "$HOME/.ssh:/home/gol/.ssh:ro" \
  -e GOL_SSH_TARGETS="user@example.com private_key=/home/gol/.ssh/id_rsa /var/log/*.log" \
  ghcr.io/vitalitty/gol:latest
```

## Docker Compose

Create a local env file and adjust paths as needed:

```sh
cp docker/.env.example docker/.env
docker compose --env-file docker/.env -f docker/docker-compose.yml up -d
```

For local image development, use the tracked `docker/docker-compose.override.yml`
override. It builds `docker/Dockerfile` from this checkout and tags the result
as `ghcr.io/vitalitty/gol:local`.

From the repository root, pass both files explicitly:

```sh
docker compose --env-file docker/.env \
  -f docker/docker-compose.yml \
  -f docker/docker-compose.override.yml \
  up --build -d --force-recreate
```

From the `docker/` directory, Compose auto-loads the override:

```sh
docker compose --env-file .env up --build -d --force-recreate
```

When mounting several host directories below `/logs`, replace the base
`${GOL_LOG_DIR}:/logs:ro` mount with a writable parent such as `tmpfs`, then add
each source as a read-only child mount:

```yaml
services:
  gol:
    volumes:
      - type: tmpfs
        target: /logs
      - "C:/Users/theow/Downloads/logs:/logs/npm:ro"
      - "C:/Users/theow/Downloads/redm-dev:/logs/redm-dev:ro"
      - "C:/Users/theow/Downloads/redm-prod:/logs/redm-prod:ro"
```

## Volume Notes

- The image runs as a non-root `gol` user by default.
- Host log directories must be readable by the container user.
- Use `--user root` only when needed, such as Docker socket access.
- The default container port is `3003`.
