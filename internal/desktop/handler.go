package desktop

import (
	"net/http"
	"sync"

	"github.com/bornholm/corpus/internal/desktop/app"
	"github.com/bornholm/corpus/internal/desktop/settings"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
)

type Handler struct {
	mux   *http.ServeMux
	store *app.SettingsStore[settings.Settings]

	currentServerMutex sync.RWMutex
	currentServer      *settings.Server
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler() *Handler {
	h := &Handler{
		mux:   http.NewServeMux(),
		store: app.NewStore(settings.Defaults),
	}

	h.mux.Handle("/assets/", http.StripPrefix("/assets", common.NewHandler()))
	h.mux.Handle("GET /servers", http.HandlerFunc(h.getServerSelectionPage))
	h.mux.Handle("GET /servers/new", http.HandlerFunc(h.getNewServerPage))

	h.mux.Handle("POST /servers/actions/select", http.HandlerFunc(h.handleSelectServer))
	h.mux.Handle("POST /servers", http.HandlerFunc(h.handleNewServer))

	h.mux.Handle("GET /servers/{serverID}", http.HandlerFunc(h.getEditServerPage))
	h.mux.Handle("POST /servers/{serverID}", http.HandlerFunc(h.handleEditServer))
	h.mux.Handle("DELETE /servers/{serverID}", http.HandlerFunc(h.handleDeleteServer))

	h.mux.Handle("GET /portal", http.HandlerFunc(h.getServerPortal))
	h.mux.Handle("/", http.HandlerFunc(h.handleIndexPage))

	h.mux.Handle("GET /settings", http.HandlerFunc(h.getSettingsPage))
	h.mux.Handle("DELETE /settings", http.HandlerFunc(h.handleResetSettings))

	h.mux.Handle("GET /indexers", http.HandlerFunc(h.getIndexersPage))

	return h
}

func (h *Handler) setCurrentServer(server *settings.Server) {
	h.currentServerMutex.Lock()
	defer h.currentServerMutex.Unlock()

	h.currentServer = server
}

func (h *Handler) getCurrentServer() *settings.Server {
	h.currentServerMutex.RLock()
	defer h.currentServerMutex.RUnlock()

	return h.currentServer
}

var _ http.Handler = &Handler{}
