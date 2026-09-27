package voyage

import (
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
)

// voyageEmbeddingRequest is the body of POST /v1/embeddings.
type voyageEmbeddingRequest struct {
	Input           *schemas.EmbeddingInput `json:"input"`
	Model           string                  `json:"model"`
	InputType       *string                 `json:"input_type,omitempty"`
	Truncation      *bool                   `json:"truncation,omitempty"`
	OutputDimension *int                    `json:"output_dimension,omitempty"`
	OutputDtype     *string                 `json:"output_dtype,omitempty"`
	EncodingFormat  *string                 `json:"encoding_format,omitempty"`
	ExtraParams     map[string]interface{}  `json:"-"`
}

// GetExtraParams implements providerUtils.RequestBodyWithExtraParams.
func (r *voyageEmbeddingRequest) GetExtraParams() map[string]interface{} {
	return r.ExtraParams
}

// Voyage's accepted values for the fields it does not share with the OpenAI
// embedding contract.
var (
	voyageInputTypes   = map[string]bool{"query": true, "document": true}
	voyageOutputDtypes = map[string]bool{"float": true, "int8": true, "uint8": true, "binary": true, "ubinary": true}
)

// ToVoyageEmbeddingRequest converts a Bifrost embedding request into Voyage's
// wire shape: dimensions map to output_dimension, and the Voyage-only knobs
// (input_type, truncation, output_dtype) are lifted from extra params into
// typed, validated fields.
func ToVoyageEmbeddingRequest(bifrostReq *schemas.BifrostEmbeddingRequest) (*voyageEmbeddingRequest, error) {
	if bifrostReq == nil {
		return nil, nil
	}
	if bifrostReq.Input == nil {
		return nil, providerUtils.InvalidRequestErrorf("input is required for embeddings")
	}
	if bifrostReq.Input.Embedding != nil || bifrostReq.Input.Embeddings != nil {
		return nil, providerUtils.InvalidRequestErrorf("input must be text; Voyage takes no token arrays")
	}

	voyageReq := &voyageEmbeddingRequest{
		Input: bifrostReq.Input,
		Model: bifrostReq.Model,
	}
	if bifrostReq.Params == nil {
		return voyageReq, nil
	}

	voyageReq.OutputDimension = bifrostReq.Params.Dimensions

	// Voyage's name for the dimension knob; the canonical dimensions field wins
	// when both are set.
	extra := bifrostReq.Params.ExtraParams
	if raw, ok := extra["output_dimension"]; ok {
		outputDimension, ok := schemas.SafeExtractIntPointer(raw)
		if !ok {
			return nil, providerUtils.InvalidRequestErrorf("output_dimension has the wrong type")
		}
		if voyageReq.OutputDimension == nil {
			voyageReq.OutputDimension = outputDimension
		}
		remaining := make(map[string]interface{}, len(extra)-1)
		for k, v := range extra {
			if k != "output_dimension" {
				remaining[k] = v
			}
		}
		extra = remaining
	}

	// Voyage's encoding_format is null or base64, and null means float.
	if format := bifrostReq.Params.EncodingFormat; format != nil && *format != schemas.EmbeddingEncodingFloat {
		if *format != schemas.EmbeddingEncodingBase64 {
			return nil, providerUtils.InvalidRequestErrorf("encoding_format %q is not supported; use float or base64", *format)
		}
		voyageReq.EncodingFormat = format
	}

	var err error
	if voyageReq.InputType, extra, err = liftExtra[string](extra, "input_type"); err != nil {
		return nil, err
	}
	if voyageReq.Truncation, extra, err = liftExtra[bool](extra, "truncation"); err != nil {
		return nil, err
	}
	if voyageReq.OutputDtype, extra, err = liftExtra[string](extra, "output_dtype"); err != nil {
		return nil, err
	}
	if voyageReq.InputType != nil && !voyageInputTypes[*voyageReq.InputType] {
		return nil, providerUtils.InvalidRequestErrorf("input_type %q is not supported; use query or document", *voyageReq.InputType)
	}
	if voyageReq.OutputDtype != nil && !voyageOutputDtypes[*voyageReq.OutputDtype] {
		return nil, providerUtils.InvalidRequestErrorf("output_dtype %q is not supported", *voyageReq.OutputDtype)
	}
	voyageReq.ExtraParams = extra

	return voyageReq, nil
}

