package voyage

// voyageRerankRequest is the body of POST /v1/rerank.
type voyageRerankRequest struct {
	Model           string                 `json:"model"`
	Query           string                 `json:"query"`
	Documents       []string               `json:"documents"` // bare strings; documents are collapsed to text
	TopK            *int                   `json:"top_k,omitempty"`
	ReturnDocuments *bool                  `json:"return_documents,omitempty"`
	Truncation      *bool                  `json:"truncation,omitempty"`
	ExtraParams     map[string]interface{} `json:"-"`
}

// GetExtraParams returns fields merged into the body when extra-param
// passthrough is enabled.
func (r *voyageRerankRequest) GetExtraParams() map[string]interface{} {
	return r.ExtraParams
}

// voyageRerankResponse is the body of a successful /v1/rerank response.
type voyageRerankResponse struct {
	Object string               `json:"object"`
	Data   []voyageRerankResult `json:"data"`
	Model  string               `json:"model"`
	Usage  *voyageRerankUsage   `json:"usage,omitempty"`
}

// voyageRerankResult is one ranked document. Document is set only when
// return_documents was requested.
type voyageRerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	Document       *string `json:"document,omitempty"`
}

// voyageRerankUsage reports the tokens spent scoring the request.
type voyageRerankUsage struct {
	TotalTokens int `json:"total_tokens"`
}

// voyageErrorResponse is Voyage's error body: a flat {"detail": "..."}.
type voyageErrorResponse struct {
	Detail string `json:"detail"`
}
