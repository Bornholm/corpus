package desktop

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/desktop/component"
	"github.com/bornholm/corpus/internal/desktop/settings"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/bornholm/go-x/slogx"
	"github.com/zalando/go-keyring"
)

func (h *Handler) getSettingsPage(w http.ResponseWriter, r *http.Request) {
	st, err := h.store.Get(true)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	vmodel := h.fillSettingsPageViewModel(st)

	page := component.SettingsPage(*vmodel)

	templ.Handler(page).ServeHTTP(w, r)
}

func (h *Handler) fillSettingsPageViewModel(settings settings.Settings) *component.SettingsPageVModel {
	return &component.SettingsPageVModel{
		Settings: settings,
	}
}

func (h *Handler) handleResetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	st, err := h.store.Get(true)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	for _, s := range st.Servers {
		if err := keyring.Delete(keyringService, s.ID); err != nil {
			slog.ErrorContext(ctx, "could not delete server token from keyring", slogx.Error(err))
		}
	}

	if err := h.store.Save(settings.Defaults); err != nil {
		common.HandleError(w, r, err)
		return
	}

	h.setCurrentServer(nil)

	redirectURL := string(commonComp.BaseURL(ctx, commonComp.WithPath("/")))

	w.Header().Add("HX-Refresh", "true")
	w.Header().Add("HX-Redirect", redirectURL)

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
