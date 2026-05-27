# miosa-go

> Official Go SDK for MIOSA — the AI cloud platform for sandboxes, computers, deployments, and managed data.

[![pkg.go.dev](https://pkg.go.dev/badge/github.com/Miosa-osa/miosa-go.svg)](https://pkg.go.dev/github.com/Miosa-osa/miosa-go)
[![Go version](https://img.shields.io/badge/go-%3E%3D1.21-blue)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docs](https://img.shields.io/badge/docs-miosa.ai%2Fdocs-blue)](https://miosa.ai/docs/sdks/go)

Zero external dependencies (stdlib + gorilla/websocket). Go 1.21+.

## Install

```bash
go get github.com/Miosa-osa/miosa-go
```

## Quickstart

Every method takes `context.Context` as its first argument.

```go
package main

import (
    "context"
    "fmt"
    "log"

    miosa "github.com/Miosa-osa/miosa-go"
)

func main() {
    client := miosa.NewClient("msk_live_...")
    ctx    := context.Background()

    // Create a sandbox
    sbx, err := client.Sandboxes.Create(ctx, miosa.CreateSandboxInput{
        Name: "my-build",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Wait until running
    if err := sbx.Wait(ctx, miosa.StatusRunning); err != nil {
        log.Fatal(err)
    }

    // Run a command
    result, _ := sbx.Bash(ctx, "echo 'hello from miosa'")
    fmt.Println(result.Output) // hello from miosa

    // Expose a preview URL
    fmt.Println(sbx.PreviewURL(8000, "/"))
    // => https://8000-<slug>.sandbox.miosa.ai/

    _ = sbx.Destroy(ctx)
}
```

## Computer lifecycle

```go
computer, err := client.Computers.Create(ctx, miosa.CreateComputerInput{
    Name: "agent-desktop",
    Size: miosa.SizeMedium,
})

_ = computer.Start(ctx)
_ = computer.Wait(ctx, miosa.StatusRunning)
_ = computer.Stop(ctx)
_ = computer.Restart(ctx)
_ = computer.Destroy(ctx)

// Fetch / list
computer, _ = client.Computers.Get(ctx, "cmp_...")
list, _    := client.Computers.List(ctx, miosa.ListComputersInput{
    Status:  miosa.StatusRunning,
    PerPage: 50,
})
_ = list
```

## Desktop control

```go
png, _  := computer.Screenshot(ctx)            // raw PNG bytes
_        = computer.Click(ctx, 640, 400)
_        = computer.DoubleClick(ctx, 640, 400)
_        = computer.Type(ctx, "hello world")
_        = computer.Key(ctx, "Return")
_        = computer.Key(ctx, "ctrl+c")
_        = computer.Scroll(ctx, miosa.ScrollDown, 3)
_        = computer.Drag(ctx, 100, 100, 400, 400)

cursor, _ := computer.Cursor(ctx)
wins, _   := computer.Windows(ctx)
_          = computer.Launch(ctx, "firefox")
_ = png; _ = cursor; _ = wins
```

## Exec

```go
result, _ := computer.Bash(ctx, "ls -la /home")
fmt.Println(result.Output)   // directory listing
fmt.Println(result.ExitCode) // 0

pyResult, _ := computer.Python(ctx, "print(2 + 2)")
fmt.Println(pyResult.Output) // 4
```

## File operations

```go
_ = computer.Files.WriteFile(ctx, "/workspace/main.go", []byte(`package main`))
data, _ := computer.Files.ReadFile(ctx, "/workspace/main.go")

entries, _ := computer.Files.List(ctx, "/workspace")
for _, e := range entries {
    fmt.Println(e.Name, e.IsDir)
}

stat, _ := computer.Files.Stat(ctx, "/workspace/main.go")
_        = computer.Files.Mkdir(ctx, "/workspace/output", true)
_        = computer.Files.Rename(ctx, "/workspace/old.go", "/workspace/new.go")
_        = computer.Files.Chmod(ctx, "/workspace/run.sh", "0755")
_ = data; _ = stat
```

## Services (background processes)

```go
svc, _ := computer.Services.Create(ctx, miosa.CreateServiceInput{
    Name:       "web",
    Command:    "python -m http.server 8000",
    WorkingDir: "/workspace",
    Port:       8000,
})

_ = computer.Services.Start(ctx, svc.ID)
_ = computer.Services.Stop(ctx, svc.ID)
_ = computer.Services.Restart(ctx, svc.ID)
_ = computer.Services.Delete(ctx, svc.ID)
```

## White-label / multi-tenant

```go
sbx, _ := client.Sandboxes.Create(ctx, miosa.CreateSandboxInput{
    Name:                "customer-build",
    ExternalWorkspaceID: "dental-office-123",
    ExternalUserID:      "dr-smith-456",
})
_ = sbx
```

## Error handling

```go
import "errors"

var notFound   *miosa.NotFoundError
var rateLimited *miosa.RateLimitError

if errors.As(err, &notFound) {
    fmt.Println("not found:", notFound.Message)
} else if errors.As(err, &rateLimited) {
    fmt.Printf("rate limited; retry after %ds\n", rateLimited.RetryAfter)
}
```

Typed error hierarchy: `MiosaError`, `AuthenticationError`, `InsufficientCreditsError`, `PermissionError`, `NotFoundError`, `ValidationError`, `RateLimitError`, `ServerError`, `ConnectionError`.

The client retries `429` and `5xx` automatically (3 retries, exponential backoff + full jitter). Disable with `miosa.NewClient(key, miosa.WithMaxRetries(0))`.

## Configuration

```go
client := miosa.NewClient("msk_live_...",
    miosa.WithBaseURL("https://api.miosa.ai/api/v1"),
    miosa.WithTimeout(30 * time.Second),
    miosa.WithMaxRetries(3),
)
```

| Option | Env var | Default |
|---|---|---|
| API key | `MIOSA_API_KEY` | — |
| Base URL | `MIOSA_BASE_URL` | `https://api.miosa.ai/api/v1` |
| Max retries | — | 3 |

## Links

- [pkg.go.dev reference](https://pkg.go.dev/github.com/Miosa-osa/miosa-go)
- [Full documentation](https://miosa.ai/docs/sdks/go)
- [Quickstart](https://miosa.ai/docs/quickstart)
- [GitHub](https://github.com/Miosa-osa/miosa-go)
- [Contact](mailto:platform@miosa.ai)

## License

MIT
