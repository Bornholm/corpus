package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/pkg/errors"
)

type ListTasksResponse struct {
	Tasks []TaskStateHeader `json:"tasks"`
}

type TaskStateHeader struct {
	ID          port.TaskID     `json:"id"`
	ScheduledAt time.Time       `json:"scheduledAt"`
	Status      port.TaskStatus `json:"status"`
}

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	headers, err := h.documentManager.TaskManager.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "could not list tasks", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	slices.SortFunc(headers, func(h1, h2 port.TaskStateHeader) int {
		return h1.ScheduledAt.Compare(h2.ScheduledAt)
	})

	tasks := slices.Collect(func(yield func(TaskStateHeader) bool) {
		for _, h := range headers {
			if !yield(TaskStateHeader{ID: h.ID, ScheduledAt: h.ScheduledAt, Status: h.Status}) {
				return
			}
		}
	})
	if tasks == nil {
		tasks = make([]TaskStateHeader, 0)
	}

	res := ListTasksResponse{
		Tasks: tasks,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	w.Header().Set("Content-Type", "application/json")

	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slog.Any("error", errors.WithStack(err)))
	}
}

type ShowTaskResponse struct {
	Task *Task `json:"task"`
}

type Task struct {
	ID          port.TaskID     `json:"id"`
	Status      port.TaskStatus `json:"status"`
	Progress    float32         `json:"progress"`
	ScheduledAt time.Time       `json:"scheduledAt"`
	FinishedAt  time.Time       `json:"finishedAt"`
	Error       string          `json:"error,omitempty"`
	Message     string          `json:"message"`
}

func (h *Handler) showTask(w http.ResponseWriter, r *http.Request) {
	taskID := port.TaskID(r.PathValue("taskID"))

	ctx := r.Context()

	taskState, err := h.documentManager.TaskManager.State(ctx, taskID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		slog.ErrorContext(ctx, "could not retrieve task state", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res := ShowTaskResponse{
		Task: &Task{
			ID:          taskID,
			Status:      taskState.Status,
			Progress:    taskState.Progress,
			ScheduledAt: taskState.ScheduledAt,
			FinishedAt:  taskState.FinishedAt,
			Message:     taskState.Message,
		},
	}

	if userFacingErr, ok := taskState.Error.(common.UserFacingError); ok {
		res.Task.Error = userFacingErr.UserMessage()
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	w.Header().Set("Content-Type", "application/json")

	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slog.Any("error", errors.WithStack(err)))
	}
}
