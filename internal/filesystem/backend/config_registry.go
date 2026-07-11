package backend

import (
	"encoding/json"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
)

var (
	backendConfigFactories = make(map[string]BackendConfigFactory)
	backendConfigExamples  = make(map[string]any)
)

// BackendConfigFactory creates a Backend from raw JSON config bytes.
type BackendConfigFactory func(configJSON []byte) (filesystem.Backend, error)

// RegisterBackendConfig registers a backend type with its config example (used for schema
// generation) and a factory function that creates a Backend from raw JSON config bytes.
func RegisterBackendConfig(backendType string, example any, factory BackendConfigFactory) {
	backendConfigFactories[backendType] = factory
	backendConfigExamples[backendType] = example
}

// NewFromConfig creates a Backend from a backend type name and raw JSON config bytes.
func NewFromConfig(backendType string, configJSON []byte) (filesystem.Backend, error) {
	factory, exists := backendConfigFactories[backendType]
	if !exists {
		return nil, errors.Errorf("no backend registered for type '%s'", backendType)
	}

	b, err := factory(configJSON)
	if err != nil {
		return nil, errors.Wrapf(err, "could not create backend of type '%s'", backendType)
	}

	return b, nil
}

// AvailableTypes returns the list of registered backend type names.
func AvailableTypes() []string {
	types := make([]string, 0, len(backendConfigFactories))
	for t := range backendConfigFactories {
		types = append(types, t)
	}
	return types
}

// SchemaFor returns the JSON Schema for the given backend type's config struct.
func SchemaFor(backendType string) (json.RawMessage, error) {
	example, exists := backendConfigExamples[backendType]
	if !exists {
		return nil, errors.Errorf("no backend registered for type '%s'", backendType)
	}

	r := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	schema := r.Reflect(example)

	data, err := json.Marshal(schema)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return json.RawMessage(data), nil
}

// AllSchemas returns the JSON Schema for every registered backend type, keyed by type name.
func AllSchemas() (map[string]json.RawMessage, error) {
	schemas := make(map[string]json.RawMessage, len(backendConfigExamples))
	for t := range backendConfigExamples {
		s, err := SchemaFor(t)
		if err != nil {
			return nil, errors.Wrapf(err, "could not generate schema for backend type '%s'", t)
		}
		schemas[t] = s
	}
	return schemas, nil
}
