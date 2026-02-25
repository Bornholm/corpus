package admin

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/admin/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/pkg/errors"
)

func (h *Handler) getCollectionSharesPage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillCollectionSharesPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	page := component.CollectionSharesPage(*vmodel)
	templ.Handler(page).ServeHTTP(w, r)
}

func (h *Handler) postCollectionShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	collectionID := model.CollectionID(r.FormValue("collection_id"))
	if collectionID == "" {
		common.HandleError(w, r, errors.New("collection_id is required"))
		return
	}

	targetUserID := model.UserID(r.FormValue("user_id"))
	if targetUserID == "" {
		common.HandleError(w, r, errors.New("user_id is required"))
		return
	}

	levelStr := r.FormValue("level")
	var level model.CollectionShareLevel
	switch levelStr {
	case string(model.CollectionShareLevelRead):
		level = model.CollectionShareLevelRead
	case string(model.CollectionShareLevelWrite):
		level = model.CollectionShareLevelWrite
	default:
		common.HandleError(w, r, errors.New("invalid level: must be 'read' or 'write'"))
		return
	}

	_, err := h.documentStore.CreateCollectionShare(ctx, collectionID, targetUserID, level)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/collection-shares"))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) handleCollectionShareDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	shareID := model.CollectionShareID(r.PathValue("id"))
	if shareID == "" {
		common.HandleError(w, r, errors.New("share ID is required"))
		return
	}

	if err := h.documentStore.DeleteCollectionShare(ctx, shareID); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, errors.New("share not found"))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/collection-shares"))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) fillCollectionSharesPageViewModel(r *http.Request) (*component.CollectionSharesPageVModel, error) {
	vmodel := &component.CollectionSharesPageVModel{}
	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillCollectionSharesPageVModelNavbar,
		h.fillCollectionSharesPageVModelData,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillCollectionSharesPageVModelNavbar(ctx context.Context, vmodel *component.CollectionSharesPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}

func (h *Handler) fillCollectionSharesPageVModelData(ctx context.Context, vmodel *component.CollectionSharesPageVModel, r *http.Request) error {
	// Fetch all collections
	collections, err := h.documentStore.QueryCollections(ctx, port.QueryCollectionsOptions{})
	if err != nil {
		return errors.WithStack(err)
	}
	vmodel.Collections = collections

	// Fetch shares for each collection
	type collectionWithShares struct {
		Collection model.PersistedCollection
		Shares     []model.PersistedCollectionShare
	}

	entries := make([]component.CollectionShareEntry, 0)
	for _, coll := range collections {
		shares, err := h.documentStore.GetCollectionShares(ctx, coll.ID())
		if err != nil {
			return errors.WithStack(err)
		}
		entries = append(entries, component.CollectionShareEntry{
			Collection: coll,
			Shares:     shares,
		})
	}
	vmodel.Entries = entries

	// Fetch all users for the add-share form
	users, err := h.userStore.QueryUsers(ctx, port.QueryUsersOptions{})
	if err != nil {
		return errors.WithStack(err)
	}
	vmodel.AllUsers = users

	return nil
}
