package collection

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/collection/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/pkg/errors"
)

func (h *Handler) getCollectionEditPage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillCollectionEditPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	editPage := component.CollectionEditPage(*vmodel)

	templ.Handler(editPage).ServeHTTP(w, r)
}

func (h *Handler) handleCollectionUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	collectionID := model.CollectionID(r.PathValue("id"))
	if collectionID == "" {
		common.HandleError(w, r, errors.New("collection ID is required"))
		return
	}

	currentUser := httpCtx.User(ctx)
	if currentUser == nil {
		common.HandleError(w, r, errors.New("could not retrieve user from context"))
		return
	}

	// Only owner or write-share recipients can update collection metadata
	canWrite, err := h.documentManager.DocumentStore.CanWriteCollection(ctx, currentUser.ID(), collectionID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, errors.New("collection not found"))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}
	if !canWrite {
		common.HandleError(w, r, errors.New("vous n'avez pas la permission de modifier cette collection"))
		return
	}

	if err := r.ParseForm(); err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	label := r.FormValue("label")
	description := r.FormValue("description")

	if label == "" {
		common.HandleError(w, r, errors.New("label is required"))
		return
	}

	updates := port.CollectionUpdates{
		Label:       &label,
		Description: &description,
	}

	_, err = h.documentManager.DocumentStore.UpdateCollection(ctx, collectionID, updates)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/collections/"))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) handleCollectionShareCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	collectionID := model.CollectionID(r.PathValue("id"))
	if collectionID == "" {
		common.HandleError(w, r, errors.New("collection ID is required"))
		return
	}

	currentUser := httpCtx.User(ctx)
	if currentUser == nil {
		common.HandleError(w, r, errors.New("could not retrieve user from context"))
		return
	}

	// Only the owner can create shares
	collection, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID, false)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, errors.New("collection not found"))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	if collection.Owner().ID() != currentUser.ID() {
		common.HandleError(w, r, errors.New("only the collection owner can manage shares"))
		return
	}

	if err := r.ParseForm(); err != nil {
		common.HandleError(w, r, errors.WithStack(err))
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

	_, err = h.documentManager.DocumentStore.CreateCollectionShare(ctx, collectionID, targetUserID, level)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	slog.InfoContext(ctx, "collection share created",
		slog.String("collection_id", string(collectionID)),
		slog.String("shared_with_user_id", string(targetUserID)),
		slog.String("level", string(level)),
	)

	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/collections", string(collectionID), "edit"))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) handleCollectionShareDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	collectionID := model.CollectionID(r.PathValue("id"))
	shareID := model.CollectionShareID(r.PathValue("shareID"))

	if collectionID == "" {
		common.HandleError(w, r, errors.New("collection ID is required"))
		return
	}
	if shareID == "" {
		common.HandleError(w, r, errors.New("share ID is required"))
		return
	}

	currentUser := httpCtx.User(ctx)
	if currentUser == nil {
		common.HandleError(w, r, errors.New("could not retrieve user from context"))
		return
	}

	// Only the owner can delete shares
	collection, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID, false)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, errors.New("collection not found"))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	if collection.Owner().ID() != currentUser.ID() {
		common.HandleError(w, r, errors.New("only the collection owner can manage shares"))
		return
	}

	if err := h.documentManager.DocumentStore.DeleteCollectionShare(ctx, shareID); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, errors.New("share not found"))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	slog.InfoContext(ctx, "collection share deleted",
		slog.String("collection_id", string(collectionID)),
		slog.String("share_id", string(shareID)),
	)

	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/collections", string(collectionID), "edit"))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) fillCollectionEditPageViewModel(r *http.Request) (*component.CollectionEditPageVModel, error) {
	vmodel := &component.CollectionEditPageVModel{}

	ctx := r.Context()

	collectionID := model.CollectionID(r.PathValue("id"))
	if collectionID == "" {
		return nil, errors.New("collection ID is required")
	}

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillCollectionEditPageVModelCollection,
		h.fillCollectionEditPageVModelShares,
		h.fillCollectionEditPageVModelDocuments,
		h.fillCollectionEditPageVModelNavbar,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillCollectionEditPageVModelCollection(ctx context.Context, vmodel *component.CollectionEditPageVModel, r *http.Request) error {
	collectionID := model.CollectionID(r.PathValue("id"))

	collection, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID, false)
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.Collection = collection

	user := httpCtx.User(ctx)
	if user != nil {
		vmodel.IsOwner = collection.Owner().ID() == user.ID()

		canWrite, err := h.documentManager.DocumentStore.CanWriteCollection(ctx, user.ID(), collectionID)
		if err != nil && !errors.Is(err, port.ErrNotFound) {
			return errors.WithStack(err)
		}
		vmodel.IsWritable = canWrite
	}

	return nil
}

func (h *Handler) fillCollectionEditPageVModelShares(ctx context.Context, vmodel *component.CollectionEditPageVModel, r *http.Request) error {
	// Only fetch share data for owners
	if !vmodel.IsOwner {
		return nil
	}

	collectionID := model.CollectionID(r.PathValue("id"))

	shares, err := h.documentManager.DocumentStore.GetCollectionShares(ctx, collectionID)
	if err != nil {
		return errors.WithStack(err)
	}
	vmodel.Shares = shares

	// Build set of already-shared user IDs (plus the owner)
	sharedUserIDs := make(map[model.UserID]struct{})
	user := httpCtx.User(ctx)
	if user != nil {
		sharedUserIDs[user.ID()] = struct{}{} // exclude owner
	}
	for _, s := range shares {
		sharedUserIDs[s.SharedWith().ID()] = struct{}{}
	}

	// Fetch all active users to offer as potential share targets
	allUsers, err := h.userStore.QueryUsers(ctx, port.QueryUsersOptions{})
	if err != nil {
		slog.WarnContext(ctx, "could not fetch users for share form", slog.Any("error", err))
		return nil
	}

	available := make([]model.User, 0)
	for _, u := range allUsers {
		if _, excluded := sharedUserIDs[u.ID()]; !excluded {
			available = append(available, u)
		}
	}
	vmodel.AvailableUsers = available

	return nil
}

func (h *Handler) fillCollectionEditPageVModelNavbar(ctx context.Context, vmodel *component.CollectionEditPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}

func (h *Handler) fillCollectionEditPageVModelDocuments(ctx context.Context, vmodel *component.CollectionEditPageVModel, r *http.Request) error {
	collectionID := model.CollectionID(r.PathValue("id"))

	// Get page from query params, default to 0
	page := 0
	pageSize := 10

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	opts := port.QueryDocumentsOptions{
		Page:  &page,
		Limit: &pageSize,
	}

	documents, total, err := h.documentManager.DocumentStore.QueryDocumentsByCollectionID(ctx, collectionID, opts)
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.Documents = documents
	vmodel.CurrentPage = page
	vmodel.PageSize = pageSize
	vmodel.TotalDocuments = total
	if pageSize > 0 {
		vmodel.TotalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	} else {
		vmodel.TotalPages = 0
	}

	return nil
}
