package miosa_test

// Contract conformance against the shared sprint fixtures
// (sdks/contract-fixtures/sprint-2026-10-10, written from miosa-compute
// train/next-2026-10-10). Every response fixture is served to the matching SDK
// method and the decoded value is re-encoded: each non-zero value the API sent
// must survive. Every request fixture is rebuilt through the SDK and the body
// the SDK sends must equal the fixture body. Every error fixture must map to
// the documented code, retry hint and typed error.
//
// The fixtures live outside this module. Set MIOSA_SPRINT_FIXTURES_ROOT to
// point elsewhere. When the directory is missing the tests skip with a
// message, so a checkout of sdks/go alone still passes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
	"gopkg.in/yaml.v3"
)

type sprintFixture struct {
	Path    string            `yaml:"path"`
	Method  string            `yaml:"method"`
	Request bool              `yaml:"request"`
	Status  int               `yaml:"status"`
	Query   map[string]string `yaml:"query"`
	Headers map[string]string `yaml:"headers"`
	Body    interface{}       `yaml:"body"`
}

type sprintManifest struct {
	Fixtures []struct {
		File   string `yaml:"file"`
		Kind   string `yaml:"kind"`
		Method string `yaml:"method"`
		Path   string `yaml:"path"`
	} `yaml:"fixtures"`
}

func sprintRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("MIOSA_SPRINT_FIXTURES_ROOT")
	if root == "" {
		root = filepath.Join("..", "contract-fixtures", "sprint-2026-10-10")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(abs, "manifest.yaml")); err != nil {
		t.Skipf("sprint fixtures not found at %s (set MIOSA_SPRINT_FIXTURES_ROOT): %v", abs, err)
	}
	return abs
}

func loadSprint(t *testing.T, file string) sprintFixture {
	t.Helper()
	root := sprintRoot(t)
	data, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		t.Fatalf("fixture %s: %v", file, err)
	}
	var fx sprintFixture
	if err := yaml.Unmarshal(data, &fx); err != nil {
		t.Fatalf("fixture %s: %v", file, err)
	}
	fx.Body = jsonNormalize(t, fx.Body)
	return fx
}

// jsonNormalize turns a YAML-decoded value into what encoding/json would
// decode (numbers as float64, timestamps as RFC 3339 strings).
func jsonNormalize(t *testing.T, v interface{}) interface{} {
	t.Helper()
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	return out
}

// pathPattern turns "/environments/{id}/rename" into a matcher.
func pathPattern(tmpl string) *regexp.Regexp {
	re := regexp.MustCompile(`\\\{[^}]+\\\}`).ReplaceAllString(regexp.QuoteMeta(tmpl), `[^/]+`)
	return regexp.MustCompile("^" + re + "$")
}

// serveFixture answers every request with the fixture and records requests.
// wrapData serves the body inside {"data": body} when the fixture is flat but
// the API wraps (see the report: harness-sessions/run-response.yaml).
func serveFixture(t *testing.T, fx sprintFixture, wrapData bool, opts ...miosa.ClientOption) (*miosa.Client, *recorder) {
	t.Helper()
	body := fx.Body
	if wrapData {
		body = map[string]interface{}{"data": body}
	}
	status := fx.Status
	if status == 0 {
		status = 200
	}
	return newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range fx.Headers {
			w.Header().Set(k, v)
		}
		if body == nil {
			w.WriteHeader(status)
			return
		}
		writeJSON(w, status, body)
	}, opts...)
}

// assertFidelity fails when a non-zero value of want is missing from got.
// Zero values (null, "", 0, false, empty list or object) may be absent: the
// SDK types drop them with omitempty.
func assertFidelity(t *testing.T, path string, want, got interface{}, skip map[string]bool) {
	t.Helper()
	if skip[path] {
		return
	}
	switch w := want.(type) {
	case map[string]interface{}:
		g, _ := got.(map[string]interface{})
		keys := make([]string, 0, len(w))
		for k := range w {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := path + "." + k
			if skip[child] {
				continue
			}
			gv, present := g[k]
			if !present {
				if !isZeroJSON(w[k]) {
					t.Errorf("%s: the SDK dropped %v", child, w[k])
				}
				continue
			}
			assertFidelity(t, child, w[k], gv, skip)
		}
	case []interface{}:
		g, _ := got.([]interface{})
		if len(g) != len(w) {
			if len(w) != 0 || len(g) != 0 {
				t.Errorf("%s: %d items in the fixture, %d after decoding", path, len(w), len(g))
			}
			return
		}
		for i := range w {
			assertFidelity(t, fmt.Sprintf("%s[%d]", path, i), w[i], g[i], skip)
		}
	default:
		if isZeroJSON(want) && (got == nil || isZeroJSON(got)) {
			return
		}
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s: fixture has %#v, SDK value is %#v", path, want, got)
		}
	}
}

func isZeroJSON(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case float64:
		return x == 0
	case bool:
		return !x
	case []interface{}:
		return len(x) == 0
	case map[string]interface{}:
		return len(x) == 0
	}
	return false
}

