# Building a multi-tenant platform with the Go SDK

This guide is for backends that resell MIOSA sandboxes and computers to their own customers.
Every snippet uses calls that have httptest coverage in this module.

## 1. One client per platform, one view per customer

```go
platform := miosa.NewClient(os.Getenv("MIOSA_API_KEY"))

// Same transport and connection pool, different tenant header.
acme := platform.ForTenant("tenant_acme")

// Act as one end user with a scoped token.
tok, _ := acme.ScopedTokens.Mint(ctx, miosa.MintScopedTokenInput{ /* scopes, TTL */ })
user := acme.AsUser(tok.Token)
```

`Client.With(opts...)` derives a client with any other option, for example `WithBillTo` or `WithUserAgentOverride` for an unbranded user agent.

## 2. Governance at three tiers

`client.Policies` reads and writes quotas, permissions, features and billing at the tenant, workspace and end-user tier.
`EffectiveForExternalUser` returns the merged result and the tier each section came from.

## 3. Create sandboxes at scale

```go
acc, _ := user.Sandboxes.Batches.Create(ctx, miosa.BatchCreateInput{
    Count: 200, Concurrency: 20, TemplateID: tpl, NamePrefix: "ci",
})
st, err := user.Sandboxes.Batches.Wait(ctx, acc.ID, 2*time.Second)
```

Cancel with `Batches.Cancel` (a finished batch is 409 `BATCH_ALREADY_TERMINAL`).
Act on many existing sandboxes with `client.Bulk.SandboxAction(ctx, "pause", ids)` or `SandboxActionWhere` with a state or workspace filter, then poll `client.Bulk.Job`.

## 4. Cap spend per tenant

```go
cap := int64(50_00)
spend, _ := platform.SandboxSpend.Update(ctx, "tenant_acme",
    miosa.SandboxSpendUpdate{Mode: "limited", CapCents: &cap, AlertThresholds: []int{50, 90}})
```

Both spend routes need a user credential and the update needs an owner or admin.

## 5. Let end users see a desktop

```go
pw, _ := client.Computers.RotateViewerPassword(ctx, id)   // shown once
// Later, from the end user's browser session, with no account credential:
sess, _ := client.Computers.CreateViewerSession(ctx, id, pw.ViewerPassword, 600)
```

`Embed` and `StreamToken` return URLs and tokens for an iframe or WebSocket viewer.

## 6. SSH

See the `sandboxssh` package: `sandboxssh.Dial(ctx, client, sandboxID, sandboxssh.Options{})` returns an `*ssh.Client`.

## 7. Events

`miosa.WebhookHandler` verifies signatures and decodes events.
Event subscriptions, replay and the outbox live under `client.Events`.

## Known backend gaps

- No per-request workspace header for REST calls. Use a scoped token or a workspace-bound key.
- `GET /tenant/policy` is not enveloped in `data`; the SDK accepts both shapes.
- Webhook signatures cover the body only, so receivers cannot reject replays by timestamp.
- Sandbox tags are a map in list filters but a list in `PATCH /sandboxes/:id/tags`.
