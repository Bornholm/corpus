package model

import (
	"encoding/json"
	"time"

	"github.com/rs/xid"
)

type FilesystemSourceID string

func NewFilesystemSourceID() FilesystemSourceID {
	return FilesystemSourceID(xid.New().String())
}

type FilesystemSourceOptions struct {
	Directory      string `json:"directory"`
	Recursive      bool   `json:"recursive"`
	Filter         string `json:"filter"`
	ETagStrategy   string `json:"etagStrategy"`
	Concurrency    int    `json:"concurrency"`
	DeleteOrphans  bool   `json:"deleteOrphans"`
	SourceTemplate string `json:"sourceTemplate"`
}

func DefaultFilesystemSourceOptions() FilesystemSourceOptions {
	return FilesystemSourceOptions{
		Directory:    ".",
		Recursive:    true,
		ETagStrategy: "modtime",
		Concurrency:  8,
	}
}

type FilesystemSource interface {
	ID() FilesystemSourceID
	Label() string
	BackendType() string
	BackendConfig() json.RawMessage
	CollectionIDs() []CollectionID
	Options() FilesystemSourceOptions
	LastSyncAt() *time.Time
	LastSyncTaskID() *TaskID
	SyncInterval() *time.Duration
}

type BaseFilesystemSource struct {
	id             FilesystemSourceID
	label          string
	backendType    string
	backendConfig  json.RawMessage
	collectionIDs  []CollectionID
	options        FilesystemSourceOptions
	lastSyncAt     *time.Time
	lastSyncTaskID *TaskID
	syncInterval   *time.Duration
}

func (s *BaseFilesystemSource) ID() FilesystemSourceID           { return s.id }
func (s *BaseFilesystemSource) Label() string                    { return s.label }
func (s *BaseFilesystemSource) BackendType() string              { return s.backendType }
func (s *BaseFilesystemSource) BackendConfig() json.RawMessage   { return s.backendConfig }
func (s *BaseFilesystemSource) CollectionIDs() []CollectionID    { return s.collectionIDs }
func (s *BaseFilesystemSource) Options() FilesystemSourceOptions { return s.options }
func (s *BaseFilesystemSource) LastSyncAt() *time.Time           { return s.lastSyncAt }
func (s *BaseFilesystemSource) LastSyncTaskID() *TaskID          { return s.lastSyncTaskID }
func (s *BaseFilesystemSource) SyncInterval() *time.Duration     { return s.syncInterval }

var _ FilesystemSource = &BaseFilesystemSource{}

func NewFilesystemSource(
	id FilesystemSourceID,
	label string,
	backendType string,
	backendConfig json.RawMessage,
	collectionIDs []CollectionID,
	opts FilesystemSourceOptions,
	lastSyncAt *time.Time,
	lastSyncTaskID *TaskID,
	syncInterval *time.Duration,
) *BaseFilesystemSource {
	return &BaseFilesystemSource{
		id:             id,
		label:          label,
		backendType:    backendType,
		backendConfig:  backendConfig,
		collectionIDs:  collectionIDs,
		options:        opts,
		lastSyncAt:     lastSyncAt,
		lastSyncTaskID: lastSyncTaskID,
		syncInterval:   syncInterval,
	}
}
