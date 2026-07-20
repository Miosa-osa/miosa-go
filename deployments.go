package miosa

// Deployments — sandbox→production publishing surface.
//
// Backend phase status (2026-05-15):
//   - List / Get / ListBuilds / GetBuild / Env: pre-existing repo flow.
//   - Publish / Versions / Rollback / Domains: Phase 2B/3 target.
//   - PublishFromSandbox: backward-compat bridge that works today.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
)

// DeploymentsService provides CRUD + publish/rollback for Deployments.
type DeploymentsService struct {
	client *Client
}

// ─── Inputs ───────────────────────────────────────────────────────────────

// ListDeploymentsInput is optional query for GET /deployments.
type ListDeploymentsInput struct {
	ProjectID           string
	State               string
	Limit               int
	Cursor              string
	ExternalWorkspaceID string
	ExternalUserID      string
	ExternalProjectID   string
}

// CreateDeploymentInput is the request body for POST /deployments
// (or /projects/:id/deployments).
type CreateDeploymentInput struct {
	Name                string                 `json:"name"`
	ProjectID           string                 `json:"-"`
	SourceType          DeploymentSourceType   `json:"source_type,omitempty"`
	RepoURL             string                 `json:"repo_url,omitempty"`
	Branch              string                 `json:"branch,omitempty"`
	BuildCommand        string                 `json:"build_command,omitempty"`
	RunCommand          string                 `json:"run_command,omitempty"`
	AutoDeploy          *bool                  `json:"auto_deploy,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	ExternalWorkspaceID string                 `json:"external_workspace_id,omitempty"`
	ExternalUserID      string                 `json:"external_user_id,omitempty"`
	ExternalProjectID   string                 `json:"external_project_id,omitempty"`
	IdempotencyKey      string                 `json:"-"`
}

// UpdateDeploymentInput patches a deployment.
type UpdateDeploymentInput struct {
	Name                string                 `json:"name,omitempty"`
	AutoDeploy          *bool                  `json:"auto_deploy,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	ExternalWorkspaceID string                 `json:"external_workspace_id,omitempty"`
	ExternalUserID      string                 `json:"external_user_id,omitempty"`
	ExternalProjectID   string                 `json:"external_project_id,omitempty"`
}

// PublishInput is the request body for POST /deployments/:id/publish.
type PublishInput struct {
	SourceSandboxID     string   `json:"source_sandbox_id"`
	Kind                string   `json:"kind,omitempty"` // "auto" | "static" | "dynamic"
	Environment         string   `json:"environment,omitempty"`
	OutputPath          string   `json:"output_path,omitempty"`
	BuildCommand        string   `json:"build_command,omitempty"`
	RunCommand          string   `json:"run_command,omitempty"`
	Port                int      `json:"port,omitempty"`
	HealthCheckPath     string   `json:"health_check_path,omitempty"`
	DataServices        []string `json:"data_services,omitempty"`
	ExternalWorkspaceID string   `json:"external_workspace_id,omitempty"`
	ExternalUserID      string   `json:"external_user_id,omitempty"`
	ExternalProjectID   string   `json:"external_project_id,omitempty"`
	IdempotencyKey      string   `json:"-"`
}

// RollbackInput targets an older ready version. Omit VersionID to default
// to the previous one.
type RollbackInput struct {
	VersionID      string `json:"version_id,omitempty"`
	IdempotencyKey string `json:"-"`
}

// ListVersionsInput is the query for GET /deployments/:id/versions.
type ListVersionsInput struct {
	State               string
	Limit               int
	Cursor              string
	ExternalWorkspaceID string
	ExternalUserID      string
	ExternalProjectID   string
}

// AddDomainInput is the request body for POST /deployments/:id/domains.
type AddDomainInput struct {
	Domain              string `json:"domain"`
	RedirectPolicy      string `json:"redirect_policy,omitempty"` // "none" | "www_to_apex" | "apex_to_www"
	ExternalWorkspaceID string `json:"external_workspace_id,omitempty"`
	ExternalUserID      string `json:"external_user_id,omitempty"`
	ExternalProjectID   string `json:"external_project_id,omitempty"`
	IdempotencyKey      string `json:"-"`
}

// DeploymentProofCheck is one read-only assertion about a deployment.
type DeploymentProofCheck struct {
	ID       string                 `json:"id"`
	OK       bool                   `json:"ok"`
	Message  string                 `json:"message"`
	Details  map[string]interface{} `json:"details,omitempty"`
	Recovery []string               `json:"recovery,omitempty"`
}

