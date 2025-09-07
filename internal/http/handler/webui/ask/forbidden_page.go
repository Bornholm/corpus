package ask

import (
	"net/http"

	"github.com/a-h/templ"
	common "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
)

func (h *Handler) getForbiddenPage(w http.ResponseWriter, r *http.Request) {
	forbiddenPage := common.ErrorPage(common.ErrorPageVModel{
		Message: "L'accès à cette page ne vous est pas autorisé. Veuillez contacter l'administrateur.",
		Links: []common.LinkItem{
			common.LinkItem{
				URL:   string(common.BaseURL(r.Context(), common.WithPath("/auth/logout"))),
				Label: "Se déconnecter",
			},
		},
	})

	w.WriteHeader(http.StatusForbidden)

	templ.Handler(forbiddenPage).ServeHTTP(w, r)
}
