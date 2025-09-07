package authn

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/http/handler/authn/component"
)

func (h *Handler) getLoginPage(w http.ResponseWriter, r *http.Request) {
	vmodel := component.LoginPageVModel{
		Providers: h.providers,
	}

	loginPage := component.LoginPage(vmodel)

	templ.Handler(loginPage).ServeHTTP(w, r)
}