// DeploymentProofResult summarizes whether a deployment is really live.
type DeploymentProofResult struct {
	OK                bool                   `json:"ok"`
	Deployment        DeploymentData         `json:"deployment"`
	DeploymentProduct DeploymentProduct      `json:"deployment_product"`
	PublicURL         string                 `json:"public_url,omitempty"`
	Checks            []DeploymentProofCheck `json:"checks"`
	NextActions       []string               `json:"next_actions,omitempty"`
}

// ─── List wrappers ───────────────────────────────────────────────────────

// DeploymentListResponse wraps GET /deployments.
type DeploymentListResponse struct {
	Data []DeploymentData `json:"data"`
}

// DeploymentVersionListResponse wraps GET /deployments/:id/versions.
type DeploymentVersionListResponse struct {
	Data []DeploymentVersionData `json:"data"`
}

// DeploymentBuildListResponse wraps GET /deployments/:id/builds.
type DeploymentBuildListResponse struct {
	Data []DeploymentBuildData `json:"data"`
}

// ─── Top-level methods ───────────────────────────────────────────────────

// List returns deployments for the authenticated tenant.
func (s *DeploymentsService) List(ctx context.Context, input ListDeploymentsInput) (*DeploymentListResponse, error) {
	params := map[string]string{}
	if input.ProjectID != "" {
		params["project_id"] = input.ProjectID
	}
	if input.State != "" {
		params["state"] = input.State
	}
	if input.Limit > 0 {
		params["limit"] = strconv.Itoa(input.Limit)
	}
	if input.Cursor != "" {
		params["cursor"] = input.Cursor
	}
	addAttrParams(params, input.ExternalWorkspaceID, input.ExternalUserID, input.ExternalProjectID)
	var out DeploymentListResponse
	if err := s.client.getJSON(ctx, "/deployments"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single deployment by ID.
func (s *DeploymentsService) Get(ctx context.Context, id string) (*DeploymentData, error) {
	var env apiResponse[DeploymentData]
	if err := s.client.getJSON(ctx, "/deployments/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Prove fetches the deployment and validates the concrete runtime facts.
func (s *DeploymentsService) Prove(ctx context.Context, deploymentID string) (*DeploymentProofResult, error) {
	deployment, err := s.Get(ctx, deploymentID)
	if err != nil {
		return nil, err
	}

	checks := []DeploymentProofCheck{}
	addDeploymentProofChecks(&checks, deployment)

	if deploymentProduct(deployment) == DeploymentProductDockerDeploy {
		var host *DockerDeployHostData
		hostID := dockerDeployHostID(deployment)
		if hostID != "" {
			var out dockerDeployHostResponse
			if err := s.client.getJSON(ctx, "/docker-deploy/hosts/"+hostID, &out); err != nil {
				return nil, err
			}
			host = unwrapDockerDeployHost(out)
		}
		addDockerDeployProofChecks(&checks, deployment, host)
	}

	nextActions := []string{}
	for _, check := range checks {
		if check.OK {
			continue
		}
		for _, action := range check.Recovery {
			if !containsString(nextActions, action) {
				nextActions = append(nextActions, action)
			}
		}
	}

	return &DeploymentProofResult{
		OK:                allProofChecksOK(checks),
		Deployment:        *deployment,
		DeploymentProduct: deploymentProduct(deployment),
		PublicURL:         deployment.PublicURL,
		Checks:            checks,
		NextActions:       nextActions,
	}, nil
}

// Create provisions a new deployment.
func (s *DeploymentsService) Create(ctx context.Context, input CreateDeploymentInput) (*DeploymentData, error) {
	path := "/deployments"
	if input.ProjectID != "" {
		path = "/projects/" + input.ProjectID + "/deployments"
	}
	var env apiResponse[DeploymentData]
	if err := s.client.postJSONIdempotent(ctx, path, input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// CreateDockerDeploy creates a deployment marked for the workspace App Engine appliance.
func (s *DeploymentsService) CreateDockerDeploy(ctx context.Context, input CreateDeploymentInput) (*DeploymentData, error) {
	input.Metadata = dockerDeployMetadata(input.Metadata)
	return s.Create(ctx, input)
}

// Update patches a deployment.
func (s *DeploymentsService) Update(ctx context.Context, id string, input UpdateDeploymentInput) (*DeploymentData, error) {
	var env apiResponse[DeploymentData]
	if err := s.client.patchJSON(ctx, "/deployments/"+id, input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Delete tears down a deployment.
func (s *DeploymentsService) Delete(ctx context.Context, id string) error {
	return s.client.deleteJSON(ctx, "/deployments/"+id, nil)
}

// Publish freezes a source snapshot from a sandbox, runs the build, and
// promotes the result. Phase 2B/3 target endpoint.
func (s *DeploymentsService) Publish(ctx context.Context, deploymentID string, input PublishInput) (*PublishResult, error) {
	if input.Kind == "" {
		input.Kind = "auto"
	}
	if input.Environment == "" {
		input.Environment = "production"
	}
	var env apiResponse[PublishResult]
	if err := s.client.postJSONIdempotent(ctx, "/deployments/"+deploymentID+"/publish", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// PublishFromSandbox uses the backward-compat bridge POST /sandboxes/:id/deploy.
// Works today. Prefer Publish once the release pipeline lands.
func (s *DeploymentsService) PublishFromSandbox(ctx context.Context, sandboxID string, input PublishInput) (map[string]interface{}, error) {
	if input.Kind == "" {
		input.Kind = "auto"
	}
	if input.Environment == "" {
		input.Environment = "production"
	}
	input.SourceSandboxID = sandboxID
	var out map[string]interface{}
	if err := s.client.postJSONIdempotent(ctx, "/sandboxes/"+sandboxID+"/deploy", input, &out, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return out, nil
}

// PublishFromSandboxDocker publishes a sandbox through App Engine.
func (s *DeploymentsService) PublishFromSandboxDocker(ctx context.Context, sandboxID string, input PublishInput) (map[string]interface{}, error) {
	input.Kind = firstNonEmpty(input.Kind, "auto")
	if input.Environment == "" {
		input.Environment = "production"
	}
	body := map[string]interface{}{
		"source_sandbox_id":     sandboxID,
		"kind":                  input.Kind,
		"environment":           input.Environment,
		"output_path":           input.OutputPath,
		"build_command":         input.BuildCommand,
		"run_command":           input.RunCommand,
		"port":                  input.Port,
		"health_check_path":     input.HealthCheckPath,
		"data_services":         input.DataServices,
		"external_workspace_id": input.ExternalWorkspaceID,
		"external_user_id":      input.ExternalUserID,
		"external_project_id":   input.ExternalProjectID,
		"deployment_type":       "docker_deploy",
	}
	var out map[string]interface{}
	if err := s.client.postJSONIdempotent(ctx, "/sandboxes/"+sandboxID+"/deploy", compactMap(body), &out, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return out, nil
}

// Rollback repoints a deployment at an older ready version.
func (s *DeploymentsService) Rollback(ctx context.Context, deploymentID string, input RollbackInput) (*DeploymentData, error) {
	var env apiResponse[DeploymentData]
	if err := s.client.postJSONIdempotent(ctx, "/deployments/"+deploymentID+"/rollback", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// ListBuilds lists historical builds for a deployment (repo-based flow).
func (s *DeploymentsService) ListBuilds(ctx context.Context, deploymentID string) (*DeploymentBuildListResponse, error) {
	var out DeploymentBuildListResponse
	if err := s.client.getJSON(ctx, "/deployments/"+deploymentID+"/builds", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBuild fetches a single build.
func (s *DeploymentsService) GetBuild(ctx context.Context, deploymentID, buildID string) (*DeploymentBuildData, error) {
	var env apiResponse[DeploymentBuildData]
	if err := s.client.getJSON(ctx, "/deployments/"+deploymentID+"/builds/"+buildID, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// ─── Versions sub-resource ───────────────────────────────────────────────

// Versions returns a sub-resource handle for the deployment's versions.
func (s *DeploymentsService) Versions(deploymentID string) *DeploymentVersionsService {
	return &DeploymentVersionsService{client: s.client, deploymentID: deploymentID}
}

// DeploymentVersionsService is scoped to one deployment.
type DeploymentVersionsService struct {
	client       *Client
	deploymentID string
}

// List returns versions for a deployment.
func (s *DeploymentVersionsService) List(ctx context.Context, input ListVersionsInput) (*DeploymentVersionListResponse, error) {
	params := map[string]string{}
	if input.State != "" {
		params["state"] = input.State
	}
	if input.Limit > 0 {
		params["limit"] = strconv.Itoa(input.Limit)
	}
	if input.Cursor != "" {
		params["cursor"] = input.Cursor
	}
	addAttrParams(params, input.ExternalWorkspaceID, input.ExternalUserID, input.ExternalProjectID)
	var out DeploymentVersionListResponse
	if err := s.client.getJSON(ctx, "/deployments/"+s.deploymentID+"/versions"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single version.
func (s *DeploymentVersionsService) Get(ctx context.Context, versionID string) (*DeploymentVersionData, error) {
	var env apiResponse[DeploymentVersionData]
	if err := s.client.getJSON(ctx, "/deployments/"+s.deploymentID+"/versions/"+versionID, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// Promote makes a specific version the active one for the deployment.
type PromoteInput struct {
	Environment    string `json:"environment,omitempty"`
	IdempotencyKey string `json:"-"`
}

// Promote sets active_version_id.
func (s *DeploymentVersionsService) Promote(ctx context.Context, versionID string, input PromoteInput) (*DeploymentData, error) {
	var env apiResponse[DeploymentData]
	if err := s.client.postJSONIdempotent(ctx, "/deployments/"+s.deploymentID+"/versions/"+versionID+"/promote", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// ─── Domains sub-resource ────────────────────────────────────────────────

// Domains returns a sub-resource handle for the deployment's custom domains.
func (s *DeploymentsService) Domains(deploymentID string) *DeploymentDomainsService {
	return &DeploymentDomainsService{client: s.client, deploymentID: deploymentID}
}

// DeploymentDomainsService is scoped to one deployment.
type DeploymentDomainsService struct {
	client       *Client
	deploymentID string
}

// Add attaches a custom domain to the deployment and returns DNS instructions.
func (s *DeploymentDomainsService) Add(ctx context.Context, input AddDomainInput) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSONIdempotent(ctx, "/deployments/"+s.deploymentID+"/domains", input, &out, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return out, nil
}

// List returns custom domains attached to the deployment.
func (s *DeploymentDomainsService) List(ctx context.Context) ([]map[string]interface{}, error) {
	var out struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := s.client.getJSON(ctx, "/deployments/"+s.deploymentID+"/domains", &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// Verify triggers DNS + TLS verification on a pending domain.
func (s *DeploymentDomainsService) Verify(ctx context.Context, domainID string) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := s.client.postJSON(ctx, "/deployments/"+s.deploymentID+"/domains/"+domainID+"/verify", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete detaches a custom domain.
func (s *DeploymentDomainsService) Delete(ctx context.Context, domainID string) error {
	return s.client.deleteJSON(ctx, "/deployments/"+s.deploymentID+"/domains/"+domainID, nil)
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func addAttrParams(params map[string]string, ws, user, proj string) {
	if ws != "" {
		params["external_workspace_id"] = ws
	}
	if user != "" {
		params["external_user_id"] = user
	}
	if proj != "" {
		params["external_project_id"] = proj
	}
}

func idemKey(provided string) string {
	if provided != "" {
		return provided
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Should never happen on a healthy host; fall back to empty so
		// the server generates one. The Idempotency-Key header is
		// optional and the server tolerates absence.
		return ""
	}
	return hex.EncodeToString(b[:])
}

func dockerDeployMetadata(metadata map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range metadata {
		out[k] = v
	}
	out["deployment_product"] = "docker_deploy"
	return out
}

func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func compactMap(in map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range in {
		switch value := v.(type) {
		case string:
			if value != "" {
				out[k] = value
			}
		case int:
			if value != 0 {
				out[k] = value
			}
		case []string:
			if len(value) > 0 {
				out[k] = value
			}
		case nil:
		default:
			out[k] = value
		}
	}
	return out
}

func deploymentProduct(deployment *DeploymentData) DeploymentProduct {
	if deployment.DeploymentProduct != "" {
		return deployment.DeploymentProduct
	}
	if value, ok := deployment.Metadata["deployment_product"].(string); ok && value != "" {
		return DeploymentProduct(value)
	}
	return DeploymentProductMiosaDeploy
}

func dockerDeployHostID(deployment *DeploymentData) string {
	if deployment.DockerDeployHostID != "" {
		return deployment.DockerDeployHostID
	}
	if value, ok := deployment.Metadata["docker_deploy_host_id"].(string); ok && value != "" {
		return value
	}
	if deployment.DockerDeployApp != nil {
		if deployment.DockerDeployApp.DockerDeployHostID != "" {
			return deployment.DockerDeployApp.DockerDeployHostID
		}
		if value, ok := deployment.DockerDeployApp.Metadata["host_id"].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func addProofCheck(
	checks *[]DeploymentProofCheck,
	id string,
	ok bool,
	message string,
	details map[string]interface{},
	recovery []string,
) {
	*checks = append(*checks, DeploymentProofCheck{
		ID:       id,
		OK:       ok,
		Message:  message,
		Details:  details,
		Recovery: recovery,
	})
}

func addDeploymentProofChecks(checks *[]DeploymentProofCheck, deployment *DeploymentData) {
	addProofCheck(
		checks,
		"deployment_row",
		true,
		"Deployment "+deployment.ID+" exists with state="+string(deployment.State)+".",
		map[string]interface{}{"state": deployment.State},
		nil,
	)

	running := deployment.State == DeploymentStateRunning
	message := "Deployment is marked running."
	if !running {
		message = "Deployment state is " + string(deployment.State) + ", expected running."
	}
	addProofCheck(
		checks,
		"deployment_running",
		running,
		message,
		map[string]interface{}{"state": deployment.State},
		[]string{"Inspect deployment logs.", "Redeploy the deployment."},
	)

	publicURLMessage := "Deployment has no public URL."
	if deployment.PublicURL != "" {
		publicURLMessage = "Public URL is " + deployment.PublicURL + "."
	}
	addProofCheck(
		checks,
		"public_url_present",
		deployment.PublicURL != "",
		publicURLMessage,
		map[string]interface{}{"public_url": deployment.PublicURL},
		nil,
	)
	if deployment.PublicURL == "" {
		(*checks)[len(*checks)-1].Message = "Deployment has no public URL."
	}
}

func addDockerDeployProofChecks(
	checks *[]DeploymentProofCheck,
	deployment *DeploymentData,
	host *DockerDeployHostData,
) {
	app := deployment.DockerDeployApp
	hostID := dockerDeployHostID(deployment)
	hostLinkMessage := "Deployment has no App Engine host id."
	if hostID != "" {
		hostLinkMessage = "Deployment links App Engine host " + hostID + "."
	}
	addProofCheck(
		checks,
		"docker_deploy_host_link",
		hostID != "",
		hostLinkMessage,
		map[string]interface{}{"docker_deploy_host_id": hostID},
		[]string{"Ensure the workspace App Engine appliance."},
	)

	if host != nil {
		hostOK := host.Status == DockerDeployHostActive &&
			host.ApplianceStatus == DockerDeployApplianceHealthy
		addProofCheck(
			checks,
			"docker_deploy_host_ready",
			hostOK,
			"Host status="+string(host.Status)+", appliance="+string(host.ApplianceStatus)+".",
			map[string]interface{}{
				"status":           host.Status,
				"appliance_status": host.ApplianceStatus,
			},
			[]string{"Check App Engine host health."},
		)
	}

	addProofCheck(
		checks,
		"docker_deploy_app_row",
		app != nil,
		dockerDeployAppRowMessage(app),
		dockerDeployAppRowDetails(app),
		[]string{"Publish through App Engine again."},
	)

	routeOK := app != nil &&
		app.Status == "running" &&
		app.ContainerID != "" &&
		app.RuntimeIP != "" &&
		app.RuntimePort > 0
	addProofCheck(
		checks,
		"docker_deploy_container_route",
		routeOK,
		dockerDeployContainerRouteMessage(app),
		dockerDeployContainerRouteDetails(app),
		[]string{"Run App Engine doctor.", "Check appliance container health."},
	)
}

func dockerDeployAppRowMessage(app *DockerDeployAppData) string {
	if app == nil {
		return "App Engine app row is missing."
	}
	return "App Engine app status=" + firstNonEmpty(app.Status, "unknown") + "."
}

func dockerDeployAppRowDetails(app *DockerDeployAppData) map[string]interface{} {
	if app == nil {
		return nil
	}
	return map[string]interface{}{
		"app_id":       app.AppID,
		"container_id": app.ContainerID,
		"status":       app.Status,
	}
}

func dockerDeployContainerRouteMessage(app *DockerDeployAppData) string {
	if app == nil {
		return "Cannot verify container route without App Engine app row."
	}
	port := strconv.Itoa(app.RuntimePort)
	if app.RuntimePort == 0 {
		port = "missing"
	}
	return "Container=" + firstNonEmpty(app.ContainerID, "missing") +
		", route=" + firstNonEmpty(app.RuntimeIP, "missing") + ":" + port + "."
}

func dockerDeployContainerRouteDetails(app *DockerDeployAppData) map[string]interface{} {
	if app == nil {
		return map[string]interface{}{
			"container_id": nil,
			"runtime_ip":   nil,
			"runtime_port": nil,
		}
	}
	return map[string]interface{}{
		"container_id": app.ContainerID,
		"runtime_ip":   app.RuntimeIP,
		"runtime_port": app.RuntimePort,
	}
}

func allProofChecksOK(checks []DeploymentProofCheck) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
