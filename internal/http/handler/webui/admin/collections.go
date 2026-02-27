package admin

import (
	"context"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/admin/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/pkg/errors"
)

func (h *Handler) getCollectionsPage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillCollectionsPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	collectionsPage := component.CollectionsPage(*vmodel)
	templ.Handler(collectionsPage).ServeHTTP(w, r)
}

func (h *Handler) getCollectionPage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillCollectionPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	collectionPage := component.CollectionPage(*vmodel)
	templ.Handler(collectionPage).ServeHTTP(w, r)
}

func (h *Handler) postReindexCollectionFromCollectionPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get the user from context
	user := httpCtx.User(ctx)
	if user == nil {
		common.HandleError(w, r, errors.New("could not retrieve user from context"))
		return
	}

	collectionID := model.CollectionID(r.PathValue("id"))
	if collectionID == "" {
		common.HandleError(w, r, errors.New("collection ID is required"))
		return
	}

	// Schedule the reindex task
	taskID, err := h.documentManager.ReindexCollection(ctx, user, collectionID)
	if err != nil {
		common.HandleError(w, r, errors.Wrap(err, "could not schedule reindex task"))
		return
	}

	// Redirect to the task page
	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/tasks", string(taskID)))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) fillCollectionsPageViewModel(r *http.Request) (*component.CollectionsPageVModel, error) {
	vmodel := &component.CollectionsPageVModel{}
	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillCollectionsPageVModelNavbar,
		h.fillCollectionsPageVModelCollections,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillCollectionPageViewModel(r *http.Request) (*component.CollectionPageVModel, error) {
	vmodel := &component.CollectionPageVModel{}
	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillCollectionPageVModelNavbar,
		h.fillCollectionPageVModelCollection,
		h.fillCollectionPageVModelShares,
		h.fillCollectionPageVModelDocuments,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	collectionID := model.CollectionID(r.PathValue("id"))
	if collectionID == "" {
		common.HandleError(w, r, errors.New("collection ID is required"))
		return
	}

	// Delete the collection
	if err := h.documentStore.DeleteCollection(ctx, collectionID); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, errors.New("collection not found"))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	// Redirect to collections list
	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/collections"))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) fillCollectionsPageVModelNavbar(ctx context.Context, vmodel *component.CollectionsPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}

func (h *Handler) fillCollectionsPageVModelCollections(ctx context.Context, vmodel *component.CollectionsPageVModel, r *http.Request) error {
	// Parse pagination parameters
	page := 0
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p - 1 // Convert to 0-based
		}
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	opts := port.QueryCollectionsOptions{
		Page:  &page,
		Limit: &limit,
	}

	collections, err := h.documentStore.QueryCollections(ctx, opts)
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.Collections = collections
	vmodel.CurrentPage = page + 1 // Convert back to 1-based
	vmodel.PageSize = limit

	return nil
}

func (h *Handler) fillCollectionPageVModelNavbar(ctx context.Context, vmodel *component.CollectionPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}

func (h *Handler) fillCollectionPageVModelCollection(ctx context.Context, vmodel *component.CollectionPageVModel, r *http.Request) error {
	collectionID := model.CollectionID(r.PathValue("id"))
	if collectionID == "" {
		return errors.New("collection ID is required")
	}

	collection, err := h.documentStore.GetCollectionByID(ctx, collectionID, true)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return common.NewHTTPError(http.StatusNotFound)
		}
		return errors.WithStack(err)
	}

	stats, err := h.documentStore.GetCollectionStats(ctx, collectionID)
	if err != nil {
		return errors.WithStack(err)
	}

	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	// Check if current user is the owner
	isOwner := collection.Owner().ID() == user.ID()

	vmodel.Collection = collection
	vmodel.Stats = stats
	vmodel.IsOwner = isOwner

	return nil
}

func (h *Handler) fillCollectionPageVModelShares(ctx context.Context, vmodel *component.CollectionPageVModel, r *http.Request) error {
	collectionID := model.CollectionID(r.PathValue("id"))
	if collectionID == "" {
		return errors.New("collection ID is required")
	}

	// Fetch shares for this collection
	shares, err := h.documentStore.GetCollectionShares(ctx, collectionID)
	if err != nil {
		return errors.WithStack(err)
	}
	vmodel.Shares = shares

	// Fetch all users for the add-share form
	users, err := h.userStore.QueryUsers(ctx, port.QueryUsersOptions{})
	if err != nil {
		return errors.WithStack(err)
	}
	vmodel.AllUsers = users

	return nil
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

	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/collections", string(collectionID)))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) handleCollectionShareDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	shareID := model.CollectionShareID(r.PathValue("id"))
	if shareID == "" {
		common.HandleError(w, r, errors.New("share ID is required"))
		return
	}

	// Get collection ID before deleting (we need it for redirect)
	var collectionID model.CollectionID
	collections, err := h.documentStore.QueryCollections(ctx, port.QueryCollectionsOptions{})
	if err == nil {
		for _, coll := range collections {
			shares, err := h.documentStore.GetCollectionShares(ctx, coll.ID())
			if err == nil {
				for _, s := range shares {
					if s.ID() == shareID {
						collectionID = coll.ID()
						break
					}
				}
			}
			if collectionID != "" {
				break
			}
		}
	}

	if err := h.documentStore.DeleteCollectionShare(ctx, shareID); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, errors.New("share not found"))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	// Redirect back to collection page
	if collectionID != "" {
		redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/collections", string(collectionID)))
		http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
		return
	}

	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/collections"))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) fillCollectionPageVModelDocuments(ctx context.Context, vmodel *component.CollectionPageVModel, r *http.Request) error {
	collectionID := model.CollectionID(r.PathValue("id"))
	if collectionID == "" {
		return errors.New("collection ID is required")
	}

	// Parse pagination parameters
	page := 0
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p - 1 // Convert to 0-based
		}
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	opts := port.QueryDocumentsOptions{
		Page:  &page,
		Limit: &limit,
	}

	documents, total, err := h.documentStore.QueryDocumentsByCollectionID(ctx, collectionID, opts)
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.Documents = documents
	vmodel.CurrentPage = page + 1 // Convert back to 1-based
	vmodel.PageSize = limit
	vmodel.TotalDocuments = total
	vmodel.TotalPages = int(total) / limit
	if int(total)%limit > 0 {
		vmodel.TotalPages++
	}

	return nil
}
