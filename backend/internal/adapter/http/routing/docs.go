package routing

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

const (
	DocumentPath  = "/openapi.yaml"
	ReferencePath = "/docs"
)

const (
	ContentTypeYAML = "application/yaml; charset=utf-8"
	ContentTypeHTML = "text/html; charset=utf-8"
	CacheControl    = "no-cache"
)

type Documentation struct {
	document  []byte
	reference []byte
}

func NewDocumentation(document, reference []byte) Documentation {
	return Documentation{document: document, reference: reference}
}

func MountDocumentation(r chi.Router, handler Documentation) {
	r.Get(DocumentPath, handler.Document)
	r.Get(ReferencePath, handler.Reference)
}

// @ID          getOpenAPIDocument
// @Summary     Fetch the OpenAPI document
// @Description The contract for every route above, generated from the handlers by swag and compiled into the binary. Registered only when DOCS_ENABLED is true, and never rate limited, so a client that has spent its budget can still read why.
// @Tags        documentation
// @Produce     application/yaml
// @Success     200 {string} string "the OpenAPI 3.1 document"
// @Header      all {string} X-Request-Id "correlation id, present on every response"
// @Header      all {string} Cache-Control "no-cache, because the document changes on deploy"
// @Router      /openapi.yaml [get]
func (h Documentation) Document(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", ContentTypeYAML)
	w.Header().Set("Cache-Control", CacheControl)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.document)
}

// @ID          getAPIReference
// @Summary     Browse the API reference
// @Description A Scalar reference page that fetches the document from /openapi.yaml, same origin, and renders it. The page itself is static and embedded; its script comes from a version-pinned CDN with a subresource-integrity hash, so the page renders blank without internet access. Registered only when DOCS_ENABLED is true.
// @Tags        documentation
// @Produce     html
// @Success     200 {string} string "the reference page"
// @Header      all {string} X-Request-Id "correlation id, present on every response"
// @Header      all {string} Cache-Control "no-cache, because the document changes on deploy"
// @Router      /docs [get]
func (h Documentation) Reference(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", ContentTypeHTML)
	w.Header().Set("Cache-Control", CacheControl)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.reference)
}
