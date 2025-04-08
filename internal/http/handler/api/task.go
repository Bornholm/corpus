package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/pkg/errors"
)

type ListTasksResponse struct {
	Tasks []port.TaskID `json:"tasks"`
}

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tasks, err := h.documentManager.TaskManager.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "could not list tasks", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
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
	Progress    float64         `json:"progress"`
	ScheduledAt time.Time       `json:"scheduledAt"`
	FinishedAt  time.Time       `json:"finishedAt"`
	Error       string          `json:"error,omitempty"`
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
