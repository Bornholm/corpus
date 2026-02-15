package pubshare

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/bornholm/corpus/internal/http/handler/webui/pubshare/component"
	corpusLLM "github.com/bornholm/corpus/internal/llm"
	"github.com/bornholm/corpus/internal/metrics"
	"github.com/bornholm/genai/llm"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *Handler) getPublicSharePage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillPublicSharePageVModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	askPage := component.PublicSharePage(*vmodel)

	templ.Handler(askPage).ServeHTTP(w, r)
}

func (h *Handler) handleAsk(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	vmodel, err := h.fillPublicSharePageVModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	renderPage := func() {
		vmodel.Duration = time.Since(start)
		askPage := component.PublicSharePage(*vmodel)
		templ.Handler(askPage).ServeHTTP(w, r)
	}

	if vmodel.Query == "" {
		renderPage()
		return
	}

	metrics.PublicShareTotalQuestions.With(prometheus.Labels{
		metrics.LabelPublicShareID: string(vmodel.PublicShare.ID()),
	}).Add(1)

	incrementFailures := func() {
		metrics.PublicShareFailedQuestions.With(prometheus.Labels{
			metrics.LabelPublicShareID: string(vmodel.PublicShare.ID()),
		}).Add(1)
	}

	ctx := r.Context()

	ctx = corpusLLM.WithHighPriority(ctx)

	searchOptions := make([]service.DocumentManagerSearchOptionFunc, 0)

	collectionIDs := slices.Collect(func(yield func(id model.CollectionID) bool) {
		for _, c := range vmodel.PublicShare.Collections() {
			if !yield(c.ID()) {
				return
			}
		}
	})

	searchOptions = append(searchOptions, service.WithDocumentManagerSearchCollections(collectionIDs...))

	results, err := h.documentManager.Search(ctx, vmodel.Query, searchOptions...)
	if err != nil && !errors.Is(err, service.ErrNoResults) {
		defer incrementFailures()

		if errors.Is(err, llm.ErrRateLimit) {
			common.HandleError(w, r, common.NewError(err.Error(), "Service surchargé. Veuillez réessayer ultérieurement.", http.StatusServiceUnavailable))
			return
		}

		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	vmodel.Results = results

	if len(results) > 0 {
		response, contents, err := h.documentManager.Ask(ctx, vmodel.Query, results)
		if err != nil {
			defer incrementFailures()

			if errors.Is(err, llm.ErrRateLimit) {
				common.HandleError(w, r, common.NewError(err.Error(), "Service surchargé. Veuillez réessayer ultérieurement.", http.StatusServiceUnavailable))
				return
			}

			common.HandleError(w, r, errors.WithStack(err))
			return
		}

		vmodel.Response = response
		vmodel.SectionContents = contents

		metrics.PublicShareSucceededQuestions.With(prometheus.Labels{
			metrics.LabelPublicShareID: string(vmodel.PublicShare.ID()),
		}).Add(1)
	} else {
		metrics.PublicShareUnansweredQuestions.With(prometheus.Labels{
			metrics.LabelPublicShareID: string(vmodel.PublicShare.ID()),
		}).Add(1)
	}

	renderPage()
}

func (h *Handler) fillPublicSharePageVModel(r *http.Request) (*component.PublicSharePageVModel, error) {
	vmodel := &component.PublicSharePageVModel{}

	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillPublicShareVModelQuery,
		h.fillPublicShareVModelPublicShare,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillPublicShareVModelPublicShare(ctx context.Context, vmodel *component.PublicSharePageVModel, r *http.Request) error {
	publicShare := ctxPubShare(ctx)

	vmodel.PublicShare = publicShare

	return nil
}

func (h *Handler) fillPublicShareVModelQuery(ctx context.Context, vmodel *component.PublicSharePageVModel, r *http.Request) error {
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
