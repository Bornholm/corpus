package profile

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/crypto"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	common "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/profile/component"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
	"github.com/bornholm/go-x/slogx"
)

type Handler struct {
	mux       *http.ServeMux
	userStore port.UserStore
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(userStore port.UserStore) *Handler {
	h := &Handler{
		mux:       http.NewServeMux(),
		userStore: userStore,
	}

	// Require authentication for all profile routes
	assertUser := authz.Middleware(http.HandlerFunc(h.getForbiddenPage), authz.OneOf(authz.Has(authz.RoleUser), authz.Has(authz.RoleAdmin)))

	h.mux.Handle("GET /", assertUser(http.HandlerFunc(h.getProfilePage)))
	h.mux.Handle("POST /tokens", assertUser(http.HandlerFunc(h.createToken)))
	h.mux.Handle("DELETE /tokens/{tokenID}", assertUser(http.HandlerFunc(h.deleteToken)))

	return h
}

func (h *Handler) getProfilePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := httpCtx.User(ctx)

	// Fetch user's auth tokens
	tokens, err := h.userStore.GetUserAuthTokens(ctx, user.ID())
	if err != nil {
		slog.ErrorContext(ctx, "could not fetch user auth tokens", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Check for success messages
	createdToken := r.URL.Query().Get("token_created")
	deletedToken := r.URL.Query().Get("token_deleted")

	vmodel := component.ProfilePageVModel{
		User:         user,
		AuthTokens:   tokens,
		TokenForm:    newTokenForm(),
		CreatedToken: createdToken,
		DeletedToken: deletedToken != "",
		Navbar: &common.NavbarVModel{
			User: user,
		},
	}

	profilePage := component.ProfilePage(vmodel)
	templ.Handler(profilePage).ServeHTTP(w, r)
}

func (h *Handler) createToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := httpCtx.User(ctx)

	tokenForm := newTokenForm()

	if err := tokenForm.Handle(r); err != nil {
		slog.ErrorContext(ctx, "could not parse form", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if !tokenForm.IsValid(ctx) {
		tokens, err := h.userStore.GetUserAuthTokens(ctx, user.ID())
		if err != nil {
			slog.ErrorContext(ctx, "could not fetch user auth tokens", slogx.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		vmodel := component.ProfilePageVModel{
			User:         user,
			AuthTokens:   tokens,
			TokenForm:    tokenForm,
			CreatedToken: "",
			DeletedToken: false,
			Navbar: &common.NavbarVModel{
				User: user,
			},
		}

		profilePage := component.ProfilePage(vmodel)
		templ.Handler(profilePage).ServeHTTP(w, r)
		return
	}

	// Generate secure token
	tokenValue, err := crypto.GenerateSecureToken()
	if err != nil {
		slog.ErrorContext(ctx, "could not generate secure token", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	label, _ := tokenForm.GetFieldValue("label")

	// Create auth token
	authToken := model.NewAuthToken(user, label, tokenValue)
	if err := h.userStore.CreateAuthToken(ctx, authToken); err != nil {
		slog.ErrorContext(ctx, "could not create auth token", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Redirect back to profile page with success message
	http.Redirect(w, r, "/profile/?token_created="+tokenValue, http.StatusSeeOther)
}

func (h *Handler) deleteToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := httpCtx.User(ctx)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	tokenID := r.PathValue("tokenID")
	if tokenID == "" {
		http.Error(w, "Token ID is required", http.StatusBadRequest)
		return
	}

	// Verify the token belongs to the current user
	tokens, err := h.userStore.GetUserAuthTokens(ctx, user.ID())
	if err != nil {
		slog.ErrorContext(ctx, "could not fetch user auth tokens", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Check if token belongs to user
	found := false
	for _, token := range tokens {
		if string(token.ID()) == tokenID {
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "Token not found", http.StatusNotFound)
		return
	}

	// Delete the token
	if err := h.userStore.DeleteAuthToken(ctx, model.AuthTokenID(tokenID)); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, "Token not found", http.StatusNotFound)
			return
		}
		slog.ErrorContext(ctx, "could not delete auth token", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Redirect back to profile page
	http.Redirect(w, r, "/profile/?token_deleted=1", http.StatusSeeOther)
}

func (h *Handler) getForbiddenPage(w http.ResponseWriter, r *http.Request) {
	forbiddenPage := common.ErrorPage(common.ErrorPageVModel{
		Message: "L'accès à cette page ne vous est pas autorisé. Veuillez contacter l'administrateur.",
		Links: []common.LinkItem{
			{
				URL:   string(common.BaseURL(r.Context(), common.WithPath("/auth/oidc/logout"))),
				Label: "Se déconnecter",
			},
		},
	})

	w.WriteHeader(http.StatusForbidden)
	templ.Handler(forbiddenPage).ServeHTTP(w, r)
}

var _ http.Handler = &Handler{}
