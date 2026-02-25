package collection

import (
	"context"
	"log/slog"
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

func (h *Handler) getCollectionListPage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillCollectionListPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	listPage := component.CollectionListPage(*vmodel)

	templ.Handler(listPage).ServeHTTP(w, r)
}

func (h *Handler) fillCollectionListPageViewModel(r *http.Request) (*component.CollectionListPageVModel, error) {
	vmodel := &component.CollectionListPageVModel{}

	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillCollectionListPageVModelCollections,
		h.fillCollectionListPageVModelNavbar,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillCollectionListPageVModelCollections(ctx context.Context, vmodel *component.CollectionListPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)

	readableCollections, total, err := h.documentManager.DocumentStore.QueryUserReadableCollections(ctx, user.ID(), port.QueryCollectionsOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.CollectionStats = make(map[model.CollectionID]*model.CollectionStats)

	for _, c := range readableCollections {
		stats, err := h.documentManager.DocumentStore.GetCollectionStats(ctx, c.ID())
		if err != nil {
			slog.WarnContext(ctx, "could not get collection stats", slog.Any("error", err), slog.String("collection_id", string(c.ID())))
			// Continue without stats rather than failing
			vmodel.CollectionStats[c.ID()] = &model.CollectionStats{TotalDocuments: 0}
		} else {
			vmodel.CollectionStats[c.ID()] = stats
		}
	}

	vmodel.Collections = readableCollections
	vmodel.TotalCollections = total
	vmodel.CurrentUserID = user.ID()

	return nil
}

func (h *Handler) fillCollectionListPageVModelNavbar(ctx context.Context, vmodel *component.CollectionListPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}
