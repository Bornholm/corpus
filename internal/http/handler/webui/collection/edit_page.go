package collection

import (
	"context"
	"net/http"

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

	_, err := h.documentManager.DocumentStore.UpdateCollection(ctx, collectionID, updates)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	http.Redirect(w, r, "/collections/", http.StatusSeeOther)
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
		h.fillCollectionEditPageVModelNavbar,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillCollectionEditPageVModelCollection(ctx context.Context, vmodel *component.CollectionEditPageVModel, r *http.Request) error {
	collectionID := model.CollectionID(r.PathValue("id"))

	collection, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID)
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.Collection = collection

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
