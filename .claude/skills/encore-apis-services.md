# Encore APIs and Services

## API Definition

Create type-safe APIs from regular Go functions using `//encore:api` annotation.

### Access Controls

- **public**: Accessible to anyone on the internet
- **private**: Only accessible within app and via cron jobs
- **auth**: Public but requires valid authentication

### Function Signatures

```go
func Foo(ctx context.Context, p *Params) (*Response, error)  // full
func Foo(ctx context.Context) (*Response, error)             // response only
func Foo(ctx context.Context, p *Params) error               // request only
func Foo(ctx context.Context) error                          // minimal
```

### Request/Response Data Locations

- **header**: Use `header` tag for HTTP headers
- **query**: Default for GET/HEAD/DELETE, uses snake_case, supports basic types/slices
- **body**: Default for other methods, uses `json` tag, supports complex types

### Path Parameters

Use `:name` for variables, `*name` for wildcards. Place at end of path.

### Sensitive Data

- **Field level**: `encore:"sensitive"` tag, auto-redacted in tracing
- **Endpoint level**: Add `sensitive` to `//encore:api` annotation

### Type Support by Location

- **headers/path**: bool, numeric, string, time.Time, UUID, json.RawMessage
- **query**: All above plus lists
- **body**: All types including structs, maps, pointers

## Services

A service is defined by creating at least one API within a Go package. Package name becomes service name.

### Service with Initialization

Use `//encore:service` annotation for custom initialization and graceful shutdown:

```go
type Service struct {
    // Dependencies here
}

func initService() (*Service, error) {
    // Initialization code
}

//encore:api public
func (s *Service) MyAPI(ctx context.Context) error {
    // API implementation
}
```

### Graceful Shutdown

```go
func (s *Service) Shutdown(force context.Context)
```

- **Graceful phase**: Several seconds for completion
- **Forced phase**: When force context canceled, terminate immediately

## Raw Endpoints

For lower-level HTTP access (webhooks, WebSockets):

```go
//encore:api public raw
func Webhook(w http.ResponseWriter, req *http.Request) {
    // Process raw HTTP request
}

//encore:api public raw method=POST path=/webhook/:id
func Webhook(w http.ResponseWriter, req *http.Request) {
    id := encore.CurrentRequest().PathParams.Get("id")
}
```

## API Calls Between Services

Call APIs like regular functions with automatic type checking:

```go
import "encore.app/hello"

//encore:api public
func MyOtherAPI(ctx context.Context) error {
    resp, err := hello.Ping(ctx, &hello.PingParams{Name: "World"})
    if err == nil {
        log.Println(resp.Message) // "Hello, World!"
    }
    return err
}
```
