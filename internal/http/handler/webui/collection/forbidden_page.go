package collection

import (
	"net/http"

	"github.com/a-h/templ"
	common "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
)

func (h *Handler) getForbiddenPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vmodel := common.ErrorPageVModel{
		Message: "L'accès à cette page ne vous est pas autorisé. Veuillez contacter l'administrateur.",
		Links: []common.LinkItem{
			{
				URL:   string(common.BaseURL(ctx, common.WithPath("/auth/oidc/logout"))),
				Label: "Se déconnecter",
			},
		},
	}

	forbiddenPage := common.ErrorPage(vmodel)

	w.WriteHeader(http.StatusForbidden)

	templ.Handler(forbiddenPage).ServeHTTP(w, r)
}
