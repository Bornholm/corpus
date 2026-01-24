package collection

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/pkg/errors"
)

func (h *Handler) handleCollectionDelete(w http.ResponseWriter, r *http.Request) {
	collectionID := model.CollectionID(r.PathValue("id"))
	if collectionID == "" {
		common.HandleError(w, r, errors.New("collection ID is required"))
		return
	}

	// Note: This is a simple delete handler. In a real application, you might want to:
	// 1. Check if the user owns the collection
	// 2. Handle documents that are associated with this collection
	// 3. Add confirmation dialog
	// 4. Implement soft delete instead of hard delete

	// For now, we'll just return an error since the DocumentStore interface
	// doesn't have a DeleteCollection method
	common.HandleError(w, r, errors.New("collection deletion not implemented yet"))
}
