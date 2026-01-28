package admin

import (
	"context"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/admin/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/pkg/errors"
)

func (h *Handler) getTasksPage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillTasksPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	tasksPage := component.TasksPage(*vmodel)
	templ.Handler(tasksPage).ServeHTTP(w, r)
}

func (h *Handler) getTaskPage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillTaskPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	taskPage := component.TaskPage(*vmodel)
	templ.Handler(taskPage).ServeHTTP(w, r)
}

func (h *Handler) fillTasksPageViewModel(r *http.Request) (*component.TasksPageVModel, error) {
	vmodel := &component.TasksPageVModel{}
	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillTasksPageVModelNavbar,
		h.fillTasksPageVModelTasks,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillTaskPageViewModel(r *http.Request) (*component.TaskPageVModel, error) {
	vmodel := &component.TaskPageVModel{}
	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillTaskPageVModelNavbar,
		h.fillTaskPageVModelTask,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillTasksPageVModelNavbar(ctx context.Context, vmodel *component.TasksPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}

func (h *Handler) fillTasksPageVModelTasks(ctx context.Context, vmodel *component.TasksPageVModel, r *http.Request) error {
	// Parse pagination parameters
	page := 0
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p - 1 // Convert to 0-based
		}
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	taskHeaders, err := h.taskRunner.List(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	// Simple pagination logic
	start := page * limit
	end := start + limit
	if start > len(taskHeaders) {
		start = len(taskHeaders)
	}
	if end > len(taskHeaders) {
		end = len(taskHeaders)
	}

	var paginatedTasks []port.TaskStateHeader
	if start < len(taskHeaders) {
		paginatedTasks = taskHeaders[start:end]
	} else {
		paginatedTasks = make([]port.TaskStateHeader, 0)
	}

	vmodel.Tasks = paginatedTasks
	vmodel.CurrentPage = page + 1 // Convert back to 1-based
	vmodel.PageSize = limit
	vmodel.TotalTasks = len(taskHeaders)

	return nil
}

func (h *Handler) fillTaskPageVModelNavbar(ctx context.Context, vmodel *component.TaskPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}

func (h *Handler) fillTaskPageVModelTask(ctx context.Context, vmodel *component.TaskPageVModel, r *http.Request) error {
	taskID := model.TaskID(r.PathValue("id"))
	if taskID == "" {
		return errors.New("task ID is required")
	}

	taskState, err := h.taskRunner.State(ctx, taskID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return errors.New("task not found")
		}
		return errors.WithStack(err)
	}

	vmodel.Task = taskState

	return nil
}
