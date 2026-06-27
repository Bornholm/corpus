package gorm

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bornholm/corpus/pkg/adapter/memory/syncx"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/go-x/slogx"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// TaskRecord est le modèle GORM persistant pour les tâches.
type TaskRecord struct {
	ID          string     `gorm:"primaryKey;autoIncrement:false"`
	Type        string     `gorm:"index"`
	OwnerID     string     `gorm:"index"`
	Payload     []byte
	Status      string     `gorm:"index;default:'pending'"`
	Attempts    int        `gorm:"default:0"`
	ScheduledAt time.Time  `gorm:"index"`
	StartedAt   *time.Time
	FinishedAt  *time.Time
	Progress    float32
	Message     string
	LastError   string
}

// GormTaskRunner est un task runner persistant SQLite/GORM.
type GormTaskRunner struct {
	getDatabase     func(ctx context.Context) (*gorm.DB, error)
	handlers        syncx.Map[model.TaskType, port.TaskHandler]
	factories       syncx.Map[model.TaskType, port.TaskFactory]
	cancelFuncs     syncx.Map[model.TaskID, context.CancelFunc]
	parallelism     int
	cleanupDelay    time.Duration
	cleanupInterval time.Duration
	// notify réveille les workers quand une nouvelle tâche est enqueued.
	notify chan struct{}
}

var _ port.TaskRunner = &GormTaskRunner{}
var _ port.PersistentTaskRunner = &GormTaskRunner{}

func NewGormTaskRunner(db *gorm.DB, parallelism int, cleanupDelay, cleanupInterval time.Duration) *GormTaskRunner {
	return &GormTaskRunner{
		getDatabase:     createGetDatabase(db, &TaskRecord{}),
		parallelism:     parallelism,
		cleanupDelay:    cleanupDelay,
		cleanupInterval: cleanupInterval,
		notify:          make(chan struct{}, 1),
	}
}

// RegisterFactory implements port.PersistentTaskRunner.
func (r *GormTaskRunner) RegisterFactory(taskType model.TaskType, factory port.TaskFactory) {
	r.factories.Store(taskType, factory)
}

// RegisterTask implements port.TaskRunner.
func (r *GormTaskRunner) RegisterTask(taskType model.TaskType, handler port.TaskHandler) {
	r.handlers.Store(taskType, handler)
}

