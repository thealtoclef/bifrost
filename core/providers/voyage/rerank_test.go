package voyage

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestToVoyageRerankRequestWireShape(t *testing.T) {
	topK := 2
	returnDocuments := true
	docID := "doc-2"

	req, err := ToVoyageRerankRequest(&schemas.BifrostRerankRequest{
		Model: "rerank-2.5",
		Query: "what is bifrost?",
		Documents: []schemas.RerankDocument{
			{Text: "doc one"},
			{Text: "doc two", ID: &docID},
			// A structured body is JSON-encoded into the string.
			{Data: map[string]interface{}{"title": "doc three"}},
		},
		Params: &schemas.RerankParameters{
			TopN:            &topK,
			ReturnDocuments: &returnDocuments,
		},
	})
	require.NoError(t, err)

	body, err := sonic.Marshal(req)
	require.NoError(t, err)

	// top_n is Voyage's top_k, and documents go out as bare strings.
	assert.JSONEq(t, `{
		"model": "rerank-2.5",
		"query": "what is bifrost?",
		"documents": ["doc one", "doc two", "{\"title\":\"doc three\"}"],
		"top_k": 2,
		"return_documents": true
	}`, string(body))
}

// truncation rides in extra params as a typed, validated field.
func TestToVoyageRerankRequestTruncationFromExtraParams(t *testing.T) {
	req, err := ToVoyageRerankRequest(&schemas.BifrostRerankRequest{
		Model:     "rerank-2.5",
		Query:     "q",
		Documents: []schemas.RerankDocument{{Text: "d"}},
		Params: &schemas.RerankParameters{
			ExtraParams: map[string]interface{}{"truncation": false, "other": "kept"},
		},
	})
	require.NoError(t, err)

	require.NotNil(t, req.Truncation, "explicit false must survive as a value, not be dropped")
	assert.False(t, *req.Truncation)

	// truncation is lifted out of the passthrough copy.
	assert.Equal(t, map[string]interface{}{"other": "kept"}, req.ExtraParams)
}

func TestToVoyageRerankRequestRejectsNonBooleanTruncation(t *testing.T) {
	_, err := ToVoyageRerankRequest(&schemas.BifrostRerankRequest{
		Model:     "rerank-2.5",
		Query:     "q",
		Documents: []schemas.RerankDocument{{Text: "d"}},
		Params: &schemas.RerankParameters{
			ExtraParams: map[string]interface{}{"truncation": "no"},
		},
	})
	require.Error(t, err, "an invalid truncation must be rejected, never silently ignored")
	assert.Contains(t, err.Error(), "truncation must be a boolean")
}

func TestToBifrostRerankResponse(t *testing.T) {
	docID := "doc-2"
	documents := []schemas.RerankDocument{
		{Text: "doc one"},
		{Text: "doc two", ID: &docID},
	}

	// Sorted by descending relevance; total_tokens becomes prompt tokens.
	resp, err := ToBifrostRerankResponse(&voyageRerankResponse{
		Object: "list",
		Model:  "rerank-2.5",
		Usage:  &voyageRerankUsage{TotalTokens: 123},
		Data: []voyageRerankResult{
			{Index: 0, RelevanceScore: 0.2},
			{Index: 1, RelevanceScore: 0.9},
		},
	}, documents, true)
	require.NoError(t, err)

	assert.Equal(t, "rerank-2.5", resp.Model)
	require.Len(t, resp.Results, 2)
	assert.Equal(t, 1, resp.Results[0].Index, "results are sorted by descending relevance")
	assert.Equal(t, 0.9, resp.Results[0].RelevanceScore)
	assert.Equal(t, 0, resp.Results[1].Index)

	require.NotNil(t, resp.Results[0].Document, "documents are re-attached when requested")
	assert.Equal(t, "doc two", resp.Results[0].Document.Text)
	require.NotNil(t, resp.Results[0].ID, "the caller's document ID survives the round trip")
	assert.Equal(t, "doc-2", *resp.Results[0].ID)

	require.NotNil(t, resp.Usage)
	assert.Equal(t, 123, resp.Usage.PromptTokens)
	assert.Equal(t, 123, resp.Usage.TotalTokens)
	assert.Equal(t, 0, resp.Usage.CompletionTokens)
}

func TestToBifrostRerankResponseKeepsIDWithoutDocuments(t *testing.T) {
	docID := "doc-1"
	documents := []schemas.RerankDocument{{Text: "doc one", ID: &docID}}

	resp, err := ToBifrostRerankResponse(&voyageRerankResponse{
		Data: []voyageRerankResult{{Index: 0, RelevanceScore: 0.5}},
	}, documents, false)
	require.NoError(t, err)

	require.Len(t, resp.Results, 1)
	assert.Nil(t, resp.Results[0].Document, "document content is omitted when not requested")
	require.NotNil(t, resp.Results[0].ID, "the caller's ID is identity and survives without the document")
	assert.Equal(t, "doc-1", *resp.Results[0].ID)
}

func TestToBifrostRerankResponseRejections(t *testing.T) {
	documents := []schemas.RerankDocument{{Text: "only"}}

	t.Run("duplicate index", func(t *testing.T) {
		_, err := ToBifrostRerankResponse(&voyageRerankResponse{
			Data: []voyageRerankResult{{Index: 0, RelevanceScore: 1}, {Index: 0, RelevanceScore: 0.5}},
		}, documents, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate index")
	})

	t.Run("index out of range", func(t *testing.T) {
		_, err := ToBifrostRerankResponse(&voyageRerankResponse{
			Data: []voyageRerankResult{{Index: 7, RelevanceScore: 1}},
		}, documents, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("nil payload", func(t *testing.T) {
		_, err := ToBifrostRerankResponse(nil, documents, false)
		require.Error(t, err)
	})
}

func TestParseVoyageErrorSurfaceDetail(t *testing.T) {
	var resp fasthttp.Response
	resp.SetStatusCode(fasthttp.StatusUnauthorized)
	resp.SetBodyString(`{"detail":"Unauthorized"}`)

	bifrostErr := parseVoyageError(&resp)
	require.NotNil(t, bifrostErr.Error)
	assert.Equal(t, "Unauthorized", bifrostErr.Error.Message)
	require.NotNil(t, bifrostErr.StatusCode)
	assert.Equal(t, 401, *bifrostErr.StatusCode)
}
