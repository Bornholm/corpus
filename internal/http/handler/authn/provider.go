package authn

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/pkg/errors"
)

func (h *Handler) handleProvider(w http.ResponseWriter, r *http.Request) {
	if _, err := gothic.CompleteUserAuth(w, r); err == nil {
		http.Redirect(w, r, "/auth/logout", http.StatusTemporaryRedirect)
	} else {
		gothic.BeginAuthHandler(w, r)
	}
}

func (h *Handler) handleProviderCallback(w http.ResponseWriter, r *http.Request) {
	gothUser, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		slog.ErrorContext(r.Context(), "could not complete user auth", slog.Any("error", errors.WithStack(err)))
		http.Redirect(w, r, "/auth/logout", http.StatusTemporaryRedirect)
		return
	}

	ctx := r.Context()

	slog.DebugContext(ctx, "authenticated user", slog.Any("user", gothUser))

	user := &User{
		Email:       gothUser.Email,
		Provider:    gothUser.Provider,
		Subject:     gothUser.UserID,
		AccessToken: gothUser.AccessToken,
		DisplayName: getUserDisplayName(gothUser),
	}

	if user.Email == "" {
		slog.ErrorContext(r.Context(), "could not authenticate user", slog.Any("error", errors.New("user email missing")))
		http.Redirect(w, r, "/auth/logout", http.StatusTemporaryRedirect)
		return
	}

	if user.Provider == "" {
		slog.ErrorContext(r.Context(), "could not authenticate user", slog.Any("error", errors.New("user provider missing")))
		http.Redirect(w, r, "/auth/logout", http.StatusTemporaryRedirect)
		return
	}

	if err := h.storeSessionUser(w, r, user); err != nil {
		slog.ErrorContext(r.Context(), "could not store session user", slog.Any("error", errors.WithStack(err)))
		http.Redirect(w, r, "/auth/logout", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	user, err := h.retrieveSessionUser(r)
	if err != nil && !errors.Is(err, errSessionNotFound) {
		log.Printf("[ERROR] %+v", errors.WithStack(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := h.clearSession(w, r); err != nil && !errors.Is(err, errSessionNotFound) {
		log.Printf("[ERROR] %+v", errors.WithStack(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if user == nil {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	redirectURL := fmt.Sprintf("/auth/providers/%s/logout", user.Provider)

	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (h *Handler) handleProviderLogout(w http.ResponseWriter, r *http.Request) {
	if err := gothic.Logout(w, r); err != nil {
		log.Printf("[ERROR] %+v", errors.WithStack(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func getUserDisplayName(user goth.User) string {
	var displayName string

	rawPreferredUsername, exists := user.RawData["preferred_username"]
	if exists {
		if preferredUsername, ok := rawPreferredUsername.(string); ok {
			displayName = preferredUsername
		}
	}

	if displayName == "" {
		displayName = user.NickName
	}

	if displayName == "" {
		displayName = user.Name
	}

	if displayName == "" {
		displayName = user.FirstName + " " + user.LastName
	}

	if displayName == "" {
		displayName = user.UserID
	}

	return displayName
}
