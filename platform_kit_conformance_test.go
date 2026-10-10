package miosa_test

import (
	"os"
	"path/filepath"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
	"gopkg.in/yaml.v3"
)

// Platform-kit fixtures (fixtures/platform-kit/*). They join the same
// response and request tables the sprint fixtures use, so the manifest guard in
// TestSprintManifestIsFullyCovered sees them.

func ptr[T any](v T) *T { return &v }

func init() {
	const k = "fixtures/platform-kit/"
	responseCases = append(responseCases,
		responseCase{file: k + "audit-log-page-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) {
				return c.AuditLog.ListPage(bg(), miosa.AuditLogOptions{After: "c0", Type: "sandbox.created", Limit: 50})
			}},
		responseCase{file: k + "branding-asset-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) {
				u, err := c.Settings.UploadBrandingAsset(bg(), "logo", "logo.png", []byte("png"))
				return map[string]string{"url": u}, err
			}},
		responseCase{file: k + "branding-discover-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) { return c.Settings.DiscoverBranding(bg(), "acme.test") }},
		responseCase{file: k + "branding-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) { return c.Settings.Branding(bg()) }},
		responseCase{file: k + "computer-share-link-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) {
				return c.Computers.CreateShare(bg(), "c1", miosa.CreateComputerShareInput{Kind: "link", Role: "viewer", ExpiresInHours: 24})
			}},
		responseCase{file: k + "domain-response.yaml", unwrap: "data",
			call: func(c *miosa.Client) (interface{}, error) { return c.Domains.Get(bg(), "app.customer.com") }},
		responseCase{file: k + "event-redeliver-response.yaml", unwrap: "data",
			call: func(c *miosa.Client) (interface{}, error) { return c.Events.Redeliver(bg(), "d1") }},
		responseCase{file: k + "event-subscription-deliveries-response.yaml", unwrap: "data",
			call: func(c *miosa.Client) (interface{}, error) { return c.Events.Deliveries(bg(), "s1") }},
		responseCase{file: k + "event-subscription-list-response.yaml", unwrap: "data",
			call: func(c *miosa.Client) (interface{}, error) {
				return c.Events.List(bg(), miosa.ListEventSubscriptionsOptions{ConsumerKind: "webhook", ConsumerID: "w1"})
			}},
		responseCase{file: k + "quota-response.yaml", unwrap: "data",
			call: func(c *miosa.Client) (interface{}, error) { return c.Quotas.Get(bg(), "end-user-42") }},
		responseCase{file: k + "sandbox-batch-create-response.yaml", unwrap: "data",
			call: func(c *miosa.Client) (interface{}, error) {
				return c.Sandboxes.Batches.Create(bg(), miosa.BatchCreateInput{Count: 25})
			}},
		responseCase{file: k + "sandbox-batch-items-response.yaml", unwrap: "data",
			call: func(c *miosa.Client) (interface{}, error) { return c.Sandboxes.Batches.Items(bg(), "b1", 100) }},
		responseCase{file: k + "sandbox-list-page-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) {
				return c.Sandboxes.ListPage(bg(), miosa.ListSandboxesInput{State: "running", Limit: 50})
			}},
		responseCase{file: k + "sandbox-run-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) {
				return c.Sandboxes.Run(bg(), miosa.RunSandboxInput{Command: "echo hi"})
			}},
		responseCase{file: k + "sandbox-spend-response.yaml", unwrap: "data",
			call: func(c *miosa.Client) (interface{}, error) { return c.SandboxSpend.Get(bg(), "t1") }},
		responseCase{file: k + "scoped-token-create-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) {
				return c.ScopedTokens.Mint(bg(), miosa.MintScopedTokenInput{UserID: "u", WorkspaceID: "w"})
			}},
		responseCase{file: k + "scoped-token-list-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) {
				return c.ScopedTokens.List(bg(), "6f1c0a3e-1111-4222-8333-444455556666")
			}},
		responseCase{file: k + "scoped-token-revoke-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) {
				err := c.ScopedTokens.Revoke(bg(), "t1")
				return map[string]bool{"revoked": err == nil}, err
			}},
		responseCase{file: k + "service-account-key-response.yaml", unwrap: "data",
			call: func(c *miosa.Client) (interface{}, error) {
				return c.ServiceAccounts.CreateKey(bg(), "sa1", miosa.ServiceAccountKeyInput{Scopes: []string{"sandboxes:create"}})
			}},
		responseCase{file: k + "shared-link-resolve-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) { return c.SharedLinks.Resolve(bg(), "RAWTOKEN") }},
		responseCase{file: k + "usage-rollup-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) {
				return c.Usage.GetRollup(bg(), miosa.UsageRollupInput{GroupBy: "external_user_id", Period: "30d", Bucket: "day"})
			}},
		responseCase{file: k + "webhook-events-response.yaml",
			call: func(c *miosa.Client) (interface{}, error) { return c.Webhooks.Events(bg()) }},
		responseCase{file: k + "webhook-replay-response.yaml", unwrap: "data",
			call: func(c *miosa.Client) (interface{}, error) { return c.Webhooks.Replay(bg(), "w1", "d1", "") }},
	)

	requestCases = append(requestCases,
		requestCase{file: k + "branding-update-request.yaml", send: func(c *miosa.Client) error {
			_, err := c.Settings.SetBranding(bg(), miosa.BrandingUpdate{
				CustomAppName: ptr("Acme Cloud"), BrandColorPrimary: ptr("#0b5fff"), PoweredByVisible: ptr(false)})
			return err
		}},
		requestCase{file: k + "computer-share-link-request.yaml", send: func(c *miosa.Client) error {
			_, err := c.Computers.CreateShare(bg(), "c1", miosa.CreateComputerShareInput{Kind: "link", Role: "viewer", ExpiresInHours: 24})
			return err
		}},
		requestCase{file: k + "domain-assign-request.yaml", send: func(c *miosa.Client) error {
			_, err := c.Domains.Assign(bg(), "app.customer.com", miosa.AssignDomainInput{
				DeploymentID: "6f1c0a3e-1111-4222-8333-444455556666", RedirectPolicy: "none"})
			return err
		}},
		requestCase{file: k + "domain-create-request.yaml", send: func(c *miosa.Client) error {
			_, err := c.Domains.Create(bg(), miosa.CreateDomainInput{
				Hostname: "app.customer.com", DeploymentID: "6f1c0a3e-1111-4222-8333-444455556666"})
			return err
		}},
		requestCase{file: k + "event-subscription-create-request.yaml", send: func(c *miosa.Client) error {
			_, err := c.Events.Create(bg(), miosa.CreateEventSubscriptionInput{
				ConsumerKind: "webhook", ConsumerID: "9a2e0a3e-1111-4222-8333-444455556666", EventType: "sandbox.created",
				Filter: map[string]interface{}{"workspace_id": "1b7d0a3e-1111-4222-8333-444455556666"}})
			return err
		}},
		requestCase{file: k + "quota-put-request.yaml", send: func(c *miosa.Client) error {
			_, err := c.Quotas.Set(bg(), "end-user-42", miosa.SetQuotaInput{
				MaxSandboxes: ptr(5), MaxConcurrent: ptr(2), MaxStorageGB: ptr(10), MaxCreditCents: ptr(500)})
			return err
		}},
		requestCase{file: k + "sandbox-batch-create-request.yaml", send: func(c *miosa.Client) error {
			_, err := c.Sandboxes.Batches.Create(bg(), miosa.BatchCreateInput{
				Count: 25, Size: "small", TemplateID: "miosa-sandbox", SetupFile: "echo hi", Environment: "prod"})
			return err
		}},
		requestCase{file: k + "sandbox-run-request.yaml", send: func(c *miosa.Client) error {
			_, err := c.Sandboxes.Run(bg(), miosa.RunSandboxInput{Command: "echo hi", TemplateID: "miosa-sandbox", Size: "small"})
			return err
		}},
		requestCase{file: k + "sandbox-spend-update-request.yaml", send: func(c *miosa.Client) error {
			_, err := c.SandboxSpend.Update(bg(), "t1", miosa.SandboxSpendUpdate{
				Mode: "limited", CapCents: ptr(int64(5000)), AlertThresholds: []int{50, 80}})
			return err
		}},
		requestCase{file: k + "scoped-token-create-request.yaml", send: func(c *miosa.Client) error {
			_, err := c.ScopedTokens.Mint(bg(), miosa.MintScopedTokenInput{
				UserID: "end-user-42", WorkspaceID: "1b7d0a3e-1111-4222-8333-444455556666",
				Scopes: []string{"sandboxes:read", "sandboxes:exec"}, ExpiresInSeconds: 900})
			return err
		}},
		requestCase{file: k + "service-account-create-request.yaml", send: func(c *miosa.Client) error {
			_, err := c.ServiceAccounts.Create(bg(), miosa.CreateServiceAccountInput{Name: "ci", Description: "CI pipeline"})
			return err
		}},
	)
}

