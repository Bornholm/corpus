package mcp

import (
	"encoding/gob"
	"net/http"

	"github.com/pkg/errors"
)

func init() {
	gob.Register(&SessionData{})
	gob.Register(sessionKey(""))
}

const sessionName = "corpus-mcp"

type sessionKey string

const (
	sessionKeySession sessionKey = "session"
)

func (h *Handler) getSession(r *http.Request) *SessionData {
	sess, _ := h.sessions.Get(r, sessionName)

	var sessionData *SessionData
	rawSessionData, ok := sess.Values[sessionKeySession]
	if ok {
		sessionData = rawSessionData.(*SessionData)
	}

	if sessionData == nil {
		return &SessionData{
			Collections: make([]string, 0),
		}
	}

	return sessionData
}

func (h *Handler) saveSession(w http.ResponseWriter, r *http.Request, data *SessionData) error {
	sess, _ := h.sessions.Get(r, sessionName)

	sess.Values[sessionKeySession] = &data

	if err := h.sessions.Save(r, w, sess); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

type SessionData struct {
	Collections []string
}
