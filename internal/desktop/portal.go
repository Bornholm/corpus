package desktop

import (
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/desktop/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/zalando/go-keyring"
)

const keyringService = "corpus-desktop"

func (h *Handler) getServerPortal(w http.ResponseWriter, r *http.Request) {
	server := h.getCurrentServer()

	if server == nil {
		w.Header().Add("HX-Refresh", "true")
		w.Header().Add("HX-Redirect", "/servers")
		http.Redirect(w, r, "/servers", http.StatusTemporaryRedirect)
		return
	}

	// get password
	token, err := keyring.Get(keyringService, server.ID)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	loginURL := serverURL.JoinPath("/auth/token/login")

	page := component.ServerPortalPage(component.ServerPortalVModel{
		Server:   *server,
		LoginURL: loginURL.String(),
		Token:    token,
	})

	templ.Handler(page).ServeHTTP(w, r)
}
