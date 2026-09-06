# Go API Design Guidelines

Standards and architectural rules for designing and implementing HTTP APIs in Go for Doula Cloud. These guidelines are distilled from proven production API practices (see [Sean Goedecke on Good API Design](https://www.seangoedecke.com/good-api-design/)) and adapted to modern Go 1.22+ idioms.

---

## 1. Core Philosophy: "Good APIs Are Boring"

* **Familiarity Over Novelty**: An API consumer should understand how an endpoint behaves before reading documentation. Default to standard HTTP semantics, status codes, and clear JSON payloads over complex or idiosyncratic abstractions.
* **Domain-Driven, Not Database-Driven**: Endpoints must model business entities and relationships defined in [`CONTEXT.md`](../CONTEXT.md) (e.g., `Practice`, `Staff`, `Client`, `Engagement`, `Visit`, `Invoice`). Never leak low-level database schemas, internal job queues, or storage mechanics into the public API contract.
* **Simple Integration**: Integrations frequently begin life as simple scripts or frontend fetch calls. Avoid forcing unnecessary complexity (e.g., GraphQL or multi-step handshakes) when a clean REST endpoint with query parameters suffices.

---

## 2. Contract Stability ("We Do Not Break Userspace")

Once an API contract is live, downstream consumers (the Svelte app, third-party integrations, mobile clients) rely on its exact structure.

### Rules
1. **Decouple HTTP DTOs from Database Schemas**: Never marshal raw SQL/database models directly to HTTP responses. Always define dedicated request/response Data Transfer Object (DTO) structs in handler/transport packages.
2. **Explicit JSON Tagging**: Every field in an API DTO must have an explicit `json:"fieldName"` tag. Use camelCase for API JSON fields matching frontend conventions.
3. **Additive Changes Only**:
   * Adding new fields to response structs is non-breaking (consumers must tolerate unknown fields).
   * Renaming, removing, or changing the data type of an existing field is a breaking change and is strictly prohibited.
4. **Avoid Proliferation of Versions (`/v1/`, `/v2/`)**:
   * Versioning introduces duplicate routing, fragmented test suites, and branching in core business logic.
   * Design endpoints defensively upfront so version bumps remain a rare last resort.

```go
// Good: Clear DTO isolated from DB row structures
type ClientResponse struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"createdAt"`
    // Additive field added later safely
    Status    string    `json:"status,omitempty"`
}
```

---

## 3. Idempotency & Safe Retries for Mutating Operations

Network calls can time out, drop connections, or return 500s mid-flight. Callers need to retry without risking duplicate side-effects (e.g., double-charging an invoice or creating duplicate visits).

### Rules
1. **`Idempotency-Key` Header**: Support an optional `Idempotency-Key` header on all non-idempotent mutating requests (`POST` endpoints that create records, dispatch messages, or trigger billing actions).
2. **Replay Stored Responses**: When a request with an existing idempotency key is received:
   * Do not re-execute the business logic.
   * Return the cached HTTP status code and response body from the initial execution.
3. **Storage & Scope**: Store idempotency keys scoped by `PracticeID` and `StaffMemberID` with an appropriate TTL (typically 24–48 hours).
4. **Naturally Idempotent Methods**: `GET`, `PUT` (full replacement), and `DELETE /{id}` are inherently idempotent by convention and do not require idempotency keys.

```go
// Handler pattern for idempotent operations
func CreateVisitHandler(service VisitService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        idempotencyKey := r.Header.Get("Idempotency-Key")
        // Check cache / transactionally insert idempotency record
        // ...
    }
}
```

---

## 4. Cursor-Based Pagination for Scalable Collections

`OFFSET / LIMIT` pagination suffers from quadratic performance degradation on large tables and skips/duplicates records when items are concurrently inserted or deleted.

### Rules
1. **Default to Cursor Pagination**: For any unbounded or growing dataset (`Messages`, `Visits`, `Invoices`, `Clients`), use cursor-based pagination using indexed columns (e.g. `(created_at, id)` or sequential IDs).
2. **Consistent Response Envelope**: Wrap paginated list responses in a standard envelope providing `items`, `nextCursor`, and `hasMore`.

```go
type PaginatedResponse[T any] struct {
    Items      []T     `json:"items"`
    NextCursor *string `json:"nextCursor,omitempty"`
    HasMore    bool    `json:"hasMore"`
}
```

3. **Efficient Database Querying**:
```sql
-- Query by cursor comparison rather than OFFSET
SELECT id, practice_id, client_id, scheduled_at, created_at
FROM visits
WHERE practice_id = $1 AND (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT $4;
```

---

## 5. Keep Default Payloads Lean (Use `?include=`)

Default endpoints should be fast, minimal, and avoid heavy joins or expensive sub-queries.

### Rules
1. **Lightweight Defaults**: Return the core resource without unconditionally fetching nested collections or heavy external calculations.
2. **Selective Inclusion via `?include=`**: Allow callers to opt-in to related resources using query parameters rather than forcing a GraphQL layer or returning bloated payloads:
   * Example: `GET /api/practices/{practiceId}/engagements/{id}?include=visits,carePlan`
3. **Parse and Fetch Selectively**: In handlers/services, inspect requested includes and execute additional queries only when explicitly requested.

---

## 6. Defensive Rate Limiting & Safety Controls

APIs run at code speed, not human click speed. Protect the backend against unthrottled polling loops and runaway scripts.

### Rules
1. **Standard Rate Limit Headers**: When rate limiting is enforced, always supply informational headers:
   * `RateLimit-Limit`: Total allowed requests in the time window.
   * `RateLimit-Remaining`: Remaining quota in the current window.
   * `Retry-After`: Seconds to wait before retrying when `429 Too Many Requests` is returned.
2. **Stricter Limits on Heavy Endpoints**: Apply tighter rate limits on operations that trigger expensive database queries, PDF generation, or third-party API calls (e.g. Stripe, e-signatures).
3. **Tenant-Level Isolation & Killswitches**: Provide the ability to rate limit or disable access at the `Practice` or `Staff` level to isolate noisy neighbors.

### What's built (#602)

`api/internal/ratelimit` is the one seam every limited handler wraps in, the same decorator
shape as `idempotency.Wrap`: `ratelimit.Wrap(db, endpoint, rules)(handler)`. Counters live in
Postgres (`rate_limit_buckets`, migration `00060`), not process memory — Cloud Run runs more
than one instance, so an in-process counter would not actually limit anything (ADR-0004 made
the same call for sessions and idempotency keys). A `Rule` is one dimension: a name, how to
read its key off the request, a cap, and a window; an endpoint combines more than one so that
evading any single dimension still runs into another. A refused request gets `429`, the
headers above, and section 7's structured error body; the refusal is also appended to
`rate_limit_refusals` (migration `00060`) so repeated refusals against one address can be seen
after the fact. That table is not an `activity` row (ADR-0022) — every endpoint below runs
before any Practice exists or is known, and `activity.practice_id` is `NOT NULL` — the same
shape ADR-0022 itself names for `staff_work_state_events` (00043): where `activity` cannot
hold the fact, the record lives on the table that owns it.

Every public unauthenticated endpoint that existed when this landed, and its disposition:

| Route | Rules | Reason |
| :--- | :--- | :--- |
| `POST /api/session` (login) | Bearer-token digest 30/hr, IP 100/hr | Fires on every sign-in, well above the once-per-person endpoints below; a cached Identity Platform ID token is legitimately reused for close to an hour. |
| `POST /api/staff/signup` | Bearer-token digest 5/hr, IP 50/hr | Once-per-person bootstrap event (`authn.BeginBootstrap`). Values are generous rather than tight — nothing has ever limited this endpoint before, so any finite cap is a real improvement — and sized above the Playwright e2e suite's own call volume (one shared BFF and IP per run) and a 14-doula pilot agency's onboarding burst from one connection. |
| `POST /api/staff/accept-invite` | Bearer-token digest 5/hr, IP 50/hr | Same bootstrap shape as signup. |
| `POST /api/portal/accept-invite` | `inviteToken` digest 10/hr, IP 50/hr | #617: a Client has no Identity Platform account to bootstrap through any more, so this reads no Bearer token either -- the invitation's own token is the whole credential, the same shape as the two token-spend rows below. |
| `POST /api/portal/magic-link/request` | `email` 5/hr, IP 20/hr | #617, ADR-0026: public, keyed on the posted address (`ratelimit.JSONFieldRule`) -- the sizing #166/#170 reserved below, now built. Same shape as `password-reset/request`. |
| `POST /api/portal/magic-link` | `token` digest 10/hr, IP 50/hr | #617. Pre-account, spends a mailed link; same shape as `verify-email`/`password-reset`'s own spend endpoints. |
| `GET /api/offers/{offerId}` | `offerId` 10/hr, IP 50/hr | Pre-account, token+code authenticated (#230); carries no Bearer token or email to key on before its own check runs, so the Offer being probed is the natural "subject" dimension. The per-Offer code-guess cap (`maxAccessCodeAttempts`, `00041`) already bounds one Offer's brute force permanently — this rule set adds an hourly cap on the same thing (10, matching that constant) plus IP volume across many Offers. |
| `POST /api/offers/{offerId}/decline` | `offerId` 10/hr, IP 50/hr | Same shape as the read above. |
| `POST /api/staff/verify-email/request` | Session digest 10/hr, IP 50/hr | #613. Signed-in re-request; no Bearer token, only a `__session` cookie to key on (`ratelimit.SessionCookieRule`). |
| `POST /api/staff/verify-email` | `token` digest 10/hr, IP 50/hr | #613. Pre-account, spends a mailed link; keyed on the link's own token (`ratelimit.HashedJSONFieldRule`) since there is no Bearer token or session yet. |
| `POST /api/staff/password-reset/request` | `email` 5/hr, IP 20/hr | #613, same sizing #166 reserved below — public, keyed on the posted address (`ratelimit.JSONFieldRule`). |
| `POST /api/staff/password-reset` | `token` digest 10/hr, IP 50/hr | #613. Same shape as verify-email's spend endpoint. |
| `POST /api/staff/mfa-recovery/spend` | `email` digest 10/hr, IP 50/hr | #615. Public, pre-account — a locked-out person cannot sign in first (Identity Platform challenges for the second factor on every sign-in once one exists). Keyed on the posted address rather than the code itself: #615's AC asks explicitly for a per-account limit on failed attempts, not only per IP, because the issued code is 8 decimal digits (`authtoken.MintCode`) read aloud over a phone call — brute-forceable across the full `10^8` keyspace without this, unlike the 128-bit link tokens `token` digest keys elsewhere in this table. |
| `POST /api/staff/mfa-recovery/saved-codes/rotate` | Session digest 10/hr, IP 50/hr | #615. Signed-in self-service re-request, same shape as `verify-email/request`: revokes and re-mints a sole Owner's whole saved-code set, so it gets the same self-inflicted-spam guard that pure state reads (`PUT /api/staff/work-state`) do not need. |
| `POST /api/staff/mfa` | Bearer-token digest 30/hr, IP 100/hr | #606. Finishes a TOTP enrolment by exchanging a just-enrolled ID token for a session -- same shape and same limits as `POST /api/session`, because it is an ordinary sign-in path (the enrolment flow's own re-sign-in) rather than a once-per-person bootstrap event. |
| `DELETE /api/staff/mfa` | Session digest 10/hr, IP 50/hr | #606. Signed-in voluntary removal of her own factor, guarded by `RequireRecentAuth`'s step-up rather than by a tight limit -- same shape as `verify-email/request`, a low-risk self-service action already gated by a live session. |

Deliberately not limited:

- `GET /api/hello` — a liveness/readiness probe with no side effect and no cost, curled in a
  loop by CI's own smoke tests and by Cloud Run's health check. Limiting it would break exactly
  the callers it exists for.
- `DELETE /api/session` — only ever clears a cookie the caller already holds (or no-ops if
  there is none); no credential is checked, so there is nothing for an attacker to gain by
  repeating it.
- `GET /api/staff/session`, `PUT /api/staff/work-state`, `PUT /api/staff/email`,
  `GET /api/portal/session`, and every route behind `staffauth.Middleware` /
  `clientauth.Middleware` — gated by `authn.Begin`'s own `__session` cookie check. A missing or
  invalid session is a `401` at that gate; there is no bootstrap-style window here for an
  attacker to spend.
- `POST /api/internal/**` and `POST /api/stripe/**` / `POST /api/mailgun/webhook` — authenticated
  by `X-Internal-Secret` or a signature over the request body, not a session, and called only by
  Cloud Scheduler, Cloud Tasks, or the vendor itself.

---

## 7. Predictable Error Responses

Avoid ad-hoc error formats. Maintain a consistent JSON error schema across all endpoints.

### Rules
1. **Uniform Error Structure**:
```go
type APIError struct {
    Code    string            `json:"code"`              // Machine-readable code: "NOT_FOUND", "INVALID_ARGUMENT", "UNAUTHORIZED"
    Message string            `json:"message"`           // Human-readable summary
    Details map[string]string `json:"details,omitempty"` // Field-level validation errors
}
```
2. **Standard Status Code Usage**:
   * `400 Bad Request` / `422 Unprocessable Entity`: Request body or parameter validation failure.
   * `401 Unauthorized`: Missing or invalid authentication token.
   * `403 Forbidden`: Authenticated user lacks permission for the practice/resource.
   * `404 Not Found`: Target resource does not exist (or caller lacks permission to know it exists).
   * `409 Conflict`: Resource state conflict or duplicate idempotency key conflict.
   * `429 Too Many Requests`: Rate limit reached.
   * `500 Internal Server Error`: Unhandled server or database error (log details internally, do not leak raw stack traces to caller).
3. **A refusal that can be pressed through**: three sign-in endpoints — `POST /api/session`, `POST /api/portal/magic-link` and `POST /api/portal/accept-invite` — answer `409 FAILED_PRECONDITION` when the caller already holds a live session in the *other* population, because minting would end it (#610, ADR-0026). Nothing is written on that refusal; the same request repeated with `X-Confirmed: true` goes through and deletes the other session. The message names the population being left and nothing else about the session behind the cookie. A client tells this 409 apart from a resource conflict by its code, never by its prose (#692).
4. **One Writer**: `api/internal/apierr` is the only place this shape is written from. Every handler calls `apierr.Write` (or `apierr.WriteError` for the common status+message case) rather than `http.Error` or a package-local helper; a new endpoint that needs a `Code` not yet in `apierr.Code`'s enumerated set adds one there (#529).

---

## 8. Summary Checklist for Code Reviews & Agents

When adding or modifying an HTTP endpoint in `api/`:

| Check | Requirement |
| :--- | :--- |
| **Domain Terms** | Uses exact vocabulary from [`CONTEXT.md`](../CONTEXT.md) (e.g., `Engagement`, `Visit`, `Care Plan`). |
| **DTO Decoupling** | Handler accepts and returns dedicated DTO structs, not database models. |
| **JSON Tags** | All DTO struct fields have explicit `json:"camelCase"` tags. |
| **Contract Stability**| Edits to existing responses are purely additive (no deletions/renames). |
| **Idempotency** | Non-idempotent mutating `POST` actions accept `Idempotency-Key`. |
| **Pagination** | Lists use cursor pagination with a standard `PaginatedResponse[T]` envelope. |
| **Lean Payloads** | Expensive relations are opt-in via `?include=`. |
| **Errors** | Handlers return consistent structured JSON errors matching `APIError`. |
