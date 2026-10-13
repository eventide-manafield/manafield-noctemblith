# Manafield Noctite Adapter

Read-only Module Protocol adapter for **Noctite**, a Misskey 2026.10.0-based
social server. This repository is **not** the Noctite fork, reverse proxy,
container orchestrator, or database migration tool.

## Architecture

```text
Manafield Core
    +-- manafield-noctite adapter :8080
          +-- GET http://noctite:3000/healthz/
                     +-- Noctite (independently managed)
                           +-- PostgreSQL (isolated)
                           +-- Redis (isolated)
                           +-- persistent media
```

Two Manafield Module Protocol operations are registered:

- `GET /manafield/health`: Module health operation
- `GET /manafield/noctite/status`: Noctite readiness

Noctite HTTP 200 maps to HTTP 200 with `status: "ok"`, while an error, outage,
or non-200 response maps to HTTP 503 with `status: "degraded"`.

Only the service's health status is read. The adapter doesn't proxy user
requests, expose the Noctite site, share credentials, or implement account SSO.

## Local development

Requires Go 1.27.1:

```bash
go test ./...
go vet ./...
go build ./cmd/manafield-noctite
go run ./cmd/manafield-noctite
```

Set `PORT` to choose the adapter's listen port (default 8080), and
`MANAFIELD_NOCTITE_HEALTH_URL` to configure a trusted internal HTTP
`/healthz/` URL (default `http://noctite:3000/healthz/`).
Without a reachable Noctite backend, the adapter intentionally returns 503.

## Manafield Instance configuration

After review, point a **separate development Instance** at this repository:

```yaml
  - id: manafield-noctite
    enabled: true
    source:
      type: git
      repository: https://github.com/eventide-manafield/manafield-noctite.git
      ref: main
    build:
      type: docker
      context: .
      dockerfile: Dockerfile
```

For production, replace `main` with an audited, immutable release tag or
commit. For local experiments, `source.type: dir` is also supported when a
corresponding module directory exists.

This entry registers **only the adapter**. Administrators must separately
deploy Noctite, Redis, PostgreSQL, persistent media volumes, and the internal
network shared with the adapter. Nothing here creates an ingress router or
publishes `noctite.manafield.studio`.

## Optional Manafield Web presentation

`manafield-web` is an **optional** presentation Shell that reads the Core
Module Registry through `GET /modules`. Registering this adapter makes its
name and status operations discoverable to Core clients; Noctite remains a
standalone SNS, and public browsing to `noctite.manafield.studio` would
require a separate reverse-proxy/ingress configuration.

Do **not** declare `manafield-web` as a hard Module capability dependency.
A future Web presentation may add a discoverable Noctite launch link and a
status card without routing the SNS user traffic through `manafield-web`.

## Integration readiness

- [x] Manafield module manifest and two HTTP operation contracts
- [x] HTTP health adapter with fail-closed readiness checks
- [x] Go unit tests, vet and local build
- [ ] Noctite image build and runtime provisioning
- [ ] Development Instance registration and full end-to-end health check
- [ ] Public routing / federation with a stable domain
- [ ] Cross-module accounts, permission scopes and SSO

Do not give the adapter Docker socket, root privileges or administrator
credentials. Don't use the operating Manafield PostgreSQL volume or old
Misskey ActivityPub identity data for Noctite development.

## License

This independently developed Manafield adapter is licensed under
**Apache License 2.0**, matching [Manafield Core](https://github.com/eventide-manafield/manafield).
See [LICENSE](LICENSE).

The separate [Noctite/Misskey fork](https://github.com/eventide-manafield/noctite)
remains **AGPL-3.0**. This adapter does not copy or redistribute the
Misskey source code.
