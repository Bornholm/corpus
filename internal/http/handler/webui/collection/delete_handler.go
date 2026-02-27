package collection

import (
	"log/slog"
	"net/http"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/pkg/errors"
)

func (h *Handler) handleCollectionDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	collectionID := model.CollectionID(r.PathValue("id"))
	if collectionID == "" {
		common.HandleError(w, r, errors.New("collection ID is required"))
		return
	}

	// Get the current user from context
	user := httpCtx.User(ctx)
	if user == nil {
		common.HandleError(w, r, errors.New("could not retrieve user from context"))
		return
	}

	// Get the collection to verify it exists and check ownership
	collection, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID, false)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, errors.New("collection not found"))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	// Only the collection owner can delete it (write-share recipients cannot)
	if collection.Owner().ID() != user.ID() {
		common.HandleError(w, r, errors.New("only the collection owner can delete it"))
		return
	}

	slog.InfoContext(ctx, "deleting collection",
		slog.String("collection_id", string(collectionID)),
		slog.String("collection_label", collection.Label()),
		slog.String("user_id", string(user.ID())))

	// Delete the collection (this will cascade delete the documents via GORM)
	if err := h.documentManager.DocumentStore.DeleteCollection(ctx, collectionID); err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	// Schedule index cleanup to remove orphaned entries
	// This is done asynchronously to avoid blocking the user
	if _, err := h.documentManager.CleanupIndex(ctx, user); err != nil {
		slog.ErrorContext(ctx, "failed to schedule index cleanup after collection deletion",
			slog.String("collection_id", string(collectionID)),
			slog.Any("error", err))
		// Don't fail the request if cleanup scheduling fails
	}

	slog.InfoContext(ctx, "collection deleted successfully",
		slog.String("collection_id", string(collectionID)))

	// Redirect back to collections list
	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/collections/"))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) handleDocumentDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	collectionID := model.CollectionID(r.PathValue("id"))
	if collectionID == "" {
		common.HandleError(w, r, errors.New("collection ID is required"))
		return
	}

	documentID := model.DocumentID(r.PathValue("docID"))
	if documentID == "" {
		common.HandleError(w, r, errors.New("document ID is required"))
		return
	}

	// Get the current user from context
	user := httpCtx.User(ctx)
	if user == nil {
		common.HandleError(w, r, errors.New("could not retrieve user from context"))
		return
	}

	// Check if user can write to the collection
	canWrite, err := h.documentManager.DocumentStore.CanWriteCollection(ctx, user.ID(), collectionID)
	if err != nil {
		common.HandleError(w, r, errors.Wrap(err, "could not check collection access"))
		return
	}

	if !canWrite {
		common.HandleError(w, r, errors.New("you don't have permission to delete documents in this collection"))
		return
	}

	slog.InfoContext(ctx, "deleting document",
		slog.String("collection_id", string(collectionID)),
		slog.String("document_id", string(documentID)),
		slog.String("user_id", string(user.ID())))

	// Delete the document
	if err := h.documentManager.DocumentStore.DeleteDocumentByID(ctx, documentID); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, errors.New("document not found"))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	slog.InfoContext(ctx, "document deleted successfully",
		slog.String("document_id", string(documentID)))

	// Redirect back to collection edit page
	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/collections", string(collectionID), "edit"))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}
