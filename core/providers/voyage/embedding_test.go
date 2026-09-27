package voyage

import (
	"testing"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToVoyageEmbeddingRequestWireShape(t *testing.T) {
	dimensions := 1024
	inputType := "query"
	truncation := false
	outputDtype := "int8"
	floatFormat := schemas.EmbeddingEncodingFloat
	base64Format := schemas.EmbeddingEncodingBase64
	text := "hello world"
	texts := []string{"a", "b"}

	cases := []struct {
		name    string
		request *schemas.BifrostEmbeddingRequest
		want    string
	}{
		{
			name: "single text",
			request: &schemas.BifrostEmbeddingRequest{
				Model: "voyage-4-lite",
				Input: &schemas.EmbeddingInput{Text: &text},
			},
			want: `{"input":"hello world","model":"voyage-4-lite"}`,
		},
		{
			name: "texts with dimensions",
			request: &schemas.BifrostEmbeddingRequest{
				Model:  "voyage-4-lite",
				Input:  &schemas.EmbeddingInput{Texts: texts},
				Params: &schemas.EmbeddingParameters{Dimensions: &dimensions},
			},
			want: `{"input":["a","b"],"model":"voyage-4-lite","output_dimension":1024}`,
		},
		{
			name: "Voyage-native output_dimension wins only when dimensions is unset",
			request: &schemas.BifrostEmbeddingRequest{
				Model: "voyage-4-lite",
				Input: &schemas.EmbeddingInput{Text: &text},
				Params: &schemas.EmbeddingParameters{
					ExtraParams: map[string]interface{}{"output_dimension": 512},
				},
			},
			want: `{"input":"hello world","model":"voyage-4-lite","output_dimension":512}`,
		},
		{
			name: "canonical dimensions wins over output_dimension",
			request: &schemas.BifrostEmbeddingRequest{
				Model: "voyage-4-lite",
				Input: &schemas.EmbeddingInput{Text: &text},
				Params: &schemas.EmbeddingParameters{
					Dimensions:  &dimensions,
					ExtraParams: map[string]interface{}{"output_dimension": 512},
				},
			},
			want: `{"input":"hello world","model":"voyage-4-lite","output_dimension":1024}`,
		},
		{
			name: "Voyage knobs from extra params",
			request: &schemas.BifrostEmbeddingRequest{
				Model: "voyage-4-lite",
				Input: &schemas.EmbeddingInput{Text: &text},
				Params: &schemas.EmbeddingParameters{
					ExtraParams: map[string]interface{}{
						"input_type":   inputType,
						"truncation":   truncation,
						"output_dtype": outputDtype,
						"other":        "kept",
					},
				},
			},
			want: `{"input":"hello world","model":"voyage-4-lite","input_type":"query","truncation":false,"output_dtype":"int8"}`,
		},
		{
			name: "float encoding_format is Voyage's null",
			request: &schemas.BifrostEmbeddingRequest{
				Model:  "voyage-4-lite",
				Input:  &schemas.EmbeddingInput{Text: &text},
				Params: &schemas.EmbeddingParameters{EncodingFormat: &floatFormat},
			},
			want: `{"input":"hello world","model":"voyage-4-lite"}`,
		},
		{
			name: "base64 encoding_format carries",
			request: &schemas.BifrostEmbeddingRequest{
				Model:  "voyage-4-lite",
				Input:  &schemas.EmbeddingInput{Text: &text},
				Params: &schemas.EmbeddingParameters{EncodingFormat: &base64Format},
			},
			want: `{"input":"hello world","model":"voyage-4-lite","encoding_format":"base64"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := ToVoyageEmbeddingRequest(tc.request)
			require.NoError(t, err)

			body, err := sonic.Marshal(req)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(body))
		})
	}

	// Lifted knobs leave the passthrough copy.
	var knobs *schemas.BifrostEmbeddingRequest
	for i, tc := range cases {
		if tc.name == "Voyage knobs from extra params" {
			knobs = cases[i].request
		}
	}
	req, err := ToVoyageEmbeddingRequest(knobs)
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"other": "kept"}, req.ExtraParams)
}

func TestToVoyageEmbeddingRequestRejections(t *testing.T) {
	text := "hi"
	tokens := []int{1, 2, 3}

	cases := []struct {
		name    string
		request *schemas.BifrostEmbeddingRequest
		wantSub string
	}{
		{
			name: "token arrays have no Voyage equivalent",
			request: &schemas.BifrostEmbeddingRequest{
				Model: "voyage-4-lite",
				Input: &schemas.EmbeddingInput{Embedding: tokens},
			},
			wantSub: "input must be text",
		},
		{
			name:    "missing input",
			request: &schemas.BifrostEmbeddingRequest{Model: "voyage-4-lite"},
			wantSub: "input is required",
		},
		{
			name: "unknown encoding_format",
			request: &schemas.BifrostEmbeddingRequest{
				Model:  "voyage-4-lite",
				Input:  &schemas.EmbeddingInput{Text: &text},
				Params: &schemas.EmbeddingParameters{EncodingFormat: new("hex")},
			},
			wantSub: "encoding_format",
		},
		{
			name: "unknown input_type",
			request: &schemas.BifrostEmbeddingRequest{
				Model: "voyage-4-lite",
				Input: &schemas.EmbeddingInput{Text: &text},
				Params: &schemas.EmbeddingParameters{
					ExtraParams: map[string]interface{}{"input_type": "passage"},
				},
			},
			wantSub: "input_type",
		},
		{
			name: "unknown output_dtype",
			request: &schemas.BifrostEmbeddingRequest{
				Model: "voyage-4-lite",
				Input: &schemas.EmbeddingInput{Text: &text},
				Params: &schemas.EmbeddingParameters{
					ExtraParams: map[string]interface{}{"output_dtype": "float16"},
				},
			},
			wantSub: "output_dtype",
		},
		{
			name: "wrong type for a lifted field",
			request: &schemas.BifrostEmbeddingRequest{
				Model: "voyage-4-lite",
				Input: &schemas.EmbeddingInput{Text: &text},
				Params: &schemas.EmbeddingParameters{
					ExtraParams: map[string]interface{}{"truncation": "no"},
				},
			},
			wantSub: "truncation",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ToVoyageEmbeddingRequest(tc.request)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantSub)
		})
	}
}

func TestToBifrostEmbeddingResponseMapsTotalTokensToPromptTokens(t *testing.T) {
	// prompt_tokens is where embedding cost bills from.
	parsed := schemas.BifrostEmbeddingResponse{
		Model: "voyage-4-lite",
		Usage: &schemas.BifrostLLMUsage{TotalTokens: 8},
	}

	if parsed.Usage.PromptTokens == 0 {
		parsed.Usage.PromptTokens = parsed.Usage.TotalTokens
	}
	parsed.BackfillParams(&schemas.BifrostEmbeddingRequest{Model: "voyage-4-lite"})

	assert.Equal(t, 8, parsed.Usage.PromptTokens)
	assert.Equal(t, "voyage-4-lite", parsed.Model)
}

// extra params arrive from JSON, where numbers decode as float64.
func TestToVoyageEmbeddingRequestAcceptsJSONNumbers(t *testing.T) {
	var parsed schemas.BifrostEmbeddingRequest
	body := `{"model":"voyage-4-lite","input":"x","output_dimension":512}`
	if err := sonic.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Unknown top-level fields land in extra params.
	parsed.Params = &schemas.EmbeddingParameters{ExtraParams: map[string]interface{}{"output_dimension": float64(512)}}

	req, err := ToVoyageEmbeddingRequest(&parsed)
	require.NoError(t, err)
	require.NotNil(t, req.OutputDimension)
	assert.Equal(t, 512, *req.OutputDimension)
	assert.Empty(t, req.ExtraParams)
}
