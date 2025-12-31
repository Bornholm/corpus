package desktop

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/desktop/component"
	"github.com/bornholm/corpus/internal/desktop/settings"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
)

func (h *Handler) getIndexersPage(w http.ResponseWriter, r *http.Request) {
	st, err := h.store.Get(true)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	vmodel := h.fillIndexersPageViewModel(st)

	page := component.IndexersPage(*vmodel)

	templ.Handler(page).ServeHTTP(w, r)
}

func (h *Handler) fillIndexersPageViewModel(settings settings.Settings) *component.IndexersPageVModel {
	return &component.IndexersPageVModel{
		Settings: settings,
	}
}
