package swagger

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

//go:embed dist/**
var swaggerFS embed.FS

//go:embed openapi.yml
var openAPISpec []byte

type Handler struct {
	mux *http.ServeMux
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler() *Handler {
	h := &Handler{
		mux: http.NewServeMux(),
	}

	files, err := fs.Sub(swaggerFS, "dist")
	if err != nil {
		panic(errors.WithStack(err))
	}

	h.mux.HandleFunc("GET /openapi.json", h.serveSpec)
	h.mux.Handle("/", http.FileServerFS(files))

	return h
}

func (h *Handler) serveSpec(w http.ResponseWriter, r *http.Request) {
	var spec any
	if err := yaml.Unmarshal(openAPISpec, &spec); err != nil {
		slog.ErrorContext(r.Context(), "could not parse openapi spec", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	w.Header().Add("Content-Type", "application/json")

	if err := encoder.Encode(spec); err != nil {
		slog.ErrorContext(r.Context(), "could not encode openapi spec", slog.Any("error", errors.WithStack(err)))
	}
}

var _ http.Handler = &Handler{}
