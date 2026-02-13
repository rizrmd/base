# Encore Advanced Patterns

## Clerk Authentication

Implement Clerk authentication:

```go
package auth

import "github.com/clerkinc/clerk-sdk-go/clerk"

type Service struct {
    client clerk.Client
}

func initService() (*Service, error) {
    client, err := clerk.NewClient(secrets.ClientSecretKey)
    if err != nil {
        return nil, err
    }
    return &Service{client: client}, nil
}

type UserData struct {
    ID                    string
    Username              *string
    FirstName             *string
    LastName              *string
    ProfileImageURL       string
    PrimaryEmailAddressID *string
    EmailAddresses        []clerk.EmailAddress
}

//encore:authhandler
func (s *Service) AuthHandler(ctx context.Context, token string) (auth.UID, *UserData, error) {
    // Token verification and user data retrieval
}
```

**Set secrets (from apps/ directory):**
- `encore secret set --prod ClientSecretKey`
- `encore secret set --dev ClientSecretKey`

## Dependency Injection

Add dependencies as struct fields for easy testing:

```go
package email

//encore:service
type Service struct {
    sendgridClient *sendgrid.Client
}

func initService() (*Service, error) {
    client, err := sendgrid.NewClient()
    if err != nil {
        return nil, err
    }
    return &Service{sendgridClient: client}, nil
}

//encore:api private
func (s *Service) Send(ctx context.Context, p *SendParams) error {
    // Use s.sendgridClient
}

// For testing, use interface
type sendgridClient interface {
    SendEmail(ctx context.Context, to, subject, body string) error
}

func TestFoo(t *testing.T) {
    svc := &Service{sendgridClient: &myMockClient{}}
    // Test
}
```

## PubSub Outbox

Transactional outbox pattern for database + Pub/Sub consistency.

```go
var SignupsTopic = pubsub.NewTopic[*SignupEvent](/* ... */)
ref := pubsub.TopicRef[pubsub.Publisher[*SignupEvent]](SignupsTopic)
ref = outbox.Bind(ref, outbox.TxPersister(tx))
```

### Required Schema

```sql
CREATE TABLE outbox (
    id BIGSERIAL PRIMARY KEY,
    topic TEXT NOT NULL,
    data JSONB NOT NULL,
    inserted_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX outbox_topic_idx ON outbox (topic, id);
```

### Relay Setup

```go
type Service struct {
    signupsRef pubsub.Publisher[*SignupEvent]
}

func initService() (*Service, error) {
    relay := outbox.NewRelay(outbox.SQLDBStore(db))
    signupsRef := pubsub.TopicRef[pubsub.Publisher[*SignupEvent]](SignupsTopic)
    outbox.RegisterTopic(relay, signupsRef)
    go relay.PollForMessage(context.Background(), -1)
    return &Service{signupsRef: signupsRef}, nil
}
```

Supports: `encore.dev/storage/sqldb`, `database/sql`, `github.com/jackc/pgx/v5`

## Streaming

Server-Sent Events (SSE) and streaming responses via raw endpoints:

```go
//encore:api public raw method=GET path=/events
func StreamEvents(w http.ResponseWriter, req *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming unsupported", http.StatusInternalServerError)
        return
    }

    // Send events
    for {
        select {
        case <-req.Context().Done():
            return
        default:
            fmt.Fprintf(w, "data: %s\n\n", "event data")
            flusher.Flush()
        }
    }
}
```

For WebSockets, use gorilla/websocket or similar with raw endpoints.

## Metrics

Custom application metrics via `encore.dev/metrics`:

```go
import "encore.dev/metrics"

// Counter - tracks cumulative value
var RequestCounter = metrics.NewCounter(
    "api_requests_total",
    "Total number of API requests",
)

// Gauge - tracks current value
var ActiveConnections = metrics.NewGauge(
    "active_connections",
    "Number of active connections",
)

// Histogram - tracks distribution
var ResponseTime = metrics.NewHistogram(
    "response_time_seconds",
    "API response time distribution",
    []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5},
)

// Usage in code
func (s *Service) MyAPI(ctx context.Context) error {
    start := time.Now()
    RequestCounter.Inc()
    defer func() {
        ResponseTime.Observe(time.Since(start).Seconds())
    }()
    // ...
}
```

### Labels for Dimensional Metrics

```go
var HTTPRequests = metrics.NewCounterVec(
    "http_requests",
    "HTTP requests by method and path",
    []string{"method", "path"},
)

HTTPRequests.WithLabelValues("GET", "/users").Inc()
```

## Health Checks

Encore provides built-in health endpoints:

- `/healthz` - Basic health check (returns 200 OK)
- `/readyz` - Readiness check (verifies app can handle requests)

### Custom Health Checks

```go
import "encore.dev/runtime/health"

func init() {
    health.Register("database", health.CheckConfig{
        Check: checkDatabaseHealth,
    })
}

func checkDatabaseHealth(ctx context.Context) error {
    // Ping database, return error if unhealthy
    return db.Ping(ctx)
}
```

Health check response includes all registered checks:
```json
{
    "status": "ok",
    "checks": {
        "database": {"status": "ok", "latency_ms": 2}
    }
}
```

## Rate Limiting

Implement rate limiting with caching or middleware:

```go
import (
    "encore.dev/storage/cache"
    "golang.org/x/time/rate"
)

// Per-user rate limiter using cache
var RateLimitCache = cache.NewKeyspace[string](cluster, cache.KeyspaceConfig{
    KeyPattern:    "ratelimit/:key",
    DefaultExpiry: cache.ExpireIn(time.Minute),
})

// Middleware for rate limiting
//encore:middleware target=all
func RateLimitMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
    userID := auth.UserID()
    if userID == "" {
        return next(req)
    }

    key := string(userID)
    count, _ := RateLimitCache.Increment(req.Context(), key, 1)

    if count > 100 { // 100 requests per minute
        return middleware.Response{
            Err: &errs.Error{
                Code:    errs.ResourceExhausted,
                Message: "rate limit exceeded",
            },
        }
    }

    return next(req)
}
```

For distributed rate limiting, use Redis-backed algorithms like token bucket or sliding window.

## Multi-Tenancy

Common patterns for multi-tenant SaaS applications:

### Tenant Context via Auth Data

```go
type TenantData struct {
    TenantID string
    Plan     string
    Features []string
}

//encore:authhandler
func AuthHandler(ctx context.Context, token string) (auth.UID, *TenantData, error) {
    // Validate token and extract tenant info
    tenantID, data := validateAndGetTenant(token)
    return auth.UID(tenantID), data, nil
}
```

### Tenant-Aware Database Queries

```go
func (s *Service) GetItems(ctx context.Context) ([]Item, error) {
    tenantID := auth.Data[*TenantData](ctx).TenantID

    var items []Item
    err := s.db.QueryRow(ctx, `
        SELECT * FROM items WHERE tenant_id = $1
    `, tenantID).Scan(&items)
    return items, err
}
```

### Schema Approaches

1. Column-based (tenant_id column on all tables) - simplest, good for moderate scale
2. Schema-based (separate schema per tenant) - better isolation
3. Database-based (separate DB per tenant) - strongest isolation, most operational overhead

### Row-Level Security in PostgreSQL

```sql
CREATE POLICY tenant_isolation ON items
    USING (tenant_id = current_setting('app.current_tenant')::text);
```
