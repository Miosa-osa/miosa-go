package miosa_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	miosa "github.com/Miosa-osa/miosa-go/v2"
)

// These examples are compiled by `go test` but never run (they have no Output
// comment): they keep the README snippets honest.

func ExampleEnvironmentsService() {
	ctx := context.Background()
	client := miosa.NewClient("msk_u_...")

	env, _ := client.Environments.Create(ctx, miosa.CreateEnvironmentInput{
		Name:         "backend",
		Variables:    map[string]string{"STRIPE_KEY": "sk_live_..."},
		Repositories: []miosa.EnvironmentRepositoryInput{{Repo: "octocat/hello-world", SetupScript: "npm ci"}},
		CLITools:     map[string]string{"gh": "latest"},
	})

	// One atomic PATCH: one request, exactly one new version.
	_, _ = client.Environments.Update(ctx, env.ID, miosa.UpdateEnvironmentInput{
		VariablesSet:   map[string]string{"REGION": "us-east-1"},
		VariablesUnset: []string{"OLD_VAR"},
	})

	sbx, _ := client.Sandboxes.Create(ctx, miosa.CreateSandboxInput{Environment: "backend"})
	fmt.Println(sbx.Environment, sbx.EnvironmentVersion, sbx.EnvironmentStatus)

	result, _ := client.Environments.Upgrade(ctx, env.ID)
	fmt.Println(result.Machines)
}

func ExampleSandboxesService_RunCommand() {
	ctx := context.Background()
	client := miosa.NewClient("msk_u_...")

	sbx, _ := client.Sandboxes.Create(ctx, miosa.CreateSandboxInput{
		SetupFile: "#!/bin/bash\nnpm install || miosa-queue-prompt 'npm install failed'\n",
	})
	sbx, _ = client.Sandboxes.WaitForSetup(ctx, sbx.ID, miosa.WaitForSetupOptions{})
	fmt.Println(sbx.SetupStatus, sbx.SetupError)

	res, _ := client.Sandboxes.RunCommand(ctx, sbx.ID, miosa.CommandInput{
		Command:        "npm test",
		Cwd:            "my-repo",
		TimeoutSeconds: 120,
		WaitUntilReady: time.Minute,
	})
	fmt.Println(res.ExitCode, res.Stdout, res.DurationMS, res.TimedOut)

	_, _ = client.Sandboxes.QueueAgentPrompt(ctx, sbx.ID, "fix the failing test")
	_, _ = client.Sandboxes.StopAgent(ctx, sbx.ID)

	created, _ := client.Sandboxes.CreateDetailed(ctx, miosa.CreateSandboxInput{WaitSeconds: 45})
	fmt.Println(created.WaitOutcome, created.BilledOrganizationID)
}

func ExampleAccountSnapshotsService() {
	ctx := context.Background()
	client := miosa.NewClient("msk_u_...")

	snaps, _ := client.Snapshots.List(ctx, miosa.ListSnapshotsOptions{ResourceID: "sbx_1"})
	tree, _ := client.Snapshots.Tree(ctx, snaps[0].ID, "/home/user", miosa.BrowseOptions{})
	fmt.Println(tree.Entries)
	_, _ = client.Snapshots.Download(ctx, snaps[0].ID, "/home/user/notes.txt", os.Stdout, miosa.BrowseOptions{})

	fork, _ := client.Snapshots.Fork(ctx, snaps[0].ID, miosa.ForkSnapshotInput{})
	fmt.Println(fork.MachineType, fork.ID, fork.State)

	err := client.Snapshots.Delete(ctx, snaps[0].ID)
	var inUse *miosa.SnapshotInUseError
	if errors.As(err, &inUse) {
		fmt.Println(inUse.Dependents)
	}

	_, _ = client.NamedSnapshots.Save(ctx, miosa.SaveNamedSnapshotInput{Name: "web-stack", SandboxID: "sbx_1"})
	_, _ = client.NamedSnapshots.Deploy(ctx, "web-stack", miosa.ForkSnapshotInput{})
	out, _ := client.NamedSnapshots.Remove(ctx, "web-stack")
	fmt.Println(out.Outcome)
}

