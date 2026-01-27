package desktop

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/desktop/component"
	"github.com/bornholm/corpus/internal/desktop/settings"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/pkg/errors"
	"github.com/rs/xid"
	"github.com/zalando/go-keyring"

	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
)

func (h *Handler) getServerSelectionPage(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.Get(false)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	vmodel := h.fillServerSelectionPageViewModel(settings)

	page := component.ServerSelectionPage(*vmodel)

	templ.Handler(page).ServeHTTP(w, r)
}

func (h *Handler) fillServerSelectionPageViewModel(settings settings.Settings) *component.ServerSelectionPageVModel {
	return &component.ServerSelectionPageVModel{
		Servers: settings.Servers,
	}
}

func (h *Handler) getNewServerPage(w http.ResponseWriter, r *http.Request) {
	vmodel := h.fillNewServerPageVModel()

	page := component.NewServerPage(*vmodel)

	templ.Handler(page).ServeHTTP(w, r)
}

func (h *Handler) handleNewServer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	st, err := h.store.Get(true)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	serverForm := newServerForm()

	if err := serverForm.Handle(r); err != nil {
		common.HandleError(w, r, err)
		return
	}

	// Validate form
	if !serverForm.IsValid(ctx) {
		// Re-render form with errors
		vmodel := h.fillNewServerPageVModel()

		// Update form with validation errors
		vmodel.Form = serverForm

		page := component.NewServerPage(*vmodel)
		templ.Handler(page).ServeHTTP(w, r)
		return
	}

	serverID := xid.New().String()

	token, _ := serverForm.GetFieldValue("token")

	if err := keyring.Set(keyringService, serverID, token); err != nil {
		common.HandleError(w, r, err)
		return
	}

	url, _ := serverForm.GetFieldValue("url")
	label, _ := serverForm.GetFieldValue("label")
	rawPreferred, _ := serverForm.GetFieldValue("preferred")

	st.Servers = append(st.Servers, settings.Server{
		ID:        serverID,
		URL:       url,
		Label:     label,
		Preferred: rawPreferred == "on",
	})

	if err := h.store.Save(st); err != nil {
		common.HandleError(w, r, err)
		return
	}

	redirectURL := commonComp.BaseURL(ctx, commonComp.WithPath("/"))

	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) fillNewServerPageVModel() *component.NewServerPageVModel {
	return &component.NewServerPageVModel{
		Form: newServerForm(),
	}
}

func (h *Handler) handleSelectServer(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		common.HandleError(w, r, err)
		return
	}

	st, err := h.store.Get(false)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	serverID := r.FormValue("server")

	var server *settings.Server
	for _, s := range st.Servers {
		if serverID != s.ID {
			continue
		}

		server = &s
		break
	}

	if server == nil {
		common.HandleError(w, r, common.NewHTTPError(http.StatusNotFound))
		return
	}

	h.setCurrentServer(server)

	w.Header().Add("HX-Refresh", "true")
	w.Header().Add("HX-Redirect", "/portal")

	http.Redirect(w, r, "/portal", http.StatusSeeOther)
}

func (h *Handler) getEditServerPage(w http.ResponseWriter, r *http.Request) {
	server, err := h.getServerFromRequest(r)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	vmodel := h.fillEditServerPageVModel(server)

	page := component.EditServerPage(*vmodel)

	templ.Handler(page).ServeHTTP(w, r)
}

func (h *Handler) handleEditServer(w http.ResponseWriter, r *http.Request) {
	server, err := h.getServerFromRequest(r)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	ctx := r.Context()

	st, err := h.store.Get(true)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	serverForm := newServerForm()

	if err := serverForm.Handle(r); err != nil {
		common.HandleError(w, r, err)
		return
	}

	// Validate form
	if !serverForm.IsValid(ctx) {
		vmodel := h.fillEditServerPageVModel(server)
		vmodel.Form = serverForm

		page := component.EditServerPage(*vmodel)
		templ.Handler(page).ServeHTTP(w, r)
		return
	}

	url, _ := serverForm.GetFieldValue("url")
	label, _ := serverForm.GetFieldValue("label")
	rawPreferred, _ := serverForm.GetFieldValue("preferred")
	isPreferred := rawPreferred == "on"

	st.Servers = slices.Collect(func(yield func(settings.Server) bool) {
		for _, s := range st.Servers {
			if s.ID == server.ID {
				s = settings.Server{
					ID:        s.ID,
					URL:       url,
					Label:     label,
					Preferred: isPreferred,
				}
			} else if isPreferred {
				s.Preferred = false
			}

			if !yield(s) {
				return
			}
		}
	})

	if err := h.store.Save(st); err != nil {
		common.HandleError(w, r, err)
		return
	}

	if token, exists := serverForm.GetFieldValue("token"); exists && token != dummyToken {
		if err := keyring.Set(keyringService, server.ID, token); err != nil {
			common.HandleError(w, r, err)
			return
		}
	}

	redirectURL := commonComp.BaseURL(ctx, commonComp.WithPath("/servers"))

	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

const dummyToken string = "*************************************"

func (h *Handler) fillEditServerPageVModel(server *settings.Server) *component.EditServerPageVModel {
	form := newServerForm()

	form.SetFieldValues("token", dummyToken)
	form.SetFieldValues("label", server.Label)
	form.SetFieldValues("url", server.URL)
	form.SetFieldValues("preferred", strconv.FormatBool(server.Preferred))

	return &component.EditServerPageVModel{
		Form:   form,
		Server: server,
	}
}

func (h *Handler) getServerFromRequest(r *http.Request) (*settings.Server, error) {
	serverID := r.PathValue("serverID")
	if serverID == "" {
		return nil, common.NewHTTPError(http.StatusBadRequest)
	}

	st, err := h.store.Get(false)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var server *settings.Server
	for _, s := range st.Servers {
		if s.ID != serverID {
			continue
		}

		server = &s
		break
	}

	if server == nil {
		return nil, common.NewHTTPError(http.StatusNotFound)
	}

	return server, nil
}

func (h *Handler) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	server, err := h.getServerFromRequest(r)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	ctx := r.Context()

	st, err := h.store.Get(true)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	st.Servers = slices.Collect(func(yield func(settings.Server) bool) {
		for _, s := range st.Servers {
			if s.ID == server.ID {
				continue
			}

			if !yield(s) {
				return
			}
		}
	})

	if err := h.store.Save(st); err != nil {
		common.HandleError(w, r, err)
		return
	}

	if err := keyring.Delete(keyringService, server.ID); err != nil {
		common.HandleError(w, r, err)
		return
	}

	if currentServer := h.getCurrentServer(); currentServer != nil && server.ID == currentServer.ID {
		h.setCurrentServer(nil)
	}

	redirectURL := commonComp.BaseURL(ctx, commonComp.WithPath("/servers"))

	w.Header().Add("HX-Refresh", "true")
	w.Header().Add("HX-Redirect", "/servers")

	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}
