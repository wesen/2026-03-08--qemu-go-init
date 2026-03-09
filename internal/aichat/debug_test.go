package aichat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugSnapshotIncludesRawRegistryAndResolvedProvider(t *testing.T) {
	sharedRoot := makeSharedPinocchioRuntime(t, "https://api.openai.com/v1")

	snapshot, err := DebugSnapshot(context.Background(), Options{
		StateRoot: filepath.Join(sharedRoot, "bbs"),
	})
	if err != nil {
		t.Fatalf("debug snapshot: %v", err)
	}

	if !snapshot.ConfigFile.Exists {
		t.Fatalf("config file should exist")
	}
	if !strings.Contains(snapshot.ConfigFile.Raw, "test-openai-key") {
		t.Fatalf("config file raw should include api key, got %q", snapshot.ConfigFile.Raw)
	}
	if len(snapshot.RegistrySources) != 1 {
		t.Fatalf("registry source count = %d, want 1", len(snapshot.RegistrySources))
	}
	if !strings.Contains(snapshot.RegistrySources[0].File.Raw, "gpt-5-nano") {
		t.Fatalf("registry raw should include profile slug, got %q", snapshot.RegistrySources[0].File.Raw)
	}
	if got, want := snapshot.Provider.APIType, "openai-responses"; got != want {
		t.Fatalf("provider api type = %q, want %q", got, want)
	}
	if got, want := snapshot.Provider.SelectedAPIKey, "test-openai-key"; got != want {
		t.Fatalf("selected api key = %q, want %q", got, want)
	}
	if got, want := snapshot.Provider.HTTPSProbeURL, "https://api.openai.com/v1/models"; got != want {
		t.Fatalf("https probe url = %q, want %q", got, want)
	}
}

func TestProbeProviderHTTPSUsesResolvedSettings(t *testing.T) {
	var authorization string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		if got, want := r.URL.Path, "/v1/models"; got != want {
			t.Errorf("request path = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5-nano"}]}`))
	}))
	defer server.Close()

	sharedRoot := makeSharedPinocchioRuntime(t, server.URL+"/v1")
	details, err := loadRuntimeDetails(context.Background(), Options{
		StateRoot: filepath.Join(sharedRoot, "bbs"),
	})
	if err != nil {
		t.Fatalf("load runtime details: %v", err)
	}

	provider := providerDebug(details.resolved.EffectiveStepSettings)
	result, err := probeProviderHTTPSWithClient(context.Background(), provider, server.Client())
	if err != nil {
		t.Fatalf("probe provider https: %v", err)
	}

	if got, want := authorization, "Bearer test-openai-key"; got != want {
		t.Fatalf("authorization = %q, want %q", got, want)
	}
	if got, want := result.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
	if !strings.Contains(result.BodyPreview, "gpt-5-nano") {
		t.Fatalf("body preview should include model id, got %q", result.BodyPreview)
	}
	if len(result.Trace) == 0 {
		t.Fatalf("trace should not be empty")
	}
}

func makeSharedPinocchioRuntime(t *testing.T, baseURL string) string {
	t.Helper()
	t.Setenv("GO_INIT_PINOCCHIO_CONFIG_HOME", "")
	t.Setenv("PINOCCHIO_PROFILE_REGISTRIES", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	root := t.TempDir()
	sharedRoot := filepath.Join(root, "shared-state")
	mkdirAll(t, filepath.Join(sharedRoot, "bbs"))
	mkdirAll(t, filepath.Join(sharedRoot, "pinocchio"))

	writeFile(t, filepath.Join(sharedRoot, "pinocchio", "config.yaml"), ""+
		"openai-chat:\n"+
		"  openai-api-key: test-openai-key\n"+
		"  openai-base-url: "+baseURL+"\n")
	writeFile(t, filepath.Join(sharedRoot, "pinocchio", "profiles.yaml"), ""+
		"slug: default\n"+
		"profiles:\n"+
		"  gpt-5-nano:\n"+
		"    slug: gpt-5-nano\n"+
		"    runtime:\n"+
		"      step_settings_patch:\n"+
		"        ai-chat:\n"+
		"          ai-api-type: openai-responses\n"+
		"          ai-engine: gpt-5-nano\n")
	return sharedRoot
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