// jsonOf re-encodes an SDK value the way a caller would see it on the wire.
func jsonOf(t *testing.T, v interface{}) interface{} {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode %T: %v", v, err)
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func unwrap(body interface{}, key string) interface{} {
	if key == "" {
		return body
	}
	m, _ := body.(map[string]interface{})
	return m[key]
}

// ─── Response fixtures ───────────────────────────────────────────────────────

type responseCase struct {
	file string
	// call runs the SDK method that fixture answers and returns the value to
	// compare.
	call func(c *miosa.Client) (interface{}, error)
	// unwrap is the envelope key of the fixture body the value corresponds to.
	unwrap string
	// skip lists fixture paths (".a.b") the SDK intentionally does not model.
	skip []string
	// wrapData serves a flat fixture inside {"data": ...}.
	wrapData bool
	// check runs extra assertions.
	check func(t *testing.T, got interface{}, fx sprintFixture)
	// drift, when set, skips the case: the fixture disagrees with the
	// controller on train/next-2026-10-10, and the SDK follows the controller.
	drift string
	// reshape maps the SDK value to the shape of the fixture body when the SDK
	// deliberately exposes a friendlier structure.
	reshape func(v interface{}) interface{}
}

func bg() context.Context { return context.Background() }

var responseCases = []responseCase{
	// environments
	{file: "fixtures/environments/list-response.yaml", skip: []string{".data"},
		call: func(c *miosa.Client) (interface{}, error) { return c.Environments.List(bg()) }},
	{file: "fixtures/environments/billing-response.yaml", unwrap: "billing",
		call: func(c *miosa.Client) (interface{}, error) { return c.Environments.Billing(bg()) }},
	{file: "fixtures/environments/create-response.yaml", unwrap: "environment",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Environments.Create(bg(), miosa.CreateEnvironmentInput{Name: "x"})
		}},
	{file: "fixtures/environments/show-response.yaml", unwrap: "environment",
		call: func(c *miosa.Client) (interface{}, error) { return c.Environments.Get(bg(), "prod") }},
	{file: "fixtures/environments/edit-response.yaml", unwrap: "environment",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Environments.SetVariable(bg(), "prod", "NEW_VAR", "x")
		}},
	{file: "fixtures/environments/inherit-default-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) {
			id, err := c.Environments.ForWorkspace("ws").InheritDefault(bg())
			return map[string]interface{}{"default_environment_id": id}, err
		}},
	{file: "fixtures/environments/delete-response.yaml", skip: []string{".environment", ".deleted"},
		call: func(c *miosa.Client) (interface{}, error) {
			return map[string]interface{}{}, c.Environments.Delete(bg(), "prod")
		}},
	{file: "fixtures/environments/reveal-variable-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) {
			v, err := c.Environments.RevealVariable(bg(), "prod", "STRIPE_KEY")
			return map[string]interface{}{"value": v}, err
		}, skip: []string{".name"}},
	{file: "fixtures/environments/reveal-file-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) {
			v, err := c.Environments.RevealSecretFile(bg(), "prod", "backend/.env")
			return map[string]interface{}{"contents": v}, err
		}, skip: []string{".path"}},
	{file: "fixtures/environments/effective-response.yaml", unwrap: "effective",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Environments.Effective(bg(), "prod", miosa.EffectiveOptions{})
		}},
	{file: "fixtures/environments/versions-response.yaml", unwrap: "versions",
		call: func(c *miosa.Client) (interface{}, error) { return c.Environments.Versions(bg(), "prod") }},
	{file: "fixtures/environments/upgrade-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) { return c.Environments.Upgrade(bg(), "prod") }},
	{file: "fixtures/environments/deployment-environments-list.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.Environments.DeploymentInventory(bg()) }},
	{file: "fixtures/environments/machine-json-environment-fields.yaml",
		call: func(c *miosa.Client) (interface{}, error) { return c.Sandboxes.Get(bg(), "sbx") }},
	// setup
	{file: "fixtures/setup/machine-setup-status.yaml",
		call: func(c *miosa.Client) (interface{}, error) { return c.Sandboxes.Get(bg(), "sbx") }},
	{file: "fixtures/setup/commands-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Sandboxes.RunCommand(bg(), "sbx", miosa.CommandInput{Command: "x"})
		}},
	{file: "fixtures/setup/commands-timed-out-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Sandboxes.RunCommand(bg(), "sbx", miosa.CommandInput{Command: "x"})
		},
		check: func(t *testing.T, got interface{}, _ sprintFixture) {
			if r := got.(*miosa.CommandResult); !r.TimedOut || r.ExitCode != -1 {
				t.Errorf("a timed out command is exit -1 with timed_out: %+v", r)
			}
		}},
	{file: "fixtures/setup/agent-prompt-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.Sandboxes.QueueAgentPrompt(bg(), "sbx", "go") }},
	// snapshots
	{file: "fixtures/snapshots/history-list-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Snapshots.List(bg(), miosa.ListSnapshotsOptions{})
		}},
	{file: "fixtures/snapshots/groups-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Snapshots.Groups(bg(), miosa.ListSnapshotsOptions{})
		}},
	{file: "fixtures/snapshots/show-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) { return c.Snapshots.Get(bg(), "snap") }},
	{file: "fixtures/snapshots/tree-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Snapshots.Tree(bg(), "snap", "/home/user", miosa.BrowseOptions{})
		}},
	{file: "fixtures/snapshots/fork-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Snapshots.Fork(bg(), "snap", miosa.ForkSnapshotInput{})
		},
		check: func(t *testing.T, got interface{}, fx sprintFixture) {
			r := got.(*miosa.SnapshotForkResult)
			if r.Snapshot == nil {
				t.Fatal("the source snapshot beside the machine must be kept")
			}
			assertFidelity(t, "snapshot", unwrap(fx.Body, "snapshot"), jsonOf(t, r.Snapshot), nil)
		}},
	{file: "fixtures/snapshots/delete-response.yaml", skip: []string{".data"},
		call: func(c *miosa.Client) (interface{}, error) {
			return map[string]interface{}{}, c.Snapshots.Delete(bg(), "snap")
		}},
	{file: "fixtures/snapshots/bulk-delete-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Snapshots.DeleteMany(bg(), miosa.DeleteSnapshotsInput{IDs: []string{"a"}})
		}},
	{file: "fixtures/snapshots/named-list-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) { return c.NamedSnapshots.List(bg(), "") }},
	{file: "fixtures/snapshots/named-create-response.yaml", unwrap: "data", skip: []string{".operation_id"},
		call: func(c *miosa.Client) (interface{}, error) {
			return c.NamedSnapshots.Save(bg(), miosa.SaveNamedSnapshotInput{Name: "web-stack", SnapshotID: "s"})
		}},
	{file: "fixtures/snapshots/named-show-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) { return c.NamedSnapshots.Get(bg(), "web-stack") }},
	{file: "fixtures/snapshots/named-delete-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.NamedSnapshots.Remove(bg(), "web-stack") }},
	{file: "fixtures/snapshots/named-deploy-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.NamedSnapshots.Deploy(bg(), "web-stack", miosa.ForkSnapshotInput{})
		}},
	// bill-to
	{file: "fixtures/bill-to/get-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.BillTo.Get(bg()) }},
	{file: "fixtures/bill-to/limits-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.BillTo.Limits(bg()) }},
	{file: "fixtures/bill-to/member-caps-list-response.yaml", unwrap: "data",
		reshape: func(v interface{}) interface{} { return v.(map[string]interface{})["members"] },
		call:    func(c *miosa.Client) (interface{}, error) { return c.MemberCaps.List(bg(), "tenant") }},
	// agents
	{file: "fixtures/agents/credentials-list-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.Agents.Credentials(bg(), "") }},
	{file: "fixtures/agents/credential-put-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Agents.PutCredential(bg(), "openrouter", miosa.PutAgentCredentialInput{Fields: map[string]string{"api_key": "k"}})
		}},
	{file: "fixtures/agents/settings-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.Agents.Settings(bg(), "") }},
	{file: "fixtures/agents/harness-patch-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Agents.UpdateHarness(bg(), "pi", miosa.UpdateAgentHarnessInput{})
		}},
	{file: "fixtures/agents/models-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.Agents.Models(bg(), "claude-code", "") }},
	{file: "fixtures/agents/signin-session-kimi-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.AgentAccounts.StartSignin(bg(), miosa.StartSigninInput{Provider: miosa.SigninKimiCode})
		}},
	{file: "fixtures/agents/signin-session-mistral-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.AgentAccounts.GetSignin(bg(), "s") }},
	{file: "fixtures/agents/signin-session-succeeded-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.AgentAccounts.GetSignin(bg(), "s") }},
	// connections
	{file: "fixtures/connections/list-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Connections.List(bg(), miosa.ListConnectionsOptions{})
		}},
	{file: "fixtures/connections/tool-server-item.yaml",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Connections.List(bg(), miosa.ListConnectionsOptions{})
		}},
	// harness sessions and usage
	{file: "fixtures/harness-sessions/run-response.yaml", wrapData: true,
		call: func(c *miosa.Client) (interface{}, error) { return c.Runs.Get(bg(), "run") }},
	{file: "fixtures/harness-sessions/runs-list-filter-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.Runs.List(bg(), miosa.RunListInput{ChatID: "c"}) }},
	{file: "fixtures/harness-sessions/chat-context-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.Agents.ChatContext(bg(), "chat") }},
	{file: "fixtures/harness-sessions/chat-compact-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) { return c.Agents.CompactChat(bg(), "chat", "") }},
	{file: "fixtures/usage/runs-usage-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.Runs.Usage(bg(), miosa.RunUsageOptions{}) }},
	// platform
	{file: "fixtures/platform/whoami-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) { return c.Whoami(bg()) }},
	{file: "fixtures/platform/api-key-presets-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.ApiKeys.Presets(bg()) }},
	{file: "fixtures/platform/ssh-certificate-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.Sandboxes.IssueSSHCertificate(bg(), "sbx", miosa.SSHCertificateInput{PublicKey: "k"})
		}},
	{file: "fixtures/platform/ssh-info-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) { return c.Sandboxes.SSHInfo(bg(), "sbx") }},
	{file: "fixtures/platform/create-wait-seconds-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) {
			r, err := c.Sandboxes.CreateDetailed(bg(), miosa.CreateSandboxInput{WaitSeconds: 45})
			if err != nil {
				return nil, err
			}
			return r.Sandbox, nil
		}},
	{file: "fixtures/platform/create-wait-timeout-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) {
			r, err := c.Sandboxes.CreateDetailed(bg(), miosa.CreateSandboxInput{WaitSeconds: 45})
			if err != nil {
				return nil, err
			}
			if r.WaitOutcome != "timeout" {
				return nil, fmt.Errorf("WaitOutcome = %q, want timeout", r.WaitOutcome)
			}
			return r.Sandbox, nil
		}},
	{file: "fixtures/platform/opencomputers-job-start-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.OpenComputers.Jobs.Start(bg(), "host", miosa.StartJobInput{Command: "npm test"})
		}},
	{file: "fixtures/bill-to/create-response-headers.yaml",
		call: func(c *miosa.Client) (interface{}, error) {
			r, err := c.Sandboxes.CreateDetailed(bg(), miosa.CreateSandboxInput{})
			if err != nil {
				return nil, err
			}
			if r.BilledOrganizationID != "b7000000-0000-4000-8000-000000000001" || r.BillToSource != miosa.BillToSourceRequest {
				return nil, fmt.Errorf("bill-to headers = %q %q", r.BilledOrganizationID, r.BillToSource)
			}
			return r.Sandbox, nil
		}},
	// ai gateway
	{file: "fixtures/ai-gateway/policies-list-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.AIGateway.ListPolicies(bg()) }},
	{file: "fixtures/ai-gateway/budget-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.AIGateway.Budget(bg()) }},
	{file: "fixtures/ai-gateway/health-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.AIGateway.Health(bg()) }},
	{file: "fixtures/ai-gateway/limits-response.yaml", unwrap: "data", skip: []string{".organization"},
		call: func(c *miosa.Client) (interface{}, error) { return c.AIGateway.Limits(bg()) }},
	{file: "fixtures/ai-gateway/limit-upsert-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.AIGateway.PutLimit(bg(), miosa.GatewayScopeEnvironment, "production", miosa.GatewayLimitInput{})
		}},
	{file: "fixtures/ai-gateway/environments-response.yaml", unwrap: "data",
		drift: "keys is a count here; Engine.Intelligence.KeyEnvironments.overview returns a list of {id, name, key_prefix} plus key_count",
		call:  func(c *miosa.Client) (interface{}, error) { return c.AIGateway.Environments(bg()) }},
	{file: "fixtures/ai-gateway/key-environment-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.AIGateway.SetKeyEnvironment(bg(), "k", "staging") }},
	{file: "fixtures/ai-gateway/settings-response.yaml", unwrap: "data",
		drift: "trace_retention_days; Engine.Intelligence.GatewaySettings.view returns retention_days and max_retention_days",
		call:  func(c *miosa.Client) (interface{}, error) { return c.AIGateway.Settings(bg()) }},
	{file: "fixtures/ai-gateway/requests-list-response.yaml",
		call: func(c *miosa.Client) (interface{}, error) {
			return c.AIGateway.Requests(bg(), miosa.GatewayRequestsOptions{})
		}},
	{file: "fixtures/ai-gateway/provider-health-response.yaml", unwrap: "data",
		call: func(c *miosa.Client) (interface{}, error) { return c.AIGateway.ProviderHealth(bg(), "") }},
}

func TestSprintResponseFixtures(t *testing.T) {
	sprintRoot(t)
	for _, tc := range responseCases {
		tc := tc
		t.Run(strings.TrimSuffix(strings.TrimPrefix(tc.file, "fixtures/"), ".yaml"), func(t *testing.T) {
			if tc.drift != "" {
				t.Skipf("fixture disagrees with the backend: %s", tc.drift)
			}
			fx := loadSprint(t, tc.file)
			client, rec := serveFixture(t, fx, tc.wrapData)

			got, err := tc.call(client)
			if err != nil {
				t.Fatalf("SDK call failed on the fixture: %v", err)
			}
			req := rec.Last(t)
			if !strings.EqualFold(req.Method, fx.Method) {
				t.Errorf("method = %s, fixture says %s", req.Method, fx.Method)
			}
			if !pathPattern(fx.Path).MatchString(req.Path) {
				t.Errorf("path = %s, fixture route is %s", req.Path, fx.Path)
			}
			skip := map[string]bool{}
			for _, s := range tc.skip {
				skip[s] = true
			}
			want := unwrap(fx.Body, tc.unwrap)
			encoded := jsonOf(t, got)
			if tc.reshape != nil {
				encoded = tc.reshape(encoded)
			}
			assertFidelity(t, "", want, encoded, skip)
			if tc.check != nil {
				tc.check(t, got, fx)
			}
		})
	}
}

func TestSprintTreeWarmingFixture(t *testing.T) {
	sprintRoot(t)
	fx := loadSprint(t, "fixtures/snapshots/tree-warming-202.yaml")
	client, _ := serveFixture(t, fx, false)

	_, err := client.Snapshots.Tree(bg(), "snap", "/", miosa.BrowseOptions{WaitTimeout: -1})

	var warming *miosa.SnapshotWarmingError
	if !errors.As(err, &warming) {
		t.Fatalf("error = %#v", err)
	}
	wantMS := unwrap(fx.Body, "retry_after_ms").(float64)
	if warming.RetryAfter != time.Duration(wantMS)*time.Millisecond {
		t.Errorf("RetryAfter = %v, fixture says %vms", warming.RetryAfter, wantMS)
	}
}

// ─── Request fixtures ────────────────────────────────────────────────────────

type requestCase struct {
	file string
	// send makes the SDK send the request the fixture describes.
	send func(c *miosa.Client) error
	// ignore lists fixture body keys that carry a server default (for example
	// "no_env": false) the SDK is right to omit.
	ignore []string
	// headerOnly fixtures have no body to compare; the headers are checked.
	headerOnly bool
}

func ptrBool(b bool) *bool    { return &b }
func ptrStr(s string) *string { return &s }
func ptrInt(i int) *int       { return &i }

var requestCases = []requestCase{
	{file: "fixtures/environments/create-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Environments.Create(bg(), miosa.CreateEnvironmentInput{
			Name: "prod", WorkspaceID: "1b7d0a3e-1111-4222-8333-444455556666", IsDefault: ptrBool(false), SafeForThirdParties: ptrBool(false),
			PassGithub: ptrBool(true), PassSecrets: ptrBool(true), PassSandboxCredentials: ptrBool(false), PassAgentsCredentials: ptrBool(true),
			ExtendsDefault: ptrBool(true), Variables: map[string]string{"STRIPE_KEY": "sk_live_example"},
			PlainVariables: []string{"REGION"},
			SecretFiles:    []miosa.EnvironmentSecretFileInput{{Path: "backend/.env", Contents: "A=1\n"}},
			Repositories:   []miosa.EnvironmentRepositoryInput{{Repo: "octocat/hello-world", Source: "github", BaseBranch: "develop", SetupScript: "npm ci", SetupBlocking: ptrBool(true)}},
			CLITools:       map[string]string{"gh": "latest"},
			Connections:    []string{"mcp_server:6f1c0a3e-1111-4222-8333-444455556666"},
			SetupScript:    "make setup", SetupBlocking: ptrBool(false),
		})
		return err
	}},
	{file: "fixtures/environments/patch-atomic-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Environments.Update(bg(), "prod", miosa.UpdateEnvironmentInput{
			Name: ptrStr("prod-2"), IsDefault: ptrBool(true), SafeForThirdParties: ptrBool(false), PassGithub: ptrBool(true), ExtendsDefault: ptrBool(true),
			VariablesSet: map[string]string{"NEW_VAR": "x"}, VariablesUnset: []string{"OLD_VAR"},
			SecretFilesSet:    []miosa.EnvironmentSecretFileInput{{Path: "backend/.env", Contents: "A=2\n"}},
			SecretFilesUnset:  []string{".env.old"},
			RepositoriesSet:   []miosa.EnvironmentRepositoryInput{{Repo: "octocat/hello-world", Source: "github", BaseBranch: "main", SetupScript: "npm ci", SetupBlocking: ptrBool(true)}},
			RepositoriesUnset: []miosa.EnvironmentRepositoryRef{{Repo: "octocat/old"}},
			CLIToolsSet:       map[string]string{"gh": "2.60.0"}, CLIToolsUnset: []string{"jq"},
			ConnectionsSet:   []string{"mcp_server:6f1c0a3e-1111-4222-8333-444455556666"},
			ConnectionsUnset: []string{"connected_account:9a2e0a3e-1111-4222-8333-444455556666"},
			ClearSetupScript: true, SetupBlocking: ptrBool(false), PlainVariables: []string{"REGION"},
			WorkspaceID: "1b7d0a3e-1111-4222-8333-444455556666",
		})
		return err
	}},
	{file: "fixtures/environments/rename-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Environments.Rename(bg(), "prod-1", "prod")
		return err
	}},
	{file: "fixtures/environments/make-default-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Environments.MakeDefault(bg(), "prod")
		return err
	}},
	{file: "fixtures/environments/set-variable-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Environments.SetVariable(bg(), "prod", "STRIPE_KEY", "sk_live_example")
		return err
	}},
	{file: "fixtures/environments/set-file-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Environments.SetSecretFile(bg(), "prod", "backend/.env", "A=1\n")
		return err
	}},
	{file: "fixtures/environments/add-repository-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Environments.AddRepository(bg(), "prod", miosa.EnvironmentRepositoryInput{
			Repo: "octocat/hello-world", Source: "github", BaseBranch: "develop", SetupScript: "npm ci", SetupBlocking: ptrBool(true),
		})
		return err
	}},
	{file: "fixtures/environments/upgrade-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Environments.Upgrade(bg(), "prod", "9a2e0a3e-1111-4222-8333-444455556666")
		return err
	}},
	{file: "fixtures/environments/sandbox-create-with-environment-request.yaml", ignore: []string{"no_env"}, send: func(c *miosa.Client) error {
		_, err := c.Sandboxes.Create(bg(), miosa.CreateSandboxInput{Size: miosa.SizeSmall, Environment: "prod", Env: map[string]string{"DEBUG": "1"}})
		return err
	}},
	{file: "fixtures/setup/sandbox-create-setup-file-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Sandboxes.Create(bg(), miosa.CreateSandboxInput{
			Size: miosa.SizeSmall, SetupFile: "#!/bin/bash\nnpm install || miosa-queue-prompt \"npm install failed\"\n",
		})
		return err
	}},
	{file: "fixtures/setup/commands-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Sandboxes.RunCommand(bg(), "sbx", miosa.CommandInput{Command: "bash ./setup.sh", Cwd: "my-repo", TimeoutSeconds: 60, Stdin: "optional text"})
		return err
	}},
	{file: "fixtures/setup/agent-prompt-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Sandboxes.QueueAgentPrompt(bg(), "sbx", "npm install failed, fix the install then continue")
		return err
	}},
	{file: "fixtures/snapshots/fork-request.yaml", ignore: []string{"no_env"}, send: func(c *miosa.Client) error {
		_, err := c.Snapshots.Fork(bg(), "snap", miosa.ForkSnapshotInput{MachineEnvironmentOptions: miosa.MachineEnvironmentOptions{Environment: "prod"}})
		return err
	}},
	{file: "fixtures/snapshots/bulk-delete-ids-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Snapshots.DeleteMany(bg(), miosa.DeleteSnapshotsInput{IDs: []string{"f3c10000-0000-4000-8000-000000000001", "a77e0000-0000-4000-8000-000000000001"}})
		return err
	}},
	{file: "fixtures/snapshots/bulk-delete-all-request.yaml", send: func(c *miosa.Client) error {
		id := "9a2e0a3e-1111-4222-8333-444455556666"
		_, err := c.Snapshots.DeleteMany(bg(), miosa.DeleteSnapshotsInput{ResourceID: id, All: true, Confirm: id})
		return err
	}},
	{file: "fixtures/snapshots/named-create-from-sandbox-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.NamedSnapshots.Save(bg(), miosa.SaveNamedSnapshotInput{Name: "web-stack", SandboxID: "9a2e0a3e-1111-4222-8333-444455556666"})
		return err
	}},
	{file: "fixtures/snapshots/named-create-from-snapshot-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.NamedSnapshots.Save(bg(), miosa.SaveNamedSnapshotInput{Name: "web-stack", SnapshotID: "f3c10000-0000-4000-8000-000000000001"})
		return err
	}},
	{file: "fixtures/snapshots/named-deploy-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.NamedSnapshots.Deploy(bg(), "web-stack", miosa.ForkSnapshotInput{MachineEnvironmentOptions: miosa.MachineEnvironmentOptions{Environment: "prod"}})
		return err
	}},
	{file: "fixtures/bill-to/put-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.BillTo.Set(bg(), "acme")
		return err
	}},
	{file: "fixtures/bill-to/create-with-header-request.yaml", headerOnly: true, send: func(c *miosa.Client) error {
		_, err := c.Sandboxes.Create(miosa.WithBillToContext(bg(), "acme"), miosa.CreateSandboxInput{})
		return err
	}},
	{file: "fixtures/bill-to/member-caps-update-request.yaml", send: func(c *miosa.Client) error {
		cap := int64(500)
		_, err := c.MemberCaps.Set(bg(), "t", "u", miosa.MemberCapsInput{UsageCapCents: &cap, ClearMaxConcurrentSandboxes: true})
		return err
	}},
	{file: "fixtures/agents/credential-put-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Agents.PutCredential(bg(), "openrouter", miosa.PutAgentCredentialInput{
			Fields: map[string]string{"api_key": "sk-or-example"}, UsableBy: []string{"agents"}, WorkspaceID: "1b7d0a3e-1111-4222-8333-444455556666",
		})
		return err
	}},
	{file: "fixtures/agents/settings-patch-default-harness-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Agents.SetDefaultHarness(bg(), "pi", "1b7d0a3e-1111-4222-8333-444455556666")
		return err
	}},
	{file: "fixtures/agents/harness-patch-single-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Agents.UpdateHarness(bg(), "claude-code", miosa.UpdateAgentHarnessInput{AuthMethod: ptrStr("anthropic"), DefaultModel: ptrStr("claude-sonnet-5-5"), DefaultEffort: ptrStr("high")})
		return err
	}},
	{file: "fixtures/agents/harness-patch-multi-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Agents.UpdateHarness(bg(), "pi", miosa.UpdateAgentHarnessInput{
			EnabledProviders: []string{"anthropic", "openrouter"}, DefaultModel: ptrStr("openrouter:deepseek/deepseek-v4.1-flash"), ClearDefaultEffort: true,
		})
		return err
	}},
	{file: "fixtures/agents/signin-start-kimi-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.AgentAccounts.StartSignin(bg(), miosa.StartSigninInput{Provider: miosa.SigninKimiCode, WorkspaceID: "1b7d0a3e-1111-4222-8333-444455556666"})
		return err
	}},
	{file: "fixtures/agents/signin-start-mistral-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.AgentAccounts.StartSignin(bg(), miosa.StartSigninInput{Provider: miosa.SigninMistralVibe, WorkspaceID: "1b7d0a3e-1111-4222-8333-444455556666"})
		return err
	}},
	{file: "fixtures/connections/share-app-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Connections.ShareApp(bg(), "app", true)
		return err
	}},
	{file: "fixtures/harness-sessions/run-create-chat-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Runs.Run(bg(), miosa.RunCreateInput{
			Runner: "claude-code", Instruction: "continue", TargetKind: miosa.RunTargetSandbox, TargetID: "9a2e0a3e-1111-4222-8333-444455556666",
			ChatID: "chat-3f0c", ContinueFromRunID: "6f1c0a3e-1111-4222-8333-444455556666",
		})
		return err
	}},
	{file: "fixtures/harness-sessions/chat-compact-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Agents.CompactChat(bg(), "chat-3f0c", "keep the migration plan")
		return err
	}},
	{file: "fixtures/platform/ssh-certificate-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Sandboxes.IssueSSHCertificate(bg(), "sbx", miosa.SSHCertificateInput{
			PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample user@host", TTLSeconds: 900,
		})
		return err
	}},
	{file: "fixtures/platform/ssh-key-certificate-flag-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.Sandboxes.InstallSSHKey(bg(), "sbx", miosa.InstallSSHKeyInput{
			PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample user@host", Certificate: true,
		})
		return err
	}},
	{file: "fixtures/platform/opencomputers-job-start-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.OpenComputers.Jobs.Start(bg(), "host", miosa.StartJobInput{Command: "npm test", Args: []string{"--ci"}, Env: map[string]string{"CI": "1"}, Cwd: "~", TimeoutSeconds: 300})
		return err
	}},
	{file: "fixtures/ai-gateway/policy-create-request.yaml", ignore: []string{"selector"}, send: func(c *miosa.Client) error {
		_, err := c.AIGateway.CreatePolicy(bg(), miosa.GatewayPolicyInput{
			Name: "default", Selector: map[string]interface{}{}, PrimaryModel: "gpt-6-astra",
			FallbackModels: []string{"claude-sonnet-5-5"}, Priority: ptrInt(0), Enabled: ptrBool(true),
		})
		return err
	}},
	{file: "fixtures/ai-gateway/budget-put-request.yaml", send: func(c *miosa.Client) error {
		m, per, r, hard := int64(100000), int64(500), int64(600), true
		_, err := c.AIGateway.PutBudget(bg(), miosa.GatewayBudgetInput{MonthlyCreditLimit: &m, PerRequestCreditLimit: &per, RequestsPerMinute: &r, HardLimit: &hard})
		return err
	}},
	{file: "fixtures/ai-gateway/limit-upsert-request.yaml", send: func(c *miosa.Client) error {
		r, tok, mon, hard := int64(60), int64(1000000), int64(5000), true
		_, err := c.AIGateway.PutLimit(bg(), miosa.GatewayScopeEnvironment, "production", miosa.GatewayLimitInput{RequestsPerMinute: &r, TokensPerDay: &tok, MonthlyCreditLimit: &mon, HardLimit: &hard})
		return err
	}},
	{file: "fixtures/ai-gateway/key-environment-request.yaml", send: func(c *miosa.Client) error {
		_, err := c.AIGateway.SetKeyEnvironment(bg(), "k", "staging")
		return err
	}},
}

