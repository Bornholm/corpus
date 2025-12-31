package app

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/kirsle/configdir"
	"github.com/pkg/errors"
)

const AppName = "corpus"

type SettingsStore[T any] struct {
	defaults T
	settings *T
	mutex    sync.RWMutex
	dir      string
}

func (s *SettingsStore[T]) Save(settings T) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := s.ensureDir(); err != nil {
		return errors.WithStack(err)
	}

	file, err := os.OpenFile(s.Path()+"-new", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return errors.WithStack(err)
	}

	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("could not close settings file", "error", errors.WithStack(err))
		}

		if err := os.Remove(file.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Error("could not remove temporary settings file", "error", errors.WithStack(err))
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(settings); err != nil {
		return errors.WithStack(err)
	}

	if err := os.Rename(file.Name(), s.Path()); err != nil {
		return errors.Wrap(err, "could not overwrite settings")
	}

	s.settings = &settings

	return nil
}

func (s *SettingsStore[T]) Get(reload bool) (T, error) {
	if !reload {
		s.mutex.RLock()
		if s.settings != nil {
			defer s.mutex.RUnlock()
			return *s.settings, nil
		}
		s.mutex.RUnlock()
	}

	settings, err := s.Reload()
	if err != nil {
		return s.defaults, errors.WithStack(err)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.settings = &settings

	return settings, nil
}

func (s *SettingsStore[T]) Reload() (T, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := s.ensureDir(); err != nil {
		return s.defaults, errors.WithStack(err)
	}

	file, err := os.Open(s.Path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.settings = &s.defaults
			return s.defaults, nil
		}

		return s.defaults, errors.WithStack(err)
	}

	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("could not close settings file", "error", errors.WithStack(err))
		}
	}()

	decoder := json.NewDecoder(file)

	var settings T
	if err := decoder.Decode(&settings); err != nil {
		return s.defaults, errors.WithStack(err)
	}

	s.settings = &settings

	return settings, nil
}

func (s *SettingsStore[T]) ensureDir() error {
	err := configdir.MakePath(s.dir)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (s *SettingsStore[T]) Path() string {
	return filepath.Join(s.dir, "settings.json")
}

func NewStore[T any](defaults T) *SettingsStore[T] {
	return &SettingsStore[T]{
		defaults: defaults,
		dir:      configdir.LocalConfig(AppName),
	}
}
