# Encore Databases

## SQL Databases

Encore treats SQL databases as logical resources with native PostgreSQL support.

### Create Database

```go
var tododb = sqldb.NewDatabase("todo", sqldb.DatabaseConfig{
    Migrations: "./migrations",
})
```

### Migrations

**Migration naming**: `number_description.up.sql` (e.g., `1_create_table.up.sql`)

**Migrations folder structure:**
```
service/
  migrations/
    1_create_table.up.sql
    2_add_field.up.sql
  service.go
```

### Data Operations

```go
// Insert
_, err := tododb.Exec(ctx, `
    INSERT INTO todo_item (id, title, done)
    VALUES ($1, $2, $3)
`, id, title, done)

// Query
err := tododb.QueryRow(ctx, `
    SELECT id, title, done FROM todo_item LIMIT 1
`).Scan(&item.ID, &item.Title, &item.Done)
// Use errors.Is(err, sqldb.ErrNoRows) for no results
```

### CLI Commands (run from apps/ directory)

- `encore db shell database-name [--env=name]` - Opens psql shell
- `encore db conn-uri database-name [--env=name]` - Outputs connection string
- `encore db proxy [--env=name]` - Sets up local connection proxy

## External Databases

For existing databases, create dedicated package with lazy connection pool:

```go
package externaldb

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5/pgxpool"
    "go4.org/syncutil"
)

func Get(ctx context.Context) (*pgxpool.Pool, error) {
    err := once.Do(func() error {
        var err error
        pool, err = setup(ctx)
        return err
    })
    return pool, err
}

var (
    once syncutil.Once
    pool *pgxpool.Pool
)

var secrets struct {
    ExternalDBPassword string
}

func setup(ctx context.Context) (*pgxpool.Pool, error) {
    connString := fmt.Sprintf("postgresql://%s:%s@hostname:port/dbname?sslmode=require",
        "user", secrets.ExternalDBPassword)
    return pgxpool.New(ctx, connString)
}
```

Works with Cassandra, DynamoDB, BigTable, MongoDB, Neo4j, and other services.

## Shared Databases

Default: per-service databases for isolation. To share, reference using `sqldb.Named`:

```go
// In report service, access todo service's database:
var todoDB = sqldb.Named("todo")

//encore:api method=GET path=/report/todo
func CountCompletedTodos(ctx context.Context) (*ReportResponse, error) {
    var report ReportResponse
    err := todoDB.QueryRow(ctx,`
        SELECT COUNT(*) FROM todo_item WHERE completed = TRUE
    `).Scan(&report.Total)
    return &report, err
}
```
