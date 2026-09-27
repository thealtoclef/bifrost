package voyage_test

import (
	"os"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/internal/llmtests"

	"github.com/maximhq/bifrost/core/schemas"
)

func TestVoyage(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace(os.Getenv("VOYAGE_API_KEY")) == "" {
		t.Skip("Skipping Voyage tests because VOYAGE_API_KEY is not set")
	}

	client, ctx, cancel, err := llmtests.SetupTest()
	if err != nil {
		t.Fatalf("Error initializing test setup: %v", err)
	}
	defer cancel()
	defer client.Shutdown()

	testConfig := llmtests.ComprehensiveTestConfig{
		Provider:       schemas.Voyage,
		RerankModel:    "rerank-2.5",
		EmbeddingModel: "voyage-4-lite",
		Scenarios: llmtests.TestScenarios{
			Rerank:     true,
			Embedding:  true,
			ListModels: true,
		},
	}

	t.Run("VoyageTests", func(t *testing.T) {
		llmtests.RunAllComprehensiveTests(t, client, ctx, testConfig)
	})
}
