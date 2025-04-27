package smb

import (
	"net/url"
	"strings"

	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/hirochachacha/go-smb2"
	"github.com/pkg/errors"
)

func init() {
	backend.RegisterBackendFactory("smb", FromDSN)
}

func FromDSN(dsn *url.URL) (filesystem.Backend, error) {
	addr := dsn.Host
	basePath := strings.TrimPrefix(dsn.Path, "/")

	config := &Config{}
	configurations := []ConfigureFunc{
		configureShareName,
		configureInitiator,
	}

	for _, configure := range configurations {
		if err := configure(dsn, config); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	backend := New(addr, basePath, config)

	return backend, nil
}

type ConfigureFunc func(dsn *url.URL, conf *Config) error

const paramShare = "share"

func configureShareName(dsn *url.URL, config *Config) error {
	query := dsn.Query()

	if !query.Has(paramShare) {
		return errors.Errorf("url parameter '%s' is required", paramShare)
	}

	shareName := query.Get(paramShare)

	config.ShareName = shareName

	return nil
}

func configureInitiator(dsn *url.URL, config *Config) error {
	initiator := &smb2.NTLMInitiator{
		Domain: "WORKGROUP",
	}

	initiator.User = dsn.User.Username()
	password, exists := dsn.User.Password()
	if exists {
		initiator.Password = password
	}

	query := dsn.Query()

	initiator.Domain = query.Get("domain")
	initiator.Workstation = query.Get("workstation")
	initiator.TargetSPN = query.Get("targetSPN")

	config.Initiator = initiator

	return nil
}