func TestSprintRequestFixtures(t *testing.T) {
	sprintRoot(t)
	for _, tc := range requestCases {
		tc := tc
		t.Run(strings.TrimSuffix(strings.TrimPrefix(tc.file, "fixtures/"), ".yaml"), func(t *testing.T) {
			fx := loadSprint(t, tc.file)
			// Answer with an empty success so the SDK call completes.
			client, rec := newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]interface{}{}) })
			if tc.headerOnly {
				client, rec = newHandlerClient(t, func(w http.ResponseWriter, _ *http.Request) {
					writeJSON(w, 201, map[string]interface{}{"data": map[string]interface{}{}})
				})
			}
			_ = tc.send(client) // decode errors on the empty answer are irrelevant here
			req := rec.Last(t)

			if !strings.EqualFold(req.Method, fx.Method) {
				t.Errorf("method = %s, fixture says %s", req.Method, fx.Method)
			}
			if !pathPattern(fx.Path).MatchString(req.Path) {
				t.Errorf("path = %s, fixture route is %s", req.Path, fx.Path)
			}
			for k, v := range fx.Headers {
				if got := req.Header.Get(k); got != v {
					t.Errorf("header %s = %q, fixture says %q", k, got, v)
				}
			}
			if tc.headerOnly || fx.Body == nil {
				return
			}
			var sent interface{}
			if err := json.Unmarshal(req.Raw, &sent); err != nil {
				t.Fatalf("request body is not JSON: %v (%q)", err, req.Raw)
			}
			want := fx.Body
			if m, ok := want.(map[string]interface{}); ok {
				for _, k := range tc.ignore {
					delete(m, k)
				}
			}
			if !reflect.DeepEqual(want, sent) {
				t.Errorf("request body differs\n sent:    %s\n fixture: %s", mustJSON(sent), mustJSON(want))
			}
		})
	}
}

