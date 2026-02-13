# Encore Infrastructure Services

## Caching

Redis-based distributed caching system.

```go
import "encore.dev/storage/cache"

var MyCacheCluster = cache.NewCluster("my-cache-cluster", cache.ClusterConfig{
    EvictionPolicy: cache.AllKeysLRU,
})

// Keyspace with type safety
var RequestsPerUser = cache.NewIntKeyspace[auth.UID](cluster, cache.KeyspaceConfig{
    KeyPattern:    "requests/:key",
    DefaultExpiry: cache.ExpireIn(10 * time.Second),
})

// Structured keys
type MyKey struct {
    UserID auth.UID
    ResourcePath string
}
var ResourceRequestsPerUser = cache.NewIntKeyspace[MyKey](cluster, cache.KeyspaceConfig{
    KeyPattern:    "requests/:UserID/:ResourcePath",
    DefaultExpiry: cache.ExpireIn(10 * time.Second),
})
```

Supports strings, integers, floats, structs, sets, and ordered lists.

## Object Storage

Cloud-agnostic API compatible with S3, GCS, and S3-compatible services.

```go
var ProfilePictures = objects.NewBucket("profile-pictures", objects.BucketConfig{
    Versioned: false,
})

// Public bucket with CDN
var PublicAssets = objects.NewBucket("public-assets", objects.BucketConfig{
    Public: true,
})
```

**Operations**: Upload, Download, List, Remove, Attrs, Exists

**Bucket references for permissions:**
```go
type myPerms interface {
    objects.Downloader
    objects.Uploader
}
ref := objects.BucketRef[myPerms](bucket)
```

## Pub/Sub

Asynchronous event broadcasting with automatic infrastructure provisioning.

```go
type SignupEvent struct{ UserID int }

var Signups = pubsub.NewTopic[*SignupEvent]("signups", pubsub.TopicConfig{
    DeliveryGuarantee: pubsub.AtLeastOnce,
})

// Publishing
messageID, err := Signups.Publish(ctx, &SignupEvent{UserID: id})

// Topic reference
signupRef := pubsub.TopicRef[pubsub.Publisher[*SignupEvent]](Signups)

// Subscribing
var _ = pubsub.NewSubscription(
    user.Signups, "send-welcome-email",
    pubsub.SubscriptionConfig[*SignupEvent]{
        Handler: SendWelcomeEmail,
    },
)

// Method handler with dependency injection
var _ = pubsub.NewSubscription(
    user.Signups, "send-welcome-email",
    pubsub.SubscriptionConfig[*SignupEvent]{
        Handler: pubsub.MethodHandler((*Service).SendWelcomeEmail),
    },
)
```

**Delivery guarantees:**
- AtLeastOnce: Handlers must be idempotent
- ExactlyOnce: Stronger guarantees (AWS: 300 msg/sec, GCP: 3000+ msg/sec)

**Ordering**: Use OrderingAttribute matching pubsub-attr tag

**Testing:**
```go
msgs := et.Topic(Signups).PublishedMessages()
assert.Len(t, msgs, 1)
```

## Cron Jobs

Declarative periodic tasks. Does not run locally or in Preview Environments.

```go
import "encore.dev/cron"

var _ = cron.NewJob("welcome-email", cron.JobConfig{
    Title:    "Send welcome emails",
    Every:    2 * cron.Hour,
    Endpoint: SendWelcomeEmail,
})

//encore:api private
func SendWelcomeEmail(ctx context.Context) error {
    return nil
}
```

**Scheduling options:**
- Every: Must divide 24 hours evenly (e.g., 10 * cron.Minute, 6 * cron.Hour)
- Schedule: Cron expressions (e.g., "0 4 15 * *" for 4am UTC on 15th)

**Requirements**: Endpoints must be idempotent, no request parameters, signature `func(context.Context) error` or `func(context.Context) (*T, error)`

## Secrets

Built-in secrets manager for API keys, passwords, private keys.

```go
var secrets struct {
    SSHPrivateKey string
    GitHubAPIToken string
}

func callGitHub(ctx context.Context) {
    req.Header.Add("Authorization", "token " + secrets.GitHubAPIToken)
}
```

**CLI management (run from apps/ directory):**
- `encore secret set --type production secret-name`
- `encore secret set --type development secret-name`
- `encore secret set --env env-name secret-name` (environment-specific override)

**Types**: production (prod), development (dev), preview (pr), local

**Local override via .secrets.local.cue:**
```cue
GitHubAPIToken: "my-local-override-token"
```
