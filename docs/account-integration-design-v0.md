# Noctemblith ↔ Manafield Account: opt-in SSO and just-in-time accounts (proposal v0)

**Status:** design only — not activated, not an implemented auth endpoint.
**Scope:** Manafield-embedded Noctemblith mode; standalone Noctemblith must keep working without Manafield.
**Basis:** Misskey `2026.10.0` (Noctemblith fork), Manafield Core Capability Contract v0,
and the Account Core OAuth authorization-code + PKCE implementation inspected 2026-10-13.

## Goal / UX

A person with an **active Manafield Account** may open Noctemblith, choose
"Continue with Manafield", and use a corresponding Noctemblith account.

- The first visit requires an explicit explanation/consent for creating an
  **externally visible ActivityPub account**, and a confirmed local username.
- Subsequent visits resolve the same Noctemblith user by the stable Manafield
  identity UUID (`sub`); *not* by mutable display name, account handle or email.
- Standalone Noctemblith keeps its own login/registration paths when integration is
  not enabled. Each Noctemblith instance remains free to set its own registration
  policy, moderation policy and account rules.
- Logging into Manafield alone must **not** cause bulk/background Noctemblith signup.

## Actual capabilities verified in current source

Account Core currently supports:

| Endpoint | Behavior |
|---|---|
| `GET /account/oauth/authorize` | Exact registered client/HTTPS redirect, auth code + S256 PKCE, authenticated active account |
| `POST /account/oauth/token` | Single-use code exchange, S256 PKCE verification |
| `GET /account/oauth/userinfo` | Active account only; returns `sub` (identity UUID), `preferred_username` |
| `POST /account/oauth/end-session` | Revoke the Account OAuth/session linkage per existing implementation |

The Account Core source has a client registration configuration
`MANAFIELD_ACCOUNT_OAUTH_CLIENTS_JSON` mapping client ID → **one exact HTTPS
callback URL per client**. Registration/provisioning for a Noctemblith OAuth client
is **not** yet installed or deployed.

The current Account Core module manifest advertises only its health operation
and PostgreSQL dependency; it does **not yet provide** an OAuth Capability.
A legacy `manafield-account` descriptor advertises `manafield.identity`
v1.0.0 but that broad identifier must **not** be interpreted as proof of
OAuth/PKCE availability.

Misskey/Noctemblith implements local `SignupService`, account keys and
sign-in/token handling. Its OAuth2 provider functionality is for third-party
apps authenticating *to Misskey*, not a Manafield Account OAuth consumer.
A new, opt-in **OAuth client and local-user provisioning path in Noctemblith**
is required. No existing public signup API or direct SQL INSERT shortcut
should be repurposed to bypass moderation / invariant checks.

## Proposed compatibility contract (not yet advertised)

A new, narrower Capability name prevents incompatible identity implementations
from satisfying a login dependency by accident.

- Provider Capability: `manafield.identity.oauth2` **v1.0.0**
- Consumer requirement slot: `identity`, requiring `^1.0.0`
- Provider candidate: `manafield-account-core`, *after* its manifest and
  contract tests are updated to explicitly advertise this Capability.
- Consumer: `manafield-noctemblith` **only when OAuth integration is implemented**.
- Concrete binding: administrator explicitly binds the requirement to the
  Account Core instance; do not auto-choose a provider just because it exists.
- Provider endpoint, registered client ID, redirect URL and trust settings are
  **instance configuration**, not part of a Capability version string.

See `account-capability.v0.example.json` for **illustrative snippets**.
Do not paste those changes into production manifests yet: this adapter currently
does health monitoring only, and adding a hard `requires` today would advertise
an unmet dependency.

Contract v1 semantics must specify at least:
1. Authorization Code + PKCE S256, strict allowed client ID + exact redirect URI.
2. Verified, active user UUID as `sub`; stable inside the given Manafield issuer.
3. Defined token lifetime, error/revocation response and no default email claim.
4. Issuer identity and allowed origins are administrator-pinned, not request-supplied.
5. A **separate, reviewed account-status/lifecycle mechanism** for already-created
   Noctemblith sessions. OAuth `userinfo` at sign-in does not guarantee instant
   revocation of a previously issued Noctemblith session.

A reusable provider may be published under the same capability only when all
v1 semantics have compatible observable behavior. OAuth endpoints currently
live in Account Core web handlers; additional public Module Operations may be
needed if the platform resolves providers exclusively through Operation bindings.

## Responsibility boundaries

| Component | Owns | Does not own |
|---|---|---|
| Manafield Account Core | Identity, password, active/disabled status, OAuth auth code/token/userinfo | Noctemblith username or social objects |
| Manafield Core | Module catalog, capability binding, module lifecycle | OAuth password capture or Noctemblith sessions |
| `manafield-noctemblith` | Integration status, contract/dependency declaration, later bounded integration management | User passwords, arbitrary database access or exposing Noctemblith publicly |
| Noctemblith fork | OAuth *client*, Noctemblith user records/keys, local session creation, ActivityPub identity, linking table | Manafield password DB |
| Manafield Web / future Mobile | A launch route linking to Noctemblith; UI showing integration readiness | Required server-side OAuth dependency or credential broker |

