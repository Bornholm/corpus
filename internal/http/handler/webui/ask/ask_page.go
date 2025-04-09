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
	"github.com/bornholm/corpus/internal/http/handler/webui/ask/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
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

	searchOptions := make([]service.DocumentManagerSearchOptionFunc, 0)

	if collections, exists := r.Form["collection"]; exists {
		searchOptions = append(searchOptions, service.WithDocumentManagerSearchCollections(collections...))
	}

	results, err := h.documentManager.Search(ctx, vmodel.Query, searchOptions...)
	if err != nil {
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
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillAskPageVModelTotalDocuments(ctx context.Context, vmodel *component.AskPageVModel, r *http.Request) error {
	total, err := h.documentManager.CountDocuments(ctx)
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

	vmodel.UploadFileModal = &component.UploadFileModalVModel{
		SupportedExtensions: h.documentManager.SupportedExtensions(),
	}

	return nil
}

func (h *Handler) fillAskPageVModelCollections(ctx context.Context, vmodel *component.AskPageVModel, r *http.Request) error {
	collections, err := h.documentManager.Store.QueryCollections(ctx, port.QueryCollectionsOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.CollectionStats = make(map[model.CollectionID]*model.CollectionStats)

	for _, c := range collections {
		stats, err := h.documentManager.Store.GetCollectionStats(ctx, c.ID())
		if err != nil {
			return errors.WithStack(err)
		}

		vmodel.CollectionStats[c.ID()] = stats
	}

	slices.SortFunc(collections, func(c1, c2 model.Collection) int {
		selected1 := slices.Contains(vmodel.SelectedCollectionNames, c1.Name())
		selected2 := slices.Contains(vmodel.SelectedCollectionNames, c2.Name())

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
	collections, exists := r.URL.Query()["collection"]
	if !exists {
		return nil
	}

	vmodel.SelectedCollectionNames = collections

	return nil
}
