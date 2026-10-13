# Manafield Noctite Adapter

Read-only Module Protocol adapter for the **Noctite** (Misskey 2026.10.0-based)
server, not a clone, Docker orchestrator, database migration tool, or reverse proxy.

## Integration boundary

```text
Manafield Core
    |
    +--- manafield-noctite adapter :8080
                |
                +--- GET http://noctite:3000/healthz/
                               |
                               +--- Noctite (separately managed)
                                        |-- PostgreSQL (isolated)
                                        |-- Redis (isolated)
                                        +-- media (persistent)
```

The adapter implements `GET /manafield/health` and
`GET /manafield/noctite/status`. It answers 200 / `status=ok` only when
Noctite's health endpoint returns HTTP 200, and answers 503 / `degraded`
otherwise. No cookies, account credentials, module secrets, admin controls,
or user traffic pass through the adapter.

Configuration: `MANAFIELD_NOCTITE_HEALTH_URL` can select a trusted **internal**
HTTP `/healthz/` endpoint (default: `http://noctite:3000/healthz/`).
There are **no public ingress rules** in this module.

## Run locally

```bash
go test ./...
go vet ./...
go run ./cmd/manafield-noctite
```

Without a reachable Noctite backend, the adapter starts but returns
`503 degraded` intentionally.

## Manafield Instance entry

```yaml
  - id: manafield-noctite
    enabled: true
    source:
      type: dir
    build:
      type: docker
      context: .
      dockerfile: Dockerfile
```

This entry only registers the **adapter**. A separate, administrator-approved
development deployment must first supply the Noctite server, Redis, PostgreSQL,
persistent file storage, and the shared internal Docker network. Noctite
itself must be deployed and backed up independently.

The adapter is intended to become a separately distributed public module
once the runtime contract and pilot integration are validated.

### Security

Do not grant Docker socket or sudo to the adapter.
Do not expose the health adapter to the public internet.
Do not copy Misskey databases, Redis credentials or ActivityPub federation
identities from production into development.
