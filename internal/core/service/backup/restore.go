package backup

import (
	"encoding/json"

	"github.com/bornholm/corpus/pkg/model"
	"github.com/pkg/errors"
)

// stubUser est un utilisateur minimal pour restaurer une tâche depuis la DB.
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

// RestoreRestoreBackupTask reconstruit un RestoreBackupTask depuis les données persistées.
func RestoreRestoreBackupTask(id model.TaskID, ownerID string, payload []byte) (model.Task, error) {
	t := &RestoreBackupTask{
		id:    id,
		owner: &stubUser{id: model.UserID(ownerID)},
	}
	if err := json.Unmarshal(payload, t); err != nil {
		return nil, errors.WithStack(err)
	}
	return t, nil
}
