package miosa_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

func ExampleSandboxesService_Exec() {
	ctx := context.Background()
	client := miosa.NewClient("msk_u_...")

	res, _ := client.Sandboxes.Exec(ctx, "sbx_1", miosa.ExecSandboxInput{
		Command: "npm test", Cwd: "/workspace/app", TimeoutSeconds: 300, WaitForReady: true,
	})
	fmt.Println(res.ExitCode, res.Stdout)

	events, _ := client.Sandboxes.ExecStream(ctx, "sbx_1", miosa.ExecSandboxInput{Command: "make build"})
	for ev := range events {
		fmt.Println(ev.Type, ev.Line)
	}

	sbx, _ := client.Sandboxes.GetByName(ctx, "my build")
	fmt.Println(sbx.ID)
}

func ExampleNewClient_options() {
	client := miosa.NewClient("msk_live_...",
		miosa.WithBaseURL("https://api.miosa.ai/api/v1"),
		miosa.WithTimeout(30*time.Second),
		miosa.WithMaxRetries(3),
		miosa.WithTenant("org-id"),
		miosa.WithBillTo("acme"),
		miosa.WithUserAgent("my-product/1.0"),
	)
	client = miosa.NewClient("", miosa.WithAccessToken("mat_..."))
	client = miosa.NewClient("")
	_ = client
}

func ExampleScopedTokensService_Mint() {
	ctx := context.Background()
	admin := miosa.NewClient("msk_live_...", miosa.WithUserAgentOverride("acme-platform/3.1"))

	tok, _ := admin.ScopedTokens.Mint(ctx, miosa.MintScopedTokenInput{
		UserID: "dr-smith", WorkspaceID: "ws_1",
		Scopes: []string{"sandboxes:read", "sandboxes:create", "sandboxes:exec"}, ExpiresInSeconds: 900,
	})
	user := admin.AsUser(tok.Token)

	sbx, _ := user.Sandboxes.Create(ctx, miosa.CreateSandboxInput{
		ExternalUserID: "dr-smith",
		Metadata:       map[string]string{"customer": "acme"},
	})
	fmt.Println(sbx.ID)

	max := 3
	_, _ = admin.Policies.SetExternalUser(ctx, "dr-smith", miosa.Policy{Quotas: &miosa.PolicyQuotas{MaxSandboxes: &max}})
	eff, _ := admin.Policies.EffectiveForExternalUser(ctx, "dr-smith", "ws_1")
	quotas, source, _ := eff.Quotas()
	fmt.Println(quotas.MaxSandboxes, source)

	mine, _ := admin.Sandboxes.ListAll(ctx, miosa.ListSandboxesInput{
		ExternalUserID: "dr-smith", Tags: map[string]string{"customer": "acme"},
	})
	fmt.Println(len(mine))

	acme := admin.ForTenant("org-id").With(miosa.WithBillTo("acme"))
	_ = acme
}

func ExampleWebhookHandler() {
	secret := "whsec_..."
	http.Handle("/miosa", miosa.WebhookHandler(secret, func(ctx context.Context, ev *miosa.WebhookEvent) error {
		var sandbox struct {
			ID string `json:"id"`
		}
		if err := ev.Decode(&sandbox); err != nil {
			return err
		}
		fmt.Println(ev.Event, ev.DeliveryID, sandbox.ID)
		return nil
	}))
}

func ExampleWebhooksService_Replay() {
	ctx := context.Background()
	client := miosa.NewClient("msk_u_...")

	wh, _ := client.Webhooks.Create(ctx, miosa.CreateWebhookInput{
		URL: "https://example.com/miosa", Events: []string{"sandbox.created"}, WorkspaceID: "ws_1",
	})
	deliveries, _ := client.Webhooks.Deliveries(ctx, wh.ID)
	_, _ = client.Webhooks.Replay(ctx, wh.ID, deliveries.Data[0].ID, "")

	_, _ = client.Events.Create(ctx, miosa.CreateEventSubscriptionInput{
		ConsumerKind: "workflow", ConsumerID: "wf_1", EventType: "sandbox.created",
	})
	page, _ := client.Events.Outbox(ctx, miosa.PlatformEventsOptions{Type: "sandbox.created"})
	fmt.Println(len(page.Data))
}