func mustJSON(v interface{}) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

// ─── Error fixtures ──────────────────────────────────────────────────────────

func TestSprintErrorFixtures(t *testing.T) {
	root := sprintRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var m sprintManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range m.Fixtures {
		if entry.Kind != "error" {
			continue
		}
		count++
		entry := entry
		if entry.File == "fixtures/snapshots/bulk-delete-all-blocked-409.yaml" {
			// A fully blocked bulk delete is 409 with a result body, not an
			// error envelope; TestSprintBulkDeleteAllBlockedFixture covers it.
			continue
		}
		t.Run(strings.TrimSuffix(strings.TrimPrefix(entry.File, "fixtures/"), ".yaml"), func(t *testing.T) {
			fx := loadSprint(t, entry.File)
			client, _ := serveFixture(t, fx, false)

			_, err := client.Whoami(bg())

			m := miosa.AsMiosaError(err)
			if m == nil {
				t.Fatalf("error = %#v, want an API error", err)
			}
			if m.StatusCode != fx.Status {
				t.Errorf("StatusCode = %d, fixture %d", m.StatusCode, fx.Status)
			}
			assertTypedError(t, err, m.Code)
			body, _ := fx.Body.(map[string]interface{})
			switch e := body["error"].(type) {
			case map[string]interface{}:
				code, _ := e["code"].(string)
				if m.Code != code {
					t.Errorf("Code = %q, fixture %q", m.Code, code)
				}
				if msg, _ := e["message"].(string); msg != "" && m.Message != msg {
					t.Errorf("Message = %q, fixture %q", m.Message, msg)
				}
				if r, _ := e["retryable"].(bool); r && !m.Retryable {
					t.Errorf("fixture says retryable, SDK says not")
				}
				if ms, ok := e["retry_after_ms"].(float64); ok {
					// body hint is used unless the Retry-After header is present
					want := time.Duration(ms) * time.Millisecond
					if fx.Headers["Retry-After"] != "" {
						want = time.Duration(0)
					}
					if want != 0 && m.RetryDelay != want {
						t.Errorf("RetryDelay = %v, fixture %v", m.RetryDelay, want)
					}
				}
				if d, ok := e["details"]; ok && !reflect.DeepEqual(jsonOf(t, m.Details), d) {
					t.Errorf("Details = %v, fixture %v", m.Details, d)
				}
			case string:
				// legacy flat form: {"error": "<code>", "message"?: "text"}
				want := e
				if msg, _ := body["message"].(string); msg != "" {
					want = msg
				}
				if m.Message != want {
					t.Errorf("Message = %q, fixture %q", m.Message, want)
				}
			default:
				t.Errorf("fixture error body has unexpected shape: %v", body)
			}
			if h := fx.Headers["Retry-After"]; h != "" {
				if d, ok := miosa.RetryAfter(err); !ok || fmt.Sprintf("%d", int(d.Seconds())) != h {
					t.Errorf("RetryAfter = %v, header %s", d, h)
				}
			}
		})
	}
	if count == 0 {
		t.Fatal("manifest lists no error fixtures")
	}
}

