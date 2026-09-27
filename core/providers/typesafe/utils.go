package typesafe

import (
	"strings"

	schemas "github.com/maximhq/bifrost/core/schemas"
)

// typesafeDefaultBaseURL is the default Typesafe API host.
const typesafeDefaultBaseURL = "https://api.typesafe.ai"

// typesafeSystemOnePath is the single evaluation endpoint for the System One
// model class; every jev model is served by it, selected via the request's
// model field.
const typesafeSystemOnePath = "/v1/systemone"

// typesafeModelsPath lists the models the endpoint serves. An endpoint without
// it answers 404 or 405.
const typesafeModelsPath = "/v1/models"

// typesafeReleaseDateAttribute carries a listing's release date on the canonical
// model, which has no field of its own for it.
const typesafeReleaseDateAttribute = "release_date"

// typesafeUpstreamModels is a model listing as a SystemOne endpoint returns it:
// models[].name, or data[].id for the OpenAI-compatible shape.
type typesafeUpstreamModels struct {
	Models []typesafeUpstreamModel `json:"models"`
	Data   []typesafeUpstreamModel `json:"data"`
}

type typesafeUpstreamModel struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	ReleaseDate string `json:"release_date,omitempty"`
	Created     int64  `json:"created,omitempty"`
}

// toCatalog maps either listing shape onto catalog entries, skipping entries
// that name no model.
func (listing *typesafeUpstreamModels) toCatalog() []typesafeModel {
	entries := listing.Models
	if len(entries) == 0 {
		entries = listing.Data
	}

	catalog := make([]typesafeModel, 0, len(entries))
	for _, entry := range entries {
		id := entry.Name
		if id == "" {
			id = entry.ID
		}
		if id == "" {
			continue
		}
		catalog = append(catalog, typesafeModel{
			ID:          id,
			Name:        id,
			Description: entry.Description,
			ReleaseDate: entry.ReleaseDate,
			Created:     entry.Created,
		})
	}
	return catalog
}

// typesafeModel is one entry of the model catalog.
type typesafeModel struct {
	ID          string
	Name        string
	Description string
	ReleaseDate string
	Created     int64
}

// typesafeModels is the static catalog served by ListModels. Typesafe documents
// no model-listing endpoint, so the catalog is pinned here and in the hosted
// datasheet. Aliases resolve upstream: jev-latest and jev-preview both point at
// jev-1.13.0 today.
var typesafeModels = []typesafeModel{
	{
		ID:          "jev-1.13.0",
		Name:        "Jev 1.13.0",
		Description: "TypeSafe's System One Model: Jev, version 1.13.0",
		// The SDKs validate release_date on every listed model; 1.13.0 shipped
		// at the same instant its aliases were minted.
		ReleaseDate: "2026-09-10T18:38:01.391457+00:00",
	},
	{
		ID:          "jev-latest",
		Name:        "Jev (latest)",
		Description: "The latest iteration of TypeSafe's System One Model: Jev",
		ReleaseDate: "2026-09-10T18:38:01.391457+00:00",
	},
	{
		ID:          "jev-preview",
		Name:        "Jev (preview)",
		Description: "A preview version of `jev-latest`: should be better in most ways",
		ReleaseDate: "2026-09-10T18:39:06.057655+00:00",
	},
}

// TypesafeNativeModel is one entry of the native model-listing shape.
type TypesafeNativeModel struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ReleaseDate string `json:"release_date,omitempty"`
}

// TypesafeNativeListModelsResponse is the native model-listing shape served on
// /typesafe/v1/models. Typesafe documents no upstream listing endpoint, so
// this is synthesized from the static catalog.
type TypesafeNativeListModelsResponse struct {
	Models []TypesafeNativeModel `json:"models"`
}

// ToTypesafeNativeListModelsResponse converts a Bifrost model listing into the
// native shape, restoring bare upstream model names and re-attaching catalog
// descriptions and release dates.
func ToTypesafeNativeListModelsResponse(resp *schemas.BifrostListModelsResponse) *TypesafeNativeListModelsResponse {
	native := &TypesafeNativeListModelsResponse{Models: []TypesafeNativeModel{}}
	if resp == nil {
		return native
	}
	catalog := make(map[string]typesafeModel, len(typesafeModels))
	for _, model := range typesafeModels {
		catalog[model.ID] = model
	}
	// Model IDs are provider-prefixed; a custom provider serves its own name.
	providerPrefix := string(schemas.Typesafe) + "/"
	if provider := resp.ExtraFields.Provider; provider != "" {
		providerPrefix = string(provider) + "/"
	}
	for _, model := range resp.Data {
		name := strings.TrimPrefix(model.ID, providerPrefix)
		if idx := strings.Index(name, "/"); idx >= 0 {
			// No provider name on the response: fall back to the catalog to decide
			// whether the leading segment is the prefix.
			if _, ok := catalog[name[idx+1:]]; ok {
				name = name[idx+1:]
			}
		}
		entry := TypesafeNativeModel{Name: name}
		if model.Description != nil {
			entry.Description = *model.Description
		}
		if releaseDate, ok := model.AdditionalAttributes[typesafeReleaseDateAttribute]; ok {
			entry.ReleaseDate = releaseDate
		}
		if known, ok := catalog[name]; ok {
			if entry.Description == "" {
				entry.Description = known.Description
			}
			if entry.ReleaseDate == "" {
				entry.ReleaseDate = known.ReleaseDate
			}
		}
		native.Models = append(native.Models, entry)
	}
	return native
}
