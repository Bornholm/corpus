package ask

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/http/handler/webui/ask/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
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
