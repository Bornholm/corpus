package pubshare

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/pubshare/component"
	corpusLLM "github.com/bornholm/corpus/internal/llm"
	"github.com/bornholm/corpus/internal/metrics"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *Handler) getPublicSharePage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillPublicSharePageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	publicSharePage := component.PublicSharePage(*vmodel)
	templ.Handler(publicSharePage).ServeHTTP(w, r)
}

func (h *Handler) handleAsk(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	vmodel, err := h.fillPublicSharePageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	renderPage := func() {
		vmodel.Duration = time.Since(start)
		publicSharePage := component.PublicSharePage(*vmodel)
		templ.Handler(publicSharePage).ServeHTTP(w, r)
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

	collectionIDs := slices.Collect(func(yield func(id model.CollectionID) bool) {
		for _, c := range vmodel.PublicShare.Collections() {
			if !yield(c.ID()) {
				return
			}
		}
	})

	result, err := h.documentManager.AskWithRetrieval(ctx, vmodel.Query, collectionIDs)
	if err != nil {
		defer incrementFailures()

		if corpusLLM.IsRateLimit(err) {
			common.HandleError(w, r, common.NewError(err.Error(), "Service surchargé. Veuillez réessayer ultérieurement.", http.StatusServiceUnavailable))
			return
		}

		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	vmodel.Results = result.Results

	if result.Grounding != nil {
		vmodel.Grounding = &commonComp.GroundingVModel{
			Status:      string(result.Grounding.Status),
			Score:       result.Grounding.Score,
			Explanation: result.Grounding.Explanation,
		}
	}

	if len(result.Results) > 0 {
		vmodel.Response = result.Answer
		vmodel.SectionContents = result.Contents

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

func (h *Handler) fillPublicSharePageViewModel(r *http.Request) (*component.PublicSharePageVModel, error) {
	vmodel := &component.PublicSharePageVModel{}
	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillPublicSharePageVModelPublicShare,
		h.fillPublicSharePageVModelQuery,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillPublicSharePageVModelPublicShare(ctx context.Context, vmodel *component.PublicSharePageVModel, r *http.Request) error {
	publicShare := ctxPubShare(ctx)

	vmodel.PublicShare = publicShare

	return nil
}

func (h *Handler) fillPublicSharePageVModelQuery(ctx context.Context, vmodel *component.PublicSharePageVModel, r *http.Request) error {
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