// TestPlatformKitWebhookSignatureVector checks the signing contract against the
// shared vector: the signature of the raw body verifies, a tampered body does not.
func TestPlatformKitWebhookSignatureVector(t *testing.T) {
	root := sprintRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "fixtures/platform-kit/webhook-signature-vector.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Secret                   string            `yaml:"secret"`
		RawBody                  string            `yaml:"raw_body"`
		Headers                  map[string]string `yaml:"headers"`
		TamperedBody             string            `yaml:"tampered_body"`
		ExpectedValid            bool              `yaml:"expected_valid"`
		ExpectedValidForTampered bool              `yaml:"expected_valid_for_tampered"`
	}
	if err := yaml.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	sig := v.Headers["x-miosa-signature"]
	if got := miosa.SignWebhook([]byte(v.RawBody), v.Secret); got != sig {
		t.Errorf("SignWebhook = %s, vector says %s", got, sig)
	}
	if err := miosa.VerifyWebhook([]byte(v.RawBody), sig, v.Secret); (err == nil) != v.ExpectedValid {
		t.Errorf("VerifyWebhook(raw) err = %v, want valid=%v", err, v.ExpectedValid)
	}
	if err := miosa.VerifyWebhook([]byte(v.TamperedBody), sig, v.Secret); (err == nil) != v.ExpectedValidForTampered {
		t.Errorf("VerifyWebhook(tampered) err = %v, want valid=%v", err, v.ExpectedValidForTampered)
	}
}
