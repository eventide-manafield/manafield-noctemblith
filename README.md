# Manafield Noctemblith Adapter

A private, read-only [Manafield Module Protocol](https://github.com/eventide-manafield/manafield) adapter for an **independently deployed** Noctemblith instance (a Misskey-based ActivityPub server).

## Responsibilities and dependency boundary

```text
Manafield Core (registry)
   |
   +-- module-manafield-noctemblith (this adapter, no public exposure)
           |
           +-- edge network --> GET http://noctemblith-dev:3000/healthz/
           |                     Noctemblith owns its own PostgreSQL, Redis, media
           |
           +-- modules network --> GET http://module-manafield-account-core:8080/manafield/health
                                  Account Core owns its PostgreSQL Capability binding
```

The adapter **does not** connect to either database. A Manafield `database.postgresql` Capability would be misleading and would expose unnecessary privileges. The owning Account Core module already declares `requires.capabilities.state=database.postgresql`; Noctemblith uses its own internal Compose PostgreSQL container.

Its endpoints are `GET /manafield/health` and `GET /manafield/noctemblith/status`. Both report JSON:
- `status`: `ok` or `degraded`
- `service`: `manafield-noctemblith`
- `version`: adapter version
- `noctemblithReady`: Noctemblith returned HTTP 200
- `accountCoreReady`: Account Core reports the expected service, status `ok`, and database status `ready`
- `accountDatabaseReady`: Account Core reports its DB binding as `ready`

Both dependencies are required for an `ok` result; otherwise the adapter fails closed with HTTP 503. No access tokens or secret state are exposed. Health readiness **does not** imply OAuth login or account linking is implemented.

## Runtime contract

| Configuration variable | Default | Used for |
|---|---|---|
| `MANAFIELD_NOCTEMBLITH_HEALTH_URL` | `http://noctemblith-dev:3000/healthz/` | Noctemblith app health |
| `MANAFIELD_NOCTEMBLITH_ACCOUNT_HEALTH_URL` | `http://module-manafield-account-core:8080/manafield/health` | Account Core and its database readiness |
| `PORT` | `8080` | Adapter listener |

The two URLs must be fixed, internal HTTP health endpoints. Never accept a URL from a caller. The adapter must join the existing `manafield-modules` and `manafield-edge` networks. No host ports, ingress rule or database credentials are needed. HTTP redirects are not followed.

## Develop and validate

```bash
export PATH=/opt/manafield/ai-workspace/toolchains/go1.27.1/bin:$PATH
go test ./...
go vet ./...
go run ./cmd/manafield-noctemblith
```

On a machine without the expected networks, the module still starts but correctly returns 503.

## Instance module registration

Add to the `modules:` section in an owner-reviewed Instance definition:

```yaml
  - id: manafield-noctemblith
    enabled: true
    source:
      type: dir
    build:
      type: docker
      context: .
      dockerfile: Dockerfile
```

The `id` must match the module source directory and manifest. Do not add `bindings.state`: this module needs no DB Capability. Keep Account Core enabled and ensure both independent services are on their expected networks.

## SSO milestone (separate from this adapter)

Account Core already implements OAuth authorization-code + PKCE S256 and `/account/oauth/userinfo`. Noctemblith currently has only OAuth protocol helpers; it still requires a callback route, identity-to-local-user mapping, explicit existing-account linking flow, session issuance and front-end controls. Its OAuth client must be added to Account Core's approved redirect URI list **when those routes are implemented**:

```text
client_id: manafield-noctemblith-dev
redirect_uri: https://noctemblith-dev.manafield.studio/auth/manafield/callback
```

These are proposed values, not an assertion that the OAuth client is currently registered. Bind accounts by immutable Account Core `sub`, never a mutable username; no raw PostgreSQL access should cross the service boundary.

## Deployment safety

The current Noctemblith domain is a development-only ActivityPub origin. The production domain, image digest, database backup, routing and SSO account policy are separate release decisions. Do not destroy the dev database to install this adapter or join the two DBs. Module registration in a file does not deploy containers automatically.