// ScheduleTask implements port.TaskRunner.
func (r *GormTaskRunner) ScheduleTask(ctx context.Context, task model.Task) error {
	payload, err := task.MarshalJSON()
	if err != nil {
		return errors.WithStack(err)
	}

	var ownerID string
	if task.Owner() != nil {
		ownerID = string(task.Owner().ID())
	}

	record := &TaskRecord{
		ID:          string(task.ID()),
		Type:        string(task.Type()),
		OwnerID:     ownerID,
		Payload:     payload,
		Status:      string(port.TaskStatusPending),
		ScheduledAt: time.Now(),
	}

	db, err := r.getDatabase(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := db.Create(record).Error; err != nil {
		return errors.WithStack(err)
	}

	// Notifier les workers sans bloquer.
	select {
	case r.notify <- struct{}{}:
	default:
	}

	return nil
}

// GetTaskState implements port.TaskRunner.
func (r *GormTaskRunner) GetTaskState(ctx context.Context, id model.TaskID) (*port.TaskState, error) {
	record, err := r.loadRecord(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return recordToState(record), nil
}

// GetTask implements port.TaskRunner.
func (r *GormTaskRunner) GetTask(ctx context.Context, id model.TaskID) (model.Task, error) {
	record, err := r.loadRecord(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	task, err := r.reconstructTask(record)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return task, nil
}

// ListTasks implements port.TaskRunner.
func (r *GormTaskRunner) ListTasks(ctx context.Context) ([]port.TaskStateHeader, error) {
	db, err := r.getDatabase(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var records []TaskRecord
	if err := db.Find(&records).Error; err != nil {
		return nil, errors.WithStack(err)
	}

	headers := make([]port.TaskStateHeader, len(records))
	for i, rec := range records {
		headers[i] = recordToState(&rec).TaskStateHeader
	}
	return headers, nil
}

// CancelTask implements port.TaskRunner.
func (r *GormTaskRunner) CancelTask(ctx context.Context, id model.TaskID) error {
	if cancelFn, ok := r.cancelFuncs.Load(id); ok {
		cancelFn()
	}

	db, err := r.getDatabase(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	now := time.Now()
	result := db.Model(&TaskRecord{}).Where("id = ? AND status IN ?", string(id), []string{
		string(port.TaskStatusPending),
		string(port.TaskStatusRunning),
	}).Updates(map[string]any{
		"status":     string(port.TaskStatusFailed),
		"last_error": port.ErrCanceled.Error(),
		"finished_at": now,
	})
	if result.Error != nil {
		return errors.WithStack(result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.WithStack(port.ErrNotFound)
	}
	return nil
}

// Run implements port.TaskRunner.
func (r *GormTaskRunner) Run(ctx context.Context) error {
	db, err := r.getDatabase(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	// Réclamation des tâches orphelines (running au moment d'un crash précédent).
	if err := db.Model(&TaskRecord{}).
		Where("status = ?", string(port.TaskStatusRunning)).
		Updates(map[string]any{
			"status":     string(port.TaskStatusPending),
			"started_at": nil,
		}).Error; err != nil {
		slog.ErrorContext(ctx, "could not reclaim orphaned running tasks", slog.Any("error", errors.WithStack(err)))
	}

	// Workers permanents.
	var wg sync.WaitGroup
	for i := 0; i < r.parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.runWorker(ctx)
		}()
	}

	// Goroutine de nettoyage.
	go r.runCleanup(ctx)

	<-ctx.Done()
	wg.Wait()
	return errors.WithStack(ctx.Err())
}

// runWorker est un worker permanent qui réclame et exécute des tâches.
func (r *GormTaskRunner) runWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.notify:
		case <-time.After(500 * time.Millisecond):
		}

		record, err := r.claimTask(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "error claiming task", slog.Any("error", errors.WithStack(err)))
			continue
		}
		if record == nil {
			continue
		}
		r.executeTask(ctx, record)
	}
}

// claimTask tente de réclamer une tâche pending dans une transaction atomique.
// Retourne nil si aucune tâche n'est disponible.
func (r *GormTaskRunner) claimTask(ctx context.Context) (*TaskRecord, error) {
	var claimed *TaskRecord

	err := r.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
		var record TaskRecord
		if err := db.Where("status = ?", string(port.TaskStatusPending)).
			Order("scheduled_at ASC").
			First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return errors.WithStack(err)
		}

		now := time.Now()
		record.Status = string(port.TaskStatusRunning)
		record.StartedAt = &now
		record.Attempts++

		if err := db.Save(&record).Error; err != nil {
			return errors.WithStack(err)
		}

		claimed = &record
		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)

	return claimed, errors.WithStack(err)
}

// executeTask exécute une tâche réclamée.
func (r *GormTaskRunner) executeTask(ctx context.Context, record *TaskRecord) {
	taskID := model.TaskID(record.ID)
	taskCtx := slogx.WithAttrs(ctx,
		slog.String("taskID", string(taskID)),
		slog.String("taskType", record.Type),
	)

	task, err := r.reconstructTask(record)
	if err != nil {
		slog.ErrorContext(taskCtx, "could not reconstruct task, marking as failed", slog.Any("error", errors.WithStack(err)))
		r.updateRecord(ctx, taskID, func(rec *TaskRecord) {
			rec.Status = string(port.TaskStatusFailed)
			now := time.Now()
			rec.FinishedAt = &now
			rec.LastError = err.Error()
		})
		return
	}

	handler, ok := r.handlers.Load(model.TaskType(record.Type))
	if !ok {
		errMsg := "no handler registered for task type " + record.Type
		slog.ErrorContext(taskCtx, errMsg)
		r.updateRecord(ctx, taskID, func(rec *TaskRecord) {
			rec.Status = string(port.TaskStatusFailed)
			now := time.Now()
			rec.FinishedAt = &now
			rec.LastError = errMsg
		})
		return
	}

	taskCtx, cancelFn := context.WithCancel(taskCtx)
	r.cancelFuncs.Store(taskID, cancelFn)
	defer func() {
		cancelFn()
		r.cancelFuncs.Delete(taskID)
	}()

	events := make(chan port.TaskEvent, 100)
	var eventsWg sync.WaitGroup
	eventsWg.Add(1)
	go func() {
		defer eventsWg.Done()
		for e := range events {
			r.updateRecord(ctx, taskID, func(rec *TaskRecord) {
				if e.Progress != nil {
					rec.Progress = max(min(*e.Progress, 1), 0)
				}
				if e.Message != nil {
					rec.Message = *e.Message
				}
			})
		}
	}()

	handlerErr := handler.Handle(taskCtx, task, events)
	close(events)
	eventsWg.Wait()

	now := time.Now()
	if handlerErr != nil {
		r.updateRecord(ctx, taskID, func(rec *TaskRecord) {
			rec.Status = string(port.TaskStatusFailed)
			rec.FinishedAt = &now
			rec.LastError = handlerErr.Error()
		})
		return
	}

	r.updateRecord(ctx, taskID, func(rec *TaskRecord) {
		rec.Status = string(port.TaskStatusSucceeded)
		rec.FinishedAt = &now
		rec.Progress = 1
	})
}

