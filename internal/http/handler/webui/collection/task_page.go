package collection

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/collection/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/pkg/errors"
)

func (h *Handler) getTaskPage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillTaskPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	taskPage := component.TaskPage(*vmodel)

	templ.Handler(taskPage).ServeHTTP(w, r)
}

func (h *Handler) fillTaskPageViewModel(r *http.Request) (*component.TaskPageVModel, error) {
	vmodel := &component.TaskPageVModel{}

	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillTaskPageVModelTask,
		h.fillTaskPageVModelAppLayout,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillTaskPageVModelTask(ctx context.Context, vmodel *component.TaskPageVModel, r *http.Request) error {
	taskID := model.TaskID(r.PathValue("taskID"))

	taskState, err := h.taskRunner.GetTaskState(ctx, taskID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return errors.WithStack(common.NewError(err.Error(), "La tâche n'a pas pu être trouvée.", http.StatusNotFound))
		}

		return errors.WithStack(err)
	}

	vmodel.Task = taskState

	return nil
}

func (h *Handler) fillTaskPageVModelAppLayout(ctx context.Context, vmodel *component.TaskPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.AppLayoutVModel = commonComp.AppLayoutVModel{
		User:         user,
		SelectedItem: "collections",
		NavigationItems: func(vmodel commonComp.AppLayoutVModel) templ.Component {
			return commonComp.AppNavigationItems(vmodel)
		},
		FooterItems: func(vmodel commonComp.AppLayoutVModel) templ.Component {
			return commonComp.AppFooterItems(vmodel)
		},
	}

	return nil
}
