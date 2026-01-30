package ask

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/ask/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/bornholm/corpus/internal/llm"
	"github.com/pkg/errors"
)

func (h *Handler) getAskPage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillAskPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	askPage := component.AskPage(*vmodel)

	templ.Handler(askPage).ServeHTTP(w, r)
}

func (h *Handler) handleAsk(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	vmodel, err := h.fillAskPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	renderPage := func() {
		vmodel.Duration = time.Since(start)
		askPage := component.AskPage(*vmodel)
		templ.Handler(askPage).ServeHTTP(w, r)
	}

	if vmodel.Query == "" {
		renderPage()
		return
	}

	ctx := r.Context()

	ctx = llm.WithHighPriority(ctx)

	searchOptions := make([]service.DocumentManagerSearchOptionFunc, 0)

	collections, err := h.getReadableCollections(ctx, r.Form["collection"])
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	searchOptions = append(searchOptions, service.WithDocumentManagerSearchCollections(collections...))

	results, err := h.documentManager.Search(ctx, vmodel.Query, searchOptions...)
	if err != nil && !errors.Is(err, service.ErrNoResults) {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	vmodel.Results = results

	if len(results) > 0 {
		response, contents, err := h.documentManager.Ask(ctx, vmodel.Query, results)
		if err != nil {
			common.HandleError(w, r, errors.WithStack(err))
			return
		}

		vmodel.Response = response
		vmodel.SectionContents = contents
	}

	renderPage()
}

func (h *Handler) fillAskPageViewModel(r *http.Request) (*component.AskPageVModel, error) {
	vmodel := &component.AskPageVModel{}

	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillAskPageVModelTotalDocuments,
		h.fillAskPageVModelQuery,
		h.fillAskPageVModelFileUploadModal,
		h.fillAskPageVModelSelectedCollectionIDs,
		h.fillAskPageVModelCollections,
		h.fillAskPageVModelNavbar,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillAskPageVModelTotalDocuments(ctx context.Context, vmodel *component.AskPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	total, err := h.documentManager.CountReadableDocuments(ctx, user.ID())
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.TotalDocuments = total

	return nil
}

func (h *Handler) fillAskPageVModelQuery(ctx context.Context, vmodel *component.AskPageVModel, r *http.Request) error {
	if r.Method != http.MethodPost {
		return nil
	}

	if err := r.ParseForm(); err != nil {
		slog.ErrorContext(ctx, "could not parse form", slog.Any("error", errors.WithStack(err)))
		return nil
	}

	vmodel.Query = r.FormValue("q")

	return nil
}

func (h *Handler) fillAskPageVModelFileUploadModal(ctx context.Context, vmodel *component.AskPageVModel, r *http.Request) error {
	enabled := r.URL.Query().Get("action") == "upload"
	if !enabled {
		return nil
	}

	user := httpCtx.User(ctx)
	writableCollections, _, err := h.documentManager.DocumentStore.QueryUserWritableCollections(ctx, user.ID(), port.QueryCollectionsOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	baseURL := httpCtx.BaseURL(ctx)
	createCollectionURL := baseURL.JoinPath("/collections/create")

	vmodel.UploadFileModal = &component.UploadFileModalVModel{
		SupportedExtensions: h.documentManager.SupportedExtensions(),
		WritableCollections: writableCollections,
		CreateCollectionURL: createCollectionURL.String(),
	}

	return nil
}

func (h *Handler) fillAskPageVModelCollections(ctx context.Context, vmodel *component.AskPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	collections, _, err := h.documentManager.DocumentStore.QueryUserReadableCollections(ctx, user.ID(), port.QueryCollectionsOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.CollectionStats = make(map[model.CollectionID]*model.CollectionStats)

	for _, c := range collections {
		stats, err := h.documentManager.DocumentStore.GetCollectionStats(ctx, c.ID())
		if err != nil {
			return errors.WithStack(err)
		}

		vmodel.CollectionStats[c.ID()] = stats
	}

	slices.SortFunc(collections, func(c1, c2 model.PersistedCollection) int {
		selected1 := slices.Contains(vmodel.SelectedCollections, c1.ID())
		selected2 := slices.Contains(vmodel.SelectedCollections, c2.ID())

		if selected1 && !selected2 {
			return -1
		}
		if selected2 && !selected1 {
			return 1
		}

		stats1 := vmodel.CollectionStats[c1.ID()]
		stats2 := vmodel.CollectionStats[c2.ID()]

		return int(stats2.TotalDocuments - stats1.TotalDocuments)
	})

	vmodel.Collections = collections

	return nil
}

func (h *Handler) fillAskPageVModelSelectedCollectionIDs(ctx context.Context, vmodel *component.AskPageVModel, r *http.Request) error {
	rawCollections, exists := r.URL.Query()["collection"]
	if !exists {
		return nil
	}

	collections := slices.Collect(func(yield func(model.CollectionID) bool) {
		for _, rawCollectionID := range rawCollections {
			if !yield(model.CollectionID(rawCollectionID)) {
				return
			}
		}
	})

	vmodel.SelectedCollections = collections

	return nil
}

func (h *Handler) fillAskPageVModelNavbar(ctx context.Context, vmodel *component.AskPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}

func (h *Handler) getReadableCollections(ctx context.Context, rawCollections []string) ([]model.CollectionID, error) {
	user := httpCtx.User(ctx)

	readableCollections, _, err := h.documentManager.DocumentStore.QueryUserReadableCollections(ctx, user.ID(), port.QueryCollectionsOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(readableCollections) == 0 {
		return nil, common.NewError("no collection", "VOus n'avez pas encore d'accès à une collection.", http.StatusBadRequest)
	}

	collections := make([]model.CollectionID, 0)

	if len(rawCollections) > 0 {
		for _, rawCollectionID := range rawCollections {
			collectionID := model.CollectionID(rawCollectionID)

			isWritable := slices.ContainsFunc(readableCollections, func(c model.PersistedCollection) bool {
				return collectionID == c.ID()
			})

			if !isWritable {
				return nil, common.NewHTTPError(http.StatusForbidden)
			}

			collections = append(collections, collectionID)
		}
	} else {
		collections = slices.Collect(func(yield func(model.CollectionID) bool) {
			for _, c := range readableCollections {
				if !yield(c.ID()) {
					return
				}
			}
		})
	}

	return collections, nil
}