func ExampleBillToService() {
	ctx := context.Background()
	client := miosa.NewClient("msk_u_...")

	b, _ := client.BillTo.Get(ctx)
	fmt.Println(b.Billing.Name)
	_, _ = client.BillTo.Set(ctx, "acme")
	l, _ := client.BillTo.Limits(ctx)
	if !l.CanStart {
		fmt.Println(l.BlockedReasons)
	}

	_, _ = client.Sandboxes.Create(miosa.WithBillToContext(ctx, "acme"), miosa.CreateSandboxInput{})
	client = miosa.NewClient("msk_u_...", miosa.WithBillTo("acme"))

	limit := int64(500)
	_, _ = client.MemberCaps.Set(ctx, "tenant", "user", miosa.MemberCapsInput{UsageCapCents: &limit, ClearMaxConcurrentSandboxes: true})
}

func ExampleAgentsService() {
	ctx := context.Background()
	client := miosa.NewClient("msk_u_...")

	s, _ := client.Agents.Settings(ctx, "")
	fmt.Println(s.DefaultHarness)
	models, _ := client.Agents.Models(ctx, "claude-code", "")
	fmt.Println(len(models.Models))
	_, _ = client.Agents.PutCredential(ctx, "openrouter", miosa.PutAgentCredentialInput{Fields: map[string]string{"api_key": "sk-or-..."}})
	model := "claude-sonnet-5-5"
	_, _ = client.Agents.UpdateHarness(ctx, "pi", miosa.UpdateAgentHarnessInput{DefaultModel: &model})

	sess, _ := client.AgentAccounts.StartSignin(ctx, miosa.StartSigninInput{Provider: miosa.SigninKimiCode})
	fmt.Println(sess.VerificationURL, sess.UserCode)
	sess, _ = client.AgentAccounts.GetSignin(ctx, sess.ID)

	conns, _ := client.Connections.List(ctx, miosa.ListConnectionsOptions{Family: "models"})
	fmt.Println(conns.Total)

	run, _ := client.Runs.Run(ctx, miosa.RunCreateInput{Instruction: "fix the build", ChatID: "chat-1", Target: "new", WorkspaceID: "ws"})
	fmt.Println(run.ID)

	cc, _ := client.Agents.ChatContext(ctx, "chat-1")
	if cc.Compactable {
		_, _ = client.Agents.CompactChat(ctx, "chat-1", "keep the schema decisions")
	}
	usage, _ := client.Runs.Usage(ctx, miosa.RunUsageOptions{GroupBy: []miosa.RunUsageGroupBy{miosa.RunUsageByAgent, miosa.RunUsageByDay}})
	fmt.Println(usage.Totals.EstimatedUSD)
}

func ExampleClient_Whoami() {
	ctx := context.Background()
	client := miosa.NewClient("msk_u_...")

	who, _ := client.Whoami(ctx)
	fmt.Println(who.Organization.Name, who.Auth.Scopes)

	presets, _ := client.ApiKeys.Presets(ctx)
	fmt.Println(len(presets))
	key, _ := client.ApiKeys.CreateFromPreset(ctx, "ci", miosa.CreateApiKeyInput{Name: "ci", ExpiresInDays: 90})
	rotated, _ := client.ApiKeys.Rotate(ctx, key.ID, "")
	fmt.Println(rotated.Key)

	cert, _ := client.Sandboxes.IssueSSHCertificate(ctx, "sbx_1", miosa.SSHCertificateInput{PublicKey: "ssh-ed25519 AAAA user@host"})
	fmt.Println(cert.TunnelURL, cert.Principal)

	policies, _ := client.AIGateway.ListPolicies(ctx)
	traces, _ := client.AIGateway.Requests(ctx, miosa.GatewayRequestsOptions{Status: "error", Limit: 50})
	fmt.Println(len(policies), len(traces.Data))

	started, _ := client.OpenComputers.Jobs.Start(ctx, "host_1", miosa.StartJobInput{Command: "make test", TimeoutSeconds: 300})
	fmt.Println(started.Job.ID)
}

func ExampleAsMiosaError() {
	client := miosa.NewClient("msk_u_...")
	_, err := client.Sandboxes.RunCommand(context.Background(), "sbx_1", miosa.CommandInput{Command: "ls"})

	if miosa.IsStarting(err) {
		if wait, ok := miosa.RetryAfter(err); ok {
			time.Sleep(wait)
		}
	}
	if m := miosa.AsMiosaError(err); m != nil {
		fmt.Println(m.StatusCode, m.Code, m.Retryable, m.Details)
	}
	if miosa.IsCode(err, miosa.CodeSnapshotInUse) {
		fmt.Println("snapshot has dependents")
	}
}
