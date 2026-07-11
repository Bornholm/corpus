package api

import (
	"encoding/json"
	"net/http"
	"time"

	fsbackend "github.com/bornholm/corpus/internal/filesystem/backend"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	documentTask "github.com/bornholm/corpus/internal/task/document"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/pkg/errors"

	// Filesystem backends — ensure configs are registered
	_ "github.com/bornholm/corpus/internal/filesystem/backend/ftp"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/git"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/local"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/minio"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/sftp"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/smb"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/webdav"
)

type FilesystemSourceResponse struct {
	ID             string                        `json:"id"`
	Label          string                        `json:"label"`
	BackendType    string                        `json:"backend_type"`
	BackendConfig  json.RawMessage               `json:"backend_config"`
	CollectionIDs  []model.CollectionID          `json:"collection_ids"`
	Options        model.FilesystemSourceOptions `json:"options"`
	LastSyncAt     *time.Time                    `json:"last_sync_at,omitempty"`
	LastSyncTaskID *string                       `json:"last_sync_task_id,omitempty"`
	SyncIntervalMs *int64                        `json:"sync_interval_ms,omitempty"`
}

type ListFilesystemSourcesResponse struct {
	Sources []FilesystemSourceResponse `json:"sources"`
	Total   int64                      `json:"total"`
	Page    int                        `json:"page"`
	Limit   int                        `json:"limit"`
}

type CreateFilesystemSourceRequest struct {
	Label          string                        `json:"label"`
	BackendType    string                        `json:"backend_type"`
	BackendConfig  json.RawMessage               `json:"backend_config"`
	CollectionIDs  []model.CollectionID          `json:"collection_ids"`
	Options        *model.FilesystemSourceOptions `json:"options,omitempty"`
	SyncIntervalMs *int64                        `json:"sync_interval_ms,omitempty"`
}

type UpdateFilesystemSourceRequest struct {
	Label          *string                        `json:"label,omitempty"`
	BackendType    *string                        `json:"backend_type,omitempty"`
	BackendConfig  *json.RawMessage               `json:"backend_config,omitempty"`
	CollectionIDs  []model.CollectionID           `json:"collection_ids,omitempty"`
	Options        *model.FilesystemSourceOptions `json:"options,omitempty"`
	SyncIntervalMs *int64                         `json:"sync_interval_ms,omitempty"`
	ClearInterval  bool                           `json:"clear_interval,omitempty"`
}

func toFilesystemSourceResponse(src model.FilesystemSource) FilesystemSourceResponse {
	resp := FilesystemSourceResponse{
		ID:            string(src.ID()),
		Label:         src.Label(),
		BackendType:   src.BackendType(),
		BackendConfig: src.BackendConfig(),
		CollectionIDs: src.CollectionIDs(),
		Options:       src.Options(),
		LastSyncAt:    src.LastSyncAt(),
	}
	if src.LastSyncTaskID() != nil {
		s := string(*src.LastSyncTaskID())
		resp.LastSyncTaskID = &s
	}
	if src.SyncInterval() != nil {
		ms := src.SyncInterval().Milliseconds()
		resp.SyncIntervalMs = &ms
	}
	return resp
}

func (h *Handler) handleListFilesystemSources(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := getQueryInt(r.URL.Query(), "page", 0)
	limit := getQueryInt(r.URL.Query(), "limit", 20)

	sources, total, err := h.filesystemSourceStore.QueryFilesystemSources(ctx, page, limit)
	if err != nil {
		writeError(w, errors.WithStack(err), http.StatusInternalServerError)
		return
	}

	resp := ListFilesystemSourcesResponse{
		Sources: make([]FilesystemSourceResponse, len(sources)),
		Total:   total,
		Page:    page,
		Limit:   limit,
	}
	for i, src := range sources {
		resp.Sources[i] = toFilesystemSourceResponse(src)
	}

	writeJSON(w, resp)
}

func (h *Handler) handleGetFilesystemBackendSchemas(w http.ResponseWriter, _ *http.Request) {
	schemas, err := fsbackend.AllSchemas()
	if err != nil {
		writeError(w, errors.WithStack(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, schemas)
}

func (h *Handler) handleCreateFilesystemSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateFilesystemSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errors.WithStack(err), http.StatusBadRequest)
		return
	}

	if req.Label == "" {
		writeError(w, errors.New("label is required"), http.StatusBadRequest)
		return
	}
	if req.BackendType == "" {
		writeError(w, errors.New("backend_type is required"), http.StatusBadRequest)
		return
	}
	if len(req.BackendConfig) == 0 {
		writeError(w, errors.New("backend_config is required"), http.StatusBadRequest)
		return
	}

	opts := model.DefaultFilesystemSourceOptions()
	if req.Options != nil {
		opts = *req.Options
	}

	var syncInterval *time.Duration
	if req.SyncIntervalMs != nil {
		d := time.Duration(*req.SyncIntervalMs) * time.Millisecond
		syncInterval = &d
	}

	src, err := h.filesystemSourceStore.CreateFilesystemSource(ctx, req.Label, req.BackendType, req.BackendConfig, req.CollectionIDs, opts, syncInterval)
	if err != nil {
		writeError(w, errors.WithStack(err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, toFilesystemSourceResponse(src))
}

func (h *Handler) handleGetFilesystemSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := model.FilesystemSourceID(r.PathValue("sourceID"))

	src, err := h.filesystemSourceStore.GetFilesystemSourceByID(ctx, id)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			writeError(w, errors.New("source not found"), http.StatusNotFound)
			return
		}
		writeError(w, errors.WithStack(err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, toFilesystemSourceResponse(src))
}

func (h *Handler) handleUpdateFilesystemSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := model.FilesystemSourceID(r.PathValue("sourceID"))

	var req UpdateFilesystemSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errors.WithStack(err), http.StatusBadRequest)
		return
	}

	updates := port.FilesystemSourceUpdates{
		Label:         req.Label,
		BackendType:   req.BackendType,
		BackendConfig: req.BackendConfig,
		CollectionIDs: req.CollectionIDs,
		Options:       req.Options,
	}

	if req.ClearInterval {
		var nilDur *time.Duration
		updates.SyncInterval = &nilDur
	} else if req.SyncIntervalMs != nil {
		d := time.Duration(*req.SyncIntervalMs) * time.Millisecond
		dp := &d
		updates.SyncInterval = &dp
	}

	src, err := h.filesystemSourceStore.UpdateFilesystemSource(ctx, id, updates)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			writeError(w, errors.New("source not found"), http.StatusNotFound)
			return
		}
		writeError(w, errors.WithStack(err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, toFilesystemSourceResponse(src))
}

func (h *Handler) handleDeleteFilesystemSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := model.FilesystemSourceID(r.PathValue("sourceID"))

	if err := h.filesystemSourceStore.DeleteFilesystemSource(ctx, id); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			writeError(w, errors.New("source not found"), http.StatusNotFound)
			return
		}
		writeError(w, errors.WithStack(err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleSyncFilesystemSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := model.FilesystemSourceID(r.PathValue("sourceID"))

	if _, err := h.filesystemSourceStore.GetFilesystemSourceByID(ctx, id); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			writeError(w, errors.New("source not found"), http.StatusNotFound)
			return
		}
		writeError(w, errors.WithStack(err), http.StatusInternalServerError)
		return
	}

	user := httpCtx.User(ctx)

	syncTask := documentTask.NewSyncFilesystemSourceTask(user, id)
	if err := h.taskRunner.ScheduleTask(ctx, syncTask); err != nil {
		writeError(w, errors.WithStack(err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"task_id": string(syncTask.ID())})
}
