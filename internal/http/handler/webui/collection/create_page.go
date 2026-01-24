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

func (h *Handler) getCollectionCreatePage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillCollectionCreatePageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	createPage := component.CollectionCreatePage(*vmodel)

	templ.Handler(createPage).ServeHTTP(w, r)
}

func (h *Handler) handleCollectionCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	user := httpCtx.User(ctx)
	if user == nil {
		common.HandleError(w, r, errors.New("user not found in context"))
		return
	}

	label := r.FormValue("label")
	description := r.FormValue("description")

	if label == "" {
		common.HandleError(w, r, errors.New("label is required"))
		return
	}

	_, err := h.documentManager.DocumentStore.CreateCollection(ctx, user.ID(), label)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	// Update collection with description if provided
	if description != "" {
		collections, err := h.documentManager.DocumentStore.QueryCollections(ctx, port.QueryCollectionsOptions{})
		if err != nil {
			common.HandleError(w, r, errors.WithStack(err))
			return
		}

		// Find the newly created collection (it should be the last one with matching label and owner)
		var newCollection model.Collection
		for _, c := range collections {
			if c.Label() == label && c.OwnerID() == user.ID() {
				newCollection = c
			}
		}

		if newCollection != nil {
			_, err = h.documentManager.DocumentStore.UpdateCollection(ctx, newCollection.ID(), port.CollectionUpdates{
				Description: &description,
			})
			if err != nil {
				common.HandleError(w, r, errors.WithStack(err))
				return
			}
		}
	}

	http.Redirect(w, r, "/collections/", http.StatusSeeOther)
}

func (h *Handler) fillCollectionCreatePageViewModel(r *http.Request) (*component.CollectionCreatePageVModel, error) {
	vmodel := &component.CollectionCreatePageVModel{}

	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillCollectionCreatePageVModelNavbar,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillCollectionCreatePageVModelNavbar(ctx context.Context, vmodel *component.CollectionCreatePageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}
