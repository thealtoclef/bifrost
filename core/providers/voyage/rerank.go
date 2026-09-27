package voyage

import (
	"fmt"
	"sort"

	"github.com/bytedance/sonic"
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
)

// ToVoyageRerankRequest maps top_n to top_k, collapses documents to the bare
// strings Voyage ranks, and lifts boolean truncation out of extra params.
func ToVoyageRerankRequest(bifrostReq *schemas.BifrostRerankRequest) (*voyageRerankRequest, error) {
	if bifrostReq == nil {
		return nil, nil
	}

	voyageReq := &voyageRerankRequest{
		Model:     bifrostReq.Model,
		Query:     bifrostReq.Query,
		Documents: make([]string, len(bifrostReq.Documents)),
	}
	for i, doc := range bifrostReq.Documents {
		voyageReq.Documents[i] = rerankDocumentText(doc)
	}

	if bifrostReq.Params != nil {
		voyageReq.TopK = bifrostReq.Params.TopN
		voyageReq.ReturnDocuments = bifrostReq.Params.ReturnDocuments

		extraParams := bifrostReq.Params.ExtraParams
		if raw, ok := extraParams["truncation"]; ok {
			truncation, ok := raw.(bool)
			if !ok {
				return nil, providerUtils.InvalidRequestErrorf("truncation must be a boolean, got %T", raw)
			}
			voyageReq.Truncation = &truncation

			remaining := make(map[string]interface{}, len(extraParams)-1)
			for key, value := range extraParams {
				if key != "truncation" {
					remaining[key] = value
				}
			}
			voyageReq.ExtraParams = remaining
		} else {
			voyageReq.ExtraParams = extraParams
		}
	}

	return voyageReq, nil
}

// ToBifrostRerankResponse sorts results by descending relevance, maps
// total_tokens onto prompt tokens, and restores documents by result index.
func ToBifrostRerankResponse(payload *voyageRerankResponse, documents []schemas.RerankDocument, returnDocuments bool) (*schemas.BifrostRerankResponse, error) {
	if payload == nil {
		return nil, fmt.Errorf("voyage rerank response is nil")
	}

	response := &schemas.BifrostRerankResponse{
		Model:   payload.Model,
		Results: make([]schemas.RerankResult, 0, len(payload.Data)),
	}

	if payload.Usage != nil {
		response.Usage = &schemas.BifrostLLMUsage{
			PromptTokens: payload.Usage.TotalTokens,
			TotalTokens:  payload.Usage.TotalTokens,
		}
	}

	seenIndices := make(map[int]struct{}, len(payload.Data))
	for _, item := range payload.Data {
		if item.Index < 0 || item.Index >= len(documents) {
			return nil, fmt.Errorf("invalid voyage rerank response: result index %d out of range", item.Index)
		}
		if _, exists := seenIndices[item.Index]; exists {
			return nil, fmt.Errorf("invalid voyage rerank response: duplicate index %d", item.Index)
		}
		seenIndices[item.Index] = struct{}{}

		result := schemas.RerankResult{
			Index:          item.Index,
			RelevanceScore: item.RelevanceScore,
			ID:             documents[item.Index].ID,
		}
		if returnDocuments {
			doc := documents[item.Index]
			result.Document = &doc
		}
		response.Results = append(response.Results, result)
	}

	sort.SliceStable(response.Results, func(i, j int) bool {
		if response.Results[i].RelevanceScore == response.Results[j].RelevanceScore {
			return response.Results[i].Index < response.Results[j].Index
		}
		return response.Results[i].RelevanceScore > response.Results[j].RelevanceScore
	})

	return response, nil
}

// rerankDocumentText renders a document as the string Voyage ranks; a
// structured body is JSON-encoded.
func rerankDocumentText(doc schemas.RerankDocument) string {
	if doc.Text != "" || len(doc.Data) == 0 {
		return doc.Text
	}
	encoded, err := sonic.Marshal(doc.Data)
	if err != nil {
		return doc.Text
	}
	return string(encoded)
}