// runCleanup supprime périodiquement les tâches terminées depuis longtemps.
func (r *GormTaskRunner) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			db, err := r.getDatabase(ctx)
			if err != nil {
				slog.ErrorContext(ctx, "cleanup: could not get db", slog.Any("error", errors.WithStack(err)))
				continue
			}
			cutoff := time.Now().Add(-r.cleanupDelay)
			if err := db.Where("finished_at IS NOT NULL AND finished_at < ?", cutoff).
				Delete(&TaskRecord{}).Error; err != nil {
				slog.ErrorContext(ctx, "cleanup: could not delete old tasks", slog.Any("error", errors.WithStack(err)))
			}
		}
	}
}

// loadRecord charge un TaskRecord depuis la DB.
func (r *GormTaskRunner) loadRecord(ctx context.Context, id model.TaskID) (*TaskRecord, error) {
	db, err := r.getDatabase(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var record TaskRecord
	if err := db.First(&record, "id = ?", string(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.WithStack(port.ErrNotFound)
		}
		return nil, errors.WithStack(err)
	}
	return &record, nil
}

// updateRecord met à jour un TaskRecord en DB.
func (r *GormTaskRunner) updateRecord(ctx context.Context, id model.TaskID, fn func(*TaskRecord)) {
	db, err := r.getDatabase(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "updateRecord: could not get db", slog.Any("error", errors.WithStack(err)))
		return
	}
	var record TaskRecord
	if err := db.First(&record, "id = ?", string(id)).Error; err != nil {
		slog.ErrorContext(ctx, "updateRecord: could not load record", slog.Any("error", errors.WithStack(err)))
		return
	}
	fn(&record)
	if err := db.Save(&record).Error; err != nil {
		slog.ErrorContext(ctx, "updateRecord: could not save record", slog.Any("error", errors.WithStack(err)))
	}
}

// reconstructTask reconstruit un model.Task depuis un TaskRecord.
func (r *GormTaskRunner) reconstructTask(record *TaskRecord) (model.Task, error) {
	factory, ok := r.factories.Load(model.TaskType(record.Type))
	if !ok {
		return nil, errors.Errorf("no factory registered for task type %q", record.Type)
	}
	task, err := factory(model.TaskID(record.ID), record.OwnerID, record.Payload)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return task, nil
}

// withRetry délègue au même pattern que Store pour les erreurs SQLite transitoires.
func (r *GormTaskRunner) withRetry(ctx context.Context, withTx bool, fn func(ctx context.Context, db *gorm.DB) error, codes ...sqlite3.ErrorCode) error {
	s := &Store{getDatabase: r.getDatabase}
	return s.withRetry(ctx, withTx, fn, codes...)
}

func recordToState(rec *TaskRecord) *port.TaskState {
	state := &port.TaskState{
		TaskStateHeader: port.TaskStateHeader{
			ID:          model.TaskID(rec.ID),
			Type:        model.TaskType(rec.Type),
			ScheduledAt: rec.ScheduledAt,
			Status:      port.TaskStatus(rec.Status),
		},
		Progress: rec.Progress,
		Message:  rec.Message,
	}
	if rec.FinishedAt != nil {
		state.FinishedAt = *rec.FinishedAt
	}
	if rec.LastError != "" {
		state.Error = errors.New(rec.LastError)
	}
	return state
}
