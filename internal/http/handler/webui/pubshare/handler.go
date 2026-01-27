package pubshare

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
)

type Handler struct {
	mux             *http.ServeMux
	documentManager *service.DocumentManager
	pubShareStore   port.PublicShareStore
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(documentManager *service.DocumentManager, pubShareStore port.PublicShareStore) *Handler {
	h := &Handler{
		mux:             http.NewServeMux(),
		documentManager: documentManager,
		pubShareStore:   pubShareStore,
	}

	h.mux.Handle("GET /{publicShareToken}", h.assertToken(http.HandlerFunc(h.getPublicSharePage)))
	h.mux.Handle("POST /{publicShareToken}", h.assertToken(http.HandlerFunc(h.handleAsk)))

	return h
}

var _ http.Handler = &Handler{}
