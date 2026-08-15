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
