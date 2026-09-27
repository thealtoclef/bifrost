package typesafe_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/internal/llmtests"
	"github.com/maximhq/bifrost/core/providers/typesafe"
	"github.com/maximhq/bifrost/core/schemas"
)

func TestTypesafe(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")) == "" {
		t.Skip("Skipping Typesafe tests because TYPESAFE_API_KEY is not set")
	}

	client, ctx, cancel, err := llmtests.SetupTest()
	if err != nil {
		t.Fatalf("Error initializing test setup: %v", err)
	}
	defer cancel()
	defer client.Shutdown()

	testConfig := llmtests.ComprehensiveTestConfig{
		Provider:      schemas.Typesafe,
		DecisionModel: "jev-1.13.0",
		Scenarios: llmtests.TestScenarios{
			// Typesafe serves the decision operation only; every other operation
			// returns the standard unsupported-operation error. ListModels is
			// served from the static catalog without an upstream call.
			Decision:   true,
			ListModels: true,
		},
	}

	t.Run("TypesafeTests", func(t *testing.T) {
		llmtests.RunAllComprehensiveTests(t, client, ctx, testConfig)
	})
}

// A custom provider reports its configured key; the account store and key
// lookup are keyed by that name.
func TestTypesafeProviderKeyResolvesCustomName(t *testing.T) {
	logger := bifrost.NewDefaultLogger(schemas.LogLevelError)

	standard, err := typesafe.NewTypesafeProvider(&schemas.ProviderConfig{}, logger)
	if err != nil {
		t.Fatalf("standard provider: %v", err)
	}
	if got := standard.GetProviderKey(); got != schemas.Typesafe {
		t.Errorf("standard provider key = %q, want %q", got, schemas.Typesafe)
	}

	custom, err := typesafe.NewTypesafeProvider(&schemas.ProviderConfig{
		CustomProviderConfig: &schemas.CustomProviderConfig{
			BaseProviderType:  schemas.Typesafe,
			CustomProviderKey: "systemone",
		},
	}, logger)
	if err != nil {
		t.Fatalf("custom provider: %v", err)
	}
	if got := custom.GetProviderKey(); got != schemas.ModelProvider("systemone") {
		t.Errorf("custom provider key = %q, want %q", got, "systemone")
	}
}

// A custom provider honours its own base_url and decisions path override.
func TestTypesafeDecisionResolvesConfiguredBaseURLAndPath(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("content-type", "application/json")
		fmt.Fprint(w, `{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.9}}}`)
	}))
	defer upstream.Close()

	cases := []struct {
		name      string
		baseURL   string
		overrides map[schemas.RequestType]string
		wantPath  string
	}{
		{
			name:     "base url carries a path prefix",
			baseURL:  upstream.URL + "/base",
			wantPath: "/base/v1/systemone",
		},
		{
			name:      "path override replaces the protocol path",
			baseURL:   upstream.URL,
			overrides: map[schemas.RequestType]string{schemas.DecisionRequest: "/override/systemone"},
			wantPath:  "/override/systemone",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := typesafe.NewTypesafeProvider(&schemas.ProviderConfig{
				NetworkConfig: schemas.NetworkConfig{BaseURL: tc.baseURL, AllowPrivateNetwork: true},
				CustomProviderConfig: &schemas.CustomProviderConfig{
					BaseProviderType:     schemas.Typesafe,
					CustomProviderKey:    "systemone",
					RequestPathOverrides: tc.overrides,
				},
			}, bifrost.NewDefaultLogger(schemas.LogLevelError))
			if err != nil {
				t.Fatalf("provider: %v", err)
			}

			ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
			_, bifrostErr := provider.Decision(ctx, schemas.Key{Value: *schemas.NewSecretVar("dummy")}, &schemas.BifrostDecisionRequest{
				Model: "jev-1.13.0",
				State: "state",
				Questions: map[string]schemas.DecisionQuestion{
					"q": {Kind: schemas.DecisionKindNoul, Instructions: "yes?"},
				},
			})
			if bifrostErr != nil {
				t.Fatalf("decision: %v", bifrostErr.Error.Message)
			}
			if gotPath != tc.wantPath {
				t.Errorf("upstream path = %q, want %q", gotPath, tc.wantPath)
			}
		})
	}
}

// The native listing carries bare model names.
func TestToTypesafeNativeListModelsResponseStripsServingProvider(t *testing.T) {
	listing := func(provider schemas.ModelProvider) *schemas.BifrostListModelsResponse {
		return &schemas.BifrostListModelsResponse{
			Data: []schemas.Model{
				{ID: string(provider) + "/jev-1.13.0"},
				{ID: string(provider) + "/jev-latest"},
			},
			ExtraFields: schemas.BifrostResponseExtraFields{Provider: provider},
		}
	}

	for _, provider := range []schemas.ModelProvider{schemas.Typesafe, "systemone"} {
		native := typesafe.ToTypesafeNativeListModelsResponse(listing(provider))
		got := make([]string, 0, len(native.Models))
		for _, model := range native.Models {
			got = append(got, model.Name)
		}
		if len(got) != 2 || got[0] != "jev-1.13.0" || got[1] != "jev-latest" {
			t.Errorf("provider %q: names = %v, want bare model names", provider, got)
		}
		if native.Models[0].Description == "" {
			t.Errorf("provider %q: description lost, so the catalog lookup missed", provider)
		}
	}

	// A response carrying no provider name still resolves through the catalog.
	unnamed := typesafe.ToTypesafeNativeListModelsResponse(&schemas.BifrostListModelsResponse{
		Data: []schemas.Model{{ID: "systemone/jev-1.13.0"}},
	})
	if len(unnamed.Models) != 1 || unnamed.Models[0].Name != "jev-1.13.0" {
		t.Errorf("unnamed response names = %v, want jev-1.13.0", unnamed.Models)
	}
}

