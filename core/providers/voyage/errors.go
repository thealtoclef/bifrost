package voyage

import (
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

// parseVoyageError parses a Voyage error response: a flat {"detail": "..."}
// body carrying the upstream status code.
func parseVoyageError(resp *fasthttp.Response) *schemas.BifrostError {
	var errorResp voyageErrorResponse
	bifrostErr := providerUtils.HandleProviderAPIError(resp, &errorResp)

	if errorResp.Detail != "" {
		bifrostErr.Error.Message = errorResp.Detail
	} else if bifrostErr.Error.Message == "" {
		bifrostErr.Error.Message = "Voyage API request failed"
	}

	return bifrostErr
}
