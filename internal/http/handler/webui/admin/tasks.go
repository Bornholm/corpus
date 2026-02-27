package admin

import (
	"cmp"
	"context"
	"net/http"
	"slices"
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
		h.fillTasksPageVModelCollections,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillTasksPageVModelCollections(ctx context.Context, vmodel *component.TasksPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	collections, _, err := h.documentStore.QueryUserWritableCollections(ctx, user.ID(), port.QueryCollectionsOptions{})
	if err != nil {
		return errors.Wrap(err, "could not query collections")
	}

	vmodel.Collections = make([]component.CollectionOption, len(collections))
	for i, coll := range collections {
		vmodel.Collections[i] = component.CollectionOption{
			ID:    coll.ID(),
			Label: coll.Label(),
		}
	}

	return nil
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

	taskHeaders, err := h.taskRunner.ListTasks(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	// Sort by status priority (failed first, then running, then pending, then succeeded)
	// then by date in reverse chronological order (most recent first)
	slices.SortFunc(taskHeaders, func(a, b port.TaskStateHeader) int {
		priorityA := getStatusPriority(a.Status)
		priorityB := getStatusPriority(b.Status)

		if priorityA != priorityB {
			return cmp.Compare(priorityA, priorityB)
		}

		// Same priority, sort by date descending (most recent first)
		return cmp.Compare(b.ScheduledAt.Unix(), a.ScheduledAt.Unix())
	})

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

// getStatusPriority returns the priority for a task status.
// Lower values = higher priority (should appear first).
// Order: failed (1) -> running (2) -> pending (3) -> succeeded (4)
func getStatusPriority(status port.TaskStatus) int {
	switch status {
	case port.TaskStatusFailed:
		return 1
	case port.TaskStatusRunning:
		return 2
	case port.TaskStatusPending:
		return 3
	case port.TaskStatusSucceeded:
		return 4
	default:
		return 5
	}
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

	taskState, err := h.taskRunner.GetTaskState(ctx, taskID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return common.NewHTTPError(http.StatusNotFound)
		}
		return errors.WithStack(err)
	}

	task, err := h.taskRunner.GetTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return common.NewHTTPError(http.StatusNotFound)
		}
		return errors.WithStack(err)
	}

	vmodel.State = taskState
	vmodel.Task = task
	// A task is cancelable if it's pending or running
	vmodel.Cancelable = taskState.Status == port.TaskStatusPending || taskState.Status == port.TaskStatusRunning

	return nil
}

func (h *Handler) postReindexCollection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get the user from context
	user := httpCtx.User(ctx)
	if user == nil {
		common.HandleError(w, r, errors.New("could not retrieve user from context"))
		return
	}

	// Parse the collection ID from the form
	if err := r.ParseForm(); err != nil {
		common.HandleError(w, r, errors.Wrap(err, "could not parse form"))
		return
	}

	collectionIDStr := r.Form.Get("collection_id")
	if collectionIDStr == "" {
		common.HandleError(w, r, errors.New("collection ID is required"))
		return
	}

	collectionID := model.CollectionID(collectionIDStr)

	// Check if the user has access to the collection
	canWrite, err := h.documentStore.CanWriteCollection(ctx, user.ID(), collectionID)
	if err != nil {
		common.HandleError(w, r, errors.Wrap(err, "could not check collection access"))
		return
	}

	if !canWrite {
		common.HandleError(w, r, errors.New("you don't have permission to reindex this collection"))
		return
	}

	// Schedule the reindex task
	taskID, err := h.documentManager.ReindexCollection(ctx, user, collectionID)
	if err != nil {
		common.HandleError(w, r, errors.Wrap(err, "could not schedule reindex task"))
		return
	}

	// Redirect to the task page
	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/tasks", string(taskID)))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) postCancelTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	taskID := model.TaskID(r.PathValue("id"))
	if taskID == "" {
		common.HandleError(w, r, errors.New("task ID is required"))
		return
	}

	err := h.taskRunner.CancelTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, port.ErrCanceled) {
			// Task is already canceled or cannot be canceled
			common.HandleError(w, r, errors.Wrap(err, "cannot cancel task"))
			return
		}
		common.HandleError(w, r, errors.Wrap(err, "could not cancel task"))
		return
	}

	// Redirect back to the task page
	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/tasks", string(taskID)))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}