// liveListingProvider points a typesafe provider at a fixture that answers
// GET /v1/models with body and status.
func liveListingProvider(t *testing.T, status int, body string) *typesafe.TypesafeProvider {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"detail":"not found"}`)
			return
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(upstream.Close)

	provider, err := typesafe.NewTypesafeProvider(&schemas.ProviderConfig{
		NetworkConfig: schemas.NetworkConfig{BaseURL: upstream.URL, AllowPrivateNetwork: true},
	}, bifrost.NewDefaultLogger(schemas.LogLevelError))
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	return provider
}

// The endpoint's listing wins; the pinned catalog is the fallback.
func TestListModelsPrefersTheEndpointListing(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "models shape",
			body: `{"models":[{"name":"multilingual","description":"Multilingual decision model","release_date":"2026-09-20"}]}`,
		},
		{
			name: "OpenAI-compatible data shape",
			body: `{"object":"list","data":[{"id":"jev-1.13-free","object":"model","created":1790498001}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := liveListingProvider(t, http.StatusOK, tc.body)
			ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)

			resp, bifrostErr := provider.ListModels(ctx, []schemas.Key{{Models: schemas.WhiteList{"*"}}},
				&schemas.BifrostListModelsRequest{})
			if bifrostErr != nil {
				t.Fatalf("list models: %v", bifrostErr.Error.Message)
			}
			if len(resp.Data) != 1 {
				t.Fatalf("expected the endpoint's single model, got %d entries: %+v", len(resp.Data), resp.Data)
			}
			if strings.Contains(resp.Data[0].ID, "jev-1.13.0") {
				t.Errorf("served the pinned catalog instead of the endpoint listing: %+v", resp.Data[0])
			}
		})
	}
}

// An endpoint with no listing is served the pinned catalog.
func TestListModelsFallsBackToPinnedCatalog(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusMethodNotAllowed} {
		provider := liveListingProvider(t, status, `{"detail":"no listing here"}`)
		ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)

		resp, bifrostErr := provider.ListModels(ctx, []schemas.Key{{Models: schemas.WhiteList{"*"}}},
			&schemas.BifrostListModelsRequest{})
		if bifrostErr != nil {
			t.Fatalf("status %d: %v", status, bifrostErr.Error.Message)
		}
		ids := make([]string, 0, len(resp.Data))
		for _, model := range resp.Data {
			ids = append(ids, model.ID)
		}
		if !slices.Contains(ids, "typesafe/jev-latest") {
			t.Errorf("status %d: expected the pinned catalog, got %v", status, ids)
		}
	}
}

// Other failures are returned, not absorbed.
func TestListModelsSurfacesEndpointFailures(t *testing.T) {
	provider := liveListingProvider(t, http.StatusUnauthorized, `{"detail":"Invalid API key"}`)
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)

	_, bifrostErr := provider.ListModels(ctx, []schemas.Key{{Value: *schemas.NewSecretVar("bad")}},
		&schemas.BifrostListModelsRequest{})
	if bifrostErr == nil {
		t.Fatal("expected the endpoint's error, got a listing")
	}
	if bifrostErr.StatusCode == nil || *bifrostErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %v, want 401", bifrostErr.StatusCode)
	}
}

// A listing's release date reaches the native shape.
func TestNativeListingCarriesLiveReleaseDate(t *testing.T) {
	provider := liveListingProvider(t, http.StatusOK,
		`{"models":[{"name":"multilingual","description":"Multilingual decision model","release_date":"2026-09-20"}]}`)
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)

	resp, bifrostErr := provider.ListModels(ctx, []schemas.Key{{Models: schemas.WhiteList{"*"}}},
		&schemas.BifrostListModelsRequest{})
	if bifrostErr != nil {
		t.Fatalf("list models: %v", bifrostErr.Error.Message)
	}
	resp.ExtraFields.Provider = provider.GetProviderKey()

	native := typesafe.ToTypesafeNativeListModelsResponse(resp)
	if len(native.Models) != 1 {
		t.Fatalf("expected one model, got %+v", native.Models)
	}
	if native.Models[0].Name != "multilingual" || native.Models[0].ReleaseDate != "2026-09-20" {
		t.Errorf("native entry = %+v, want the endpoint's name and release date", native.Models[0])
	}
}
