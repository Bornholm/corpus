package smb

import (
	"encoding/json"
	"fmt"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/hirochachacha/go-smb2"
	"github.com/pkg/errors"
)

func init() {
	backend.RegisterBackendConfig("smb", &SMBConfig{}, FromConfig)
}

// SMBConfig holds the configuration for an SMB/CIFS backend.
// Named SMBConfig to avoid collision with the internal Config type in this package.
type SMBConfig struct {
	Host        string `json:"host"               jsonschema:"required,description=SMB server hostname or IP"`
	Port        int    `json:"port,omitempty"     jsonschema:"default=445,description=SMB server port"`
	Share       string `json:"share"              jsonschema:"required,description=SMB share name"`
	Username    string `json:"username,omitempty" jsonschema:"description=SMB username"`
	Password    string `json:"password,omitempty" jsonschema:"description=SMB password"`
	Domain      string `json:"domain,omitempty"   jsonschema:"default=WORKGROUP,description=NTLM domain"`
	Workstation string `json:"workstation,omitempty" jsonschema:"description=NTLM workstation name"`
	TargetSPN   string `json:"targetSPN,omitempty"   jsonschema:"description=NTLM target SPN"`
	BasePath    string `json:"basePath,omitempty"    jsonschema:"description=Base path within the share"`
}

func FromConfig(configJSON []byte) (filesystem.Backend, error) {
	var cfg SMBConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, errors.Wrap(err, "could not parse smb backend config")
	}
	if cfg.Host == "" {
		return nil, errors.New("smb backend config: host is required")
	}
	if cfg.Share == "" {
		return nil, errors.New("smb backend config: share is required")
	}

	port := cfg.Port
	if port == 0 {
		port = 445
	}

	domain := cfg.Domain
	if domain == "" {
		domain = "WORKGROUP"
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, port)
	initiator := &smb2.NTLMInitiator{
		User:        cfg.Username,
		Password:    cfg.Password,
		Domain:      domain,
		Workstation: cfg.Workstation,
		TargetSPN:   cfg.TargetSPN,
	}

	return New(addr, cfg.BasePath, &Config{
		ShareName: cfg.Share,
		Initiator: initiator,
	}), nil
}