// assertTypedError checks the codes that have a dedicated Go type map to it.
func assertTypedError(t *testing.T, err error, code string) {
	t.Helper()
	switch code {
	case miosa.CodeSandboxStarting, miosa.CodeComputerStarting:
		var e *miosa.ResourceStartingError
		if !errors.As(err, &e) || !miosa.IsStarting(err) || !e.Retryable {
			t.Errorf("%s must be a retryable *ResourceStartingError, got %T", code, err)
		}
	case miosa.CodeSnapshotInUse:
		var e *miosa.SnapshotInUseError
		if !errors.As(err, &e) || len(e.Dependents) == 0 {
			t.Errorf("%s must be a *SnapshotInUseError with dependents, got %T", code, err)
		}
	case miosa.CodeEnvironmentMemoryWithheld:
		var e *miosa.EnvironmentMemoryWithheldError
		if !errors.As(err, &e) || len(e.Reasons) == 0 {
			t.Errorf("%s must be a *EnvironmentMemoryWithheldError with reasons, got %T", code, err)
		}
	case miosa.CodeEnvironmentScrubFailed:
		var e *miosa.EnvironmentScrubFailedError
		if !errors.As(err, &e) || !e.Retryable {
			t.Errorf("%s must be a retryable *EnvironmentScrubFailedError, got %T", code, err)
		}
	}
}

