# Docker Runtime

This folder contains a container build and Compose examples for running `gol`
without installing the binary on the host.

## Build

```sh
docker build -f docker/Dockerfile -t vitalitty/gol:local .
```

## Run With Host Log Files

Mount a host log directory read-only at `/logs` and pass the glob pattern to
`gol`.

```sh
docker run --rm \
  -p 3003:3003 \
  -v "$PWD/docker/logs:/logs:ro" \
  vitalitty/gol:local \
  --host=0.0.0.0 --open=false -f="/logs/*.log"
```

Open `http://localhost:3003`.

## Run With Docker Logs

Docker log mode needs access to the host Docker socket. Mounting the socket
gives the container control over Docker on the host, so only use it for trusted
local deployments.

```sh
docker run --rm \
  -p 3003:3003 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --user root \
  vitalitty/gol:local \
  --host=0.0.0.0 --open=false -d=""
```

Read a specific file inside a container:

```sh
docker run --rm \
  -p 3003:3003 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --user root \
  vitalitty/gol:local \
  --host=0.0.0.0 --open=false -d="container-id /app/logs.log"
```

## Docker Compose

Create a local env file and adjust paths as needed:

```sh
cp docker/.env.example docker/.env
```

File log mode:

```sh
docker compose --env-file docker/.env -f docker/compose.yml --profile files up --build
```

Docker log mode:

```sh
docker compose --env-file docker/.env -f docker/compose.yml --profile docker up --build
```

## SSH Logs

Mount SSH keys read-only and pass the normal `-s` flag:

```sh
docker run --rm \
  -p 3003:3003 \
  -v "$HOME/.ssh:/home/gol/.ssh:ro" \
  vitalitty/gol:local \
  --host=0.0.0.0 --open=false \
  -s="user@example.com private_key=/home/gol/.ssh/id_rsa /var/log/*.log"
```

## Volume Notes

- The image runs as a non-root `gol` user by default.
- Host log directories should be readable by the container user.
- Use `--user root` only when needed, such as Docker socket access.
- The default container port is `3003`.
