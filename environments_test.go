package miosa_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

var envJSON = map[string]interface{}{
	"id": "env_1", "name": "prod", "workspace_id": nil, "scope": "organization",
	"is_default": true, "effective_default": true, "safe_for_third_parties": false,
	"pass_github": true, "pass_secrets": true, "pass_sandbox_credentials": false, "pass_agents_credentials": true,
	"effective":      map[string]interface{}{"pass_github": true, "pass_secrets": true, "pass_sandbox_credentials": false, "pass_agents_credentials": true},
	"latest_version": 3, "version_count": 3, "pinned_machine_count": 5, "pinned_sandbox_count": 4,
	"pinned_computer_count": 1, "outdated_machine_count": 2, "extends_default": true,
	"variables": []map[string]interface{}{
		{"name": "STRIPE_KEY", "secret": true, "preview": "sk_liv...0123"},
		{"name": "REGION", "secret": false, "preview": "us-east-1"},
	},
	"secret_files": []map[string]interface{}{{"path": "backend/.env", "size_bytes": 120}},
	"repositories": []map[string]interface{}{{
		"source": "github", "repo": "octocat/hello-world", "base_branch": "develop",
		"setup_script": "npm ci", "setup_blocking": true, "path": "hello-world",
	}},
	"cli_tools":    map[string]interface{}{"gh": "2.60.0"},
	"connections":  []string{"mcp_server:0b7c"},
	"setup_script": "apt-get install -y jq", "setup_blocking": false,
	"metadata": map[string]interface{}{}, "created_at": "2026-10-09T12:00:00Z", "deleted_at": nil,
}

func TestEnvironmentsDecodeFullObject(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"environment": envJSON})

	env, err := client.Environments.Get(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/environments/prod")

	if env.Scope != miosa.EnvironmentScopeOrganization || !env.EffectiveDefault || env.LatestVersion != 3 {
		t.Errorf("header fields: %+v", env)
	}
	if env.PinnedSandboxCount != 4 || env.OutdatedMachineCount != 2 || !env.ExtendsDefault {
		t.Errorf("counts: %+v", env)
	}
	if len(env.Variables) != 2 || !env.Variables[0].Secret || env.Variables[1].Secret || env.Variables[0].Preview != "sk_liv...0123" {
		t.Errorf("variables: %+v", env.Variables)
	}
	if env.Repositories[0].Path != "hello-world" || !env.Repositories[0].SetupBlocking {
		t.Errorf("repositories: %+v", env.Repositories)
	}
	if env.CLITools["gh"] != "2.60.0" || env.Connections[0] != "mcp_server:0b7c" || env.SetupScript == "" {
		t.Errorf("tools/connections/setup: %+v", env)
	}
	if !env.Effective.PassAgentsCredentials || env.Effective.PassSandboxCredentials {
		t.Errorf("effective: %+v", env.Effective)
	}
}

func TestEnvironmentsListAndScope(t *testing.T) {
	body := map[string]interface{}{
		"environments":           []interface{}{envJSON},
		"default_environment_id": "env_1",
		"billing":                map[string]interface{}{"kind": "organization", "id": "org_1", "name": "Acme Inc", "wallet": nil},
	}
	client, rec := newFixedClient(t, 200, body)

	list, err := client.Environments.ForWorkspace("ws_9").ListWith(context.Background(), miosa.ListEnvironmentsOptions{IncludeDeleted: true})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/environments")
	if got.RawQuery != "include_deleted=true&workspace_id=ws_9" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if list.DefaultEnvironmentID != "env_1" || list.Billing == nil || list.Billing.Name != "Acme Inc" || list.Billing.Wallet != nil {
		t.Errorf("list: %+v", list)
	}

	def, err := client.Environments.Default(context.Background())
	if err != nil || def.ID != "env_1" {
		t.Fatalf("Default = %+v, %v", def, err)
	}
}

func TestEnvironmentsBilling(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"billing": map[string]interface{}{"kind": "organization", "id": "o", "name": "Acme"}})
	b, err := client.Environments.Billing(context.Background())
	if err != nil || b.Name != "Acme" {
		t.Fatalf("%+v %v", b, err)
	}
	assertReq(t, rec.Last(t), "GET", "/environments/billing")
}

