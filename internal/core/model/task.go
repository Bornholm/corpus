package model

import (
	"github.com/rs/xid"
)

type TaskID string

func NewTaskID() TaskID {
	return TaskID(xid.New().String())
}

type Task interface {
	WithOwner

	ID() TaskID
	Type() TaskType
}

type TaskType string
