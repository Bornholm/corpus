package document

import (
	"encoding/json"

	"github.com/bornholm/corpus/pkg/model"
	"github.com/pkg/errors"
)

// stubUser est un utilisateur minimal pour restaurer une tâche depuis la DB.
// Le handler re-récupère l'utilisateur complet via UserStore.GetUserByID.
type stubUser struct {
	id model.UserID
}

func (u *stubUser) ID() model.UserID                     { return u.id }
func (u *stubUser) Email() string                        { return "" }
func (u *stubUser) Subject() string                      { return "" }
func (u *stubUser) Provider() string                     { return "" }
func (u *stubUser) DisplayName() string                  { return "" }
func (u *stubUser) Roles() []string                      { return nil }
func (u *stubUser) Active() bool                         { return true }
func (u *stubUser) Preferences() model.UserPreferences  { return model.NewUserPreferences() }

var _ model.User = &stubUser{}

// RestoreIndexFileTask reconstruit un IndexFileTask depuis les données persistées.
func RestoreIndexFileTask(id model.TaskID, ownerID string, payload []byte) (model.Task, error) {
	t := &IndexFileTask{
		id:    id,
		owner: &stubUser{id: model.UserID(ownerID)},
	}
	if err := json.Unmarshal(payload, t); err != nil {
		return nil, errors.WithStack(err)
	}
	return t, nil
}

// RestoreCleanupTask reconstruit un CleanupTask depuis les données persistées.
func RestoreCleanupTask(id model.TaskID, ownerID string, payload []byte) (model.Task, error) {
	t := &CleanupTask{
		id:    id,
		owner: &stubUser{id: model.UserID(ownerID)},
	}
	if err := json.Unmarshal(payload, t); err != nil {
		return nil, errors.WithStack(err)
	}
	return t, nil
}

// RestoreReindexCollectionTask reconstruit un ReindexCollectionTask depuis les données persistées.
func RestoreReindexCollectionTask(id model.TaskID, ownerID string, payload []byte) (model.Task, error) {
	t := &ReindexCollectionTask{
		id:    id,
		owner: &stubUser{id: model.UserID(ownerID)},
	}
	if err := json.Unmarshal(payload, t); err != nil {
		return nil, errors.WithStack(err)
	}
	return t, nil
}

// RestoreReindexBleveTask reconstruit un ReindexBleveTask depuis les données persistées.
func RestoreReindexBleveTask(id model.TaskID, ownerID string, payload []byte) (model.Task, error) {
	t := &ReindexBleveTask{
		id:    id,
		owner: &stubUser{id: model.UserID(ownerID)},
	}
	if err := json.Unmarshal(payload, t); err != nil {
		return nil, errors.WithStack(err)
	}
	return t, nil
}
