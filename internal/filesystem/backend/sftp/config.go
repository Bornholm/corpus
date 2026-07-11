package sftp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func init() {
	backend.RegisterBackendConfig("sftp", &Config{}, FromConfig)
}

// Config holds the configuration for an SFTP backend.
type Config struct {
	Host                 string           `json:"host"                           jsonschema:"required,description=SFTP server hostname or IP"`
	Port                 int              `json:"port,omitempty"                 jsonschema:"default=22,description=SFTP server port"`
	Username             string           `json:"username"                       jsonschema:"required,description=SSH username"`
	Password             string           `json:"password,omitempty"             jsonschema:"description=SSH password (mutually exclusive with privateKey)"`
	PrivateKey           *backend.FileRef `json:"privateKey,omitempty"           jsonschema:"description=SSH private key file"`
	PrivateKeyPassphrase string           `json:"privateKeyPassphrase,omitempty" jsonschema:"description=Passphrase for an encrypted private key"`
	HostKey              *backend.FileRef `json:"hostKey,omitempty"              jsonschema:"description=SSH host public key file for verification"`
	InsecureIgnoreHostKey bool            `json:"insecureIgnoreHostKey,omitempty" jsonschema:"description=Disable host key verification (insecure)"`
	BasePath             string           `json:"basePath,omitempty"             jsonschema:"description=Base path on the remote server"`
	Timeout              string           `json:"timeout,omitempty"              jsonschema:"default=30s,description=Connection timeout (e.g. 30s)"`
}

func FromConfig(configJSON []byte) (filesystem.Backend, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, errors.Wrap(err, "could not parse sftp backend config")
	}
	if cfg.Host == "" {
		return nil, errors.New("sftp backend config: host is required")
	}
	if cfg.Username == "" {
		return nil, errors.New("sftp backend config: username is required")
	}

	port := cfg.Port
	if port == 0 {
		port = 22
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, port)

	sshCfg := &ssh.ClientConfig{
		User: cfg.Username,
	}

	// Auth
	authMethods := make([]ssh.AuthMethod, 0)
	if cfg.PrivateKey != nil {
		rawKey, err := cfg.PrivateKey.Read()
		if err != nil {
			return nil, errors.Wrap(err, "could not read sftp private key")
		}

		var signer ssh.Signer
		if cfg.PrivateKeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(rawKey, []byte(cfg.PrivateKeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(rawKey)
		}
		if err != nil {
			return nil, errors.Wrap(err, "could not parse sftp private key")
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}
	sshCfg.Auth = authMethods

	// Host key
	if cfg.InsecureIgnoreHostKey {
		sshCfg.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else if cfg.HostKey != nil {
		rawPubKey, err := cfg.HostKey.Read()
		if err != nil {
			return nil, errors.Wrap(err, "could not read sftp host key")
		}
		pubKey, err := ssh.ParsePublicKey(rawPubKey)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse sftp host public key")
		}
		sshCfg.HostKeyCallback = ssh.FixedHostKey(pubKey)
	} else {
		return nil, errors.New("sftp backend config: either insecureIgnoreHostKey or hostKey is required")
	}

	// Timeout
	if cfg.Timeout != "" {
		d, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse timeout '%s'", cfg.Timeout)
		}
		sshCfg.Timeout = d
	}

	return New(addr, cfg.BasePath, sshCfg), nil
}
