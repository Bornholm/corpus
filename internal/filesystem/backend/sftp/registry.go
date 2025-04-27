package sftp

import (
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func init() {
	backend.RegisterBackendFactory("sftp", FromDSN)
}

func FromDSN(dsn *url.URL) (filesystem.Backend, error) {
	addr := dsn.Host
	basePath := strings.TrimPrefix(dsn.Path, "/")

	config := &ssh.ClientConfig{
		User: dsn.User.Username(),
	}

	configurations := []ConfigureFunc{
		configureAuthMethods,
		configureTimeout,
		configureHostKeyCallback,
	}

	for _, configure := range configurations {
		if err := configure(dsn, config); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	backend := New(addr, basePath, config)

	return backend, nil
}

type ConfigureFunc func(dsn *url.URL, config *ssh.ClientConfig) error

const (
	paramPrivateKey           = "privateKey"
	paramPrivateKeyPassphrase = "privateKeyPassphrase"
)

func configureAuthMethods(dsn *url.URL, config *ssh.ClientConfig) error {
	authMethods := make([]ssh.AuthMethod, 0)

	query := dsn.Query()

	if query.Has(paramPrivateKey) {
		privateKeyPath := query.Get(paramPrivateKey)
		rawPrivateKey, err := os.ReadFile(privateKeyPath)
		if err != nil {
			return errors.Wrapf(err, "could not read ssh private key '%s'", privateKeyPath)
		}

		passphrase := query.Get(paramPrivateKeyPassphrase)

		var signer ssh.Signer
		if passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(rawPrivateKey, []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(rawPrivateKey)
		}
		if err != nil {
			return errors.Wrapf(err, "could not parse ssh private key '%s'", privateKeyPath)
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if password, exists := dsn.User.Password(); exists {
		authMethods = append(authMethods, ssh.Password(password))
	}

	config.Auth = authMethods

	return nil
}

const paramTimeout = "timeout"

func configureTimeout(dsn *url.URL, config *ssh.ClientConfig) error {
	query := dsn.Query()

	if !query.Has(paramTimeout) {
		return nil
	}

	rawTimeout := query.Get(paramTimeout)
	timeout, err := time.ParseDuration(rawTimeout)
	if err != nil {
		return errors.Wrapf(err, "could not parse query value '%s' as duration", rawTimeout)
	}

	config.Timeout = timeout

	return nil
}

const (
	paramHostKey  = "hostKey"
	ignoreHostKey = "insecure-ignore"
)

func configureHostKeyCallback(dsn *url.URL, config *ssh.ClientConfig) error {
	query := dsn.Query()

	if !query.Has(paramHostKey) {
		return errors.Errorf("url parameter '%s' is required", paramHostKey)
	}

	hostKey := query.Get(paramHostKey)

	if hostKey == ignoreHostKey {
		config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		return nil
	}

	rawPubKey, err := os.ReadFile(hostKey)
	if err != nil {
		return errors.WithStack(err)
	}

	pubKey, err := ssh.ParsePublicKey(rawPubKey)
	if err != nil {
		return errors.WithStack(err)
	}

	config.HostKeyCallback = ssh.FixedHostKey(pubKey)

	return nil
}
