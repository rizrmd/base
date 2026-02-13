# Encore Authentication, Configuration & Errors

## Errors

Structured errors via `encore.dev/beta/errs` package. // Note: package path may vary by Encore version

```go
return &errs.Error{
    Code: errs.NotFound,
    Message: "sprocket not found",
}
// Returns HTTP 404 {"code": "not_found", "message": "sprocket not found"}
```

### Wrapping

```go
errs.Wrap(err, msg, metaPairs...)
errs.WrapCode(err, code, msg, metaPairs...)
```

### Builder Pattern

```go
eb := errs.B().Meta("board_id", params.ID)
return eb.Code(errs.NotFound).Msg("board not found").Err()
```

### Error Codes

- OK(200), Canceled(499), Unknown(500), InvalidArgument(400), DeadlineExceeded(504), NotFound(404), AlreadyExists(409), PermissionDenied(403), ResourceExhausted(429), FailedPrecondition(400), Aborted(409), OutOfRange(400), Unimplemented(501), Internal(500), Unavailable(503), DataLoss(500), Unauthenticated(401)

### Inspection

```go
errs.Code(err), errs.Meta(err), errs.Details(err)
```

## Authentication

Flexible auth with different access levels.

```go
import "encore.dev/beta/auth" // Note: package path may vary by Encore version

// Basic
//encore:authhandler
func AuthHandler(ctx context.Context, token string) (auth.UID, error) {
    // Validate token and return user ID
}

// With user data
type Data struct {
    Username string
}

//encore:authhandler
func AuthHandler(ctx context.Context, token string) (auth.UID, *Data, error) {
    // Return user ID and custom data
}

// Structured auth params
type MyAuthParams struct {
    SessionCookie *http.Cookie `cookie:"session"`
    ClientID string `query:"client_id"`
    Authorization string `header:"Authorization"`
}

//encore:authhandler
func AuthHandler(ctx context.Context, p *MyAuthParams) (auth.UID, error) {
    // Process structured auth params
}
```

**Usage**: `auth.Data()`, `auth.UserID()`

**Override for testing**: `auth.WithContext(ctx, auth.UID("my-user-id"), &MyAuthData{})`

**Error handling:**
```go
return "", &errs.Error{
    Code: errs.Unauthenticated,
    Message: "invalid token",
}
```

## Configuration

Environment-specific config using CUE files.

```go
package mysvc

import "encore.dev/config"

type SomeConfigType struct {
    ReadOnly config.Bool
    Example  config.String
}

var cfg *SomeConfigType = config.Load[*SomeConfigType]()
```

### CUE Tags for Constraints

```go
type FooBar struct {
    A int `cue:">100"`
    B int `cue:"A-50"`
    C int `cue:"A+B"`
}
```

### Config Types

config.String, config.Bool, config.Int, config.Float64, config.Time, config.UUID, config.Value[T], config.Values[T]

### Meta Values

- APIBaseURL, Environment.Name, Environment.Type (production/development/ephemeral/test), Environment.Cloud (aws/gcp/encore/local)

### Testing

```go
et.SetCfg(cfg.SendEmails, true)
```

### CUE Patterns

- Defaults: `value: type | *default_value`
- Switch: array with conditionals, take [0]

## CORS

Configure in `encore.app` file:

- debug: Enable CORS debug logging
- allow_headers: Additional accepted headers ("*" allows all)
- expose_headers: Additional exposed headers
- allow_origins_without_credentials: Defaults to ["*"]
- allow_origins_with_credentials: For authenticated requests, supports wildcards like "https://*.example.com"

**IMPORTANT: Avoid setting the `id` field in encore.app**
Setting `id: "encore.app"` (or any app ID) forces the use of Encore's cloud services. This project does NOT use Encore cloud - we deploy using Docker with our own infrastructure. Never set an app ID in encore.app, as it will incorrectly link the project to Encore's managed services.

## Metadata

Access app and request info via `encore.dev` package.

```go
// Application metadata
meta := encore.Meta()
// meta.AppID, meta.APIBaseURL, meta.Environment, meta.Build, meta.Deploy

// Request metadata
req := encore.CurrentRequest()
// req.Service, req.Endpoint, req.Path, req.StartTime

// Cloud-specific behavior
switch encore.Meta().Environment.Cloud {
case encore.CloudAWS:
    return writeIntoRedshift(ctx, action, user)
case encore.CloudGCP:
    return writeIntoBigQuery(ctx, action, user)
}
```
