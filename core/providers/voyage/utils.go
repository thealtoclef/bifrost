package voyage

// voyageDefaultBaseURL excludes /v1; request paths carry the version segment.
const voyageDefaultBaseURL = "https://api.voyageai.com"

// voyageRerankPath and voyageEmbeddingPath are the protocol paths appended to
// the base URL.
const (
	voyageRerankPath    = "/v1/rerank"
	voyageEmbeddingPath = "/v1/embeddings"
)

type voyageModel struct {
	ID          string
	Name        string
	Description string
}

// voyageModels is the static catalog returned by ListModels: the rerank models,
// then the embedding models POST /v1/embeddings serves.
var voyageModels = []voyageModel{
	{
		ID:          "rerank-3-lite",
		Name:        "Voyage rerank-3-lite",
		Description: "Latency-optimized reranker.",
	},
	{
		ID:          "rerank-3",
		Name:        "Voyage rerank-3",
		Description: "Highest-accuracy reranker.",
	},
	{
		ID:          "rerank-2.5-lite",
		Name:        "Voyage rerank-2.5-lite",
		Description: "Latency-optimized reranker (previous generation).",
	},
	{
		ID:          "rerank-2.5",
		Name:        "Voyage rerank-2.5",
		Description: "General-purpose reranker, multilingual and instruction-following.",
	},
	{
		ID:          "rerank-2-lite",
		Name:        "Voyage rerank-2-lite",
		Description: "Fast reranker (initial generation).",
	},
	{
		ID:          "rerank-2",
		Name:        "Voyage rerank-2",
		Description: "General-purpose reranker (initial generation).",
	},
	{
		ID:          "voyage-4-large",
		Name:        "Voyage 4 Large",
		Description: "Highest-accuracy general-purpose embedding model.",
	},
	{
		ID:          "voyage-4",
		Name:        "Voyage 4",
		Description: "General-purpose embedding model.",
	},
	{
		ID:          "voyage-4-lite",
		Name:        "Voyage 4 Lite",
		Description: "Latency- and cost-optimized general-purpose embedding model.",
	},
	{
		ID:          "voyage-3.5",
		Name:        "Voyage 3.5",
		Description: "General-purpose embedding model (previous generation).",
	},
	{
		ID:          "voyage-3.5-lite",
		Name:        "Voyage 3.5 Lite",
		Description: "Latency- and cost-optimized embedding model (previous generation).",
	},
	{
		ID:          "voyage-code-4",
		Name:        "Voyage Code 4",
		Description: "Embedding model for code retrieval.",
	},
	{
		ID:          "voyage-finance-2",
		Name:        "Voyage Finance 2",
		Description: "Embedding model for financial documents.",
	},
	{
		ID:          "voyage-law-2",
		Name:        "Voyage Law 2",
		Description: "Embedding model for legal documents.",
	},
}