// liftExtra moves a provider-only field out of the passthrough copy into a typed
// field, so the generic merge cannot overwrite it.
func liftExtra[T any](extra map[string]interface{}, key string) (*T, map[string]interface{}, error) {
	raw, ok := extra[key]
	if !ok {
		return nil, extra, nil
	}
	value, ok := raw.(T)
	if !ok {
		return nil, extra, providerUtils.InvalidRequestErrorf("%s has the wrong type", key)
	}

	remaining := make(map[string]interface{}, len(extra)-1)
	for k, v := range extra {
		if k != key {
			remaining[k] = v
		}
	}
	return &value, remaining, nil
}

// Embedding performs an embedding request against POST /v1/embeddings.
func (provider *VoyageProvider) Embedding(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostEmbeddingRequest) (*schemas.BifrostEmbeddingResponse, *schemas.BifrostError) {
	if err := providerUtils.CheckOperationAllowed(schemas.Voyage, provider.customProviderConfig, schemas.EmbeddingRequest); err != nil {
		return nil, err
	}

	jsonBody, bifrostErr := providerUtils.CheckContextAndGetRequestBody(
		ctx,
		request,
		func() (providerUtils.RequestBodyWithExtraParams, error) {
			return ToVoyageEmbeddingRequest(request)
		})
	if bifrostErr != nil {
		return nil, bifrostErr
	}

	sendBackRawRequest := providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest)
	sendBackRawResponse := providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse)

	responseBody, latency, providerResponseHeaders, bifrostErr := provider.completeRequest(
		ctx, jsonBody, provider.buildRequestURL(ctx, voyageEmbeddingPath, schemas.EmbeddingRequest), key.Value.GetValue())
	if providerResponseHeaders != nil {
		ctx.SetValue(schemas.BifrostContextKeyProviderResponseHeaders, providerResponseHeaders)
	}
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, nil, sendBackRawRequest, sendBackRawResponse, latency)
	}

	// Large response mode: return a lightweight response with metadata only.
	if isLargeResp, _ := ctx.Value(schemas.BifrostContextKeyLargeResponseMode).(bool); isLargeResp {
		return &schemas.BifrostEmbeddingResponse{
			Model: request.Model,
			ExtraFields: schemas.BifrostResponseExtraFields{
				Latency:                 latency.Milliseconds(),
				ProviderResponseHeaders: providerResponseHeaders,
			},
		}, nil
	}

	var voyageResp schemas.BifrostEmbeddingResponse
	rawRequest, rawResponse, bifrostErr := providerUtils.HandleProviderResponseCtx(ctx, responseBody, &voyageResp, jsonBody, sendBackRawRequest, sendBackRawResponse)
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, responseBody, sendBackRawRequest, sendBackRawResponse, latency)
	}

	// Embedding cost bills from prompt tokens; Voyage reports total_tokens only.
	if voyageResp.Usage != nil && voyageResp.Usage.PromptTokens == 0 {
		voyageResp.Usage.PromptTokens = voyageResp.Usage.TotalTokens
	}
	voyageResp.BackfillParams(request)

	voyageResp.ExtraFields.Latency = latency.Milliseconds()
	voyageResp.ExtraFields.ProviderResponseHeaders = providerResponseHeaders
	if sendBackRawRequest {
		voyageResp.ExtraFields.RawRequest = rawRequest
	}
	if sendBackRawResponse {
		voyageResp.ExtraFields.RawResponse = rawResponse
	}

	return &voyageResp, nil
}
