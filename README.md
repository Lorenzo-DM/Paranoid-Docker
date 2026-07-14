# Paranoid Docker Update

Web UI for managing Docker container and Compose stack updates with rollback support.

## Features

- List and update standalone containers and Compose stacks
- Pull new images with real-time SSE progress streaming
- Save images to `.tar` archives before updating (rollback support)
- Snapshot Compose stack configs (including env files)
- Stream live container/stack logs
- Download saved images and rollback archives
- Bulk update multiple containers or stacks at once

## Architecture

```
cmd/server/          # Entry point — Echo HTTP server
internal/
  docker/            # Docker client wrapper
  handler/           # HTTP handlers + SSE streaming + job stores
  model/             # Domain types (Container, ComposeStack, StackEvent, ...)
  repository/        # Docker API queries
  service/           # Business logic (update, save, rollback, compose, digest check)
webui/               # React frontend (see webui/README.md)
images/              # Saved image tarballs
rollbacks/           # Rollback snapshots (per container/stack)
```

## Requirements

- Go 1.26+
- Docker daemon running and accessible (default socket)
- `docker compose` CLI available in `PATH` (for stack operations)

## Running

### Local (development)

```bash
go run ./cmd/server
```

Server listens on `:1323`. Frontend dev server at `http://localhost:5173` (CORS pre-configured).

### Docker Compose (production)

Use pre-built image from GHCR:

```bash
docker compose up -d
```

Image: `ghcr.io/lorenzo-dm/paranoid-docker:latest`

#### Build locally

```bash
docker compose build
docker compose up -d
```

Builds 3-stage image: Bun (frontend) → Go (backend) → Alpine runtime.

#### Configuration

Env vars in `docker-compose.yaml`:

| Variable | Default | Purpose |
|----------|---------|---------|
| `TZ` | `Europe/Rome` | Timezone |
| `LISTEN_ADDR` | `:1323` | Server bind address |
| `ALLOWED_ORIGINS` | `http://localhost:5173` | CORS origins (comma-separated) |
| `COMPOSE_STACKS_DIR` | *(not set)* | Host path to compose stacks directory (optional) |
| `ROLLBACKS_DIR` | `rollbacks` | Directory for rollback snapshots |
| `IMAGES_DIR` | `images` | Directory for saved image tarballs |

#### Security hardening

The shipped `docker-compose.yaml` runs with `no-new-privileges:true` and
`cap_drop: ALL`. The container still runs as **root** by default because it
must talk to the docker socket.

**Non-root (opt-in):** the container user needs the docker socket's group.
Find the GID on the host and set the compose `user:` accordingly:

```bash
stat -c %g /var/run/docker.sock   # e.g. 998
```

```yaml
    user: "1000:998"
```

Note: compose-file rollback mode runs the `docker` CLI inside the container;
the non-root user must retain read access to the mounted stacks directory.

#### Volumes

- `/var/run/docker.sock` — Docker daemon socket (read-only)
- `rollbacks/` — Rollback snapshots (named volume)
- `images/` — Saved image tarballs (named volume)
- `COMPOSE_STACKS_DIR` — (optional) Mount your compose stacks directory for file-based rollbacks and native `docker compose` updates. Without it, rollbacks and updates use `docker inspect` data only. To enable, uncomment the volume line in `docker-compose.yaml` and set `COMPOSE_STACKS_DIR`

## API

Base path: `/api/v1`

### Stacks

| Method | Path | Description |
|--------|------|-------------|
| GET | `/stacks` | List Compose stacks |
| POST | `/stacks/:name/update` | Trigger stack update |
| GET | `/stacks/:name/update-status` | SSE stream for update progress |
| POST | `/stacks/:name/save-images` | Save all stack images |
| GET | `/stacks/:name/save-status` | SSE stream for save progress |
| POST | `/stacks/:name/save-and-update` | Save images then update |
| GET | `/stacks/:name/save-update-status` | SSE stream for save+update |
| POST | `/stacks/:name/snapshot` | Snapshot stack config |
| GET | `/stacks/:name/logs` | SSE stream of stack logs |
| GET | `/stacks/:name/rollbacks` | List rollback archives |
| GET | `/stacks/:name/rollbacks/*` | Download rollback archive |

### Containers

| Method | Path | Description |
|--------|------|-------------|
| GET | `/containers` | List standalone containers |
| POST | `/containers/:id/update` | Trigger image pull + recreate |
| GET | `/containers/:id/pull-status` | SSE stream for pull progress |
| GET | `/containers/:id/logs` | SSE stream of container logs |
| GET | `/containers/:id/rollbacks` | List rollback archives |
| GET | `/containers/:id/rollbacks/:filename` | Download rollback archive |
| POST | `/containers/:id/save-image` | Save current image to tar |
| GET | `/containers/:id/save-status` | SSE stream for save progress |

### Images

| Method | Path | Description |
|--------|------|-------------|
| GET | `/images` | List saved image tarballs |
| GET | `/images/:filename` | Download image tarball |

## Update request body

`POST` update/save-and-update endpoints accept:

```json
{ "include_env": true }
```

`include_env: true` includes `.env` files in stack snapshots.