func TestEnvironmentsCreateBody(t *testing.T) {
	yes, no := true, false
	client, rec := newFixedClient(t, 201, map[string]interface{}{"environment": envJSON})

	_, err := client.Environments.ForWorkspace("ws_1").Create(context.Background(), miosa.CreateEnvironmentInput{
		Name:                "staging",
		SafeForThirdParties: &yes,
		PassGithub:          &no,
		ExtendsDefault:      &no,
		Variables:           map[string]string{"A": "1"},
		PlainVariables:      []string{"A"},
		SecretFiles:         []miosa.EnvironmentSecretFileInput{{Path: ".env", Contents: "X=1"}},
		Repositories:        []miosa.EnvironmentRepositoryInput{{Repo: "o/r", Source: "forge", SetupBlocking: &yes}},
		CLITools:            map[string]string{"gh": "latest"},
		Connections:         []string{"mcp_server:1"},
		SetupScript:         "make",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/environments")
	body := got.Body(t)
	for k, want := range map[string]interface{}{
		"name": "staging", "workspace_id": "ws_1", "safe_for_third_parties": true,
		"pass_github": false, "extends_default": false, "setup_script": "make",
	} {
		if body[k] != want {
			t.Errorf("body[%s] = %v, want %v", k, body[k], want)
		}
	}
	if _, present := body["pass_secrets"]; present {
		t.Error("unset toggles must be omitted so the server defaults apply")
	}
	repos := body["repositories"].([]interface{})[0].(map[string]interface{})
	if repos["repo"] != "o/r" || repos["source"] != "forge" || repos["setup_blocking"] != true {
		t.Errorf("repositories = %v", repos)
	}
}

func TestEnvironmentsUpdateIsOneAtomicPatch(t *testing.T) {
	yes := true
	name := "renamed"
	script := "echo hi"
	client, rec := newFixedClient(t, 200, map[string]interface{}{"environment": envJSON})

	_, err := client.Environments.Update(context.Background(), "env_1", miosa.UpdateEnvironmentInput{
		Name:              &name,
		PassSecrets:       &yes,
		VariablesSet:      map[string]string{"A": "1"},
		VariablesUnset:    []string{"B"},
		SecretFilesSet:    []miosa.EnvironmentSecretFileInput{{Path: "a/.env", Contents: "x"}},
		SecretFilesUnset:  []string{"old/.env"},
		RepositoriesSet:   []miosa.EnvironmentRepositoryInput{{Repo: "o/new"}},
		RepositoriesUnset: []miosa.EnvironmentRepositoryRef{{Repo: "o/old"}, {Repo: "o/forge", Source: "forge"}},
		CLIToolsSet:       map[string]string{"gh": "latest"},
		CLIToolsUnset:     []string{"aws"},
		ConnectionsSet:    []string{"mcp_server:2"},
		ConnectionsUnset:  []string{"mcp_server:1"},
		SetupScript:       &script,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "PATCH", "/environments/env_1")
	body := got.Body(t)
	wantKeys := []string{"name", "pass_secrets", "variables_set", "variables_unset", "secret_files_set", "secret_files_unset",
		"repositories_set", "repositories_unset", "cli_tools_set", "cli_tools_unset", "connections_set", "connections_unset", "setup_script"}
	if len(body) != len(wantKeys) {
		t.Errorf("body has %d keys, want %d: %v", len(body), len(wantKeys), body)
	}
	for _, k := range wantKeys {
		if _, ok := body[k]; !ok {
			t.Errorf("missing %q", k)
		}
	}
	unset := body["repositories_unset"].([]interface{})
	if unset[0] != "o/old" {
		t.Errorf("a ref without a source is the bare string, got %v", unset[0])
	}
	if ref, ok := unset[1].(map[string]interface{}); !ok || ref["repo"] != "o/forge" || ref["source"] != "forge" {
		t.Errorf("a ref with a source is an object, got %v", unset[1])
	}
}

func TestEnvironmentsUpdateClearsSetupScriptWithNull(t *testing.T) {
	raw, err := json.Marshal(miosa.UpdateEnvironmentInput{ClearSetupScript: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"setup_script":null}` {
		t.Errorf("got %s", raw)
	}
	raw, _ = json.Marshal(miosa.UpdateEnvironmentInput{})
	if string(raw) != `{}` {
		t.Errorf("empty update must send nothing, got %s", raw)
	}
}

func TestEnvironmentsSimpleMutations(t *testing.T) {
	env := map[string]interface{}{"environment": envJSON}
	cases := []struct {
		name   string
		call   func(*miosa.Client) error
		method string
		path   string
		query  string
		body   map[string]interface{}
	}{
		{"rename", func(c *miosa.Client) error {
			_, err := c.Environments.Rename(context.Background(), "env_1", "prod2")
			return err
		},
			"POST", "/environments/env_1/rename", "", map[string]interface{}{"name": "prod2"}},
		{"make default", func(c *miosa.Client) error {
			_, err := c.Environments.MakeDefault(context.Background(), "env_1")
			return err
		},
			"POST", "/environments/env_1/default", "", map[string]interface{}{}},
		{"set variable", func(c *miosa.Client) error {
			_, err := c.Environments.SetVariable(context.Background(), "env_1", "STRIPE_KEY", "sk")
			return err
		}, "PUT", "/environments/env_1/variables/STRIPE_KEY", "", map[string]interface{}{"value": "sk"}},
		{"delete variable", func(c *miosa.Client) error {
			_, err := c.Environments.DeleteVariable(context.Background(), "env_1", "STRIPE_KEY")
			return err
		}, "DELETE", "/environments/env_1/variables/STRIPE_KEY", "", nil},
		{"set file", func(c *miosa.Client) error {
			_, err := c.Environments.SetSecretFile(context.Background(), "env_1", "backend/.env", "A=1")
			return err
		}, "PUT", "/environments/env_1/files", "", map[string]interface{}{"path": "backend/.env", "contents": "A=1"}},
		{"delete file", func(c *miosa.Client) error {
			_, err := c.Environments.DeleteSecretFile(context.Background(), "env_1", "backend/.env")
			return err
		}, "DELETE", "/environments/env_1/files", "path=backend%2F.env", nil},
		{"add repository", func(c *miosa.Client) error {
			_, err := c.Environments.AddRepository(context.Background(), "env_1", miosa.EnvironmentRepositoryInput{Repo: "o/r", BaseBranch: "dev", SetupScript: "npm ci"})
			return err
		}, "POST", "/environments/env_1/repositories", "", map[string]interface{}{"repo": "o/r", "base_branch": "dev", "setup_script": "npm ci"}},
		{"remove repository", func(c *miosa.Client) error {
			_, err := c.Environments.RemoveRepository(context.Background(), "env_1", "o/r")
			return err
		}, "DELETE", "/environments/env_1/repositories", "repo=o%2Fr", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, rec := newFixedClient(t, 200, env)
			if err := tc.call(client); err != nil {
				t.Fatal(err)
			}
			got := rec.Last(t)
			assertReq(t, got, tc.method, tc.path)
			if got.RawQuery != tc.query {
				t.Errorf("query = %q, want %q", got.RawQuery, tc.query)
			}
			if tc.body != nil && !reflect.DeepEqual(got.Body(t), tc.body) {
				t.Errorf("body = %v, want %v", got.Body(t), tc.body)
			}
		})
	}
}

func TestEnvironmentsScopeGoesInBodyForWritesAndQueryForReads(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"environment": envJSON})
	scoped := client.Environments.ForWorkspace("ws_1")

	if _, err := scoped.MakeDefault(context.Background(), "env_1"); err != nil {
		t.Fatal(err)
	}
	if got := rec.Last(t); got.RawQuery != "" || got.Body(t)["workspace_id"] != "ws_1" {
		t.Errorf("write: query=%q body=%v", got.RawQuery, got.Body(t))
	}
	if _, err := scoped.Get(context.Background(), "env_1"); err != nil {
		t.Fatal(err)
	}
	if got := rec.Last(t); got.RawQuery != "workspace_id=ws_1" {
		t.Errorf("read: query=%q", got.RawQuery)
	}
}

func TestEnvironmentsInheritDefault(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"default_environment_id": "org_base"})

	id, err := client.Environments.ForWorkspace("ws_1").InheritDefault(context.Background())
	if err != nil || id != "org_base" {
		t.Fatalf("%q %v", id, err)
	}
	got := rec.Last(t)
	assertReq(t, got, "DELETE", "/environments/default")
	if got.RawQuery != "workspace_id=ws_1" {
		t.Errorf("query = %q", got.RawQuery)
	}
}

func TestEnvironmentsDeleteDefaultRefusal(t *testing.T) {
	client, _ := newFixedClient(t, http.StatusConflict, errorEnvelope("ENVIRONMENT_IS_DEFAULT", "make another default first", nil))

	err := client.Environments.Delete(context.Background(), "base")

	if !miosa.IsCode(err, "ENVIRONMENT_IS_DEFAULT") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnvironmentsReveal(t *testing.T) {
	t.Run("variable", func(t *testing.T) {
		client, rec := newFixedClient(t, 200, map[string]interface{}{"name": "STRIPE_KEY", "value": "sk_live_x"})
		v, err := client.Environments.RevealVariable(context.Background(), "env_1", "STRIPE_KEY")
		if err != nil || v != "sk_live_x" {
			t.Fatalf("%q %v", v, err)
		}
		assertReq(t, rec.Last(t), "GET", "/environments/env_1/variables/STRIPE_KEY/reveal")
	})
	t.Run("file", func(t *testing.T) {
		client, rec := newFixedClient(t, 200, map[string]interface{}{"path": "a/.env", "contents": "A=1"})
		c, err := client.Environments.RevealSecretFile(context.Background(), "env_1", "a/.env")
		if err != nil || c != "A=1" {
			t.Fatalf("%q %v", c, err)
		}
		got := rec.Last(t)
		assertReq(t, got, "GET", "/environments/env_1/files/reveal")
		if got.RawQuery != "path=a%2F.env" {
			t.Errorf("query = %q", got.RawQuery)
		}
	})
}

func TestEnvironmentsVersions(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"versions": []map[string]interface{}{{
		"version": 3, "created_by": "u1", "pinned_machine_count": 2, "pinned_sandbox_count": 2,
		"pass_github": true, "variable_names": []string{"STRIPE_KEY"}, "secret_file_paths": []string{"backend/.env"},
		"repositories": []map[string]interface{}{{"repo": "o/r", "setup_blocking": false}},
	}}})

	vs, err := client.Environments.Versions(context.Background(), "env_1")
	if err != nil {
		t.Fatal(err)
	}
	assertReq(t, rec.Last(t), "GET", "/environments/env_1/versions")
	if len(vs) != 1 || vs[0].Version != 3 || vs[0].VariableNames[0] != "STRIPE_KEY" || vs[0].PinnedMachineCount != 2 {
		t.Errorf("versions: %+v", vs)
	}
}

func TestEnvironmentsEffective(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"effective": map[string]interface{}{
		"environment_id": "env_1", "version": 3,
		"layers":       []map[string]interface{}{{"source": "organization_defaults", "variables": []map[string]interface{}{{"name": "ORG_FLAG", "target": "all"}}}},
		"variables":    []map[string]interface{}{{"name": "SHARED", "source": "environment"}},
		"secret_files": []string{"backend/.env"},
		"repositories": []map[string]interface{}{{"repo": "o/r", "path": "r"}},
		"cli_tools":    map[string]string{"gh": "latest"}, "connections": []string{},
		"setup_script": nil, "setup_blocking": false,
		"effective": map[string]interface{}{"pass_github": true},
	}})

	view, err := client.Environments.Effective(context.Background(), "env_1", miosa.EffectiveOptions{AgentID: "agent_1"})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "GET", "/environments/env_1/effective")
	if got.RawQuery != "agent_id=agent_1" {
		t.Errorf("query = %q", got.RawQuery)
	}
	if view.Version != 3 || view.Layers[0].Source != "organization_defaults" || view.Layers[0].Variables[0].Target != "all" {
		t.Errorf("layers: %+v", view.Layers)
	}
	if view.Variables[0].Source != "environment" || view.SecretFiles[0] != "backend/.env" || view.SetupScript != nil {
		t.Errorf("view: %+v", view)
	}
}

func TestEnvironmentsUpgrade(t *testing.T) {
	resp := map[string]interface{}{"environment_id": "env_1", "latest_version": 3, "machines": []map[string]interface{}{
		{"kind": "sandbox", "id": "s1", "from_version": 2, "to_version": 3, "via": "environment", "status": "applying"},
		{"kind": "computer", "id": "c1", "from_version": 1, "to_version": 3, "via": "default", "status": "pending_resume"},
	}}
	client, rec := newFixedClient(t, 200, resp)

	res, err := client.Environments.Upgrade(context.Background(), "env_1", "s1", "c1")
	if err != nil {
		t.Fatal(err)
	}
	got := rec.Last(t)
	assertReq(t, got, "POST", "/environments/env_1/upgrade")
	if ids := got.Body(t)["machine_ids"].([]interface{}); len(ids) != 2 || ids[0] != "s1" {
		t.Errorf("machine_ids = %v", ids)
	}
	if res.LatestVersion != 3 || len(res.Machines) != 2 || res.Machines[1].Via != "default" || res.Machines[1].Status != "pending_resume" {
		t.Errorf("result: %+v", res)
	}

	if _, err := client.Environments.Upgrade(context.Background(), "env_1"); err != nil {
		t.Fatal(err)
	}
	if _, present := rec.Last(t).Body(t)["machine_ids"]; present {
		t.Error("no ids must mean every live machine behind: machine_ids omitted")
	}
}

func TestEnvironmentsNotFoundIsTyped(t *testing.T) {
	client, _ := newFixedClient(t, 404, errorEnvelope("ENVIRONMENT_NOT_FOUND", "no such environment", nil))

	_, err := client.Environments.Get(context.Background(), "nope")

	var nf *miosa.NotFoundError
	if !errors.As(err, &nf) || nf.Code != miosa.CodeEnvironmentNotFound {
		t.Fatalf("error = %#v", err)
	}
}

func TestEnvironmentNamesArePathEscaped(t *testing.T) {
	client, rec := newFixedClient(t, 200, map[string]interface{}{"environment": envJSON})
	if _, err := client.Environments.Get(context.Background(), "a b/c"); err != nil {
		t.Fatal(err)
	}
	if got := rec.Last(t); got.Path != "/environments/a b/c" {
		t.Errorf("decoded path = %q", got.Path)
	}
}