**OAuth callback terminates inside Noctemblith** on a configured route such as
`https://noctemblith-dev.manafield.studio/auth/manafield/callback` (route **not
implemented**). The Noctemblith backend stores PKCE verifier + state transiently
and exchanges the code with Account Core. It calls `userinfo` **server-side**
and obtains the trusted UUID, then invokes its *internal* account link/create
path. Do not pass bearer tokens, PKCE verifier or a login session token to a
frontend widget, browser URL, logs, or `manafield-noctemblith` health endpoint.

A Noctemblith OAuth consumer can be toggled off without affecting standalone
Noctemblith functionality. Installing an adapter does not imply enabling SSO
silently.

## JIT provisioning state machine

```text
Noctemblith entry
    ├── integration disabled → native Noctemblith login
    └── click "Continue with Manafield"
          → save one-time state, PKCE verifier, intended destination
          → Account Core authorize (registered client, exact callback)
          → Noctemblith callback validates state + PKCE exchange + userinfo
          → verified issuer + active sub
              ├── existing link → enforce account/moderation/session policy
              │                     → Noctemblith local session
              ├── no link → show consent + available username picker
              │              ├── declined → no account created
              │              └── accepted → transactional local provisioning
              │                                  → atomic link, session
              └── ambiguous/collision/revoked → fail closed, human remediation
```

Link key: `(trusted_provider_id, manafield_sub)` → `noctemblith_user_id`.
The pair must be unique, and each Noctemblith user may be linked to at most
one Manafield identity for this provider (unless an explicit migration
protocol is later designed). Database constraints, transaction boundaries
and idempotency are **Noctemblith-owned**. Concurrency / retry must return the
same linked identity and never generate duplicate ActivityPub actors.

Only a selected, checked local username becomes the ActivityPub handle.
`preferred_username` from Manafield may *suggest* a candidate, not
automatically claim an existing Noctemblith username. Missing email claim is
normal; never fabricate a verified email. An existing native Noctemblith account
with the same name is **not** automatically seized or linked. Migration/linking
an existing Noctemblith account requires proving control of **both** identities,
explicit consent, and separate security review.

If the Manafield account is disabled, deleted or its session revoked, a
bound Noctemblith session must be invalidated according to a defined policy;
Noctemblith-local account deletion, user data retention, moderation and
ActivityPub federation cannot be assumed equivalent to Manafield deletion.
Define logout propagation and timely session revocation before shipping
SSO. Account Core's `userinfo` rejects disabled subjects for newly
validated tokens, but it does not alone terminate all Noctemblith sessions.

## Failures / abuse cases to test before implementation completes

| Scenario | Expected result |
|---|---|
| Manafield OAuth client not registered | Integration unavailable; no fallback account creation |
| State mismatch, PKCE replay, callback with wrong issuer | Reject, no link/no session |
| Manafield user disabled or invalid bearer | Reject; no Noctemblith session issued |
| Username occupied/reserved or prohibited | Prompt for another, do not claim account |
| Two concurrent first logins for same UUID | Exactly one Noctemblith actor/link, safe retry |
| Two UUIDs claim same Noctemblith actor | Reject; prove pre-existing ownership to migrate |
| Noctemblith suspended user with valid Manafield UUID | Preserve local suspension |
| First-login consent declined | Nothing provisioned |
| Provider unreachable/timeout | Fail closed; don't silently switch to native signup |
| Noctemblith in standalone mode | Native login continues with no Manafield dependency |
| Central disable/logout after Noctemblith login | Session revoked within specified policy SLA — **not solved yet** |
| User requests account unlink/deletion | Explicit lifecycle policy; no implicit actor deletion |

## Implementation milestones

**A. Contract** — add provider `manafield.identity.oauth2` only after v1
contract review and API verification; add corresponding consumer requirement
to the adapter only after the actual SSO path exists and a binding can be
supplied. Preserve existing health adapter compatibility in the meantime.

**B. Noctemblith fork** — OAuth client + strictly registered callback;
first-login consent UI; local identity map; privileged internal create/login
operations; Noctemblith account/session tests. Keep AGPL-3.0 notices in fork.

**C. Account Core integration** — provision exact callback client via
admin-owned config, add lifecycle/session invalidation events or another
bounded revalidation path, test revocation. Do not reuse profile-lifecycle
secrets for Noctemblith; grant least privilege if a new secret is needed.

**D. Full pilot** — isolated dev stack / fake federation (not public);
run 2-user collision, race and disable/logout scenarios; only then bind the
Capability in the dev Instance and consider a production rollout.

**Non-goals for this proposal:** modifying live instance.yaml, publishing
Noctemblith under ActivityPub, sharing login passwords, changing Core resolver
semantics, importing existing old Misskey accounts, or implementing
Noctemblith/Echo gaming widgets.