func TestSprintBulkDeleteAllBlockedFixture(t *testing.T) {
	sprintRoot(t)
	fx := loadSprint(t, "fixtures/snapshots/bulk-delete-all-blocked-409.yaml")
	client, _ := serveFixture(t, fx, false)

	res, err := client.Snapshots.DeleteMany(bg(), miosa.DeleteSnapshotsInput{IDs: []string{"f3c10000-0000-4000-8000-000000000001"}})

	if err != nil {
		t.Fatalf("a fully blocked bulk delete is a result, not an error: %v", err)
	}
	if len(res.Blocked) != 1 || res.Blocked[0].Dependents[0].Label != "web-stack" {
		t.Errorf("result = %+v", res)
	}
}

// ─── Coverage guard ──────────────────────────────────────────────────────────

// TestSprintManifestIsFullyCovered fails when the manifest gains a fixture no
// test here exercises, so new fixtures cannot be forgotten.
func TestSprintManifestIsFullyCovered(t *testing.T) {
	root := sprintRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var m sprintManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{
		// Not an HTTP exchange: the WSS handshake of the sandbox port tunnel.
		"fixtures/platform/tunnel-websocket-contract.yaml": true,
		// Served by TestSprintTreeWarmingFixture.
		"fixtures/snapshots/tree-warming-202.yaml": true,
		// Served by TestPlatformKitWebhookSignatureVector.
		"fixtures/platform-kit/webhook-signature-vector.yaml": true,
	}
	for _, c := range responseCases {
		covered[c.file] = true
	}
	for _, c := range requestCases {
		covered[c.file] = true
	}
	var missing []string
	for _, f := range m.Fixtures {
		if f.Kind == "error" {
			continue // every error fixture runs in TestSprintErrorFixtures
		}
		if !covered[f.File] {
			missing = append(missing, f.File)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("fixtures without a test:\n  %s", strings.Join(missing, "\n  "))
	}
}
