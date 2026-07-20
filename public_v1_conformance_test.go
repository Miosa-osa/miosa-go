package miosa_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	miosa "github.com/Miosa-osa/miosa-go/v2"
	"gopkg.in/yaml.v3"
)

type conformanceFixture struct {
	Path   string                 `yaml:"path"`
	Method string                 `yaml:"method"`
	Body   map[string]interface{} `yaml:"body"`
}

const expectedContractVersion = "1.0.0"
const expectedContractCommit = "774abcbc97380b599009759632691dc60d8e6b38"

func conformanceRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("MIOSA_API_CONTRACTS_ROOT")
	configured := root != ""
	if root == "" {
		root = filepath.Join("..", "contract-fixtures", "public-v1")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("resolve contracts root %q: %v", root, err)
	}
	contractPath := filepath.Join(root, "openapi", "public-v1.yaml")
	if !configured {
		contractPath = filepath.Join(root, "CONTRACT_VERSION")
	}
	data, err := os.ReadFile(contractPath)
	if err == nil {
		actualVersion := strings.TrimSpace(string(data))
		if configured {
			var contract struct {
				Info struct {
					Version string `yaml:"version"`
				} `yaml:"info"`
			}
			err = yaml.Unmarshal(data, &contract)
			actualVersion = contract.Info.Version
		}
		if err == nil && actualVersion != expectedContractVersion {
			err = fmt.Errorf("found OpenAPI version %q", actualVersion)
		}
		if err == nil && !configured {
			commit, readErr := os.ReadFile(filepath.Join(root, "CONTRACT_COMMIT"))
			if readErr != nil {
				err = readErr
			} else if strings.TrimSpace(string(commit)) != expectedContractCommit {
				err = fmt.Errorf("found contract commit %q", strings.TrimSpace(string(commit)))
			}
		}
	}
	if err != nil {
		t.Fatalf("MIOSA API contracts unavailable or incompatible: resolved root=%s; expected public-v1 version=%s; expected commit=%s: %v", root, expectedContractVersion, expectedContractCommit, err)
	}
	return root
}

func loadConformanceFixture(t *testing.T, name string) conformanceFixture {
	t.Helper()
	root := conformanceRoot(t)
	path := filepath.Join(root, "fixtures", "conformance", name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot load conformance fixture %s; resolved root=%s; expected public-v1 version=%s; expected commit=%s: %v", path, root, expectedContractVersion, expectedContractCommit, err)
	}
	var fixture conformanceFixture
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestPublicV1ConformanceFixtures(t *testing.T) {
	create := loadConformanceFixture(t, "create-default-small-request")
	sandboxFixture := loadConformanceFixture(t, "sandbox-response")
	usageFixture := loadConformanceFixture(t, "usage-response")
	templatesFixture := loadConformanceFixture(t, "templates-response")

	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		expectedJSON, err := json.Marshal(create.Body)
		if err != nil {
			t.Fatal(err)
		}
		var expected map[string]interface{}
		if err := json.Unmarshal(expectedJSON, &expected); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(body, expected) {
			t.Fatalf("create request does not match fixture: %#v", body)
		}
		writeJSON(w, http.StatusCreated, sandboxFixture.Body)
	})
	id := sandboxFixture.Body["id"].(string)
	mux.HandleFunc("/sandboxes/"+id+"/usage", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, usageFixture.Body)
	})
	mux.HandleFunc("/templates", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, templatesFixture.Body)
	})

	client := newTestClient(t, mux)
	sandbox, err := client.Sandboxes.Create(context.Background(), miosa.CreateSandboxInput{})
	if err != nil {
		t.Fatal(err)
	}
	if sandbox.ResourceContract == nil || sandbox.ResourceContract.ID != "sandbox/small@v1" || sandbox.TimeoutRemainingMS == nil {
		t.Fatalf("sandbox fixture fields were not decoded: %#v", sandbox)
	}
	usage, err := client.Sandboxes.Usage(context.Background(), id)
	if err != nil || usage.ProvisionedVCPUMS != 94_000 {
		t.Fatalf("usage fixture mismatch: %#v %v", usage, err)
	}
	catalog, err := client.Templates.Catalog(context.Background())
	if err != nil || len(catalog.Templates) != 1 || catalog.Templates[0].DefaultSize != "small" {
		t.Fatalf("templates fixture mismatch: %#v %v", catalog, err)
	}
}
